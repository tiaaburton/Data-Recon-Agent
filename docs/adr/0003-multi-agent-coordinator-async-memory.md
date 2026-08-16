# ADR-0003: Multi-Agent Coordinator-Worker Pattern with Go Channels for Async Memory

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: Architecture Team
- **GCP Services Involved**: Cloud Run, Cloud Firestore Native

## Context & Problem Statement
A monolithic agent that tries to query and reason across ServiceNow, Salesforce, and SAP simultaneously incurs high hallucination rates and tool invocation errors. Furthermore, saving long-term conversational memory to databases introduces unwanted UI latency.

## Decision Drivers
- Isolated tool context per enterprise system.
- Sub-second UI response times without blocking on memory writes.
- Deterministic cross-system delta calculation.

## Considered Options
1. **Option 1 (Recommended)**: Coordinator-Worker Multi-Agent Pattern with Go buffered channels (`chan MemoryEvent`) for asynchronous Firestore persistence.
2. **Option 2**: Monolithic single-agent with 20+ tools registered.
3. **Option 3**: Synchronous database writes before returning agent responses.

## Decision Outcome
**Chosen Option**: **Option 1** because delegating tasks to dedicated sub-agents (`sn-worker`, `sf-worker`, `sap-worker`) reduces tool confusion to $<0.5\%$, and buffered channels decouple memory persistence from user-facing latency.

### Positive Consequences
- Zero UI thread blocking for memory generation.
- Independent scalability and testing for individual system connectors.
