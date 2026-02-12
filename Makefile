# =============================================================================
# Terraform Provider for RadosGW - Makefile
# =============================================================================

HOSTNAME = registry.local
NAMESPACE = fitbeard
NAME = radosgw
VERSION = $(strip 1.1.1)# x-release-please-version
OS_ARCH = $(shell go env GOOS)_$(shell go env GOARCH)
PLUGIN_DIR = ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

# Ceph development cluster settings
CEPH_DIR ?= /tmp/ceph-dev
CEPH_CONF = $(CEPH_DIR)/ceph.conf
RGW_PORT = 7480
RGW_ENDPOINT = http://127.0.0.1:$(RGW_PORT)

# Test settings
TEST_DIR = test
TEST_TIMEOUT = 5m
TEST_PARALLEL = 10
TEST_USER = admin
TEST_ACCESS_KEY = admin
TEST_SECRET_KEY = secretkey

# =============================================================================
# Default Target
# =============================================================================

.DEFAULT_GOAL := help

# =============================================================================
# Build & Install
# =============================================================================

.PHONY: build
build: ## Build the provider
	go build -v ./...

.PHONY: install
install: build ## Build and install provider to local Terraform plugins directory
	go install -v ./...
	mkdir -p $(PLUGIN_DIR)
	cp $(shell go env GOPATH)/bin/terraform-provider-$(NAME) $(PLUGIN_DIR)/terraform-provider-$(NAME)

# =============================================================================
# Code Quality
# =============================================================================

.PHONY: fmt
fmt: ## Format Go code
	gofmt -s -w -e .

.PHONY: fmt-check
fmt-check: ## Check if code is properly formatted
	@echo "Checking Go code formatting..."
	@test -z "$$(gofmt -s -l .)" || (echo "Code is not formatted. Run 'make fmt'" && gofmt -s -l . && exit 1)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: verify
verify: fmt-check lint vet ## Run all verification checks
	@echo "All verification checks passed!"

# =============================================================================
# Testing
# =============================================================================

.PHONY: test
test: ## Run unit tests
	go test -v -cover -timeout=120s -parallel=$(TEST_PARALLEL) ./...

.PHONY: testacc
testacc: ## Run acceptance tests (requires running Ceph cluster)
	TF_ACC=1 \
	RADOSGW_ENDPOINT=$(RGW_ENDPOINT) \
	RADOSGW_ACCESS_KEY=$(TEST_ACCESS_KEY) \
	RADOSGW_SECRET_KEY=$(TEST_SECRET_KEY) \
	go test -v -cover -timeout $(TEST_TIMEOUT) ./...

# =============================================================================
# Documentation
# =============================================================================

.PHONY: generate
generate: docs ## Generate documentation (alias for docs)

.PHONY: docs
docs: ## Generate provider documentation
	@echo "Formatting example Terraform files..."
	-@command -v tofu >/dev/null 2>&1 && tofu fmt -recursive ./examples/ || terraform fmt -recursive ./examples/
	@echo "Generating documentation..."
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir . --provider-name $(NAME)
	@echo "Transforming to Argument Reference format..."
	@./scripts/transform-docs.sh docs
	@echo "Documentation generated in docs/ directory"

.PHONY: docs-validate
docs-validate: ## Validate documentation
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-dir . --provider-name $(NAME)

# =============================================================================
# Ceph Development Cluster
# =============================================================================

.PHONY: ceph-bootstrap
ceph-bootstrap: ## Bootstrap Ceph development cluster
	@echo "Bootstrapping Ceph development cluster..."
	@chmod +x ./scripts/bootstrap-ceph.sh
	CEPH_DIR=$(CEPH_DIR) ./scripts/bootstrap-ceph.sh

.PHONY: ceph-user
ceph-user: ## Create RGW admin user for testing
	@echo "Creating RGW test user..."
	@chmod +x ./scripts/create-rgw-user.sh
	CEPH_DIR=$(CEPH_DIR) ./scripts/create-rgw-user.sh $(TEST_USER) "Test Admin User"

.PHONY: ceph-stop
ceph-stop: ## Stop Ceph development cluster
	@echo "Stopping Ceph cluster..."
	@chmod +x ./scripts/stop-ceph.sh
	CEPH_DIR=$(CEPH_DIR) ./scripts/stop-ceph.sh || true

.PHONY: ceph-status
ceph-status: ## Check Ceph cluster status
	@echo "Checking Ceph cluster status..."
	@chmod +x ./scripts/status-ceph.sh
	CEPH_DIR=$(CEPH_DIR) ./scripts/status-ceph.sh

