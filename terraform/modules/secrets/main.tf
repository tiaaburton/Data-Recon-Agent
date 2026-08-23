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
    "jwt-signing-key"
  ]
}

resource "google_secret_manager_secret" "secrets" {
  for_each  = toset(var.secrets)
  secret_id = each.key
  project   = var.project_id

  replication {
    auto {}
  }
}

output "secret_ids" {
  value       = { for k, v in google_secret_manager_secret.secrets : k => v.id }
  description = "Map of created secret IDs"
}
