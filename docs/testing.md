# Tests derived from the specification

Status: the first Go workflow has executable unit and real-Podman acceptance tests.
Broader compatibility and live-model evaluations remain planned.

The bounded real case is now driven by `make real-e2e`. It clones the pinned
public pipeline-service source into a private directory and runs its original
image and validators through the actual CLI. Set `TKN_AGENT_REAL_SOURCE` to an
existing checkout to use it as a clone source. The suite verifies review gating,
shared persistence, safe source-link handling, both seeded defects, repaired
controls, provider-free replay and inspected-helper invalidation.

Plan boundary tests cover shared analysis without inherited local approval,
tampered execution, self-input refusal, optional file appearance and intervening
writer conflicts. A fresh local CLI context also reuses shared analysis without
provider access while requiring approval. Image-store protocol units distinguish
Always, IfNotPresent and Never without claiming mock container execution.

Regression proof deliberately disabled the foreign-repository, source-fidelity
and absent-input guards. The corresponding tests failed for the intended
reasons; the guards were restored.

## Runnable checks

`make build` builds the actual CLI. `make fmt-check lint test e2e` runs formatting,
the checked-in golangci-lint configuration, race-enabled tests, and actual Podman
execution. E2E tests require an existing working Podman connection and network
access for the pinned fixture image. Runtime failures fail the suite.

`test/e2e/verify_test.go` owns LOCAL-001/002/006/010/014 for the mixed-check fixture:
recorded-provider selection, no execution before review, real accepted execution,
an untracked defect through the hook without credentials or another model call,
and renewed review after a definition change. It also covers missing image
interpreters and deadline cleanup under LOCAL-009. This is one bounded fixture,
not completion of every variant in those scenarios.

`internal/plan` covers unknown steps, command replacement, heavy-work rejection,
and definition/helper cache invalidation (LOCAL-007/008).
The heavy-operation guard was deliberately removed locally: its regression test
failed on the forbidden image-build selection, then the guard was restored.
`internal/source` covers explicit unsupported prerequisites; `internal/agent`
covers recorded provider/tool responses, input limits and credential/path boundaries.

Provider-live selection quality, native Tekton equivalence, staged-source variants,
general feature-matrix coverage, and remote CI results are not established by
these tests.

## Start from the user-visible contract

For an accepted behavior, first write a failing test at the strongest useful
boundary. Use an independent expected outcome, not output copied from the
implementation. Add focused unit tests for rules that need more precise input
coverage; do not replay every unit assertion through every layer.

The first acceptance pair is LOCAL-001/002: a real command checks a clean local
fixture, then rejects a seeded defect in the same selected check. Use the same
plan and source image so the defect, rather than another environment, explains
the changed outcome. The fix must make that same fixture pass again.

## Layers

| Layer | What it establishes | Initial scenarios |
| --- | --- | --- |
| Unit | Plan decoding, exact edit validation, complete scope accounting, input invalidation, outcome aggregation. No model or container daemon. | LOCAL-004/007/008/012 |
| Integration | Filesystem/worktree input behavior, deterministic planner boundary, result files, process cancellation and cleanup. | LOCAL-003/005/006/009/013 |
| CLI end to end | Build and invoke the actual binary with fixed planner output and real containers; assert exit status, required plan review, report, check receipts, and cleanup. | LOCAL-001/002/009/010/011/014 |
| Model evaluation | A pinned provider/model generates plans for a pinned corpus; measure correct decisions, required execution, mutation detection, time, and cost. | LOCAL-005/008/012 plus corpus-specific assertions |
| CI comparison | Run equivalent clean/broken fixtures through original Tekton checks and compare the intended verdict and scope with local results. | LOCAL-001/002 and each claimed compatibility behavior |

Each scenario has one primary test owner. Additional layers cover different
contracts, not duplicate internal plumbing. Keep normal Go tests beside their
implementation; keep fixtures with pinned origins and expected outcomes.

## Required evidence

An execution report should identify the spec revision, implementation revision,
fixture revision, source state, plan inputs, runtime/platform, image identities,
selected scope, actual receipts, exclusions, unresolved requirements, exit
status, cleanup outcome, and timing phases. Never include credential values.

Keep runtime reports in CI artifacts or ignored local output, not pass/fail
columns maintained by hand in this document.

Separate generation time, image retrieval, cold run, warm reruns, and teardown.
Report host architecture and cache conditions. Compare equivalent selected
checks separately from total outer-loop queue and runtime.

## Evaluations must not shrink their denominator

Define required checks and planted defects before running the model. A missing
plan step, blocked execution, skipped mutation, or absent execution report is
not a successful experiment. Show expected, executed, detected, blocked, and
skipped counts separately. Never report caught/executed alone as total coverage.

A deliberately unsupported case passes its boundary assertion only when it
reports the expected unsupported outcome without executing prohibited work.
It does not count as successful executable coverage.

## Determinism and isolation

Use fixed planner responses for ordinary CI. Live provider tests must be an
explicit suite with its own credentials and budget; they cannot replace
deterministic regression tests.

Use private temporary worktrees and invocation-owned resources. Never plant
defects in a contributor's checkout, clean unrelated Git changes, or prune
global runtime state. Preserve licensing and source references for corpus data.

Do not claim a mocked process validates actual Podman behavior. A real-container
suite must fail its required job when the runtime is unavailable; a deliberate
unit-only run must clearly state that container coverage was not requested.

## Introducing runnable checks

With the first Go behavior, add real build, formatting, golangci-lint, and
race-enabled test commands, then a required end-to-end job with Podman provisioned.
Use Go's standard testing package and existing tooling before adding a test
framework or a new test-generation service.

Before accepting a regression test, introduce the specific credible defect at
its implementation owner, observe the intended failure, and restore the code.
Do not weaken a test or change the spec just to make implementation output pass.
