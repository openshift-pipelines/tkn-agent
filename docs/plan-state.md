# Repository-tracked verification plans

Status: implemented bounded workflow, 2026-11-18. Shared plans and per-user
acceptance are separate. Broader portability remains bounded by the supported
source capabilities.

## Store the plan; rerun the checks

The reusable object is the reviewed selection and execution plan. A previous
passing result does not validate a new application edit.

Keep each shared plan under `.tekton/.tkn-agent`, mirroring the source filename:
`.tekton/ci.yaml` uses `.tekton/.tkn-agent/ci.yaml.plan.json`. Nested paths are
preserved. Use Git commits and diffs as its history.
An append-only history database or a directory of numbered plan copies is
unnecessary.

For generated `.tekton` output, keep the record alongside its source definitions,
for example `.konflux/.tkn-agent/tekton/ci.yaml.plan.json`. Confirm that PaC and generation tools
ignore the reserved JSON directory. Directory presence alone does not establish
that a repository contains runnable Tekton validation.

## Reuse the existing plan representation

The shared file contains version 2 of `plan.Plan`: source-relative definition
and inspected-helper hashes, complete run/skip/unresolved/provided scope, source scripts,
image references and resolved identities, platform, and execution policy.
Retain schema/capability versioning. Add provenance for resolved remote definitions
when that capability is implemented.

Exclude machine paths, provider credentials, session data, local acceptance,
execution logs and mutable caches from this shared file.

The existing `plan.Cache.Accepted` belongs in per-user local state. Committing a
plan does not grant every developer permission to execute it. A fresh checkout
can load and validate the shared plan without a model call, then request one
local approval of its exact effective digest.

## Invalidation is about effective planning inputs

| Change | Required behavior |
| --- | --- |
| Application code changes; planning inputs match | Reuse the plan, snapshot current edits and rerun selected checks. No model call. |
| Pipeline or referenced Task definition changes | Reanalyze affected scope and require review of its new effective plan. |
| An inspected helper or check configuration changes outside `.tekton` | Invalidate its recorded input hash even if `.tekton` is unchanged. |
| A relevant optional configuration appears or disappears | Refresh its recorded presence; a new configuration can change validation without changing pipeline YAML. |
| A resolved image or execution policy changes | Require review of the effective execution change; a model call is unnecessary when scope analysis itself is unchanged. |
| The shared plan is saved or committed | Do not invalidate it through a self-reference. |
| A required input is missing, unsupported or unverifiable | Block the affected scope; never reuse a passing receipt as evidence. |

Hash the explicit definition/helper inputs. Do not hash the entire `.tekton`
directory: that includes the saved plan and can create a regeneration loop.
The planner's repository tool must not record the state file as its own planning
input. Resolved external definitions need immutable source pins and hashes,
rather than names such as `latest` or an unpinned branch.

Declared optional configuration needs a presence/absence record as well as
hashes for existing files. For example, creating a tool's previously absent
repository configuration can change its behavior. The input map records `absent`
for inspected missing files and `link:TARGET` for safe inspected links, alongside
the target file hash.

Application source identity belongs in each execution receipt, independently
of the plan's reuse key. This preserves fresh-source checking without invoking
the planner on every edit.

## Update flow

1. Discover the selected CI source and shared plan.
2. Validate its schema, current source-derived execution and input fingerprints.
3. If it matches, resolve the effective runtime context and check local approval.
4. If missing or stale, generate a candidate, validate it and present its diff
   and digest. Missing provider access produces a blocked outcome.
5. After explicit approval, save the shared plan and local acceptance separately.
   Let the developer review and commit the plan with the CI/helper change.
6. Execute current source and write receipts outside the tracked directory.

Normal unattended verification must not rewrite tracked plans. Generating or
accepting a changed plan is the explicit update path. No automatic Git commit,
push, or inherited approval is implied.

## Acceptance coverage

These extend LOCAL-006/007/014. Plan/reader units cover fresh-context reuse,
source tampering, absent inputs and state self-reference. CLI acceptance covers
review, persistence, credential-free replay and helper changes. Git history
restoration itself is not separately exercised.

| Proposed ID | Independent observation |
| --- | --- |
| LOCAL-015 | A fresh clone with a valid shared plan needs local approval, then runs without provider access or a model call. |
| LOCAL-016 | An application-only defect is detected using the same shared plan and a new source receipt. |
| LOCAL-017 | A referenced check helper changes while `.tekton` stays unchanged; old scope cannot execute without the required reanalysis/review. |
| LOCAL-018 | Saving a plan does not cause the next verification to regenerate it. |
| LOCAL-019 | Shared state contains no credential values, machine paths, local approval or execution receipts. |
| LOCAL-020 | A plan restored from Git history is rejected when its definition/helper inputs no longer match. |

## Limits

- Shared state never includes local approval; fresh contexts need local review.
- Explicit approval saves atomically and refuses a conflicting intervening write.
- Source pull policy is applied before calculating the effective digest. Changed
  image identity or platform requires renewed review.
- Unsupported shared versions require `--refresh`. Legacy local plans regenerate
  rather than execute under new source semantics.

The real repository assessment also shows that clone prerequisites, workspace
aliases/subdirectories, image parameters and pull policy need explicit supported
adaptations. See `real-slice.md` for the bounded supported contract. General
remote execution, arbitrary dependencies, conditions and services remain unsupported.
