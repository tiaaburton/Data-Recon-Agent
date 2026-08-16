# ADR-0008: Handcrafted Pub/Sub Toolset with Base64 & Pull/Ack Stream Handling

- **Status**: Accepted
- **Date**: 2026-08-16
- **Deciders**: AI Systems Engineering
- **GCP Services Involved**: Cloud Pub/Sub, ADK Toolsets (`google.adk.tools.pubsub`)

## Context & Problem Statement
Auto-generated Google API tools often struggle with edge cases in streaming messaging systems, such as base64 payload serialization, dead-letter routing, and fine-grained pull/ack/nack lifecycle controls.

## Decision Drivers
- Resilient base64 message encoding and decoding.
- Rich subscribe-side API reflecting enterprise pull/ack patterns.
- Poison-pill message handling via Dead Letter Queues (DLQ).

## Considered Options
1. **Option 1 (Recommended)**: Handcrafted `google.adk.tools.pubsub` toolset with explicit `PubSubCredentialsConfig` and `PubSubToolConfig`.
2. **Option 2**: Auto-generated `google_api_tool` Pub/Sub client.
3. **Option 3**: Generic webhook push endpoints without messaging queues.

## Decision Outcome
**Chosen Option**: **Option 1** because the handcrafted Pub/Sub toolset handles base64 binary encoding natively and provides superior control over message acknowledgment and lease management.
