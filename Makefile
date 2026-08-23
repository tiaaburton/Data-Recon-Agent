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

# Include environment variables from .env and .env.local
ifneq (,$(wildcard .env))
    include .env
    export
endif
ifneq (,$(wildcard .env.local))
    include .env.local
    export
endif

# Map GCP Project / Cloud variables dynamically from .env / .env.local
PROJECT_ID       ?= $(GCP_PROJECT_ID)
REGION           ?= $(GCP_REGION)
LOCATION         ?= $(GCP_LOCATION)
SERVICE_NAME     ?= $(SERVICE_NAME)
PORT             ?= $(PORT)
GEMINI_MODEL     ?= $(GEMINI_MODEL)
ARTIFACTS_BUCKET ?= $(ARTIFACTS_BUCKET)

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
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-24s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(YELLOW)Example Commands:$(RESET)"
	@echo -e "  make setup-env                       # Interactive configuration for .env.local"
	@echo -e "  make synth COUNT=500                 # Generate 500 correlated records"
	@echo -e "  make seed                            # Load all records into Salesforce and ServiceNow"
	@echo -e "  make verify                          # Validate live record count and correlation"
	@echo -e "  make run-agent CONTRACT=CTR-2026-001 # Stream A2UI v0.9 reconciliation card"
	@echo -e "  make agent-runtime                   # Start/Deploy Go ADK Agent Runtime via .env"
	@echo -e "  make gemini-enterprise               # Connect Go ADK Agent Runtime to Gemini Enterprise"
	@echo ""

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
seed-sample: ## Seed a small sample (default 5 records) into Salesforce and ServiceNow (Usage: make seed-sample TARGET=servicenow LIMIT=5)
	@echo -e "$(CYAN)--> Seeding sample batch ($(if $(filter 0,$(LIMIT)),5,$(LIMIT)) records) to $(TARGET)...$(RESET)"
	@go run cmd/loader/main.go --input=$(OUTPUT) --target=$(TARGET) --limit=$(if $(filter 0,$(LIMIT)),5,$(LIMIT))

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

.PHONY: run-agent
run-agent: ## Execute the Data Reconciliation Agent and stream A2UI v0.9 cards (Usage: make run-agent CONTRACT=CTR-2026-001)
	@echo -e "$(CYAN)--> Running Data Reconciliation Agent for Contract $(CONTRACT)...$(RESET)"
	@go run cmd/agent/main.go --contract=$(CONTRACT)

.PHONY: build
build: ## Compile all standalone CLI binaries (cmd/synth, cmd/loader, cmd/verifier, cmd/agent)
	@echo -e "$(CYAN)--> Compiling CLI binaries to bin/...$(RESET)"
	@mkdir -p bin
	@go build -o bin/synth cmd/synth/main.go
	@go build -o bin/loader cmd/loader/main.go
	@go build -o bin/verifier cmd/verifier/main.go
	@go build -o bin/agent cmd/agent/main.go
	@echo -e "$(GREEN)Binaries built in bin/$(RESET)"

.PHONY: clean
clean: synth-clean ## Clean built binaries and temporary artifacts
	@echo -e "$(YELLOW)--> Cleaning binaries in bin/...$(RESET)"
	@rm -rf bin/
	@echo -e "$(GREEN)Clean complete.$(RESET)"

# ------------------------------------------------------------------------------
# 7. Agent Engine Runtime & Gemini Enterprise Integration (Go ADK 2.0 / A2A)
# ------------------------------------------------------------------------------

.PHONY: deploy deploy-agent-runtime
deploy: deploy-agent-runtime ## Alias for deploy-agent-runtime
deploy-agent-runtime: ## Deploy Go ADK 2.0 Agent to Vertex AI Agent Engine using adkgo
	@chmod +x scripts/deploy_agent_engine.sh
	@./scripts/deploy_agent_engine.sh

.PHONY: agent-runtime
agent-runtime: ## Execute the Go ADK 2.0 Agent Runtime with Vertex AI Agent Engine integration
	@echo -e "$(CYAN)--> Starting Go ADK 2.0 Agent Runtime ($(SERVICE_NAME))...$(RESET)"
	@echo -e "  Project:         $(PROJECT_ID)"
	@echo -e "  Region:          $(REGION)"
	@echo -e "  Model:           $(GEMINI_MODEL)"
	@echo -e "  Telemetry:       Cloud Trace / Cloud Logging (ADK_TELEMETRY_ENABLED=1)"
	@go run cmd/agent/main.go --contract=$(CONTRACT)

