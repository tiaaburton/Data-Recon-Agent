# ADR-0004: Strategic Model Routing (Gemini 3.7 Flash Preview vs Gemini 3.1 Pro)

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: AI Systems Engineering
- **GCP Services Involved**: Vertex AI Model Garden, Gemini 3.7 Flash Preview, Gemini 3.1 Pro

## Context & Problem Statement
Using a single large frontier model (Gemini 3.1 Pro) for every request wastes budget and increases latency on simple ticket lookups. Conversely, using a smaller model (Gemini 3.7 Flash Preview) for complex multi-system arbitration degrades reconciliation accuracy.

## Decision Drivers
- p95 latency targets $\le 450\text{ ms}$ for simple lookups.
- High reasoning fidelity for multi-way financial discrepancy resolution.
- Cost optimization across enterprise scale.

## Considered Options
1. **Option 1 (Recommended)**: Dynamic Strategic Model Router in Go selecting between Gemini 3.7 Flash Preview and Gemini 3.1 Pro based on intent and token heuristics.
2. **Option 2**: Gemini 3.1 Pro exclusively for all interactions.
3. **Option 3**: Gemini 3.7 Flash Preview exclusively for all interactions.

## Decision Outcome
**Chosen Option**: **Option 1** because it achieves the optimal Pareto frontier: 64% cost reduction and 450ms lookup latency without compromising reconciliation accuracy on edge cases.
