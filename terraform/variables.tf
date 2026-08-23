variable "project_id" {
  type        = string
  description = "Google Cloud Project ID"
  default     = "tias-demos"
}

variable "region" {
  type        = string
  description = "Google Cloud Region"
  default     = "us-central1"
}

variable "project_number" {
  type        = string
  description = "Google Cloud Project Number"
  default     = "14200540645"
}

variable "agent_engine_id" {
  type        = string
  description = "Vertex AI Reasoning Engine ID for Data Recon Agent"
  default     = "1487588105090236416"
}

variable "trust_domain" {
  type        = string
  description = "SPIFFE Organization Trust Domain for Agent Identity"
  default     = "agents.global.org-14200540645.system.id.goog"
}

variable "secret_data" {
  type        = map(string)
  description = "Optional map of initial secret payloads to manage via Terraform"
  default     = {}
}
