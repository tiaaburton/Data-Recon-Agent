output "project_id" {
  value       = var.project_id
  description = "Target GCP Project ID"
}

output "region" {
  value       = var.region
  description = "Target GCP Region"
}

output "gateway_name" {
  value       = "data-recon-egress-gateway"
  description = "Deployed Agent Gateway Resource Name"
}

output "agent_engine_id" {
  value       = var.agent_engine_id
  description = "Governed Vertex AI Reasoning Engine ID"
}
