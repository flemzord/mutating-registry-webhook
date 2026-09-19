/*
Copyright 2025 flemzord.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devv1alpha1 "github.com/flemzord/mutating-registry-webhook/api/v1alpha1"
)

const (
	testPodName       = "test-pod"
	testNamespace     = "default"
	testContainerName = "app"
	testImage         = "nginx:latest"
	testImageName     = "nginx"
	dockerHubMatch    = `^docker\.io/(.*)`
	ecrReplace        = `ecr.aws/dockerhub/$1`
)

func TestHandleCachesAnEmptyRuleSet(t *testing.T) {
	scheme := newTestScheme(t)
	counting := &countingClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	mutator := newTestMutator(t, scheme, counting)
	req, raw := podRequest(t)

	for range 2 {
		response := mutator.Handle(context.Background(), req)
		if !response.Allowed {
			t.Fatalf("admission was rejected: %#v", response.Result)
		}
		assertPatchedImage(t, response, raw, "nginx:latest")
	}

	if counting.listCalls != 1 {
		t.Fatalf("empty rules were listed %d times, want once", counting.listCalls)
	}
}

func TestHandleUsesDeterministicOrderForEqualPriorityRules(t *testing.T) {
	scheme := newTestScheme(t)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		rewriteRule("z-last", dockerHubMatch, `z.example/$1`),
		rewriteRule("a-first", dockerHubMatch, `a.example/$1`),
	).Build()
	mutator := newTestMutator(t, scheme, &reverseRuleListClient{Client: baseClient})
	req, raw := podRequest(t)

	response := mutator.Handle(context.Background(), req)
	if !response.Allowed {
		t.Fatalf("admission was rejected: %#v", response.Result)
	}
	assertPatchedImage(t, response, raw, "a.example/library/nginx:latest")
}

func TestHandleDoesNotRestoreRulesInvalidatedDuringReload(t *testing.T) {
	scheme := newTestScheme(t)
	oldRule := rewriteRule("rules", dockerHubMatch, `old.example/$1`)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(oldRule).Build()
	blocking := &blockingListClient{
		Client:  baseClient,
		listed:  make(chan struct{}),
		release: make(chan struct{}),
	}
	mutator := newTestMutator(t, scheme, blocking)
	req, raw := podRequest(t)
	firstDone := make(chan admission.Response, 1)

	go func() {
		firstDone <- mutator.Handle(context.Background(), req)
	}()
	<-blocking.listed

	current := &devv1alpha1.RegistryRewriteRule{}
	if err := baseClient.Get(context.Background(), client.ObjectKey{Name: "rules"}, current); err != nil {
		t.Fatal(err)
	}
	current.Spec.Rules[0].Replace = `new.example/$1`
	if err := baseClient.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	mutator.InvalidateCache()
	close(blocking.release)

	firstResponse := <-firstDone
	if !firstResponse.Allowed {
		t.Fatalf("first admission was rejected: %#v", firstResponse.Result)
	}
	response := mutator.Handle(context.Background(), req)
	if !response.Allowed {
		t.Fatalf("second admission was rejected: %#v", response.Result)
	}
	assertPatchedImage(t, response, raw, "new.example/library/nginx:latest")
}

func TestRulesWatcherReportsInvalidRegexAsNotReady(t *testing.T) {
	scheme := newTestScheme(t)
	rule := rewriteRule("invalid", "[", "replacement")
	rule.Generation = 3
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&devv1alpha1.RegistryRewriteRule{}).
		WithObjects(rule).
		Build()
	mutator := newTestMutator(t, scheme, k8sClient)
	watcher := &RulesWatcher{Client: k8sClient, Mutator: mutator}

	if _, err := watcher.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: client.ObjectKey{Name: rule.Name},
	}); err != nil {
		t.Fatalf("reconcile invalid user rule: %v", err)
	}

	updated := &devv1alpha1.RegistryRewriteRule{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: rule.Name}, updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Ready {
		t.Fatal("invalid regular expression was reported Ready")
	}
	if updated.Status.ObservedGeneration != rule.Generation {
		t.Fatalf("observed generation = %d, want %d", updated.Status.ObservedGeneration, rule.Generation)
	}
}

func TestRulesWatcherInvalidatesCacheAfterRuleDeletion(t *testing.T) {
	scheme := newTestScheme(t)
	rule := rewriteRule("temporary", dockerHubMatch, `mirror.example/$1`)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rule).Build()
	mutator := newTestMutator(t, scheme, k8sClient)
	req, raw := podRequest(t)

	before := mutator.Handle(context.Background(), req)
	assertPatchedImage(t, before, raw, "mirror.example/library/nginx:latest")
	if err := k8sClient.Delete(context.Background(), rule); err != nil {
		t.Fatal(err)
	}
	watcher := &RulesWatcher{Client: k8sClient, Mutator: mutator}
	if _, err := watcher.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: client.ObjectKey{Name: rule.Name},
	}); err != nil {
		t.Fatalf("reconcile deleted rule: %v", err)
	}

	after := mutator.Handle(context.Background(), req)
	assertPatchedImage(t, after, raw, "nginx:latest")
}

type countingClient struct {
	client.Client
	listCalls int
}

type reverseRuleListClient struct {
	client.Client
}

func (c *reverseRuleListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, list, opts...); err != nil {
		return err
	}
	rules, ok := list.(*devv1alpha1.RegistryRewriteRuleList)
	if !ok {
		return nil
	}
	for left, right := 0, len(rules.Items)-1; left < right; left, right = left+1, right-1 {
		rules.Items[left], rules.Items[right] = rules.Items[right], rules.Items[left]
	}
	return nil
}

func (c *countingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.listCalls++
	return c.Client.List(ctx, list, opts...)
}

type blockingListClient struct {
	client.Client
	mu      sync.Mutex
	blocked bool
	listed  chan struct{}
	release chan struct{}
}

func (c *blockingListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := c.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}
	c.mu.Lock()
	shouldBlock := !c.blocked
	if shouldBlock {
		c.blocked = true
	}
	c.mu.Unlock()
	if shouldBlock {
		close(c.listed)
		select {
		case <-c.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := devv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func newTestMutator(t *testing.T, scheme *runtime.Scheme, k8sClient client.Client) *PodMutator {
	t.Helper()
	mutator := &PodMutator{Client: k8sClient}
	if err := mutator.InjectDecoder(admission.NewDecoder(scheme)); err != nil {
		t.Fatal(err)
	}
	return mutator
}

func rewriteRule(name, match, replace string) *devv1alpha1.RegistryRewriteRule {
	return &devv1alpha1.RegistryRewriteRule{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: devv1alpha1.RegistryRewriteRuleSpec{Rules: []devv1alpha1.Rule{{
			Match: match, Replace: replace, Priority: 10,
		}}},
	}
}

func podRequest(t *testing.T) (admission.Request, []byte) {
	t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName, Namespace: testNamespace},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: testContainerName, Image: testImage}}},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Namespace: testNamespace,
		Object:    runtime.RawExtension{Raw: raw},
	}}, raw
}

func assertPatchedImage(t *testing.T, response admission.Response, raw []byte, expected string) {
	t.Helper()
	patchBytes, err := json.Marshal(response.Patches)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		t.Fatalf("decode admission patch: %v", err)
	}
	mutatedRaw, err := patch.Apply(raw)
	if err != nil {
		t.Fatalf("apply admission patch: %v", err)
	}
	mutated := &corev1.Pod{}
	if err := json.Unmarshal(mutatedRaw, mutated); err != nil {
		t.Fatal(err)
	}
	if got := mutated.Spec.Containers[0].Image; got != expected {
		t.Fatalf("mutated image = %q, want %q", got, expected)
	}
}
