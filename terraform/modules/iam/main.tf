# ==============================================================================
# Module: IAM Service Accounts & Least-Privilege Role Bindings
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

# Dedicated Service Account for Data Reconciliation Agent
resource "google_service_account" "agent_sa" {
  account_id   = "data-recon-agent-sa"
  display_name = "Data Reconciliation Agent Runtime Service Account"
  project      = var.project_id
}

# Roles for Data Recon Agent Service Account
locals {
  agent_roles = [
    "roles/aiplatform.user",
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/secretmanager.secretAccessor",
    "roles/storage.objectUser",
    "roles/discoveryengine.user"
  ]
}

resource "google_project_iam_member" "agent_sa_roles" {
  for_each = toset(local.agent_roles)
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:${google_service_account.agent_sa.email}"
}

# Reasoning Engine Platform Service Agent Roles (Cloud Trace / Logging / Secrets)
locals {
  re_service_agent_roles = [
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/secretmanager.secretAccessor"
  ]
}

resource "google_project_iam_member" "re_service_agent_roles" {
  for_each = toset(local.re_service_agent_roles)
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:service-${var.project_number}@gcp-sa-aiplatform-re.iam.gserviceaccount.com"
}

output "agent_service_account_email" {
  value       = google_service_account.agent_sa.email
  description = "Email of the dedicated agent service account"
}
