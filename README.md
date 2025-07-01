# Mutating Registry Webhook

A Kubernetes mutating admission webhook that automatically rewrites container image references to use pull-through cache registries (like AWS ECR Pull Through Cache).

## Description

This webhook intercepts Pod creation and update requests in your Kubernetes cluster and automatically rewrites container image references based on configurable rules. This is particularly useful when you want to:

- Use AWS ECR Pull Through Cache to reduce external registry dependencies
- Implement a corporate image proxy/cache
- Redirect images from Docker Hub, GCR, Quay.io, or other registries to your internal registry
- Apply different rules based on namespaces or pod labels

## Features

- 🔄 Automatic image URL rewriting based on regex patterns
- 🎯 Namespace and label-based targeting
- ⚡ High-performance with in-memory rule caching
- 🔒 Secure by default with cert-manager integration
- 📊 Prometheus metrics support
- 🎛️ Helm chart for easy deployment
- 🧪 Comprehensive test coverage

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster
- cert-manager v1.0+ installed in your cluster

### Quick Start

1. **Install cert-manager** (if not already installed):
```sh
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

2. **Install the webhook**:
```sh
kubectl apply -f https://github.com/flemzord/mutating-registry-webhook/releases/download/v0.2.0/install.yaml
```

3. **Create a RegistryRewriteRule**:
```yaml
apiVersion: dev.flemzord.fr/v1alpha1
kind: RegistryRewriteRule
metadata:
  name: docker-hub-to-ecr
spec:
  rules:
    # Redirect Docker Hub images to ECR pull-through cache
    - match: '^docker\.io/(.*)'
      replace: '123456789012.dkr.ecr.us-east-1.amazonaws.com/dockerhub/$1'
    
    # Handle images without explicit registry (defaults to docker.io)
    - match: '^([^/]+/[^/]+)$'
      replace: '123456789012.dkr.ecr.us-east-1.amazonaws.com/dockerhub/$1'
```

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/mutating-registry-webhook:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/mutating-registry-webhook:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/mutating-registry-webhook:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/mutating-registry-webhook/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Examples

### Basic Docker Hub to ECR

```yaml
apiVersion: dev.flemzord.fr/v1alpha1
kind: RegistryRewriteRule
metadata:
  name: dockerhub-cache
spec:
  rules:
    - match: '^docker\.io/(.*)'
      replace: '${ECR_REGISTRY}/dockerhub/$1'
    - match: '^([^/]+/[^/]+)$'  # nginx:latest becomes docker.io/nginx:latest
      replace: '${ECR_REGISTRY}/dockerhub/$1'
```

### Multiple Registries

```yaml
apiVersion: dev.flemzord.fr/v1alpha1
kind: RegistryRewriteRule
metadata:
  name: multi-registry-cache
spec:
  rules:
    # Docker Hub
    - match: '^docker\.io/(.*)'
      replace: '${ECR_REGISTRY}/dockerhub/$1'
    
    # Google Container Registry
    - match: '^gcr\.io/([^/]+)/(.+)'
      replace: '${ECR_REGISTRY}/gcr/$1/$2'
    
    # Quay.io
    - match: '^quay\.io/(.*)'
      replace: '${ECR_REGISTRY}/quay/$1'
    
    # GitHub Container Registry
    - match: '^ghcr\.io/(.*)'
      replace: '${ECR_REGISTRY}/ghcr/$1'
```

### Namespace-Specific Rules

```yaml
apiVersion: dev.flemzord.fr/v1alpha1
kind: RegistryRewriteRule
metadata:
  name: production-only
spec:
  rules:
    - match: '^docker\.io/(.*)'
      replace: '${PROD_REGISTRY}/$1'
      conditions:
        namespaces: ["production", "staging"]
```

### Label-Based Rules

```yaml
apiVersion: dev.flemzord.fr/v1alpha1
kind: RegistryRewriteRule
metadata:
  name: team-specific
spec:
  rules:
    - match: '^docker\.io/(.*)'
      replace: '${TEAM_REGISTRY}/$1'
      conditions:
        labels:
          team: "platform"
          cache: "enabled"
```

## Architecture

The webhook consists of:

1. **CRD (RegistryRewriteRule)**: Defines rewrite rules with regex patterns
2. **Mutating Webhook**: Intercepts Pod creation/update and applies rules
3. **Rules Controller**: Watches for rule changes and updates the cache
4. **In-Memory Cache**: Provides O(1) rule lookup performance

## Troubleshooting

### Check if the webhook is running:
```sh
kubectl get pods -n mutating-registry-webhook-system
```

### View webhook logs:
```sh
kubectl logs -n mutating-registry-webhook-system deployment/mutating-registry-webhook-controller-manager
```

### Test with a sample pod:
```sh
kubectl run test --image=nginx:latest --dry-run=server -o yaml
```

### Disable mutation for specific pods:
Add the annotation `rewrite-disabled: "true"` to your pod:
```yaml
metadata:
  annotations:
    rewrite-disabled: "true"
```

## Performance

- Rule compilation: O(n) on startup/rule change
- Image mutation: O(1) with cached rules
- Benchmarks: ~0.7μs per image mutation
- Memory usage: ~50MB base + rules

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

