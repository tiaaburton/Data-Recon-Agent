terraform {
  required_version = ">= 1.7.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.30.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = ">= 5.30.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.6.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# ------------------------------------------------------------------------------
# 1. Enable Required GCP APIs
# ------------------------------------------------------------------------------
locals {
  required_services = [
    "networkservices.googleapis.com",
    "networksecurity.googleapis.com",
    "agentregistry.googleapis.com",
    "agentidentity.googleapis.com",
    "agentidentitycredentials.googleapis.com",
    "discoveryengine.googleapis.com",
    "aiplatform.googleapis.com",
    "compute.googleapis.com",
    "dns.googleapis.com",
    "iam.googleapis.com",
    "storage.googleapis.com",
    "modelarmor.googleapis.com",
    "telemetry.googleapis.com",
    "monitoring.googleapis.com",
    "cloudtrace.googleapis.com",
    "logging.googleapis.com",
    "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com"
  ]
}

resource "google_project_service" "services" {
  for_each                   = toset(local.required_services)
  project                    = var.project_id
  service                    = each.key
  disable_dependent_services = false
  disable_on_destroy         = false
}

# ------------------------------------------------------------------------------
# 2. Artifact Registry Module
# ------------------------------------------------------------------------------
module "artifact_registry" {
  source     = "./modules/artifact_registry"
  project_id = var.project_id
  region     = var.region
  depends_on = [google_project_service.services]
}

# ------------------------------------------------------------------------------
# 3. Cloud Storage Module
# ------------------------------------------------------------------------------
module "storage" {
  source     = "./modules/storage"
  project_id = var.project_id
  region     = var.region
  depends_on = [google_project_service.services]
}

# ------------------------------------------------------------------------------
# 4. Secret Manager Module
# ------------------------------------------------------------------------------
module "secrets" {
  source      = "./modules/secrets"
  project_id  = var.project_id
  secret_data = var.secret_data
  depends_on  = [google_project_service.services]
}

# ------------------------------------------------------------------------------
# 5. IAM & Agent Identity Module (SPIFFE)
# ------------------------------------------------------------------------------
module "iam" {
  source              = "./modules/iam"
  project_id          = var.project_id
  project_number      = var.project_number
  region              = var.region
  reasoning_engine_id = var.agent_engine_id
  trust_domain        = var.trust_domain
  depends_on          = [google_project_service.services]
}

# ------------------------------------------------------------------------------
# 6. Agent Gateway & Registry Declarative Delivery
# ------------------------------------------------------------------------------
resource "null_resource" "agent_gateway_egress" {
  depends_on = [google_project_service.services]

  triggers = {
    manifest_hash = filemd5("${path.module}/../iac/gateway/data-recon-agent-gateway-egress.yaml")
  }

  provisioner "local-exec" {
    command = <<-EOT
      gcloud network-services agent-gateways import data-recon-egress-gateway \
        --source="${path.module}/../iac/gateway/data-recon-agent-gateway-egress.yaml" \
        --location="${var.region}" \
        --project="${var.project_id}" || true
    EOT
  }
}

resource "null_resource" "iap_authz_extension" {
  depends_on = [null_resource.agent_gateway_egress]

  triggers = {
    extension_hash = filemd5("${path.module}/../iac/gateway/iap-request-authz-extension.yaml")
    policy_hash    = filemd5("${path.module}/../iac/gateway/iap-request-authz-policy.yaml")
  }

  provisioner "local-exec" {
    command = <<-EOT
      gcloud beta service-extensions authz-extensions import data-recon-iap-authz-ext \
        --source="${path.module}/../iac/gateway/iap-request-authz-extension.yaml" \
        --location="${var.region}" \
        --project="${var.project_id}" || true

      gcloud network-security authz-policies import data-recon-iap-authz-policy \
        --source="${path.module}/../iac/gateway/iap-request-authz-policy.yaml" \
        --location="${var.region}" \
        --project="${var.project_id}" || true
    EOT
  }
}

resource "null_resource" "agent_registry_entries" {
  depends_on = [null_resource.agent_gateway_egress]

  provisioner "local-exec" {
    command = <<-EOT
      bash "${path.module}/../scripts/setup_agent_gateway.sh"
    EOT
  }
}

# ------------------------------------------------------------------------------
# 7. Gemini Enterprise Agent Platform Module
# ------------------------------------------------------------------------------
module "gemini_enterprise" {
  source              = "./modules/gemini_enterprise"
  project_id          = var.project_id
  project_number      = var.project_number
  reasoning_engine_id = var.agent_engine_id
  depends_on          = [null_resource.agent_registry_entries]
}
