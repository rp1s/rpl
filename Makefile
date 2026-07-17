APP_NAME := rpl
FINGERPRINT_APP_NAME := fingerprint
GO ?= go
MODULE_DIR := src
BUILD_DIR := build
CMD_PATH := ./cmd
FINGERPRINT_CMD_PATH := ./cmd/fingerprint
GOCACHE ?= /tmp/rpl-go-build
FINGERPRINT ?=
RPL_LDFLAGS ?= $(if $(strip $(FINGERPRINT)),-X rpl/internal/version.Fingerprint=$(FINGERPRINT),)
INSTALL_DIR ?= $(HOME)/.local/bin
ATTR_SOURCE_DIR := $(MODULE_DIR)/attrs
PROFILE ?= $(if $(wildcard $(HOME)/.zshrc),$(HOME)/.zshrc,$(if $(wildcard $(HOME)/.bashrc),$(HOME)/.bashrc,$(HOME)/.profile))
PATH_EXPORT := export PATH="$(INSTALL_DIR):$$PATH"
VSCODE_PLUGIN_DIR := editors/vscode/rpl
TARGETS ?= darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
TEST_PACKAGES := ./cmd/... ./internal/... ./pkg/...
HOST_GOOS ?= $(shell $(GO) -C $(MODULE_DIR) env GOOS)
HOST_GOARCH ?= $(shell $(GO) -C $(MODULE_DIR) env GOARCH)
HOST_EXT := $(if $(filter windows,$(HOST_GOOS)),.exe,)
HOST_BUILD_DIR := $(BUILD_DIR)/$(HOST_GOOS)-$(HOST_GOARCH)
HOST_BIN_PATH := $(HOST_BUILD_DIR)/$(APP_NAME)$(HOST_EXT)
HOST_ATTRS_PATH := $(HOST_BUILD_DIR)/.rpl/attrs
HOST_SDK_PATH := $(HOST_BUILD_DIR)/.rpl/sdk
INSTALL_ATTRS_DIR := $(INSTALL_DIR)/.rpl/attrs
INSTALL_SDK_DIR := $(INSTALL_DIR)/.rpl/sdk
.DEFAULT_GOAL := install

.PHONY: build build-all build-host build-target build-attrs-target build-sdk-target build-fingerprint-target install uninstall test test-attrs test-examples test-projects clean help plugin vscode-plugin install-host-attrs install-host-sdk

help:
	@echo "Available targets:"
	@echo "  make                 # build host binary + bundled attrs + install both"
	@echo "  make build           # clean + build all release targets with attrs + fingerprint into build/"
	@echo "  make build-all"
	@echo "  make build-host      # clean + build current platform with attrs + fingerprint into build/"
	@echo "  make build-host FINGERPRINT=<hash>  # create an optional device-locked CLI build"
	@echo "  make install"
	@echo "  make uninstall"
	@echo "  make test"
	@echo "  make test-attrs     # test every built-in attr despite ':' in directory names"
	@echo "  make test-examples  # generate every focused schema and compile every full example project"
	@echo "  make test-projects  # generate and compile every full example project"
	@echo "  make plugin          # npm install + vsce package for VS Code extension"
	@echo "  make clean"
	@echo ""
	@echo "Configured targets: $(TARGETS)"

build: clean build-all

build-all:
	@set -e; \
	for target in $(TARGETS); do \
		goos=$${target%/*}; \
		goarch=$${target#*/}; \
		"$(MAKE)" --no-print-directory build-target GOOS_TARGET="$$goos" GOARCH_TARGET="$$goarch"; \
	done

build-host: clean
	@"$(MAKE)" --no-print-directory build-target GOOS_TARGET="$(HOST_GOOS)" GOARCH_TARGET="$(HOST_GOARCH)"

