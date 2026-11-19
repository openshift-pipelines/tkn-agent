# Local verification specification

Status: draft with accepted command and review policy. Revision: 0.2, 2026-11-18.

This document describes the broader intended behavior. The implemented subset
and executable evidence are recorded in `first-slice.md` and `testing.md`.
Do not treat a scenario as fully covered merely because one variant is tested.

## Problem and outcome

A developer or coding agent edits an application and needs feedback from the
checks its CI actually uses before committing. Running a different local tool
or omitting important flags can miss the failure that CI will detect.

The first capability selects a useful, bounded scope from existing Tekton
definitions, adapts that scope explicitly to the current worktree, and executes
it locally. Success means trustworthy feedback for that declared scope.
It does not mean that the entire PipelineRun will pass.

## Requirements and proposals

Requested direction:

- A maintained `tkn-agent` project implemented in Go.
- A local verification command informed by the tkn-innerloop work.
- A human-owned specification that AI can help refine and implement.
- Observable acceptance cases and unit/end-to-end tests from the beginning.
- Deterministic execution and strong evaluations derived from real pipelines
  and a version-pinned Tekton feature inventory.

Accepted design decisions, 2026-11-18:

- Use `verify --local` for execution.
- Review every newly generated or changed plan before its first execution.
  Reuse an accepted unchanged plan without another prompt or model call.

Proposals requiring review:

- Reserve `validate` for static validation; do not add that separate command
  until its behavior is specified.
- Start with a reachable, existing Podman runtime. Installing Podman or managing
  its VM is outside the first implementation. Missing runtime access is blocked.
- Begin with an explicitly named PipelineRun/Pipeline file. Automatic pipeline
  selection and a Docker backend are later decisions.
- Scope the first implementation to an agreed corpus tranche, not all Tekton
  features. Report unsupported features rather than approximate them silently.

## Command contract

```sh
tkn-agent verify --local .tekton/checks.yaml
# Proposed future explicit-selection interface, not implemented in the first slice:
tkn-agent verify --local .tekton/checks.yaml --steps task/check
```

Both invocations have equivalent `tkn agent ...` plugin forms.
`--local` explicitly selects local execution. Omitting it must not silently
select a remote cluster; the initial command should require it.

The command discovers referenced definitions, determines the selected scope,
loads or generates a plan, validates it, obtains review when needed, executes
runnable checks, and reports the verdict with coverage. An explicit selection is mandatory scope: missing or
unsupported prerequisites block that selection before execution.

Automatic scope may exclude publishing, deployment, signing, and irrelevant
release work. It must distinguish intentional exclusions from unresolved
requirements. "Can run locally" and "useful in the edit loop" are separate
questions. No generic command-name heuristic proves latency or safety.

A new or changed plan must show its selected scope, images, commands, adaptations,
prerequisites, and execution context for review before running. Acceptance applies
to that plan and its recorded planning inputs. A change to effective commands,
scope, image identity, dependencies, or execution policy requires fresh review.
Application edits alone do not require review again.

In noninteractive use, a plan requiring review must produce a blocked outcome
without executing checks. An accepted unchanged plan runs without another prompt
or model call. The exact review UI, acceptance storage, machine-readable report
syntax, plan persistence, and refresh controls remain interface decisions.

## Behavioral guarantees

1. **Source fidelity.** Retain source image references and validation commands,
   flags, environment, working directories, and required setup. Record every
   change necessary for local context. Never replace an unavailable CI scanner
   with a different scanner and call the result equivalent.
2. **Local source.** Check the current worktree, including staged, unstaged, and
   relevant untracked files. Do not substitute a remote checkout of HEAD.
   Document differences from CI's event-based file selection.
3. **Explicit requirements.** Resolve required parameters, results, references,
   mounts, and services, or report what is missing. A hint cannot satisfy a
   missing prerequisite. Do not invent remote Task contents.
4. **Repeatable execution.** Reuse a validated plan while its definitions and
   recorded planning inputs match. Relevant helper/configuration changes
   invalidate it. Application edits alone permit reuse. Record resolved image
   identity and acknowledge mutable images and network installs.
5. **Bounded lifecycle.** Apply deadlines and cancellation. Remove only
   invocation-owned containers and temporary artifacts, including after failure.
   Keep explicit persistent tool caches separate. Do not modify unrelated
   containers, volumes, clusters, or caches.
6. **Controlled effects.** No cluster access, credential fetching, publication,
   or deployment is implied by local verification. Repository scripts and model
   output are untrusted executable content. The execution policy must state
   worktree writes, mounts, environment forwarding, and network access.
7. **Honest results.** Report selected, excluded, unresolved, executed, passed,
   failed, and intentionally tolerated failures separately. A missing execution
   receipt cannot count as a passed check. Preserve declared `onError` behavior
   only when supported and expose it in the report.
