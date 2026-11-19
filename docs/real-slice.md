# Bounded real-pipeline verification contract

Status: implemented bounded scope, 2026-11-18. The real CLI acceptance suite
passed clean, seeded-defect, repaired, warm replay and helper invalidation cases.

- `--bind-checkout TASK` explicitly substitutes current same-repository worktree
  materialization for that reviewed checkout Task. Verify repository identity,
  supported revision semantics, output workspace and safe subdirectory. Record
  it as provided, never as an executed Task or a validation pass. No name-based
  automatic skipping, remote Task execution or ambient-cluster access.
- Repeated `--param NAME=VALUE` binds declared string parameters. Preserve task
  parameter scope; reject secret overrides, unknown names and unsupported values.
- Preserve workspace aliases and the checkout subdirectory. Only dependencies
  on the explicit provided checkout are supported. Conditions, services, results,
  matrices, retries and other task dependencies remain unsupported.
- Preserve safe relative symlinks to included source files. Inspect safe file
  links with both link/target fingerprints; reject escaping, directory and
  credential-path links. Source Git metadata is not copied into execution.
- Preserve source image pull policy and pin its effective identity before review.
  Selected steps retain original validation scripts and flags. Within a selected
  task, earlier steps cannot be omitted.
- Store the approved shared `plan.Plan` under the source CI directory, mirroring
  the relative pipeline filename with `.plan.json`: for example
  `.tekton/.tkn-agent/test-pipeline-service-static-code-analysis.yaml.plan.json`.
  Use `.konflux/.tkn-agent` for Konflux sources and `.tkn-agent` otherwise.
  Git provides history; no append-only database is needed.
- Local pending candidates and approval remain in the user cache. A shared plan
  never transfers execution approval. A fresh context can reuse it without a model
  call, request local review, then execute. Explicit approval is the only operation
  that creates or updates the shared file; normal replay leaves it unchanged.
- Revalidate source execution, parameters, binding and policy before replay.
  Application edits get a fresh snapshot without replanning. Inspected helper or
  configuration changes, including previously inspected absent files appearing,
  invalidate planning. Reserved state files cannot become their own inputs.
- Reject tampered source execution and unknown shared fields. Refuse to overwrite
  a shared plan changed by another writer after planning. `--refresh` regenerates
  rather than requiring a valid old cache.

Primary acceptance owners: the actual CLI real-repository suite for
LOCAL-001/002/006/007/014-018, and plan/reader boundary units for tampering,
optional input presence and state self-reference. Recorded HTTP provider output
keeps native ADK wiring deterministic. Live selection quality remains a separate
credentialed evaluation. No commit, push or automatic Git approval is implied.
