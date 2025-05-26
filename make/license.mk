# make/license.mk

# --- Configurable license header settings ---
COPYRIGHT_YEAR          ?= $(shell date +%Y)
COPYRIGHT_OWNER         ?= NVIDIA CORPORATION & AFFILIATES
COPYRIGHT_STYLE         ?= apache
COPYRIGHT_FLAGS         ?= -s
COPYRIGHT_EXCLUDE       ?= vendor deployment .*
GIT_LS_FILES_EXCLUDES := $(foreach d,$(COPYRIGHT_EXCLUDE),:^"$(d)")

# --- Tool paths ---
BIN_DIR            ?= ./bin
ADDLICENSE         ?= $(BIN_DIR)/addlicense
ADDLICENSE_VERSION ?= latest
GO_LICENSES        ?= $(BIN_DIR)/go-licenses
GO_LICENSES_VERSION ?= latest

# Ensure bin dir exists
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Install addlicense locally
.PHONY: addlicense
addlicense: $(BIN_DIR)
	@if [ ! -f "$(ADDLICENSE)" ]; then \
		echo "Installing addlicense to $(ADDLICENSE)..."; \
		GOBIN=$(abspath $(BIN_DIR)) go install github.com/google/addlicense@$(ADDLICENSE_VERSION); \
	else \
		echo "addlicense already installed at $(ADDLICENSE)"; \
	fi

# Check headers
.PHONY: copyright-check
copyright-check: addlicense
	@echo "Checking copyright headers..."
	@git ls-files '*' $(GIT_LS_FILES_EXCLUDES) | xargs -r $(ADDLICENSE) -check -c "$(COPYRIGHT_OWNER)" -l $(COPYRIGHT_STYLE) $(COPYRIGHT_FLAGS) -y $(COPYRIGHT_YEAR)

# Fix headers
.PHONY: copyright
copyright: addlicense
	@echo "Adding copyright headers..."
	@git ls-files '*' $(GIT_LS_FILES_EXCLUDES) | xargs -r $(ADDLICENSE) -c "$(COPYRIGHT_OWNER)" -l $(COPYRIGHT_STYLE) $(COPYRIGHT_FLAGS) -y $(COPYRIGHT_YEAR)

# Install go-licenses tool locally
.PHONY: go-licenses
go-licenses: $(BIN_DIR)
	@if [ ! -f "$(GO_LICENSES)" ]; then \
		echo "Installing go-licenses to $(GO_LICENSES)..."; \
		GOBIN=$(abspath $(BIN_DIR)) go install github.com/google/go-licenses@$(GO_LICENSES_VERSION); \
	else \
		echo "go-licenses already installed at $(GO_LICENSES)"; \
	fi

# Generate THIRD_PARTY_NOTICES from go-licenses
.PHONY: third-party-licenses
third-party-licenses: go-licenses
	@echo "Collecting third-party licenses..."
	@$(GO_LICENSES) save ./... --save_path=third_party_licenses
	@echo "Generating THIRD_PARTY_NOTICES..."
	@find third_party_licenses -type f -iname "LICENSE*" | sort --ignore-case | while read -r license; do \
		echo "---"; \
		echo "## $$(basename $$(dirname "$$license"))"; \
		echo ""; \
		cat "$$license"; \
		echo ""; \
	done > THIRD_PARTY_NOTICES
	@rm -rf third_party_licenses
	@echo "THIRD_PARTY_NOTICES updated."