build-target:
	@set -e; \
	goos="$(GOOS_TARGET)"; \
	goarch="$(GOARCH_TARGET)"; \
	if [ -z "$$goos" ] || [ -z "$$goarch" ]; then \
		echo "GOOS_TARGET and GOARCH_TARGET are required"; \
		exit 1; \
	fi; \
	ext=""; \
	if [ "$$goos" = "windows" ]; then \
		ext=".exe"; \
	fi; \
	out_dir="$(CURDIR)/$(BUILD_DIR)/$$goos-$$goarch"; \
	mkdir -p "$$out_dir"; \
	echo "Building $(APP_NAME) for $$goos/$$goarch"; \
	GOCACHE="$(GOCACHE)" GOOS="$$goos" GOARCH="$$goarch" "$(GO)" -C "$(MODULE_DIR)" build $(if $(strip $(RPL_LDFLAGS)),-ldflags "$(RPL_LDFLAGS)",) -o "$$out_dir/$(APP_NAME)$$ext" "$(CMD_PATH)"; \
	"$(MAKE)" --no-print-directory build-attrs-target GOOS_TARGET="$$goos" GOARCH_TARGET="$$goarch" OUTPUT_ROOT="$$out_dir"; \
	"$(MAKE)" --no-print-directory build-sdk-target OUTPUT_ROOT="$$out_dir"; \
	"$(MAKE)" --no-print-directory build-fingerprint-target GOOS_TARGET="$$goos" GOARCH_TARGET="$$goarch"; \
	echo "Built $$out_dir"

