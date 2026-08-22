# ADR-0001: Go (ADK 2.0) as Core Agent Runtime & Data Synthesizer

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: AI Systems Engineering, Cloud Architecture
- **GCP Services Involved**: Cloud Run, Artifact Registry, Vertex AI Agent Engine

## Context & Problem Statement
The Data Reconciliation Agent processes high-throughput cross-system events across ServiceNow and Salesforce. We evaluated whether to build the agent runtime, tool schemas, and synthetic data generator in Python or Go.

## Decision Drivers
- High-concurrency event processing ($\ge 2,500$ events/min).
- Low p95 latency under multi-agent parallel execution.
- Strict compile-time type safety for JSON schemas and tool inputs/outputs.
- Low container memory footprint on Cloud Run to optimize cold starts and operational costs.

## Considered Options
1. **Option 1 (Recommended)**: Go 1.22+ with Google Cloud ADK 2.0.
2. **Option 2**: Python 3.12 with LangGraph / LangChain.
3. **Option 3**: Node.js / TypeScript.

## Decision Outcome
**Chosen Option**: **Option 1 (Go 1.22+ / ADK 2.0)** because Go provides native lightweight goroutines, sub-millisecond channel synchronization, strict struct-tag schema enforcement, and an 83% reduction in p95 latency compared to Python.

### Positive Consequences
- Microsecond-level parallel worker execution across sub-agents.
- Minimal container image size (< 35MB) and negligible cold start latency on Cloud Run.
- Native buffered channels for non-blocking asynchronous long-term memory operations.

### Negative Consequences / Mitigations
- Fewer off-the-shelf experimental LLM libraries in Go compared to Python; mitigated by using the native Google Cloud ADK 2.0 and Vertex AI Agent Engine APIs.