8. **No model in the warm loop.** An unchanged saved plan can run without pi,
   provider access, or any model request.

Proposed exit outcomes: `0` means the selected scope completed successfully;
`1` means an executed check failed; `2` means no complete successful verdict was
possible because of usage, prerequisites, unsupported behavior, or execution
infrastructure. A structured report must distinguish these reasons and preserve
partial results. Exact exit precedence is an open decision.

## Acceptance scenarios

Each scenario states an observable result, not an implementation structure.
The test strategy defines its strongest verification boundary.

| ID | Given / when | Required observation |
| --- | --- | --- |
| LOCAL-001 | A supported fixture has a clean worktree; local verification runs. | Every selected check has an execution receipt and expected success; exclusions are visible. |
| LOCAL-002 | A fixture contains a defect rejected by a selected CI check. | That check actually executes and fails; the report identifies it and the command exits unsuccessfully. |
| LOCAL-003 | The same defect is staged, unstaged, or untracked in separate runs. | Each applicable source state is checked; a clean remote revision cannot hide it. |
| LOCAL-004 | An explicit selection omits a required producer/setup step. | Verification reports the dependency and runs no checks from that invalid selection. |
| LOCAL-005 | A required reference, service, parameter, or secret is unavailable. | The selection is blocked with a concrete reason; the tool invents no value or success. |
| LOCAL-006 | Source code changes while planning inputs stay unchanged. | The plan is reused and no model call occurs; the changed source is executed. |
| LOCAL-007 | A recorded helper, configuration, or definition changes. | The stale plan cannot execute without regeneration and validation. |
| LOCAL-008 | The model returns an altered image, undeclared edit, or missing selected step. | Deterministic validation rejects the plan before executing it. |
| LOCAL-009 | Execution times out, is cancelled, or its container cannot start. | The outcome is unsuccessful, actual coverage is retained, and invocation-owned resources are cleaned up. |
| LOCAL-010 | A publishing/deployment step accompanies useful local checks. | It is visibly excluded; no publishing/deployment action is executed. Arbitrary-script safety is not inferred from this fixture. |
| LOCAL-011 | The runtime is absent or unreachable. | Verification reports a blocked prerequisite; there is no fallback to host execution or another cluster. |
| LOCAL-012 | A required check or mutation is missing from an evaluation run. | The evaluation is incomplete/unsuccessful even if all remaining executed checks passed. |
| LOCAL-013 | A supported result dependency or matrix is selected. | Consumers observe the actual producer result and all requested matrix values execute; otherwise the scope is explicitly unsupported. |
| LOCAL-014 | A new or changed plan has not been reviewed; verification is noninteractive. | No check executes and the outcome identifies required review. After acceptance, the unchanged plan runs without another prompt or model call; effective-plan changes require review again. |

## First implementation boundary

Choose a pinned PaC check fixture with a clean control and a seeded failure.
Implement one path end to end: discovery, plan validation, required review,
execution, and report.
Deterministic acceptance tests use a fixed planner response; live ADK generation
is a separately measured evaluation of the same contract.

Extend to dirty-source variants, plan reuse/invalidation, and lifecycle cases.
Then assess one Konflux validator and one downstream wrapper family. Keep cases
that are correctly blocked in the corpus; they document useful boundaries.

Do not claim full prototype parity or general Tekton compatibility from this
slice. Coverage and timing statements require stored execution evidence.

## Open decisions

- Specify review UI/acceptance storage, exit outcomes, and report interface
  before treating the CLI interface as stable. Command naming and review of
  new/changed plans are accepted above.
- The first slice uses Google ADK Go, explicit provider API credentials, and
  Go 1.26.10. Broader provider-quality guarantees and the project license remain
  decisions; the deterministic recorded-provider suite is not a live-model eval.
- Set execution policy for worktree writes, network, and secret forwarding.
  Unattended runs must honor the accepted-plan requirement. Read-only planner
  tools do not sandbox the executor.
- Agree first fixture/image pins and a warm feedback-time budget after measuring
  the baseline. Include cold setup and cleanup in separate measurements.
- Decide when the initial runtime scope should grow beyond existing Podman.

## Explicit exclusions

This specification does not authorize changing outer-loop CI requirements,
replacing Kubernetes/Tekton semantics generally, fetching cluster credentials,
or creating deployment, signing, release, MCP, or RAG capabilities.
Static YAML validity alone is not an executed local verification.

## Separate diagnostic capability

The requested `workup` capability is specified separately in
[`workup.md`](workup.md), with its own WORKUP scenarios and explicit cluster,
skills, MCP, and execution policy. That proposal is included for review with its
PoC. It does not broaden `verify --local` or authorize cluster access from local
verification.
