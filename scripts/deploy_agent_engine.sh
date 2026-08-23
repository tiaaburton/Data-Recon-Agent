#!/bin/bash
# Copyright 2026 Google LLC
# Script to deploy the Go ADK Agent to Vertex AI Agent Engine (Reasoning Engine) using adkgo

set -e

# Change to project root directory
cd "$(dirname "$0")/.."

# Load environment variables
if [ -f .env.local ]; then
  set -a
  source .env.local
  set +a
elif [ -f .env ]; then
  set -a
  source .env
  set +a
fi

PROJECT_ID="${GCP_PROJECT_ID:-tias-demos}"
REGION="${GCP_REGION:-us-central1}"
AGENT_ENGINE_NAME="${SERVICE_NAME:-data-recon-agent}"
ACTIVE_MODEL="${GEMINI_MODEL:-gemini-3.7-flash}"
MEM_MODEL="publishers/google/models/${ACTIVE_MODEL#publishers/google/models/}"

echo "================================================================="
echo "🚀 Deploying Go ADK Agent ($AGENT_ENGINE_NAME) to Vertex AI Agent Engine"
echo "================================================================="
# Clean local binary build artifacts so source archive stays well under the 8MB gRPC payload limit
rm -rf bin/

# Ensure dependencies & checksums are resolved
echo "📦 Resolving Go module dependencies..."
go mod tidy

# Execute adkgo deploy agentengine via patched adkgo CLI
echo "🐹 Running adkgo deploy agentengine..."
go run ./cmd/adkgo \
  --env_file=".env.local" \
  --project_name="$PROJECT_ID" \
  --region="$REGION" \
  --name="$AGENT_ENGINE_NAME" \
  --mem_model="$MEM_MODEL" \
  --entry_point_path="cmd/agent/main.go" \
  --source_dir="." \
  ${AGENT_ENGINE_ID:+--agent_engine_id="$AGENT_ENGINE_ID"}

echo "✅ Successfully deployed Go ADK Agent to Vertex AI Agent Engine!"
