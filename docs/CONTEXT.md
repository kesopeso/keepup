---
type: contract
status: active
---

# KeepUp knowledge base

This is an ICM knowledge bundle with a compact edit map. Topic folders have no numeric prefixes because they are shelves, not sequential stages. Application source stays in `apps/`; migrations stay in `db/migrations/`.

## Inputs

Read [agent rules](../AGENTS.md), the relevant shelf contract below, and its named sources. The existing session-start instruction to inspect every document in `docs/` remains in force; scoped loading applies to subsequent task work.

## Routing

| Task | Contract / entry |
|---|---|
| Product rules and language | [Product](product/CONTEXT.md) |
| Implementation and change impact | [System](system/CONTEXT.md) |
| Current status, sequencing, deferred work | [Planning](planning/CONTEXT.md) |
| Development and documentation maintenance | [Workflow](workflow/CONTEXT.md) |
| Add a knowledge note | [Templates](_templates/CONTEXT.md) |

## Process

Follow the subject link; edit its owning page and cited code where relevant. Product pages state desired behavior, system pages describe implementation, and planning pages state delivery status. A requirement is not proof of implementation. Use source code to resolve as-built conflicts and record gaps in planning.

## Outputs

Updated topic pages and links in the existing [MVP](mvp-spec.md), [architecture](architecture.md), [plan](implementation-plan.md), and [backlog](backlog.md) entry points as needed. These entry points preserve external path compatibility.

## Human check

Review the diff for changed decisions, accidental loss, and broken links. The owner can edit any Markdown artifact directly. This bundle imposes no new approval requirement on already-authorized implementation.

## Structure and state

Stable references live in `product/`, `system/`, and `workflow/`; current project artifacts live in `planning/`; reusable blanks live in `_templates/`. YAML `type` identifies catalog, contract, reference, plan, template, or record. `status` identifies active, specified, planned, deferred, or draft. Status pages record delivery separately from a reference's publication status.

This is a hand-maintained task catalog, not a generated file inventory. No generated indexes are needed for this small bundle.
