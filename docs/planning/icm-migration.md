---
type: record
status: active
---

# ICM knowledge-base restructure

Date: 2026-09-11. Source repository commit: `62d98cf` plus the existing uncommitted architecture skill-installation note. This records a documentation change, not application validation.

## Form and inventory

A knowledge bundle with a compact change-impact guide fits the existing small Markdown library. No object-card hierarchy or artificial numbered pipeline was added. Stable reference shelves are separate from planning records and blank templates. All original project Markdown paths remain present. The installed skill and its reference assets were not modified.

## Migration map and referrers

| Original | Owning content now | Role | Known referrers |
|---|---|---|---|
| AGENTS.md | Root routing plus workflow contracts | Catalog / contract | CLAUDE.md; agent entry convention |
| CLAUDE.md | Retained pointer to AGENTS.md | Catalog | Agent entry convention |
| README.md | Root introduction and links | Catalog | Repository entry convention |
| CONTRIBUTING.md | Links to workflow and change impact | Catalog | Contributor entry convention |
| UBIQUITOUS_LANGUAGE.md | [Glossary](../product/glossary.md) | Reference via compatibility catalog | No explicit in-repo path referrers found |
| docs/mvp-spec.md | [Product shelf](../product/CONTEXT.md), protocol/data/workflow references | Reference via catalog | README.md, AGENTS.md |
| docs/architecture.md | [System shelf](../system/CONTEXT.md), development workflow | Reference via catalog | README.md, AGENTS.md |
| docs/implementation-plan.md | [Roadmap](roadmap.md), [status](status.md) | Planning product via catalog | README.md, AGENTS.md |
| docs/backlog.md | [Backlog](backlog.md) | Planning product via catalog | README.md, AGENTS.md |

Before editing, all source documents were copied to `/tmp/keepup-icm-before`; the four original docs were checked byte-for-byte by SHA-256 against their copies. This temporary backup is not a durable knowledge-base dependency. Git retains the tracked originals. New destination names were checked for case-folded collisions. The inventory found no documentation symlink consumers; dependency symlinks were left alone.

External consumers are unknown. Existing file paths and original second-level heading anchors remain as compatibility routes rather than removing files. No file was classified as dead or silently archived. Source last-touch metadata is available in Git history; the pre-existing architecture modification was retained in the development reference.

## Consolidation decisions

- Canonical glossary replaces repeated vocabulary lists; corrected the old multiple-connection statement to match current duplicate-connection rejection.
- Product rules, technical wiring, and delivery status have separate owners. The former verbose implementation checklist is summarized with links to owning implementation references.
- Current Go dependencies replace the old suggested stack; migrations point to the existing shell helper rather than nonexistent Makefile migration targets.
- GPS filtering requirements remain requirements; missing filters are recorded as an implementation gap without changing the next planned task.
- The explicit instruction to inspect all docs on session start remains; scoped ICM loading applies after that orientation.

## Review surface

Review this record and the Git diff for the migration. Follow AGENTS.md → product, system, and planning routes to inspect the result. No application code was changed.

## Validation

Local Markdown paths and linked heading fragments checked; original second-level entry headings retained. Root routing, product rules, presence styling change impact, and next-work routes walked successfully. Every knowledge folder has a contract, including the pre-existing empty ADR shelf. `git diff --check` passed. Application tests were not run for this documentation-only change.
