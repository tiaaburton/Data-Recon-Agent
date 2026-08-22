# ADR-0005: Cryptographically Signed Webhook Intercepts for High-Stakes Mutation Gates

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: SecOps, Enterprise Architecture
- **GCP Services Involved**: Cloud Run, Secret Manager, Cloud KMS

## Context & Problem Statement
Autonomous write mutations to enterprise commercial contracts and billing ledgers (e.g. Salesforce billing schedules, ServiceNow dispute credits) carry significant financial and legal risk. We must guarantee that an agent cannot execute unauthorized writes due to prompt injection, hallucination, or unverified triggers.

## Decision Drivers
- Zero unauthorized mutations to production ERP / CRM systems.
- Cryptographic proof of human operator identity and authorization timestamp.
- Non-replayable, time-bounded approval tokens.

## Considered Options
1. **Option 1 (Recommended)**: Cryptographic state check intercept halting mutations until an HMAC-SHA256 / Ed25519 signed webhook is received.
2. **Option 2**: Pure LLM self-evaluation / internal confirmation prompt.
3. **Option 3**: Unrestricted autonomous agent write access.

## Decision Outcome
**Chosen Option**: **Option 1** because cryptographic signatures provide non-repudiable audit logs, preventing prompt injection attacks from bypassing safety controls.
