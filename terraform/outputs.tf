output "project_id" {
  value       = var.project_id
  description = "Target GCP Project ID"
}

output "region" {
  value       = var.region
  description = "Target GCP Region"
}

output "artifact_registry_repository_url" {
  value       = module.artifact_registry.repository_url
  description = "Artifact Registry Docker image repository URL"
}

output "storage_bucket_url" {
  value       = module.storage.bucket_url
  description = "Cloud Storage bucket for datasets and artifacts"
}

output "agent_service_account" {
  value       = module.iam.agent_service_account_email
  description = "Dedicated runtime Service Account for Data Recon Agent"
}

output "gateway_name" {
  value       = "data-recon-egress-gateway"
  description = "Deployed Agent Gateway Resource Name"
}

output "gemini_enterprise_app" {
  value       = module.gemini_enterprise.gemini_enterprise_app_path
  description = "Gemini Enterprise Application Path"
}

output "agent_engine_id" {
  value       = var.agent_engine_id
  description = "Governed Vertex AI Reasoning Engine ID"
}
