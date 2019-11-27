# Package related
BINARY_NAME=k8s-rdma-shared-dp
PACKAGE=k8s-rdma-shared-dev-plugin
ORG_PATH=github.com/Mellanox
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
GOPATH=$(CURDIR)/.gopath
BUILDDIR=$(CURDIR)/binary
BASE=$(GOPATH)/src/$(REPO_PATH)

export GOPATH

# Go tools
GO      = go
GOFMT   = gofmt
V = 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: fmt build

$(BASE): ; $(info  setting GOPATH...)
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(BUILDDIR)
	@cd $(BASE) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v

# Tools

.PHONY: fmt
fmt: ; $(info  running gofmt...) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

# Misc

.PHONY: clean
clean: ; $(info  Cleaning...)	 ## Cleanup everything
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'