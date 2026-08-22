# ==============================================================================
# Enterprise Data Reconciliation Agent: Developer Automation Makefile
# ==============================================================================
# Runtime: Go 1.22+
# Targets: Data Synthesis, Multi-System Seeding, Live Verification, Testing & Build
# ==============================================================================

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Colors for terminal output formatting
CYAN    := \033[36m
GREEN   := \033[32m
YELLOW  := \033[33m
RED     := \033[31m
RESET   := \033[0m
BOLD    := \033[1m

# Configurable parameter defaults
COUNT      ?= 500
OUTPUT     ?= data/correlated_recon_500.json
LIMIT      ?= 0
TARGET     ?= all
MODE       ?= LIVE
CONTRACT   ?= CTR-2026-001

.PHONY: help
help: ## Show this help message and target descriptions
	@echo -e "$(BOLD)$(CYAN)Enterprise Data Reconciliation Agent — Makefile Targets$(RESET)"
	@echo -e "$(YELLOW)Usage:$(RESET) make [target] [OPTION=value]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(YELLOW)Example Commands:$(RESET)"
	@echo -e "  make setup-env                       # Interactive configuration for .env.local"
	@echo -e "  make synth COUNT=500                 # Generate 500 correlated records"
	@echo -e "  make seed-dry-run                    # Simulate seeding without API calls"
	@echo -e "  make seed-sample LIMIT=5             # Load 5 test records into SFDC and ServiceNow"
	@echo -e "  make seed                            # Load all records (prompts with choices if credentials missing)"
	@echo -e "  make verify                          # Validate live record count and correlation"

# ------------------------------------------------------------------------------
# 0. Environment & Credentials Setup
# ------------------------------------------------------------------------------

.PHONY: setup-env
setup-env: ## Interactively configure or verify .env.local credentials
	@if [ ! -f .env.local ]; then \
		echo -e "$(YELLOW)--> .env.local not found. Creating from .env.local.example...$(RESET)"; \
		cp .env.local.example .env.local; \
	fi
	@echo -e "$(CYAN)--> Checking local environment credentials in .env.local...$(RESET)"
	@source .env.local 2>/dev/null || true; \
	echo -e "  Salesforce Instance: $${SFDC_INSTANCE_URL:-[NOT SET]}"; \
	echo -e "  Salesforce User:     $${SFDC_USERNAME:-[NOT SET]}"; \
	echo -e "  ServiceNow Instance: $${SERVICENOW_INSTANCE_URL:-[NOT SET]}"; \
	echo -e "  ServiceNow User:     $${SERVICENOW_USERNAME:-[NOT SET]}"; \
	echo ""; \
	echo -e "$(GREEN)Tip:$(RESET) If credentials are missing when running 'make seed', the tool will interactively prompt you with choices (Enter now / Dry-run / Skip)."

# ------------------------------------------------------------------------------
# 1. Data Synthesis Targets
# ------------------------------------------------------------------------------

.PHONY: synth
synth: ## Generate synthetic correlated reconciliation dataset (Usage: make synth COUNT=500)
	@echo -e "$(CYAN)--> Generating $(COUNT) correlated reconciliation records...$(RESET)"
	@go run cmd/synth/main.go --count=$(COUNT) --output=$(OUTPUT)

.PHONY: synth-clean
synth-clean: ## Remove generated synthetic dataset files
	@echo -e "$(YELLOW)--> Cleaning synthetic datasets in data/...$(RESET)"
	@rm -rf data/*.json
	@echo -e "$(GREEN)Clean complete.$(RESET)"

# ------------------------------------------------------------------------------
# 2. Multi-System Seeding Targets (Salesforce & ServiceNow)
# ------------------------------------------------------------------------------

.PHONY: seed-dry-run
seed-dry-run: ## Simulate multi-system insertion without making live API calls
	@echo -e "$(YELLOW)--> Running dry-run seeding simulation...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=all --dry-run=true

.PHONY: seed-sample
seed-sample: ## Seed a small sample (default 5 records) into Salesforce and ServiceNow
	@echo -e "$(CYAN)--> Seeding sample batch ($(if $(filter 0,$(LIMIT)),5,$(LIMIT)) records) to Salesforce & ServiceNow...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=all --limit=$(if $(filter 0,$(LIMIT)),5,$(LIMIT))

.PHONY: seed
seed: ## Seed all dataset records (prompts with interactive choices if credentials missing)
	@echo -e "$(CYAN)--> Seeding all records to Salesforce and ServiceNow...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=$(TARGET) --limit=$(LIMIT)

.PHONY: seed-sf
seed-sf: ## Seed records only into Salesforce CRM
	@echo -e "$(CYAN)--> Seeding records into Salesforce CRM...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=salesforce --limit=$(LIMIT)

.PHONY: seed-sn
seed-sn: ## Seed records only into ServiceNow ITSM
	@echo -e "$(CYAN)--> Seeding records into ServiceNow ITSM...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=servicenow --limit=$(LIMIT)

# ------------------------------------------------------------------------------
# 3. Live Verification Targets
# ------------------------------------------------------------------------------

.PHONY: verify
verify: ## Verify live records in Salesforce and ServiceNow sandboxes
	@echo -e "$(CYAN)--> Executing live multi-system verification check...$(RESET)"
	@go run cmd/verifier/main.go --input=$(OUTPUT)

# ------------------------------------------------------------------------------
# 4. Automated End-to-End Pipeline (One-Command Workflow)
# ------------------------------------------------------------------------------

.PHONY: all-data
all-data: synth seed verify ## Execute full pipeline: synth -> seed -> verify
	@echo -e "$(GREEN)$(BOLD)Data pipeline completed successfully!$(RESET)"

# ------------------------------------------------------------------------------
# 5. Testing, Quality & Benchmarking
# ------------------------------------------------------------------------------

.PHONY: test
test: ## Run unit tests across all packages
	@echo -e "$(CYAN)--> Running unit test suite...$(RESET)"
	@go test -v ./pkg/...

.PHONY: test-race
test-race: ## Run tests with race detector enabled
	@echo -e "$(CYAN)--> Running race detection tests...$(RESET)"
	@go test -v -race ./pkg/... ./tests/...

.PHONY: eval
eval: ## Run 500-sample golden evaluation benchmark suite
	@echo -e "$(CYAN)--> Running automated benchmark evaluation against golden dataset...$(RESET)"
	@go test -v ./tests/... -run TestGoldenEvaluation_500

.PHONY: lint
lint: ## Run go vet and static code checks
	@echo -e "$(CYAN)--> Running Go code linters and vetting...$(RESET)"
	@go vet ./...

# ------------------------------------------------------------------------------
# 6. Build & Execution
# ------------------------------------------------------------------------------

.PHONY: build
build: ## Compile all standalone CLI binaries (cmd/synth, cmd/loader, cmd/verifier)
	@echo -e "$(CYAN)--> Compiling CLI binaries to bin/...$(RESET)"
	@mkdir -p bin
	@go build -o bin/synth cmd/synth/main.go
	@go build -o bin/loader cmd/loader/main.go
	@go build -o bin/verifier cmd/verifier/main.go
	@echo -e "$(GREEN)Binaries built in bin/$(RESET)"

.PHONY: clean
clean: synth-clean ## Clean built binaries and temporary artifacts
	@echo -e "$(YELLOW)--> Cleaning binaries in bin/...$(RESET)"
	@rm -rf bin/
	@echo -e "$(GREEN)Clean complete.$(RESET)"
