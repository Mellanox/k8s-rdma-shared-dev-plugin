include make/license.mk
# Package related
BINARY_NAME=k8s-rdma-shared-dp
PACKAGE=k8s-rdma-shared-dev-plugin
ORG_PATH=github.com/Mellanox
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
BINDIR=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
GOFILES=$(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS=$(or $(PKG),$(shell cd $(CURDIR) && $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

# Version
VERSION?=master
DATE=`date -Iseconds`
COMMIT?=`git rev-parse --verify HEAD`
LDFLAGS="-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Docker
IMAGE_BUILDER?=@docker
IMAGEDIR=$(CURDIR)/images
DOCKERFILE?=$(CURDIR)/Dockerfile
TAG?=mellanox/k8s-rdma-shared-dev-plugin
IMAGE_BUILD_OPTS?=
# Accept proxy settings for docker
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
DOCKERARGS=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif
IMAGE_BUILD_OPTS += $(DOCKERARGS)

# Go tools
GO      = go
GOLANGCI_LINT = $(BINDIR)/golangci-lint
# golangci-lint version should be updated periodically
# we keep it fixed to avoid it from unexpectedly failing on the project
# in case of a version bump
GOLANGCI_LINT_VER = v1.62.2
TIMEOUT = 20
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: lint build test

$(BINDIR):
	@mkdir -p $@

$(BUILDDIR): | $(info Creating build directory...)
	@mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(CURDIR)/cmd/$(BINARY_NAME) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -ldflags $(LDFLAGS) -v

# Tools
$(GOLANGCI_LINT): | $(BINDIR) ; $(info  installing golangci-lint...)
	$Q GOBIN=$(BINDIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VER)

GOVERALLS = $(BINDIR)/goveralls
$(GOVERALLS): | $(BINDIR) ; $(info  installing goveralls...)
	$Q GOBIN=$(BINDIR) go install github.com/mattn/goveralls@v0.0.12

HADOLINT_TOOL = $(BINDIR)/hadolint
$(HADOLINT_TOOL): | $(BINDIR) ; $(info  installing hadolint...)
	$(call wget-install-tool,$(HADOLINT_TOOL),"https://github.com/hadolint/hadolint/releases/download/v2.12.1-beta/hadolint-Linux-x86_64")

MOCKERY = $(BINDIR)/mockery
MOCKERY_VER = v3.2.5
$(MOCKERY): | $(BINDIR) ; $(info  installing mockery...)
	$Q GOBIN=$(BINDIR) go install github.com/vektra/mockery/v3@$(MOCKERY_VER)

# Tests
.PHONY: lint
lint: | $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run --timeout=10m

.PHONY: hadolint
hadolint: $(HADOLINT_TOOL); $(info  running hadolint...) @ ## Run hadolint
	$Q $(HADOLINT_TOOL) Dockerfile

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: | ; $(info  running $(NAME:%=% )tests...) @ ## Run tests
	$Q $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: | $(GO2XUNIT) ; $(info  running $(NAME:%=% )tests...) @ ## Run tests with xUnit output
	$Q 2>&1 $(GO) test -timeout 20s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE = count
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOVERALLS)
test-coverage: COVERAGE_DIR := $(CURDIR)/test
test-coverage: test-coverage-tools | ; $(info  running coverage tests...) @ ## Run coverage tests
		$Q $(GO) test -covermode=$(COVERAGE_MODE) -coverprofile=k8s-rdma-shared-dev-plugin.cover ./...

# Container image
.PHONY: image ubi-image
image: | ; $(info Building Docker image...)  ## Build conatiner image
	$(IMAGE_BUILDER) build --progress=plain -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(IMAGE_BUILD_OPTS)

ubi-image: DOCKERFILE=$(CURDIR)/Dockerfile.ubi
ubi-image: TAG=mellanox/k8s-rdma-shared-dev-plugin-ubi
ubi-image: image       ## Build UBI-based container image

# Misc

.PHONY: generate-mocks
generate-mocks: | $(MOCKERY) ; $(info  generating mocks...) @ ## Generate mocks
	$Q $(MOCKERY)

.PHONY: clean
clean: ; $(info  Cleaning...)	 ## Cleanup everything
	@rm -rf $(BUILDDIR)
	@rm -rf $(BINDIR)
	@rm -rf  test

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

define wget-install-tool
@[ -f $(1) ] || { \
echo "Downloading $(2)" ;\
mkdir -p $(BINDIR);\
wget -O $(1) $(2);\
chmod +x $(1) ;\
}
endef
