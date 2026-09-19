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
	"regexp"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	devv1alpha1 "github.com/flemzord/mutating-registry-webhook/api/v1alpha1"
)

var _ = Describe("PodMutator", func() {
	var (
		mutator *PodMutator
		decoder admission.Decoder
		scheme  *runtime.Scheme
		ctx     context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(devv1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = admission.NewDecoder(scheme)
		mutator = &PodMutator{
			Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		}
		Expect(mutator.InjectDecoder(decoder)).To(Succeed())
	})

	Describe("normalizeImage", func() {
		It("should add docker.io prefix to images without registry", func() {
			Expect(normalizeImage("nginx:latest")).To(Equal("docker.io/library/nginx:latest"))
			Expect(normalizeImage("library/nginx")).To(Equal("docker.io/library/nginx"))
		})

		It("should keep images with registry as-is", func() {
			Expect(normalizeImage("gcr.io/project/image:tag")).To(Equal("gcr.io/project/image:tag"))
			Expect(normalizeImage("quay.io/org/image")).To(Equal("quay.io/org/image"))
		})

		It("should handle localhost images", func() {
			Expect(normalizeImage("localhost/myimage")).To(Equal("localhost/myimage"))
			Expect(normalizeImage("localhost:5000/myimage")).To(Equal("localhost:5000/myimage"))
		})
	})

	Describe("checkConditions", func() {
		var rule devv1alpha1.Rule
		var pod *corev1.Pod

		BeforeEach(func() {
			rule = devv1alpha1.Rule{
				Match:   ".*",
				Replace: "replaced",
			}
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPodName,
					Namespace: testNamespace,
				},
			}
		})

		It("should return true when no conditions", func() {
			Expect(mutator.checkConditions(rule, pod)).To(BeTrue())
		})

		It("should check namespace conditions", func() {
			rule.Conditions = &devv1alpha1.RuleConditions{
				Namespaces: []string{"kube-system", testNamespace},
			}
			Expect(mutator.checkConditions(rule, pod)).To(BeTrue())

			rule.Conditions.Namespaces = []string{"kube-system"}
			Expect(mutator.checkConditions(rule, pod)).To(BeFalse())
		})

		It("should check label conditions", func() {
			pod.Labels = map[string]string{
				testContainerName: testImageName,
				"team":            "platform",
			}

			rule.Conditions = &devv1alpha1.RuleConditions{
				Labels: map[string]string{
					testContainerName: testImageName,
				},
			}
			Expect(mutator.checkConditions(rule, pod)).To(BeTrue())

			rule.Conditions.Labels = map[string]string{
				testContainerName: testImageName,
				"team":            "frontend",
			}
			Expect(mutator.checkConditions(rule, pod)).To(BeFalse())
		})
	})

	Describe("mutateImage", func() {
		It("should apply matching rules", func() {
			rules := []compiledRule{
				{
					rule: devv1alpha1.Rule{
						Match:   dockerHubMatch,
						Replace: ecrReplace,
					},
					regex:   regexp.MustCompile(dockerHubMatch),
					replace: ecrReplace,
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPodName,
					Namespace: testNamespace,
				},
			}

			result := mutator.mutateImage(ctx, "nginx:latest", rules, pod)
			Expect(result).To(Equal("ecr.aws/dockerhub/library/nginx:latest"))
		})

		It("should respect rule priority", func() {
			rules := []compiledRule{
				{
					rule: devv1alpha1.Rule{
						Match:    `^docker\.io/library/nginx.*`,
						Replace:  `special-registry/nginx`,
						Priority: 100,
					},
					regex:   regexp.MustCompile(`^docker\.io/library/nginx.*`),
					replace: `special-registry/nginx`,
				},
				{
					rule: devv1alpha1.Rule{
						Match:    dockerHubMatch,
						Replace:  ecrReplace,
						Priority: 50,
					},
					regex:   regexp.MustCompile(dockerHubMatch),
					replace: ecrReplace,
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPodName,
					Namespace: testNamespace,
				},
			}

			result := mutator.mutateImage(ctx, "nginx:latest", rules, pod)
			Expect(result).To(Equal("special-registry/nginx"))
		})

		It("should add library prefix for docker.io images without namespace", func() {
			rules := []compiledRule{
				{
					rule: devv1alpha1.Rule{
						Match:   dockerHubMatch,
						Replace: `toto.dkr.ecr.eu-west-1.amazonaws.com/dockerhub/$1`,
					},
					regex:   regexp.MustCompile(dockerHubMatch),
					replace: `toto.dkr.ecr.eu-west-1.amazonaws.com/dockerhub/$1`,
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPodName,
					Namespace: testNamespace,
				},
			}

			// Test avec une image docker.io sans namespace
			result := mutator.mutateImage(ctx, "docker.io/caddy:2.7.6-alpine", rules, pod)
			Expect(result).To(Equal("toto.dkr.ecr.eu-west-1.amazonaws.com/dockerhub/library/caddy:2.7.6-alpine"))

			// Test avec une image sans préfixe docker.io
			result2 := mutator.mutateImage(ctx, "caddy:2.7.6-alpine", rules, pod)
			Expect(result2).To(Equal("toto.dkr.ecr.eu-west-1.amazonaws.com/dockerhub/library/caddy:2.7.6-alpine"))
		})
	})
})

