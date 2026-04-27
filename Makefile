IMG ?= ghcr.io/ledgermem/ledgermem-k8s-operator:latest

.PHONY: all build test docker-build deploy undeploy manifests fmt vet

all: build

build:
	go build -o bin/manager ./cmd

test:
	go test ./... -race -count=1

fmt:
	gofmt -s -w .

vet:
	go vet ./...

docker-build:
	docker build -t $(IMG) .

manifests:
	@echo "CRDs are committed under config/crd/bases/. Regenerate with controller-gen if installed:"
	@command -v controller-gen >/dev/null && controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases || true

deploy:
	kubectl apply -f config/crd/bases/
	kubectl apply -f config/manager/manager.yaml

undeploy:
	kubectl delete -f config/manager/manager.yaml --ignore-not-found
	kubectl delete -f config/crd/bases/ --ignore-not-found
