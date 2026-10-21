---
type: contract
status: active
---

# System shelf

Own implementation boundaries and point editors to source.

## Inputs

Choose only the route needed for the request:

| Question | Read |
|---|---|
| What a code change affects | [Change-impact guide](change-impact.md), then its matching row |
| Web screens, browser identity, rendering | [Frontend](frontend.md) |
| Android screens, saved access, live viewing, native sharing | [Mobile](mobile.md), relevant section |
| Go runtime, health, service ownership | [Backend](backend.md) |
| REST, snapshots, WebSocket events, presence timers | [API and live protocol](api-and-live.md) |
| Account authentication implementation and target client contract | [Account contract](authentication-contract.md) |
| Tables, constraints, accepted point history | [Persistence](data.md) |
| Reason for a recorded architectural decision | [ADRs](../adr/CONTEXT.md), only when needed |

## Process

Read the chosen page or section. Before changing behavior, use the matching change-impact row and the specific [product rule](../product/CONTEXT.md). Continue through the [code-navigation workflow](../workflow/documentation.md#code-navigation), using the reference's source links as starting points. Verify as-built claims against the relevant source; expand to dependent surfaces only when they are affected. Do not load every system page or ADR.

## Outputs

Update the chosen implementation reference and [architecture routing](../architecture.md).

## Human check

Check source citations and first-order effects; do not describe planned features as implemented.
