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

variable "agent_engine_id" {
  type        = string
  description = "Vertex AI Reasoning Engine ID for Data Recon Agent"
  default     = "1487588105090236416"
}
