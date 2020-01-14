# Package related
BINARY_NAME=k8s-rdma-shared-dp
PACKAGE=k8s-rdma-shared-dev-plugin
ORG_PATH=github.com/Mellanox
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
GOPATH=$(CURDIR)/.gopath
GOBIN =$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
BASE=$(GOPATH)/src/$(REPO_PATH)
GOFILES=$(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS=$(or $(PKG),$(shell cd $(BASE) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell env GOPATH=$(GOPATH) $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

export GOPATH
export GOBIN

# Docker
IMAGE_BUILDER?=@docker
IMAGEDIR=$(BASE)/images
DOCKERFILE?=$(CURDIR)/Dockerfile
TAG?=rdma/k8s-rdma-shared-dev-plugin
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
GOFMT   = gofmt
GOLINT = $(GOBIN)/golint
V = 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: fmt lint build

$(BASE): ; $(info  setting GOPATH...)
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

$(GOBIN):
	@mkdir -p $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(BASE)/cmd/$(BINARY_NAME) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v

# Tools

.PHONY: fmt
fmt: ; $(info  running gofmt...) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

$(GOLINT): | $(BASE) ; $(info  building golint...)
	$Q go get -u golang.org/x/lint/golint

# Tests

.PHONY: lint
lint: | $(BASE) $(GOLINT) ; $(info  running golint...) @ ## Run go lint test
	$Q cd $(BASE) && ret=0 && \
		test -z "$$($(GOLINT)  | tee /dev/stderr)" || ret=1 ; \
	 exit $$ret

# Container image
.PHONY: image
image: | $(BASE) ; $(info Building Docker image...)  ## Build conatiner image
	$(IMAGE_BUILDER) build -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(IMAGE_BUILD_OPTS)

# Misc

.PHONY: clean
clean: ; $(info  Cleaning...)	 ## Cleanup everything
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'