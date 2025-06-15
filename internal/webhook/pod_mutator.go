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
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devv1alpha1 "github.com/flemzord/mutating-registry-webhook/api/v1alpha1"
)

// PodMutator mutates Pods
type PodMutator struct {
	Client          client.Client
	decoder         admission.Decoder
	rulesCache      *rulesCache
	rulesCacheMutex sync.RWMutex
}

// compiledRule represents a compiled regex rule
type compiledRule struct {
	rule    devv1alpha1.Rule
	regex   *regexp.Regexp
	replace string
}

// rulesCache holds compiled rules
type rulesCache struct {
	rules []compiledRule
}

// Prometheus metrics
var (
	mutationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "registry_rewriter_mutations_total",
		Help: "Total number of mutations performed",
	}, []string{"namespace", "source_registry", "target_registry", "status"})

	mutationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "registry_rewriter_mutation_duration_seconds",
		Help:    "Duration of mutation operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"namespace"})

	rulesCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "registry_rewriter_rules_count",
		Help: "Current number of active rules",
	})

	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "registry_rewriter_cache_hits_total",
		Help: "Total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "registry_rewriter_cache_misses_total",
		Help: "Total number of cache misses",
	})
)

func init() {
	// Register metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(mutationsTotal, mutationDuration, rulesCount, cacheHits, cacheMisses)
}

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.dev.flemzord.fr,admissionReviewVersions=v1;v1beta1,sideEffects=None

