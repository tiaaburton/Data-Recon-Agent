# ADR-0006: In-Flight PII Redaction via Cloud Sensitive Data Protection (DLP) Middleware

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: SecOps, Compliance & Legal
- **GCP Services Involved**: Cloud DLP (Sensitive Data Protection API), Cloud Logging

## Context & Problem Statement
Customer records, IT incident notes, and billing documents frequently contain personally identifiable information (PII) such as SSNs, credit card numbers, personal phone numbers, and addresses. Storing unmasked PII in application logs or database session stores violates GDPR, HIPAA, and CCPA.

## Decision Drivers
- Strict compliance with enterprise data protection standards.
- Real-time in-flight sanitization before data touches persistent storage or logging sinks.
- Irreversible token replacement with zero data leakage.

## Considered Options
1. **Option 1 (Recommended)**: Go middleware integrating Google Cloud Sensitive Data Protection (DLP) API inspection templates.
2. **Option 2**: Basic regex scrubbing in application code.
3. **Option 3**: Post-ingestion batch log scrubbing.

## Decision Outcome
**Chosen Option**: **Option 1** because Cloud DLP provides enterprise-grade entity recognition (covering 150+ international infoTypes, contextual proximity matching, and structured de-identification) that far surpasses brittle regex matchers.
