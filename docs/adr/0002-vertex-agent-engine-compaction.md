# ADR-0002: Vertex AI Agent Engine with Native A2A and Token Compaction

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: AI Systems Engineering
- **GCP Services Involved**: Vertex AI Agent Engine, Vertex AI Context Caching

## Context & Problem Statement
Multi-turn data reconciliation sessions often involve extensive system JSON snapshots, contract clauses, and audit trails, leading to severe context bloat and degraded LLM accuracy over time.

## Decision Drivers
- Prevent context window bloat and token exhaustion over multi-turn interactions.
- Enable high-fidelity Agent-to-Agent (A2A) protocol communication.
- Reduce latency and token costs via Context Caching.

## Considered Options
1. **Option 1 (Recommended)**: Vertex AI Agent Engine with token-based sliding window compaction ($N=6$) and native A2A mesh.
2. **Option 2**: Naive prompt truncation (dropping oldest turns indiscriminately).
3. **Option 3**: Self-managed vector database summarization.

## Decision Outcome
**Chosen Option**: **Option 1** because Vertex AI Agent Engine natively supports context caching, structured sliding window compaction, and standardized A2A agent mesh protocols.

### Positive Consequences
- Retains full fidelity for active turns while consolidating historical turns into structured semantic cards.
- 50%+ reduction in token inference costs via Vertex AI context caching.
