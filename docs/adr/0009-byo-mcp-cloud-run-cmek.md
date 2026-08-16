# ADR-0009: BYO-MCP on Cloud Run with Single-Region CMEK & Secret Manager

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: Cloud Architecture, SecOps
- **GCP Services Involved**: Cloud Run, Secret Manager, Cloud KMS (CMEK), Artifact Registry

## Context & Problem Statement
The Model Context Protocol (MCP) allows agents to connect to external systems of record. We evaluated hosting options for the custom Go BYO-MCP server and how to securely inject runtime credentials without persistent secrets in container images.

## Decision Drivers
- Serverless scalability with zero idle cost.
- Direct VPC egress to private enterprise networks.
- Single-Region Customer-Managed Encryption Keys (CMEK) compliance.
- Runtime secret injection with automatic rotation.

## Considered Options
1. **Option 1 (Recommended)**: Cloud Run (BYO-MCP container) with GCP Secret Manager volume/env injection and Cloud KMS CMEK encryption.
2. **Option 2**: Self-managed GKE cluster with HashiCorp Vault.
3. **Option 3**: Standalone Compute Engine VMs.

## Decision Outcome
**Chosen Option**: **Option 1** because Cloud Run provides serverless autoscaling (0 to $N$), native IAM authentication, and direct integration with Secret Manager and Cloud KMS without Kubernetes operational overhead.
