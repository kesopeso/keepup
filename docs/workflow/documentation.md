---
type: reference
status: active
---

# Documentation maintenance

Treat `docs/` as project memory and source-of-truth planning artifacts. Inspect every file in this folder on session start. After orientation, use the [catalog](../CONTEXT.md) to load only the task's relevant references.

Update affected documentation in the same change as any meaningful implementation, architecture, API, workflow, or scope work; do not wait for a reminder.

| Change | Stable entry point | Owning shelf |
|---|---|---|
| Product behavior | [MVP spec](../mvp-spec.md) | [Product](../product/CONTEXT.md) |
| System design, data flow, API, infrastructure | [Architecture](../architecture.md) | [System](../system/CONTEXT.md), [workflow](CONTEXT.md) |
| Sequencing / next work | [Implementation plan](../implementation-plan.md) | [Status and roadmap](../planning/CONTEXT.md) |
| Deferred / future work | [Backlog](../backlog.md) | [Deferred work](../planning/backlog.md) |

Keep project reference content in `docs/`. Root `README.md` and agent entry files provide identification and navigation; tooling configuration and application source stay in their required locations. Consolidate overlapping content into its existing owning page rather than creating another copy.

Keep entry links accurate when changing their owning pages. Do not copy full behavior into status lists or catalogs. The glossary owns terminology; product references own requirements; source code and system references establish implementation.

Create notes from the [template](../_templates/note.md). Every new topic folder needs a CONTEXT.md with inputs, process, outputs, and a human check. Preserve existing paths or enumerate and repair all consumers before relocating a file. Preserve old section anchors when practical.

Validation: check local Markdown links and heading fragments, inspect `git diff --check`, and walk a product question, a code-change question, and a next-work question from AGENTS.md. Application tests are only needed when executable behavior changes.
