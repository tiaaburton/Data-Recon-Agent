# ADR-0007: Custom A2UI Component Catalog and Advanced Visual Styling over Default Widgets

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: Product Design, AI Systems Engineering
- **GCP Services Involved**: Gemini Enterprise Workspace, A2UI v0.9 Protocol

## Context & Problem Statement
Default chatbot responses (plain text, markdown tables, generic buttons) fail to convey the multi-dimensional complexity of cross-system financial discrepancies, leading to operator hesitation, misunderstanding, or slow adoption.

## Decision Drivers
- High-impact visual communication for critical financial discrepancies.
- Cohesive design system tokens matching enterprise brand kits (Figma integration).
- Declarative, safe UI synthesis without executing untrusted client scripts.

## Considered Options
1. **Option 1 (Recommended)**: Custom A2UI v0.9 Declarative Catalog (Explosive Variance Badges, Three-Way Diff Tables, Signed Mutation Cards, Figma Tokens).
2. **Option 2**: Standard markdown text tables and bot quick-replies.
3. **Option 3**: Dynamic iframe HTML injection.

## Decision Outcome
**Chosen Option**: **Option 1** because A2UI declarative JSON decouples agent reasoning from frontend rendering while enabling rich, branded, interactive UI components with zero XSS risk.
