COMMANDS?=version-bump
BINARIES?=$(addprefix bin/,$(COMMANDS))
IMAGES?=$(addprefix docker-,$(COMMANDS))
ARTIFACT_PLATFORMS?=linux-amd64 linux-arm64 linux-ppc64le linux-s390x darwin-amd64 darwin-arm64 windows-amd64.exe
ARTIFACTS?=$(foreach cmd,$(addprefix artifacts/,$(COMMANDS)),$(addprefix $(cmd)-,$(ARTIFACT_PLATFORMS)))
TEST_PLATFORMS?=linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le,linux/s390x
VCS_REPO?="https://github.com/sudo-bmitch/version-bump.git"
VCS_REF?=$(shell git rev-list -1 HEAD)
ifneq ($(shell git status --porcelain 2>/dev/null),)
  VCS_REF := $(VCS_REF)-dirty
endif
VCS_DATE?=$(shell date -d "@$(shell git log -1 --format=%at)" +%Y-%m-%dT%H:%M:%SZ --utc)
VCS_TAG?=$(shell git describe --tags --abbrev=0 2>/dev/null || true)
LD_FLAGS?=-s -w -extldflags -static -buildid=
GO_BUILD_FLAGS?=-trimpath -ldflags "$(LD_FLAGS)" -tags nolegacy
DOCKERFILE_EXT?=$(shell if docker build --help 2>/dev/null | grep -q -- '--progress'; then echo ".buildkit"; fi)
DOCKER_ARGS?=--build-arg "VCS_REF=$(VCS_REF)"
GOPATH?=$(shell go env GOPATH)
PWD:=$(shell pwd)
MARKDOWN_LINT_VER?=v0.20.0
GOMAJOR_VER?=v0.15.0
GOSEC_VER?=v2.22.11
GO_VULNCHECK_VER?=v1.1.4
OSV_SCANNER_VER?=v2.3.2
STATICCHECK_VER?=v0.6.1

.PHONY: .FORCE

.FORCE:

.PHONY: all
all: fmt goimports vet test lint binaries ## Full build of Go binaries (including fmt, vet, test, and lint)

.PHONY: fmt
fmt: ## go fmt
	go fmt ./...

.PHONY: goimports
goimports: $(GOPATH)/bin/goimports
	$(GOPATH)/bin/goimports -w -format-only -local github.com/sudo-bmitch/version-bump .

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: test
test: ## go test
	go test -cover -race ./...

.PHONY: lint
lint: lint-go lint-goimports lint-md lint-gosec ## Run all linting

.PHONY: lint-go
lint-go: $(GOPATH)/bin/staticcheck .FORCE ## Run linting for Go
	$(GOPATH)/bin/staticcheck -checks all ./...

.PHONY: lint-goimports
lint-goimports: $(GOPATH)/bin/goimports
	@if [ -n "$$($(GOPATH)/bin/goimports -l -format-only -local github.com/sudo-bmitch/version-bump .)" ]; then \
		echo $(GOPATH)/bin/goimports -d -format-only -local github.com/sudo-bmitch/version-bump .; \
		$(GOPATH)/bin/goimports -d -format-only -local github.com/sudo-bmitch/version-bump .; \
		exit 1; \
	fi

.PHONY: lint-gosec
lint-gosec: $(GOPATH)/bin/gosec .FORCE ## Run gosec
	$(GOPATH)/bin/gosec -terse ./...

.PHONY: lint-md
lint-md: .FORCE ## Run linting for markdown
	docker run --rm -v "$(PWD):/workdir:ro" davidanson/markdownlint-cli2:$(MARKDOWN_LINT_VER) \
	  "**/*.md" "#vendor"

.PHONY: vulnerability-scan
vulnerability-scan: osv-scanner vulncheck-go ## Run all vulnerability scanners

.PHONY: osv-scanner
osv-scanner: $(GOPATH)/bin/osv-scanner .FORCE ## Run OSV Scanner
	$(GOPATH)/bin/osv-scanner scan --config .osv-scanner.toml -r --licenses="Apache-2.0,BSD-2-Clause,BSD-3-Clause,CC-BY-SA-4.0,ISC,MIT,MPL-2.0,UNKNOWN" .

.PHONY: vulncheck-go
vulncheck-go: $(GOPATH)/bin/govulncheck .FORCE ## Run govulncheck
	$(GOPATH)/bin/govulncheck ./...

.PHONY: vendor
vendor: ## Vendor Go modules
	go mod vendor

.PHONY: binaries
binaries: $(BINARIES) ## Build Go binaries

bin/version-bump: .FORCE
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o bin/version-bump .

.PHONY: docker
docker: $(IMAGES) ## Build the docker image

