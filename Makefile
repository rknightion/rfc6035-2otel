GO ?= go
GOFLAGS ?= -mod=readonly
export GOFLAGS

BINARY := rfc6035-2otel
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
TOOLS_DIR := $(CURDIR)/.tools
HELM_DOCS_VERSION ?= v1.14.2

.PHONY: build test vet fmt fmt-check tidy tidy-check dashboard rules grafana-check helm-docs check docker

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
		{ echo "Go files require formatting"; gofmt -l $$(find . -name '*.go' -not -path './vendor/*'); exit 1; }

tidy:
	$(GO) mod tidy

tidy-check:
	$(GO) mod tidy -diff

dashboard:
	cd grafana && python3 build_dashboard.py

rules:
	cd grafana && python3 build_rules.py

grafana-check:
	cd grafana && python3 build_dashboard.py --check
	cd grafana && python3 build_rules.py --check
	cd grafana && python3 -m unittest discover -s tests -t . -q

helm-docs:
	@mkdir -p $(TOOLS_DIR)
	@test -x $(TOOLS_DIR)/helm-docs || GOBIN=$(TOOLS_DIR) $(GO) install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)
	$(TOOLS_DIR)/helm-docs --chart-search-root charts

docker:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -t $(BINARY):dev .

check: fmt-check vet test tidy-check grafana-check build
