set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
commit := `git rev-parse HEAD 2>/dev/null || echo unknown`
build_date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.buildDate=" + build_date
tools_dir := justfile_directory() + "/.tools"
chart_dir := "charts/rfc6035-2otel"

# Keep this aligned with the version in .github/workflows/ci.yml's lint job.
# renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
golangci_lint_version := "v2.13.2"
# renovate: datasource=go depName=github.com/norwoodj/helm-docs
helm_docs_version := "v1.14.2"
# Keep this aligned with the vulnerability job in .github/workflows/ci.yml.
# renovate: datasource=go depName=golang.org/x/vuln
govulncheck_version := "v1.3.0"
# Keep this aligned with the snapshot action in .github/workflows/ci.yml.
# renovate: datasource=go depName=github.com/goreleaser/goreleaser/v2
goreleaser_version := "v2.18.0"
golangci_lint_dir := tools_dir + "/golangci-lint-" + golangci_lint_version
helm_docs_dir := tools_dir + "/helm-docs-" + helm_docs_version

# Show the task surface.
default:
    @just --list </dev/null

# Install repo-local tooling (idempotent; network access required).
setup: _install-golangci-lint _install-helm-docs
    go mod download

# Install the pinned linter used by just lint.
[private]
_install-golangci-lint:
    mkdir -p {{ golangci_lint_dir }}
    test -x {{ golangci_lint_dir }}/golangci-lint || GOBIN={{ golangci_lint_dir }} go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{ golangci_lint_version }}

# Install the pinned Helm documentation generator.
[private]
_install-helm-docs:
    mkdir -p {{ helm_docs_dir }}
    test -x {{ helm_docs_dir }}/helm-docs || GOBIN={{ helm_docs_dir }} go install github.com/norwoodj/helm-docs/cmd/helm-docs@{{ helm_docs_version }}

# Format Go sources in place.
[group('dev')]
fmt:
    gofmt -w $(find . -name '*.go' -not -path './vendor/*')

# Verify Go and justfile formatting without mutating files.
[group('check')]
[no-exit-message]
fmt-check:
    files="$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))"; test -z "$files" || { echo "Go files require formatting:"; echo "$files"; exit 1; }
    just --fmt --check </dev/null

# Run static analysis with the same pinned golangci-lint release as CI.
[group('check')]
[no-exit-message]
lint: _install-golangci-lint
    {{ golangci_lint_dir }}/golangci-lint run ./...

# Run go vet.
[group('check')]
[no-exit-message]
vet:
    go vet ./...

# Run the Go test suite with the race detector; filter="Name" narrows via -run.
[group('check')]
[no-exit-message]
test filter="":
    if [ -n "{{ filter }}" ]; then go test -race -run '{{ filter }}' ./...; else go test -race ./...; fi

# Verify go.mod and go.sum need no changes.
[group('check')]
[no-exit-message]
tidy-check:
    go mod tidy -diff

# Apply go mod tidy.
[group('dev')]
tidy:
    go mod tidy

# Regenerate Grafana dashboard and alert-rule resources from the signal catalog.
[group('gen')]
gen:
    cd grafana && python3 build_dashboard.py
    cd grafana && python3 build_rules.py

# Verify generated Grafana resources and run the generator and deploy-script unit tests.
[group('check')]
[no-exit-message]
gen-check:
    cd grafana && python3 build_dashboard.py --check
    cd grafana && python3 build_rules.py --check
    cd grafana && python3 -m unittest discover -s tests -t . -q
    python3 -m unittest discover -s scripts/tests -v

# Compile the binary into bin/.
[group('build')]
build:
    go build -trimpath -ldflags "{{ ldflags }}" -o bin/rfc6035-2otel ./cmd/rfc6035-2otel

# Scan dependencies for known vulnerabilities.
[group('check')]
[no-exit-message]
vuln:
    go run golang.org/x/vuln/cmd/govulncheck@{{ govulncheck_version }} ./...

# Run the RFC 6035 report parser fuzz smoke test for ten seconds.
[group('check')]
[no-exit-message]
fuzz:
    go test ./internal/vqreport -run='^$' -fuzz='^FuzzParse$' -fuzztime=10s

# Run the complete local gate.
[group('check')]
check: fmt-check lint vet test tidy-check gen-check build vuln fuzz

# Build a snapshot release; requires cross-compilation.
[group('build')]
[no-exit-message]
snapshot:
    go run github.com/goreleaser/goreleaser/v2@{{ goreleaser_version }} release --snapshot --clean --skip=publish,sign,sbom,docker

# Build a local image for smoke testing; requires a Docker daemon.
[group('build')]
image tag="rfc6035-2otel:dev":
    docker build --build-arg VERSION={{ version }} --build-arg COMMIT={{ commit }} --build-arg BUILD_DATE={{ build_date }} -t {{ tag }} .

# Run the CI-only superset of the local gate.
[group('check')]
ci: check snapshot image

# Lint and render the Helm chart.
[group('check')]
[no-exit-message]
helm-lint:
    helm lint {{ chart_dir }}
    helm template rfc6035-2otel {{ chart_dir }} > /dev/null

# Regenerate the Helm chart README from chart metadata.
[group('gen')]
helm-docs: _install-helm-docs
    {{ helm_docs_dir }}/helm-docs --chart-search-root charts

# Verify the Helm chart README has no generated-content drift.
[group('check')]
[no-exit-message]
helm-docs-check: helm-docs
    git diff --exit-code {{ chart_dir }}/README.md

# Remove reproducible build output and downloaded tools.
[group('build')]
clean:
    rm -rf bin {{ tools_dir }}