// Handle handles Pod admission requests
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	start := time.Now()
	pod := &corev1.Pod{}
	logger := log.FromContext(ctx)

	// Record mutation duration
	defer func() {
		mutationDuration.WithLabelValues(req.Namespace).Observe(time.Since(start).Seconds())
	}()

	err := m.decoder.Decode(req, pod)
	if err != nil {
		logger.Error(err, "Failed to decode pod")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if mutation is disabled via annotation
	if pod.Annotations != nil && pod.Annotations["rewrite-disabled"] == "true" {
		logger.Info("Skipping mutation, rewrite-disabled annotation found", "pod", pod.Name, "namespace", pod.Namespace)
		return admission.Allowed("rewrite disabled")
	}

	// Get current rules
	rules, err := m.getRules(ctx)
	if err != nil {
		logger.Error(err, "Failed to get rules")
		// Don't fail the admission if we can't get rules
		return admission.Allowed("failed to get rules")
	}

	if len(rules) == 0 {
		return admission.Allowed("no rules configured")
	}

	// Apply mutations
	mutated := false

	// Mutate containers
	for i, container := range pod.Spec.Containers {
		newImage := m.mutateImage(ctx, container.Image, rules, pod)
		if newImage != container.Image {
			pod.Spec.Containers[i].Image = newImage
			mutated = true
			logger.Info("Mutated container image", "container", container.Name, "from", container.Image, "to", newImage)
		}
	}

	// Mutate init containers
	for i, container := range pod.Spec.InitContainers {
		newImage := m.mutateImage(ctx, container.Image, rules, pod)
		if newImage != container.Image {
			pod.Spec.InitContainers[i].Image = newImage
			mutated = true
			logger.Info("Mutated init container image", "container", container.Name, "from", container.Image, "to", newImage)
		}
	}

	// Mutate ephemeral containers
	for i, container := range pod.Spec.EphemeralContainers {
		newImage := m.mutateImage(ctx, container.Image, rules, pod)
		if newImage != container.Image {
			pod.Spec.EphemeralContainers[i].Image = newImage
			mutated = true
			logger.Info("Mutated ephemeral container image", "container", container.Name, "from", container.Image, "to", newImage)
		}
	}

	if !mutated {
		return admission.Allowed("no mutations needed")
	}

	// Create the patch
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		logger.Error(err, "Failed to marshal mutated pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// mutateImage applies rules to an image and returns the mutated image
func (m *PodMutator) mutateImage(ctx context.Context, image string, rules []compiledRule, pod *corev1.Pod) string {
	logger := log.FromContext(ctx)

	// Normalize image name (add docker.io prefix if needed)
	normalizedImage := normalizeImage(image)

	for _, rule := range rules {
		// Check conditions
		if !m.checkConditions(rule.rule, pod) {
			continue
		}

		// Apply regex
		if rule.regex.MatchString(normalizedImage) {
			newImage := rule.regex.ReplaceAllString(normalizedImage, rule.replace)
			logger.V(1).Info("Image matched rule", "image", normalizedImage, "match", rule.rule.Match, "newImage", newImage)

			// Extract registries for metrics
			sourceReg := extractRegistry(normalizedImage)
			targetReg := extractRegistry(newImage)
			mutationsTotal.WithLabelValues(pod.Namespace, sourceReg, targetReg, "success").Inc()

			return newImage
		}
	}

	return image
}

// checkConditions checks if a rule's conditions match the pod
func (m *PodMutator) checkConditions(rule devv1alpha1.Rule, pod *corev1.Pod) bool {
	if rule.Conditions == nil {
		return true
	}

	// Check namespace conditions
	if len(rule.Conditions.Namespaces) > 0 {
		found := false
		for _, ns := range rule.Conditions.Namespaces {
			if ns == pod.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check label conditions
	if len(rule.Conditions.Labels) > 0 {
		for k, v := range rule.Conditions.Labels {
			if pod.Labels[k] != v {
				return false
			}
		}
	}

	return true
}

// getRules fetches and compiles all rules
func (m *PodMutator) getRules(ctx context.Context) ([]compiledRule, error) {
	m.rulesCacheMutex.RLock()
	if m.rulesCache != nil && len(m.rulesCache.rules) > 0 {
		rules := m.rulesCache.rules
		m.rulesCacheMutex.RUnlock()
		cacheHits.Inc()
		return rules, nil
	}
	m.rulesCacheMutex.RUnlock()
	cacheMisses.Inc()

	// Fetch all RegistryRewriteRule resources
	ruleList := &devv1alpha1.RegistryRewriteRuleList{}
	if err := m.Client.List(ctx, ruleList); err != nil {
		return nil, fmt.Errorf("failed to list RegistryRewriteRule: %w", err)
	}

	// Compile all rules
	var compiledRules []compiledRule
	for _, rr := range ruleList.Items {
		for _, rule := range rr.Spec.Rules {
			regex, err := regexp.Compile(rule.Match)
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to compile regex", "rule", rr.Name, "match", rule.Match)
				continue
			}
			compiledRules = append(compiledRules, compiledRule{
				rule:    rule,
				regex:   regex,
				replace: rule.Replace,
			})
		}
	}

	// Sort by priority (higher first)
	sort.Slice(compiledRules, func(i, j int) bool {
		return compiledRules[i].rule.Priority > compiledRules[j].rule.Priority
	})

	// Update cache
	m.rulesCacheMutex.Lock()
	m.rulesCache = &rulesCache{rules: compiledRules}
	m.rulesCacheMutex.Unlock()

	// Update metrics
	rulesCount.Set(float64(len(compiledRules)))

	return compiledRules, nil
}

// normalizeImage adds docker.io prefix to images without registry
func normalizeImage(image string) string {
	// If image already has a registry, return as-is
	if regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-_.]*\.[a-zA-Z]{2,}/`).MatchString(image) {
		return image
	}

	// If it's a localhost image, return as-is
	if regexp.MustCompile(`^localhost(:[0-9]+)?/`).MatchString(image) {
		return image
	}

	// Add docker.io prefix
	return "docker.io/" + image
}

// InjectDecoder injects the decoder
func (m *PodMutator) InjectDecoder(d admission.Decoder) error {
	m.decoder = d
	return nil
}

// InvalidateCache invalidates the rules cache
func (m *PodMutator) InvalidateCache() {
	m.rulesCacheMutex.Lock()
	m.rulesCache = nil
	m.rulesCacheMutex.Unlock()
}

// extractRegistry extracts the registry from an image name
func extractRegistry(image string) string {
	// Handle localhost
	if regexp.MustCompile(`^localhost(:[0-9]+)?/`).MatchString(image) {
		return "localhost"
	}

	// Check if image has a registry
	parts := regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9-_.]*\.[a-zA-Z]{2,})/`).FindStringSubmatch(image)
	if len(parts) > 1 {
		return parts[1]
	}

	// No registry means docker.io
	return "docker.io"
}
