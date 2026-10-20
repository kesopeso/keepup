---
type: contract
status: active
---

# KeepUp knowledge base

This is an ICM knowledge bundle with a compact edit map. Topic folders have no numeric prefixes because they are shelves, not sequential stages. Application source stays in `apps/`; migrations stay in `db/migrations/`.

## Inputs

Read [agent rules](../AGENTS.md) and [documentation discipline](workflow/documentation.md) once per session. This page is the context map. Choose one shelf contract below, then read only the owning page or section and source files needed for the request. A contract's links are conditional inputs, not instructions to load the entire shelf.

## Routing

| Task | Contract / entry |
|---|---|
| Product rules and language | [Product](product/CONTEXT.md) |
| Implementation and change impact | [System](system/CONTEXT.md) |
| Mobile implementation | [System](system/CONTEXT.md), then its mobile route |
| Local Android setup | [Workflow](workflow/CONTEXT.md), then its Android route |
| Current status, sequencing, deferred work | [Planning](planning/CONTEXT.md) |
| Development and documentation maintenance | [Workflow](workflow/CONTEXT.md) |
| Add a knowledge note | [Templates](_templates/CONTEXT.md) |

## Process

Follow the subject link; edit its owning page and cited code where relevant. Product pages state desired behavior, system pages describe implementation, and planning pages state delivery status. A requirement is not proof of implementation. Use source code to resolve as-built conflicts and record gaps in planning.

Read planning status only for progress, task selection, or a relevant implementation gap. Load dated validation records, migration history, ADRs, and Git history only to answer an evidence or decision question. Expand context when a dependency or uncertainty requires it; do not recursively open every link. A full audit is reserved for an explicit audit request.

## Outputs

Updated topic pages and links in the existing [MVP](mvp-spec.md), [architecture](architecture.md), [plan](implementation-plan.md), and [backlog](backlog.md) entry points as needed. These entry points preserve external path compatibility.

## Human check

Review the diff for changed decisions, accidental loss, and broken links. The owner can edit any Markdown artifact directly. This bundle imposes no new approval requirement on already-authorized implementation.

## Structure and state

Stable references live in `product/`, `system/`, and `workflow/`; current progress lives in `planning/status.md`; dated evidence is separate in `planning/validation-history.md`; decision records live in `adr/`; reusable blanks live in `_templates/`. YAML `type` identifies catalog, contract, reference, plan, template, or record. `status` identifies active, specified, planned, deferred, or draft. A record preserves evidence at its date, not current implementation.

This is a hand-maintained task catalog, not a generated file inventory. No generated indexes are needed for this small bundle.
