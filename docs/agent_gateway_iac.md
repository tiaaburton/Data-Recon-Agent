# 🛡️ Google Cloud Agent Gateway & Agent Registry Infrastructure as Code (IaC)

This document details the declarative Infrastructure as Code (IaC) configuration, automated provisioning pipelines, and runtime access control models for **Google Cloud Agent Gateway** and **Agent Registry** governing the Data Reconciliation Agent.

---

## 1. Architectural Overview

The **Agent Gateway** serves as the centralized policy enforcement point for all agentic interactions across Google Cloud Agent Platform, Vertex AI Reasoning Engine, and Gemini Enterprise.

```
                    ┌────────────────────────────────────────┐
                    │    Gemini Enterprise (Web / Mobile)    │
                    └───────────────────┬────────────────────┘
                                        │
                                        ▼ (Ingress/A2A)
                    ┌────────────────────────────────────────┐
                    │       Google Cloud Agent Gateway       │
                    │  (projects/tias-demos/us-central1/...) │
                    ├────────────────────────────────────────┤
                    │  • Policy Enforcement: IAP / Authz     │
                    │  • Protocols: MCP, JSON-RPC, REST      │
                    │  • Audit Mode: DRY_RUN / ENFORCED      │
                    └───────┬────────────────────────┬───────┘
                            │                        │
               (Egress via  │                        │ (Governed
               IAP Binding) │                        │  Egress)
                            ▼                        ▼
        ┌───────────────────────────────┐ ┌──────────────────────────────────┐
        │  Vertex AI Reasoning Engine   │ │     External Managed APIs        │
        │      (Data Recon Agent)       │ │  • Salesforce Revenue Cloud      │
        │ (1487588105090236416)         │ │  • ServiceNow ITSM Incidents     │
        └───────────────────────────────┘ │  • Vertex AI Gemini Flash APIs   │
                                          └──────────────────────────────────┘
```

---

## 2. IaC Resource Definitions (`iac/gateway/`)

All Gateway and Registry components are declared as standard declarative YAML manifests:

| Manifest | Purpose | Resource Type |
|---|---|---|
| [`data-recon-agent-gateway-egress.yaml`](file:///usr/local/google/home/tiaburton/Documents/Users/tiaburton/Documents/dev_projects/Data-Recon-Agent/iac/gateway/data-recon-agent-gateway-egress.yaml) | Agent-to-Anywhere egress gateway routing outbound traffic | `networkservices.agentGateways` |
| [`data-recon-agent-gateway-ingress.yaml`](file:///usr/local/google/home/tiaburton/Documents/Users/tiaburton/Documents/dev_projects/Data-Recon-Agent/iac/gateway/data-recon-agent-gateway-ingress.yaml) | Client-to-Agent ingress gateway governing inbound invocations | `networkservices.agentGateways` |
| [`iap-request-authz-extension.yaml`](file:///usr/local/google/home/tiaburton/Documents/Users/tiaburton/Documents/dev_projects/Data-Recon-Agent/iac/gateway/iap-request-authz-extension.yaml) | Service Extension delegating authz to Identity-Aware Proxy (IAP) | `serviceextensions.authzExtensions` |
| [`iap-request-authz-policy.yaml`](file:///usr/local/google/home/tiaburton/Documents/Users/tiaburton/Documents/dev_projects/Data-Recon-Agent/iac/gateway/iap-request-authz-policy.yaml) | Network Security policy binding the Authz extension to the gateway | `networksecurity.authzPolicies` |

---

## 3. Automation & Makefile Commands

The provisioning workflow is fully automated and centralized into the Makefile:

### 1. Provision Agent Gateway & Service Extensions
```bash
make gateway-setup
```
* Enables all required Google Cloud APIs (`networkservices`, `networksecurity`, `agentregistry`, `discoveryengine`, `modelarmor`).
* Deploys the declarative Agent Gateway (`data-recon-egress-gateway`).
* Configures IAP request authorization extensions and policies.
* Registers the Reasoning Engine and outbound dependency endpoints (Salesforce, ServiceNow, Vertex AI) in Regional Agent Registry.
* Associates the live Reasoning Engine instance with the Agent Gateway.

### 2. Inspect Gateway & Binding Status
```bash
make gateway-status
```
Outputs the active gateway state, the JSON deployment spec binding on Vertex AI Reasoning Engine, and all registered endpoints in Agent Registry.

### 3. Register External Endpoints
```bash
make register-endpoints
```
Registers Salesforce Developer Org and ServiceNow ITSM instances into the regional Agent Registry catalog.

### 4. Register with Gemini Enterprise Agent Platform
```bash
make register-agent
# (or)
make gemini-enterprise
```
Enables the agent in Gemini Enterprise Discovery Engine (`gemini-app`) and links it to the deployed Reasoning Engine backend.

---

## 4. Operational Governance & Security

### IAP Dry-Run vs. Enforcement
The authorization extension is deployed in `DRY_RUN` mode by default (`iamEnforcementMode: "DRY_RUN"`) to ensure zero disruption during rollout while streaming audit logs to Google Cloud Logging. To switch to strict enforcement:
1. Update `iamEnforcementMode: "ENFORCE"` in [`iac/gateway/iap-request-authz-extension.yaml`](file:///usr/local/google/home/tiaburton/Documents/Users/tiaburton/Documents/dev_projects/Data-Recon-Agent/iac/gateway/iap-request-authz-extension.yaml).
2. Re-apply via `make gateway-setup`.