.PHONY: docker-version-bump
docker-version-bump: .FORCE
	docker build -t sudo-bmitch/version-bump -f Dockerfile$(DOCKERFILE_EXT) $(DOCKER_ARGS) .
	docker build -t sudo-bmitch/version-bump:alpine -f Dockerfile$(DOCKERFILE_EXT) --target release-alpine $(DOCKER_ARGS) .

# oci-image: $(addprefix oci-image-,$(COMMANDS)) ## Build reproducible images to an OCI Layout

# oci-image-%: bin/regctl .FORCE
# 	build/oci-image.sh -r scratch -i "$*" -p "$(TEST_PLATFORMS)"
# 	build/oci-image.sh -r alpine  -i "$*" -p "$(TEST_PLATFORMS)" -b "alpine:3"

.PHONY: test-docker
test-docker: $(addprefix test-docker-,$(COMMANDS)) ## Test the docker multi-platform image builds

.PHONY: test-docker-version-bump
test-docker-version-bump:
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit .
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit --target release-alpine .

.PHONY: artifacts
artifacts: $(ARTIFACTS) ## Generate artifacts

.PHONY: artifact-pre
artifact-pre:
	mkdir -p artifacts

artifacts/version-bump-%: artifact-pre .FORCE
	platform_ext="$*"; \
	platform="$${platform_ext%.*}"; \
	export GOOS="$${platform%%-*}"; \
	export GOARCH="$${platform#*-}"; \
	echo export GOOS=$${GOOS}; \
	echo export GOARCH=$${GOARCH}; \
	echo go build ${GO_BUILD_FLAGS} -o "$@" .; \
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o "$@" .

.PHONY: util-version-check
util-version-check: bin/version-bump .FORCE ## Check versions of dependencies in this project
	bin/version-bump check

.PHONY: util-version-update
util-version-update: bin/version-bump .FORCE ## Update versions of dependencies in this project
	bin/version-bump update

.PHONY: util-golang-major
util-golang-major: $(GOPATH)/bin/gomajor ## check for major dependency updates
	$(GOPATH)/bin/gomajor list

.PHONY: util-golang-update
util-golang-update: ## Update golang dependencies
	go get -u -t ./...
	go mod tidy
	[ ! -d vendor ] || go mod vendor

.PHONY: util-golang-update-direct
util-golang-update-direct: ## Update direct go dependencies
	go get $$(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -m all)
	go mod tidy
	[ ! -d vendor ] || go mod vendor

$(GOPATH)/bin/goimports: .FORCE
	@[ -f "$(GOPATH)/bin/goimports" ] \
	||	go install golang.org/x/tools/cmd/goimports@latest

$(GOPATH)/bin/gomajor: .FORCE
	@[ -f "$(GOPATH)/bin/gomajor" ] \
	&& [ "$$($(GOPATH)/bin/gomajor version | grep '^version' | cut -f 2 -d ' ')" = "$(GOMAJOR_VER)" ] \
	|| go install github.com/icholy/gomajor@$(GOMAJOR_VER)

$(GOPATH)/bin/gosec: .FORCE
	@[ -f $(GOPATH)/bin/gosec ] \
	&& [ "$$($(GOPATH)/bin/gosec -version | grep '^Version' | cut -f 2 -d ' ')" = "$(GOSEC_VER)" ] \
	|| go install -ldflags '-X main.Version=$(GOSEC_VER) -X main.GitTag=$(GOSEC_VER)' \
	    github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VER)

$(GOPATH)/bin/govulncheck: .FORCE
	@[ -f $(GOPATH)/bin/govulncheck ] \
	&& [ "$$(go version -m $(GOPATH)/bin/govulncheck | \
		awk -F ' ' '{ if ($$1 == "mod" && $$2 == "golang.org/x/vuln") { printf "%s\n", $$3 } }')" = "$(GO_VULNCHECK_VER)" ] \
	|| CGO_ENABLED=0 go install "golang.org/x/vuln/cmd/govulncheck@$(GO_VULNCHECK_VER)"

$(GOPATH)/bin/osv-scanner: .FORCE
	@[ -f $(GOPATH)/bin/osv-scanner ] \
	&& [ "$$(osv-scanner --version | awk -F ': ' '{ if ($$1 == "osv-scanner version") { printf "%s\n", $$2 } }')" = "$(OSV_SCANNER_VER)" ] \
	|| CGO_ENABLED=0 go install "github.com/google/osv-scanner/v2/cmd/osv-scanner@$(OSV_SCANNER_VER)"

$(GOPATH)/bin/staticcheck: .FORCE
	@[ -f $(GOPATH)/bin/staticcheck ] \
	&& [ "$$($(GOPATH)/bin/staticcheck -version | cut -f 3 -d ' ' | tr -d '()')" = "$(STATICCHECK_VER)" ] \
	|| go install "honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VER)"

help: # Display help
	@awk -F ':|##' '/^[^\t].+?:.*?##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF }' $(MAKEFILE_LIST)
