# Architecture Decision Records (ADRs)

This directory contains the formal record of all key architectural, security, design, and infrastructure decisions for the **Data Reconciliation Agent Ecosystem**.

---

## Decision Registry

| ADR ID | Title | Status | Deciders | Date | Key GCP Services |
| :---: | :--- | :---: | :--- | :---: | :--- |
| **[ADR-0001](0001-golang-agent-runtime.md)** | **Go (ADK 2.0) as Core Agent Runtime & Data Synthesizer** | `Accepted` | AI Systems Engineering | 2026-08-16 | Cloud Run, Artifact Registry |
| **[ADR-0002](0002-vertex-agent-engine-compaction.md)** | **Vertex AI Agent Engine with Native A2A & Token Compaction** | `Accepted` | AI Systems Engineering | 2026-08-16 | Vertex AI Agent Engine, Context Cache |
| **[ADR-0003](0003-multi-agent-coordinator-async-memory.md)**| **Coordinator-Worker Multi-Agent Pattern & Async Memory** | `Accepted` | Architecture Team | 2026-08-16 | Firestore, Cloud Run |
| **[ADR-0004](0004-strategic-model-routing.md)** | **Strategic Model Routing (Gemini 3.7 Flash Preview vs Gemini 3.1 Pro)** | `Accepted` | AI Systems Engineering | 2026-08-16 | Vertex AI Model Garden, Gemini 3.x |
| **[ADR-0005](0005-hitl-signed-webhook-intercepts.md)** | **Cryptographically Signed Webhook Intercepts for HITL Mutations** | `Accepted` | SecOps & Architecture | 2026-08-16 | Cloud Run, Secret Manager |
| **[ADR-0006](0006-cloud-dlp-pii-redaction.md)** | **In-Flight PII Redaction via Cloud Sensitive Data Protection (DLP)** | `Accepted` | SecOps & Compliance | 2026-08-16 | Cloud DLP API, Cloud Logging |
| **[ADR-0007](0007-custom-a2ui-catalog-styling.md)** | **Custom A2UI v0.9 Catalog over Generic Chatbot Widgets** | `Accepted` | Product Design & Eng | 2026-08-16 | Gemini Enterprise UI, A2UI v0.9 |
| **[ADR-0008](0008-pubsub-handcrafted-toolset.md)** | **Handcrafted Pub/Sub Toolset with Base64 & Pull/Ack Streaming** | `Accepted` | AI Systems Engineering | 2026-08-16 | Cloud Pub/Sub, ADK Tools |
| **[ADR-0009](0009-byo-mcp-cloud-run-cmek.md)** | **BYO-MCP on Cloud Run with Single-Region CMEK & Secret Manager** | `Accepted` | Cloud Architecture | 2026-08-16 | Cloud Run, KMS CMEK, Secret Manager |

---

## ADR Process & Guidelines
1. Every major architectural decision, protocol selection, or security boundary change must be documented as an ADR.
2. ADRs follow the standard format: Context & Problem Statement, Decision Drivers, Considered Options, Decision Outcome, and Pros & Cons.
