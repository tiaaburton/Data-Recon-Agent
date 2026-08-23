# ==============================================================================
# Module: Cloud Storage Buckets for Artifacts, Datasets & Session Dumps
# ==============================================================================

variable "project_id" {
  type        = string
  description = "Google Cloud Project ID"
}

variable "region" {
  type        = string
  description = "Google Cloud Region"
}

variable "bucket_name" {
  type        = string
  description = "Cloud Storage Bucket Name for agent artifacts"
  default     = "data-recon-artifacts"
}

resource "random_id" "bucket_suffix" {
  byte_length = 4
}

locals {
  full_bucket_name = "${var.bucket_name}-${var.project_id}-${random_id.bucket_suffix.hex}"
}

resource "google_storage_bucket" "artifacts" {
  name                        = local.full_bucket_name
  project                     = var.project_id
  location                    = var.region
  force_destroy               = false
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type = "Delete"
    }
  }

  cors {
    origin          = ["https://gemini.google.com", "https://*.corp.google.com", "http://localhost:3000"]
    method          = ["GET", "HEAD", "PUT", "POST"]
    response_header = ["*"]
    max_age_seconds = 3600
  }
}

output "bucket_name" {
  value       = google_storage_bucket.artifacts.name
  description = "Name of the created Cloud Storage bucket"
}

output "bucket_url" {
  value       = google_storage_bucket.artifacts.url
  description = "URI of the Cloud Storage bucket"
}