.PHONY: ceph-wait
ceph-wait: ## Wait for RGW to be ready
	@echo "Waiting for RadosGW to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -sf $(RGW_ENDPOINT)/ >/dev/null 2>&1; then \
			echo "RadosGW is ready!"; \
			exit 0; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "ERROR: RadosGW failed to start"; \
			exit 1; \
		fi; \
		echo "Waiting... ($$i/30)"; \
		sleep 2; \
	done

.PHONY: ceph-logs
ceph-logs: ## Show Ceph logs
	@echo "=== Ceph Status ==="
	@ceph --conf $(CEPH_CONF) status 2>/dev/null || echo "Cannot get status"
	@echo ""
	@echo "=== RGW Logs (last 100 lines) ==="
	@tail -100 $(CEPH_DIR)/rgw.log 2>/dev/null || echo "No RGW logs"
	@echo ""
	@echo "=== Monitor Logs (last 50 lines) ==="
	@tail -50 $(CEPH_DIR)/mon.log 2>/dev/null || echo "No monitor logs"

.PHONY: ceph-setup
ceph-setup: ceph-bootstrap ceph-user ceph-wait ## Full Ceph cluster setup (bootstrap + user + wait)
	@echo ""
	@echo "=== Ceph Development Cluster Ready ==="
	@echo "  RGW endpoint:  $(RGW_ENDPOINT)"
	@echo "  Access Key:    $(TEST_ACCESS_KEY)"
	@echo "  Secret Key:    $(TEST_SECRET_KEY)"
	@echo ""

# =============================================================================
# Development Workflow
# =============================================================================

.PHONY: dev-setup
dev-setup: ceph-setup install ## Setup complete development environment
	@echo ""
	@echo "=== Development Environment Ready ==="
	@echo "  Provider installed to: $(PLUGIN_DIR)"
	@echo ""
	@echo "Set these environment variables for testing:"
	@echo "  export RADOSGW_ENDPOINT=$(RGW_ENDPOINT)"
	@echo "  export RADOSGW_ACCESS_KEY=$(TEST_ACCESS_KEY)"
	@echo "  export RADOSGW_SECRET_KEY=$(TEST_SECRET_KEY)"
	@echo ""

.PHONY: dev-test
dev-test: dev-setup testacc ## Full development test cycle
	@echo "Development test cycle complete!"

# =============================================================================
# Terraform Test Directory
# =============================================================================

.PHONY: tf-init
tf-init: install ## Initialize Terraform in test directory
	cd $(TEST_DIR) && rm -rf .terraform .terraform.lock.hcl && terraform init

.PHONY: tf-plan
tf-plan: ## Plan Terraform changes in test directory
	cd $(TEST_DIR) && terraform plan

.PHONY: tf-apply
tf-apply: ## Apply Terraform configuration in test directory
	cd $(TEST_DIR) && terraform apply -auto-approve

.PHONY: tf-destroy
tf-destroy: ## Destroy Terraform resources in test directory
	cd $(TEST_DIR) && terraform destroy -auto-approve

.PHONY: tf-test
tf-test: tf-init tf-apply ## Run Terraform test (init + apply)
	@echo "Terraform test complete!"

# =============================================================================
# CI Targets
# =============================================================================

.PHONY: ci-setup
ci-setup: ceph-setup ## CI: Setup Ceph cluster for acceptance tests
	@echo "CI setup complete"

.PHONY: ci-test
ci-test: testacc ## CI: Run acceptance tests
	@echo "CI tests complete"

.PHONY: ci-cleanup
ci-cleanup: ceph-stop ## CI: Cleanup after tests
	@echo "CI cleanup complete"

# =============================================================================
# Cleanup
# =============================================================================

.PHONY: clean
clean: ## Clean generated files
	rm -rf docs/
	rm -f terraform-provider-$(NAME)
	rm -rf $(TEST_DIR)/.terraform $(TEST_DIR)/.terraform.lock.hcl
	rm -f $(TEST_DIR)/terraform.tfstate $(TEST_DIR)/terraform.tfstate.backup

.PHONY: clean-all
clean-all: clean ceph-stop ## Clean everything including Ceph cluster
	rm -rf $(CEPH_DIR)

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help: ## Show this help message
	@echo "Terraform Provider for RadosGW"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Environment Variables:"
	@echo "  CEPH_DIR        Ceph cluster directory (default: /tmp/ceph-dev)"
	@echo "  CEPH_VERSION    Ceph version for version-specific tests"
	@echo "  TEST_TIMEOUT    Acceptance test timeout (default: 120m)"
	@echo ""
