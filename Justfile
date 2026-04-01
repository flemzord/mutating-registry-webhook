set dotenv-load
set positional-arguments

default:
  @just --list

pre-commit: tidy lint generate manifests
pc: pre-commit

lint:
  golangci-lint run --fix --timeout 5m

tidy:
  go mod tidy

tests args='':
  KUBEBUILDER_ASSETS=$(setup-envtest use $(go list -m -f "{{"{{ .Version }}"}}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $3}') -p path) ginkgo -p ./...

vet:
  go vet ./...

fmt:
  go fmt ./...

build:
  go build -o bin/manager cmd/main.go

manifests:
  go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0 \
    rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate:
  go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0 \
    object:headerFile="hack/boilerplate.go.txt" paths="./..."

helm-validate args='':
  for dir in $(ls -d helm/*/); do \
    helm lint ./$dir --strict {{args}}; \
    helm template ./$dir {{args}}; \
  done