.PHONY: gemini-enterprise register-agent
register-agent: gemini-enterprise ## Alias for gemini-enterprise
gemini-enterprise: ## Register & connect Go ADK Agent to Gemini Enterprise Agent Platform and Agent Gateway
	@echo -e "$(CYAN)--> Enabling Agent Registry and Discovery Engine APIs on project $(PROJECT_ID)...$(RESET)"
	@gcloud services enable agentregistry.googleapis.com discoveryengine.googleapis.com --project=$(PROJECT_ID)
	@echo -e "$(CYAN)--> Registering and enabling Data Recon Agent in Gemini Enterprise Agent Gateway...$(RESET)"
	@bash scripts/register_gemini_enterprise.sh
	@echo -e "$(GREEN)✓ Successfully connected Go ADK Agent Runtime to Gemini Enterprise & Agent Gateway!$(RESET)"

.PHONY: gateway-setup gateway-status gateway-bind register-endpoints
gateway-setup: ## Provision Agent Gateway, Service Extensions, and Agent Registry via IaC manifests
	@echo -e "$(CYAN)--> Provisioning Google Cloud Agent Gateway & Agent Registry (IaC)...$(RESET)"
	@bash scripts/setup_agent_gateway.sh

gateway-status: ## Check Agent Gateway configuration, Reasoning Engine bindings, and registered endpoints
	@echo -e "$(CYAN)--> Checking Agent Gateway Status...$(RESET)"
	@gcloud network-services agent-gateways list --location=$(REGION) --project=$(PROJECT_ID) || true
	@echo -e "$(CYAN)--> Checking Reasoning Engine Gateway Binding...$(RESET)"
	@curl -s -H "Authorization: Bearer $$(gcloud auth print-access-token)" \
		"https://$(REGION)-aiplatform.googleapis.com/v1/projects/$(PROJECT_ID)/locations/$(REGION)/reasoningEngines/$(AGENT_ENGINE_ID)" | \
		python3 -c "import sys, json; data=json.load(sys.stdin); print(json.dumps(data.get('spec', {}).get('deploymentSpec', {}).get('agentGatewayConfig'), indent=2))" || true
	@echo -e "$(CYAN)--> Checking Registered Services in Agent Registry...$(RESET)"
	@gcloud agent-registry services list --location=$(REGION) --project=$(PROJECT_ID) || true

register-endpoints: ## Register Salesforce, ServiceNow, and Vertex AI endpoints in Agent Registry
	@echo -e "$(CYAN)--> Registering External Dependencies in Agent Registry ($(REGION))...$(RESET)"
	@gcloud agent-registry services create salesforce-revenue-cloud \
		--project=$(PROJECT_ID) --location=$(REGION) \
		--display-name="Salesforce Dev Org (Revenue Cloud)" \
		--endpoint-spec-type=no-spec \
		--interfaces="url=https://orgfarm-b2f2a8eb8d-dev-ed.develop.my.salesforce.com,protocolBinding=http-json" || true
	@gcloud agent-registry services create servicenow-itsm \
		--project=$(PROJECT_ID) --location=$(REGION) \
		--display-name="ServiceNow Dev Instance (ITSM Incidents)" \
		--endpoint-spec-type=no-spec \
		--interfaces="url=https://dev410998.service-now.com,protocolBinding=http-json" || true

.PHONY: tf-init tf-plan tf-apply
tf-init: ## Initialize Terraform providers and backend in terraform/
	@cd terraform && terraform init

tf-plan: ## Run Terraform plan for Agent Gateway & GCP infrastructure
	@cd terraform && terraform plan -var="project_id=$(PROJECT_ID)" -var="region=$(REGION)" -var="agent_engine_id=$(AGENT_ENGINE_ID)"

tf-apply: ## Apply Terraform infrastructure for Agent Gateway & GCP dependencies
	@cd terraform && terraform apply -auto-approve -var="project_id=$(PROJECT_ID)" -var="region=$(REGION)" -var="agent_engine_id=$(AGENT_ENGINE_ID)"



