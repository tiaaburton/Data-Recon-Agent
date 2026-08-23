# ==============================================================================
# Module: Artifact Registry Repository for Data Recon Agent Containers
# ==============================================================================

variable "project_id" {
  type        = string
  description = "Google Cloud Project ID"
}

variable "region" {
  type        = string
  description = "Google Cloud Region"
}

variable "repository_id" {
  type        = string
  description = "Artifact Registry repository ID"
  default     = "data-recon-repo"
}

resource "google_artifact_registry_repository" "repo" {
  project       = var.project_id
  location      = var.region
  repository_id = var.repository_id
  description   = "Docker repository for Go ADK Data Recon Agent & BYO-MCP container images"
  format        = "DOCKER"

  docker_config {
    immutable_tags = false
  }

  cleanup_policies {
    id     = "keep-minimum-versions"
    action = "KEEP"
    most_recent_versions {
      keep_count = 5
    }
  }
}

output "repository_id" {
  value       = google_artifact_registry_repository.repo.repository_id
  description = "Artifact Registry repository identifier"
}

output "repository_url" {
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.repo.repository_id}"
  description = "Artifact Registry Docker image push URI"
}