build-attrs-target:
	@set -e; \
	goos="$(GOOS_TARGET)"; \
	goarch="$(GOARCH_TARGET)"; \
	output_root="$(OUTPUT_ROOT)"; \
	if [ -z "$$goos" ] || [ -z "$$goarch" ] || [ -z "$$output_root" ]; then \
		echo "GOOS_TARGET, GOARCH_TARGET and OUTPUT_ROOT are required"; \
		exit 1; \
	fi; \
	attrs_root="$$output_root/.rpl/attrs"; \
	rm -rf "$$attrs_root"; \
	mkdir -p "$$attrs_root"; \
	ext=""; \
	if [ "$$goos" = "windows" ]; then \
		ext=".exe"; \
	fi; \
	find "$(ATTR_SOURCE_DIR)" -mindepth 1 -maxdepth 1 -type d | sort | while read dir; do \
		name=$$(basename "$$dir"); \
		safe_name=$$(printf '%s' "$$name" | tr ':' '_'); \
		target_dir="$$output_root/.rpl/attrs/$$safe_name"; \
		mkdir -p "$$target_dir"; \
		if [ -f "$$dir/manifest.xml" ]; then \
			cp "$$dir/manifest.xml" "$$target_dir/manifest.xml"; \
		fi; \
		echo "Building attr $$name for $$goos/$$goarch -> $$safe_name"; \
		( cd "$$dir" && sources=$$(find . -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' | sort) && GOCACHE="$(GOCACHE)" GOOS="$$goos" GOARCH="$$goarch" "$(GO)" build -o "$$target_dir/attr$$ext" $$sources ); \
	done

build-sdk-target:
	@set -e; \
	sdk_root="$(OUTPUT_ROOT)/.rpl/sdk"; \
	if [ -z "$(OUTPUT_ROOT)" ]; then \
		echo "OUTPUT_ROOT is required"; \
		exit 1; \
	fi; \
	rm -rf "$$sdk_root"; \
	mkdir -p "$$sdk_root/pkg"; \
	printf 'module rpl\n\ngo 1.25.6\n' > "$$sdk_root/go.mod"; \
	cp -R "$(MODULE_DIR)/pkg/sdk" "$$sdk_root/pkg/sdk"; \
	echo "Bundled attr SDK in $$sdk_root"

build-fingerprint-target:
	@set -e; \
	goos="$(GOOS_TARGET)"; \
	goarch="$(GOARCH_TARGET)"; \
	if [ -z "$$goos" ] || [ -z "$$goarch" ]; then \
		echo "GOOS_TARGET and GOARCH_TARGET are required"; \
		exit 1; \
	fi; \
	ext=""; \
	if [ "$$goos" = "windows" ]; then \
		ext=".exe"; \
	fi; \
	out_dir="$(CURDIR)/$(BUILD_DIR)/fingerprint/$$goos-$$goarch"; \
	mkdir -p "$$out_dir"; \
	echo "Building $(FINGERPRINT_APP_NAME) for $$goos/$$goarch -> $$out_dir"; \
	GOCACHE="$(GOCACHE)" GOOS="$$goos" GOARCH="$$goarch" "$(GO)" -C "$(MODULE_DIR)" build -o "$$out_dir/$(FINGERPRINT_APP_NAME)$$ext" "$(FINGERPRINT_CMD_PATH)"

install: build-host install-host-attrs install-host-sdk
	@mkdir -p "$(INSTALL_DIR)"
	@rm -f "$(INSTALL_DIR)/$(APP_NAME)$(HOST_EXT)"
	@cp "$(HOST_BIN_PATH)" "$(INSTALL_DIR)/$(APP_NAME)$(HOST_EXT)"
	@touch "$(PROFILE)"
	@if ! grep -Fqx '$(PATH_EXPORT)' "$(PROFILE)"; then \
		printf '\n# rpl\n%s\n' '$(PATH_EXPORT)' >> "$(PROFILE)"; \
		echo "Added $(INSTALL_DIR) to PATH in $(PROFILE)"; \
	else \
		echo "PATH entry already present in $(PROFILE)"; \
	fi
	@echo "Installed $(APP_NAME) to $(INSTALL_DIR)/$(APP_NAME)$(HOST_EXT)"
	@echo "Installed bundled attrs to $(INSTALL_ATTRS_DIR)"
	@echo "Installed attr SDK to $(INSTALL_SDK_DIR)"
	@echo "Open a new shell or run: source $(PROFILE)"

install-host-attrs:
	@mkdir -p "$(INSTALL_DIR)/.rpl"
	@rm -rf "$(INSTALL_ATTRS_DIR)"
	@if [ -d "$(HOST_ATTRS_PATH)" ]; then \
		cp -R "$(HOST_ATTRS_PATH)" "$(INSTALL_ATTRS_DIR)"; \
		echo "Bundled attrs copied from $(HOST_ATTRS_PATH)"; \
	else \
		echo "No bundled attrs found in $(HOST_ATTRS_PATH)"; \
	fi

install-host-sdk:
	@mkdir -p "$(INSTALL_DIR)/.rpl"
	@rm -rf "$(INSTALL_SDK_DIR)"
	@if [ -d "$(HOST_SDK_PATH)" ]; then \
		cp -R "$(HOST_SDK_PATH)" "$(INSTALL_SDK_DIR)"; \
		echo "Attr SDK copied from $(HOST_SDK_PATH)"; \
	else \
		echo "No attr SDK found in $(HOST_SDK_PATH)"; \
	fi

uninstall:
	@rm -f "$(INSTALL_DIR)/$(APP_NAME)$(HOST_EXT)"
	@rm -rf "$(INSTALL_ATTRS_DIR)"
	@rm -rf "$(INSTALL_SDK_DIR)"
	@echo "Removed $(INSTALL_DIR)/$(APP_NAME)$(HOST_EXT)"
	@echo "Removed bundled attrs from $(INSTALL_ATTRS_DIR)"
	@echo "Removed attr SDK from $(INSTALL_SDK_DIR)"
	@echo "If needed, remove this line from $(PROFILE): $(PATH_EXPORT)"

test:
	@GOCACHE="$(GOCACHE)" "$(GO)" -C "$(MODULE_DIR)" test $(TEST_PACKAGES)
	@"$(MAKE)" --no-print-directory test-attrs

test-attrs:
	@set -e; \
	find "$(ATTR_SOURCE_DIR)" -mindepth 1 -maxdepth 1 -type d | sort | while read dir; do \
		echo "Testing attr $$(basename "$$dir")"; \
		( cd "$$dir" && GOCACHE="$(GOCACHE)" "$(GO)" test *.go ); \
	done

test-projects:
	@GOCACHE="$(GOCACHE)" "$(GO)" -C "$(MODULE_DIR)" test ./internal/service/compiler -run '^TestProjectExamplesGenerateAndCompile$$' -count=1

test-examples:
	@GOCACHE="$(GOCACHE)" "$(GO)" -C "$(MODULE_DIR)" test ./internal/service/compiler -run '^(TestFocusedAttrExamplesGenerate|TestProjectExamplesGenerateAndCompile)$$' -count=1

plugin: vscode-plugin

vscode-plugin:
	@cd "$(VSCODE_PLUGIN_DIR)" && npm install
	@cd "$(VSCODE_PLUGIN_DIR)" && npx @vscode/vsce package
	@echo "Packaged VS Code extension in $(VSCODE_PLUGIN_DIR)"

clean:
	@rm -rf "$(BUILD_DIR)"
	@echo "Removed $(BUILD_DIR)"
