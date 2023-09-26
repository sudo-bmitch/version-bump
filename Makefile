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
MARKDOWN_LINT_VER?=v0.10.0
GO_VULNCHECK_VER?=v1.0.1
OSV_SCANNER_VER?=v1.4.0
STATICCHECK_VER?=v0.4.6

.PHONY: all fmt vet test lint lint-go lint-md vendor binaries docker artifacts artifact-pre .FORCE

.FORCE:

all: fmt vet test lint binaries ## Full build of Go binaries (including fmt, vet, test, and lint)

fmt: ## go fmt
	go fmt ./...

vet: ## go vet
	go vet ./...

test: ## go test
	go test -cover ./...

lint: lint-go lint-md ## Run all linting

lint-go: $(GOPATH)/bin/staticcheck .FORCE ## Run linting for Go
	$(GOPATH)/bin/staticcheck -checks all ./...

lint-md: .FORCE ## Run linting for markdown
	docker run --rm -v "$(PWD):/workdir:ro" davidanson/markdownlint-cli2:$(MARKDOWN_LINT_VER) \
	  "**/*.md" "#vendor"

.PHONY: vulnerability-scan
vulnerability-scan: osv-scanner vulncheck-go ## Run all vulnerability scanners

.PHONY: osv-scanner
osv-scanner: $(GOPATH)/bin/osv-scanner .FORCE ## Run OSV Scanner
	$(GOPATH)/bin/osv-scanner -r .

.PHONY: vulncheck-go
vulncheck-go: $(GOPATH)/bin/govulncheck .FORCE ## Run govulncheck
	$(GOPATH)/bin/govulncheck ./...

vendor: ## Vendor Go modules
	go mod vendor

binaries: vendor $(BINARIES) ## Build Go binaries

bin/version-bump: .FORCE
	CGO_ENABLED=0 go build ${GO_BUILD_FLAGS} -o bin/version-bump .

docker: $(IMAGES) ## Build the docker image

docker-version-bump: .FORCE
	docker build -t sudo-bmitch/version-bump -f Dockerfile$(DOCKERFILE_EXT) $(DOCKER_ARGS) .
	docker build -t sudo-bmitch/version-bump:alpine -f Dockerfile$(DOCKERFILE_EXT) --target release-alpine $(DOCKER_ARGS) .

# oci-image: $(addprefix oci-image-,$(COMMANDS)) ## Build reproducible images to an OCI Layout

# oci-image-%: bin/regctl .FORCE
# 	build/oci-image.sh -r scratch -i "$*" -p "$(TEST_PLATFORMS)"
# 	build/oci-image.sh -r alpine  -i "$*" -p "$(TEST_PLATFORMS)" -b "alpine:3"

test-docker: $(addprefix test-docker-,$(COMMANDS)) ## Test the docker multi-platform image builds

test-docker-version-bump:
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit .
	docker buildx build --platform="$(TEST_PLATFORMS)" -f Dockerfile.buildkit --target release-alpine .

artifacts: $(ARTIFACTS) ## Generate artifacts

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

util-version-check: bin/version-bump .FORCE ## Check versions of dependencies in this project
	bin/version-bump check

util-version-update: bin/version-bump .FORCE ## Update versions of dependencies in this project
	bin/version-bump update

util-golang-update: ## Update golang dependencies
	go get -u -t ./...
	go mod tidy
	go mod vendor

$(GOPATH)/bin/staticcheck: .FORCE
	@[ -f $(GOPATH)/bin/staticcheck ] \
	&& [ "$$($(GOPATH)/bin/staticcheck -version | cut -f 3 -d ' ' | tr -d '()')" = "$(STATICCHECK_VER)" ] \
	|| go install "honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VER)"

$(GOPATH)/bin/govulncheck: .FORCE
	@[ $$(go version -m $(GOPATH)/bin/govulncheck | \
		awk -F ' ' '{ if ($$1 == "mod" && $$2 == "golang.org/x/vuln") { printf "%s\n", $$3 } }') = "$(GO_VULNCHECK_VER)" ] \
	|| CGO_ENABLED=0 go install "golang.org/x/vuln/cmd/govulncheck@$(GO_VULNCHECK_VER)"

$(GOPATH)/bin/osv-scanner: .FORCE
	@[ $$(go version -m $(GOPATH)/bin/osv-scanner | \
		awk -F ' ' '{ if ($$1 == "mod" && $$2 == "github.com/google/osv-scanner") { printf "%s\n", $$3 } }') = "$(OSV_SCANNER_VER)" ] \
	|| CGO_ENABLED=0 go install "github.com/google/osv-scanner/cmd/osv-scanner@$(OSV_SCANNER_VER)"

help: # Display help
	@awk -F ':|##' '/^[^\t].+?:.*?##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF }' $(MAKEFILE_LIST)
