# ==============================================================================
# Module: Gemini Enterprise Agent Platform Integration
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

variable "reasoning_engine_id" {
  type        = string
  description = "Vertex AI Reasoning Engine ID"
}

variable "app_location" {
  type        = string
  description = "Gemini Enterprise location"
  default     = "us"
}

variable "engine_name" {
  type        = string
  description = "Discovery Engine app name"
  default     = "gemini-app"
}

resource "null_resource" "gemini_enterprise_agent" {
  triggers = {
    engine_id = var.reasoning_engine_id
  }

  provisioner "local-exec" {
    command = <<-EOT
      bash "${path.module}/../../scripts/register_gemini_enterprise.sh"
    EOT
  }
}

output "gemini_enterprise_app_path" {
  value       = "projects/${var.project_id}/locations/${var.app_location}/collections/default_collection/engines/${var.engine_name}"
  description = "Resource path of the Gemini Enterprise App"
}
