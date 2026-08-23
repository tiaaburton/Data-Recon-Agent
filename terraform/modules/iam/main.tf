# ==============================================================================
# Module: Agent Identity & Least-Privilege IAM Bindings
# Supports SPIFFE-based Agent Identity on Google Cloud Agent Platform
# ==============================================================================

variable "project_id" {
  type        = string
  description = "Google Cloud Project ID"
}

variable "project_number" {
  type        = string
  description = "Google Cloud Project Number"
  default     = "14200540645"
}

variable "region" {
  type        = string
  description = "Google Cloud Region"
  default     = "us-central1"
}

variable "reasoning_engine_id" {
  type        = string
  description = "Vertex AI Reasoning Engine / Agent Engine Resource ID"
  default     = "1487588105090236416"
}

variable "trust_domain" {
  type        = string
  description = "SPIFFE Organization Trust Domain"
  default     = "agents.global.org-14200540645.system.id.goog"
}

# ------------------------------------------------------------------------------
# 1. Agent Identity SPIFFE Principal Identifier
# ------------------------------------------------------------------------------
locals {
  agent_spiffe_id = "spiffe://${var.trust_domain}/resources/aiplatform/projects/${var.project_number}/locations/${var.region}/reasoningEngines/${var.reasoning_engine_id}"
  agent_principal = "principal://${var.trust_domain}/resources/aiplatform/projects/${var.project_number}/locations/${var.region}/reasoningEngines/${var.reasoning_engine_id}"

  service_agents = [
    "serviceAccount:service-${var.project_number}@gcp-sa-aiplatform-re.iam.gserviceaccount.com",
    "serviceAccount:service-${var.project_number}@gcp-sa-aiplatform.iam.gserviceaccount.com",
    "serviceAccount:service-${var.project_number}@gcp-sa-aiplatform-cc.iam.gserviceaccount.com",
    "serviceAccount:${var.project_number}-compute@developer.gserviceaccount.com"
  ]

  # Reasoning Engine Platform Service Agent Roles (Cloud Trace / Logging / Secrets)
  re_service_agent_roles = [
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/secretmanager.secretAccessor"
  ]
}

# ------------------------------------------------------------------------------
# 2. Reasoning Engine & Platform Service Agent Roles
# ------------------------------------------------------------------------------
resource "google_project_iam_member" "re_service_agent_roles" {
  for_each = {
    for pair in setproduct(local.service_agents, local.re_service_agent_roles) :
    "${pair[0]}_${pair[1]}" => {
      member = pair[0]
      role   = pair[1]
    }
  }
  project = var.project_id
  role    = each.value.role
  member  = each.value.member
}

output "agent_spiffe_id" {
  value       = local.agent_spiffe_id
  description = "Cryptographic SPIFFE URI of the Agent Identity"
}

output "agent_principal" {
  value       = local.agent_principal
  description = "IAM Principal identifier for the Agent Identity"
}
