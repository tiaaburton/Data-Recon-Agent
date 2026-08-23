# ==============================================================================
# Module: Secret Manager for CRM/ITSM Credentials and API Keys
# ==============================================================================

variable "project_id" {
  type        = string
  description = "Google Cloud Project ID"
}

variable "secrets" {
  type        = list(string)
  description = "List of Secret Manager secret IDs to initialize"
  default = [
    "salesforce-client-id",
    "salesforce-client-secret",
    "salesforce-username",
    "salesforce-password",
    "servicenow-username",
    "servicenow-password",
    "hitl-signing-secret",
    "jwt-signing-key",
    "google-api-key"
  ]
}

variable "secret_data" {
  type        = map(string)
  description = "Optional map of initial secret payloads"
  default     = {}
}

resource "google_secret_manager_secret" "secrets" {
  for_each  = toset(var.secrets)
  secret_id = each.key
  project   = var.project_id

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "versions" {
  for_each    = toset(var.secrets)
  secret      = google_secret_manager_secret.secrets[each.key].id
  secret_data = lookup(var.secret_data, each.key, "bootstrap-secret-placeholder")
}

output "secret_ids" {
  value       = { for k, v in google_secret_manager_secret.secrets : k => v.id }
  description = "Map of created secret IDs"
}
