---
type: reference
status: active
---

# Documentation maintenance

Treat `docs/` as project memory. At session start, read this policy and the [context map](../CONTEXT.md), then stop orientation. Follow the task's shelf contract to its owning reference and relevant source. Read only the necessary sections; do not load all docs, full development guides, planning records, or linked references by default.

Before application commands, read the relevant section of [development workflow](development.md); Android commands also use the [Android workflow](mobile-development.md). Application toolchains run through Docker Compose; migrations remain manual.

## Automatic maintenance

For every repository change you make, check whether the documented behavior, boundaries, API, data, workflow, scope, or delivery status becomes inaccurate. Update affected docs in the same change without waiting for a reminder. If the docs already describe the resulting state, leave them alone. Repository changes made outside the agent session do not trigger a background watcher.

When documentation needs updating, use the installed [icm-architect skill](../../.agents/skills/icm-architect/SKILL.md). Read it once when first needed, then only the references relevant to the maintenance task. Use [core rules](../../.agents/skills/icm-architect/references/core.md) for contracts and ownership; use [system-map guidance](../../.agents/skills/icm-architect/references/system-map.md) for source boundaries and change impact. Do not run a full restructure or audit for each edit.

1. Follow the context map and [change-impact guide](../system/change-impact.md) to the affected owning pages and sources.
2. Verify current implementation against code and configuration. Keep requirements, implemented behavior, and planned work distinct.
3. Update the owning facts and their direct dependencies. Keep one home per fact, link instead of copying, and keep catalogs small. Change folder contracts when their inputs, process, outputs, or human check change.
4. Repair affected routes and source citations, then validate the diff and relevant navigation paths.

Write concise, factual prose using the [project terminology](../product/glossary.md) when needed. Explain behavior, ownership, dependencies, and consequential reasoning with concrete sources. Routine maintenance is part of authorized work; file relocations follow the skill's reference-integrity and migration workflow.

## Current state and history

Owning references describe the current state; [status](../planning/status.md) summarizes current delivery and material validation limits. Do not append a narrative for each change. Git commits and PRs hold routine history. Preserve dated test evidence in the separate [validation record](../planning/validation-history.md) when useful and consequential decisions in [ADRs](../adr/CONTEXT.md). Load these records only when the task needs their evidence or reasoning, never on every documentation check. Full audits occur only when requested or when a scoped investigation cannot resolve a conflict.

| Change | Stable entry point | Owning shelf |
|---|---|---|
| Product behavior | [MVP spec](../mvp-spec.md) | [Product](../product/CONTEXT.md) |
| System design, data flow, API, infrastructure | [Architecture](../architecture.md) | [System](../system/CONTEXT.md), [workflow](CONTEXT.md) |
| Sequencing / next work | [Implementation plan](../implementation-plan.md) | [Status and roadmap](../planning/CONTEXT.md) |
| Deferred / future work | [Backlog](../backlog.md) | [Deferred work](../planning/backlog.md) |

Keep project reference content in `docs/`. Root `README.md` and agent entry files provide identification and navigation; tooling configuration and application source stay in their required locations. Consolidate overlapping content into its existing owning page rather than creating another copy.

Keep entry links accurate when changing their owning pages. Do not copy full behavior into status lists or catalogs. The glossary owns terminology; product references own requirements; source code and system references establish implementation.

Create notes from the [template](../_templates/note.md). Every new topic folder needs a CONTEXT.md with inputs, process, outputs, and a human check. Preserve existing paths or enumerate and repair all consumers before relocating a file. Preserve old section anchors when practical.

Validation: check affected local Markdown links and heading fragments and inspect `git diff --check`. Walk the affected route from AGENTS.md; for routing changes, also walk a product question, a code-change question, and a next-work question. Confirm the walk does not load unrelated references or history. Application tests are only needed when executable behavior changes.