func TestNormalizeImage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple image without registry",
			input:    testImageName,
			expected: "docker.io/library/nginx",
		},
		{
			name:     "image with tag without registry",
			input:    "nginx:latest",
			expected: "docker.io/library/nginx:latest",
		},
		{
			name:     "official image without tag",
			input:    "caddy",
			expected: "docker.io/library/caddy",
		},
		{
			name:     "official image with tag",
			input:    "redis:alpine",
			expected: "docker.io/library/redis:alpine",
		},
		{
			name:     "image with namespace without registry",
			input:    "library/nginx",
			expected: "docker.io/library/nginx",
		},
		{
			name:     "image with full registry",
			input:    "gcr.io/project/image:tag",
			expected: "gcr.io/project/image:tag",
		},
		{
			name:     "localhost image",
			input:    "localhost/myimage",
			expected: "localhost/myimage",
		},
		{
			name:     "localhost with port",
			input:    "localhost:5000/myimage",
			expected: "localhost:5000/myimage",
		},
		{
			name:     "hostname registry with port",
			input:    "registry.example.com:5000/team/myimage:latest",
			expected: "registry.example.com:5000/team/myimage:latest",
		},
		{
			name:     "IPv4 registry with port",
			input:    "10.0.0.12:5000/team/myimage:latest",
			expected: "10.0.0.12:5000/team/myimage:latest",
		},
		{
			name:     "IPv6 registry with port",
			input:    "[2001:db8::12]:5000/team/myimage:latest",
			expected: "[2001:db8::12]:5000/team/myimage:latest",
		},
		{
			name:     "public ecr aws image",
			input:    "public.ecr.aws/orga/jeffail/benthos",
			expected: "public.ecr.aws/orga/jeffail/benthos",
		},
		{
			name:     "docker.io image without namespace",
			input:    "docker.io/caddy:2.7.6-alpine",
			expected: "docker.io/library/caddy:2.7.6-alpine",
		},
		{
			name:     "docker.io image with namespace",
			input:    "docker.io/myorg/myimage:latest",
			expected: "docker.io/myorg/myimage:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeImage(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeImage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractRegistry(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		{image: "nginx:latest", expected: "docker.io"},
		{image: "localhost:5000/team/image", expected: "localhost:5000"},
		{image: "registry.example.com:5000/team/image", expected: "registry.example.com:5000"},
		{image: "10.0.0.12:5000/team/image", expected: "10.0.0.12:5000"},
		{image: "[2001:db8::12]:5000/team/image", expected: "[2001:db8::12]:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			if got := extractRegistry(tt.image); got != tt.expected {
				t.Fatalf("extractRegistry(%q) = %q, want %q", tt.image, got, tt.expected)
			}
		})
	}
}
