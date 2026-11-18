# Local verification gaps and existing execution projects

Status: source-backed assessment, 2026-11-18. Podman-first execution is accepted.
Source inspection is distinguished from local runtime observations.

## Decision this assessment informs

Build the Go agent-facing contract for scoped verification, plan review,
source identity, and trustworthy results. Do not start by writing another
general Tekton execution engine. Existing runtimes supply substantial execution
machinery, but each leaves different translation and operational obligations.

Go packaging, ADK planning, and the execution backend are separate decisions.
Changing the language or model SDK does not establish Tekton equivalence.

The requested product behavior is pipeline-aware selection of smaller checks,
replayed through a pre-commit hook while larger work stays in cluster CI.
Dagger is a reference for agent design, not the selected execution backend.

## The actual prototype backlog

The public tkn-innerloop backlog contains the following 21 open issues at this
assessment. Parent issues and their children describe the same work; do not
count them as independent coverage improvements.

| Issues | Stated gap | What tkn-agent should retain or decide |
| --- | --- | --- |
| [1](https://github.com/openshift-pipelines/tkn-innerloop/issues/1) | Go port and direct Gemini/Claude/OpenAI planning | Preserve useful contracts; avoid an independent full port with divergent behavior. |
| [2](https://github.com/openshift-pipelines/tkn-innerloop/issues/2) | Freeze plan, CLI, cache contracts and inventory fixtures | Freeze invalid-plan cases and source provenance first. Explicitly document tkn-agent's different command/review contract. |
| [3](https://github.com/openshift-pipelines/tkn-innerloop/issues/3) | Go module, CLI, packaging and embedded skill | Ship a complete tested capability before adding unused package scaffolding. |
| [4](https://github.com/openshift-pipelines/tkn-innerloop/issues/4) | YAML loading and Tekton reference resolution | Test YAML edge cases and unresolved definitions; preserve relevant unknown context without trusting it. |
| [5](https://github.com/openshift-pipelines/tkn-innerloop/issues/5) | Inventory, selection, bundles, PaC/Konflux/cache bindings | Reuse the corpus and distinguish the current checkout from other repositories/artifacts. |
| [6](https://github.com/openshift-pipelines/tkn-innerloop/issues/6) | Plan validation and assembly | Model output remains untrusted; source-image/script/dependency/input checks belong in deterministic code. |
| [7](https://github.com/openshift-pipelines/tkn-innerloop/issues/7) | Runner, tool caches, failure continuation and cleanup | Preserve actual startup/failure evidence. A shell syntax check on the host does not establish image compatibility. |
| [8](https://github.com/openshift-pipelines/tkn-innerloop/issues/8) | Replace pi with an in-process model/tool loop | Bound repository tools and provider deadlines; keep provider/session details out of execution. |
| [9](https://github.com/openshift-pipelines/tkn-innerloop/issues/9) | Model/provider contract, prompt port and quality harness | Preserve accumulated prompt corrections and compare every trial against independent expectations. |
| [10](https://github.com/openshift-pipelines/tkn-innerloop/issues/10) | Documentation, release and final Python/pi cutover | Remove the old implementation only after behavior and distribution checks pass. |
| [11](https://github.com/openshift-pipelines/tkn-innerloop/issues/11) | Parent for remote definition support | Definition resolution is distinct from satisfying execution prerequisites. |
| [12](https://github.com/openshift-pipelines/tkn-innerloop/issues/12) | Git resolver | Pin resolved commits, validate paths and credentials, mirror fixtures offline. |
| [13](https://github.com/openshift-pipelines/tkn-innerloop/issues/13) | Hub resolvers and self/other clone distinction | Pin endpoints/catalog/version and resolved bytes. Never replace another repository's checkout with this one. |
| [14](https://github.com/openshift-pipelines/tkn-innerloop/issues/14) | PaC annotations without a tkn-pac dependency | Prefer upstream resolution machinery where practical; preserve provenance and unavailable-input reporting. |
| [15](https://github.com/openshift-pipelines/tkn-innerloop/issues/15) | Parent for faster/safer runs | Establish source isolation and correctness before parallel or selective execution. |
| [16](https://github.com/openshift-pipelines/tkn-innerloop/issues/16) | Independent task branches in parallel | Preserve within-task order, workspace sharing, per-step markers, SELinux behavior, cancellation and output attribution. |
| [17](https://github.com/openshift-pipelines/tkn-innerloop/issues/17) | Changed-file filtering | Include configuration, dependency closure, staged/unstaged/untracked/deleted/renamed paths. Zero selected checks is not a validation pass. |
| [18](https://github.com/openshift-pipelines/tkn-innerloop/issues/18) | Scratch workspaces, snapshots and ownership | Prefer isolated inputs and explicit output export; never clean a contributor's live checkout. |
| [19](https://github.com/openshift-pipelines/tkn-innerloop/issues/19) | ConfigMap and explicitly supplied Secret workspaces | Materialize safe read-only paths; keep Secret values out of planning. Arbitrary executable scripts can still leak values and need an execution policy. |
| [20](https://github.com/openshift-pipelines/tkn-innerloop/issues/20) | Python/Rust download caches | Cache appropriate download directories with ownership/isolation; never share credential-bearing Cargo home wholesale. |
| [21](https://github.com/openshift-pipelines/tkn-innerloop/issues/21) | Opt-in unprivileged service sidecars | Require readiness, network/lifecycle mapping and cleanup; distinguish workload privilege from engine privilege. |

Issue descriptions are proposed work, not proof of current support. For example,
several descriptions still count 17 corpus cases while the source inventory has
18. The port issue's exit-code summary also omits the automatic partial-result
exit in current `cli.py`. Freeze contracts from executable source and evidence,
then record deliberate differences rather than copying those discrepancies.

## What existing projects supply

| Approach | Useful existing mechanisms | Gaps that remain |
| --- | --- | --- |
| Direct Podman, current tkn-innerloop | Source-preserving plan edits, scope accounting, explicit env forwarding, result files, persistent Go/npm caches, container lifecycle. | Live checkout mutation, interpreter/platform mismatch, unresolved references/services, serial scheduling, semantic obligations for each adaptation. |
| Dagger | Go client, immutable filesystem values, content-addressed execution, mutable tool caches, typed outputs/artifacts, service bindings/readiness, context cancellation and a reusable engine. | No verified native Tekton consumer. Tekton guards/results/matrix/finally/onError/retries/workspaces still need adaptation or rejection. Service containers do not automatically share a Pod's localhost or lifecycle. |
| Native local Tekton through Kind | Actual Kubernetes workers and Tekton reconciliation, including their result/volume/service/scheduling semantics. Kind exposes a Podman provider. | Installation, version/capability inventory, source ingestion, explicit local bindings, isolation, credentials, cleanup, feedback-time measurement and reporting. |
| Pact | Upstream PaC matching/resolution, source upload into a PVC, namespaces and attributed logs on real Tekton. | Existing-cluster default; heuristic clone removal; skipped symlinks; source/preview identity and cancellation concerns. Not an unchanged safe local-verification dependency. |
| act | Local checkout recognition/copying, remote action resolution, service networks/readiness and resource cleanup. | Executes GitHub Actions, not Tekton. Translating through Actions adds another semantic layer; its own compatibility gaps remain relevant cautionary evidence. |

An execution cache and a package download cache are different. A cached success
is usable only for the effective inputs it represents. Mutable tool caches,
network responses, credentials and source-import refresh can change that claim.
Report cache hits separately from newly executed checks.

## Dagger-specific findings

The inspected stable release is
[`v0.21.12`](https://github.com/dagger/dagger/releases/tag/v0.21.12),
source commit `6ccf00943c1b4f065dea791b6d6ee956573fdabe`. Its ordinary
`dagger.io/dagger` Go client can be embedded in a CLI; creating a generated
Dagger module is not required. The Go client requires Go 1.26.1.

The client can bootstrap a checksummed CLI, which in turn starts/connects to an
engine. This is useful reuse, but not a claim that the Go binary alone supplies
its entire runtime. Pin and account for client, CLI, engine and workload images.
Current website/Context7 results also describe newer 1.0-beta interfaces; use
the selected release's source and actual CLI help when reproducing it.

The documented Podman path is rootful and the engine is privileged. This does
not mean every workload must be privileged. It is an operational/trust
requirement that a "works with existing Podman" promise must disclose.

`Host.Directory` can capture dirty/untracked filesystem contents, but repeated
imports in one session can retain a prior snapshot unless explicitly refreshed.
An unchanged accepted plan is reusable; a stale source snapshot is not.
Test mutation detection both across sessions and within one long-lived session.

Typed outputs and file exports help with artifacts, but Tekton result paths,
substitutions and failure propagation remain adapter contracts. Secret injection
and output scrubbing do not prevent arbitrary commands copying secrets into
artifacts or sending them over the network. Do not infer an egress-denial
guarantee without a verified mechanism.

### The useful agent idea

Dagger also has an agent API. At the pinned v0.21.12 source, `LLM.WithEnv` binds
the objects an agent can use, `Env.WithOutput` declares typed expected outputs,
and `LLM.WithBlockedFunction` removes tools. This is a useful pattern for
tkn-agent: provide bounded repository-reading tools and require a structured
selection over known pipeline step IDs. Do not give the selector arbitrary
execution or publication tools.

The reusable idea is constrained capabilities plus a checked output contract.
It does not require adopting the Dagger engine. A selected plan still needs
source validation, human review, actual local execution, and independent evals.
Newer main-branch agent/workspace design documents describe different APIs;
they are not assumed to be the stable release's behavior.

Sources:
[LLM](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/core/llm.go),
[Env](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/core/env.go),
[schema](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/core/schema/llm.go).

### Bounded local probe

A checksum-verified Darwin/arm64 v0.21.12 CLI connected to an isolated,
digest-pinned engine on the existing rootful Podman VM. A digest-pinned Alpine
check returned `aarch64` and `smoke-ok`. Client/server versions matched.
The probe's engine container and cache volume were then removed.

This proves basic local engine connectivity and container execution only.
No Dagger agent/model call, Tekton adaptation, service, cache invalidation,
source-refresh, or semantic-equivalence test was performed. The first CLI
attempt rejected a global `--no-mod` flag in the dynamic core command; the
successful probe used the release's core command directly. The workstation's
skopeo binary also had a missing shared-library dependency, so the engine image
was retrieved and inspected with native Podman instead.

These observations support using existing mechanisms where useful; they do not
justify changing the accepted Podman-first product direction.

## Prioritized experiments

| Priority | Independent case/oracle | Why it changes the design |
| --- | --- | --- |
| P0 | Same clean check and seeded defect; expected check must execute and change verdict. | Rejects false-green coverage and an evaluator that only validates generated JSON. |
| P0 | Dirty, staged and untracked source; mutate again in the same Dagger session. | Distinguishes plan reuse from snapshot reuse. |
| P0 | Valid host shell but absent image interpreter; unavailable architecture. | Reproduces an observed failure that any backend can inherit. |
| P0 | Source check requests another repo, revision or subdirectory. | Prevents a fast but incorrect checkout substitution. |
| P0 | Review acceptance, helper/config change, and unavailable model on replay. | Tests the accepted product contract without depending on a live model. |
| P1 | Service readiness, dependent check, cancellation and owned-resource cleanup. | Determines whether service primitives save meaningful custom work. |
| P1 | Producer result, skipped/failed producer, consumer and finally path. | Establishes actual Tekton semantic boundaries rather than generic DAG support. |
| P1 | Read-only ConfigMap and explicit Secret binding, with prohibited path and output cases. | Tests supported resource materialization and honest secret boundaries. |
| P1 | Empty versus warm/contaminated package caches; effective input changes. | Separates faster downloads from trustworthy execution-result reuse. |
| P2 | Parallel task branches with ordered steps and isolated markers/workspaces. | Avoids turning serial correctness bugs into concurrency bugs. |
| P2 | Conservative changed-file filtering and a zero-selected result. | Prevents optimization from silently removing validation coverage. |

Use the same input bytes, commands, images and supported scope across direct
execution, Dagger and a native Tekton baseline. Preserve each explicit
adaptation. Unsupported rejection counts as boundary coverage, not executable
coverage. Keep model/provider trials separate from backend replay.

Record generation, image retrieval, engine/cluster startup, source transfer,
execution and cleanup separately. No comparative timing or footprint conclusion
is established by source inspection.

## Toolchain and reuse gates

- Dagger v0.21.12's Go client requires Go 1.26.1. The inspected ADK requires
  Go 1.26.10; the current workstation default is Go 1.25.0.
- Pact's inspected module requires Go 1.27.3. Its project tree has no discovered
  project license. tkn-runner likewise has no discovered project license.
  These are gates before incorporating their source, not reasons to ignore
  their design and test fixtures.
- act is MIT-licensed and its inspected module requires Go 1.26.11.
- Reuse ADK for planning only after its provider/auth/tool contract is selected;
  do not infer execution determinism from the model SDK.

## Source pins and primary references

- tkn-innerloop: `c05878019243e611329b415954f7a769b21d1190`;
  [plan validation](https://github.com/openshift-pipelines/tkn-innerloop/blob/c05878019243e611329b415954f7a769b21d1190/src/tkn_innerloop/plan.py),
  [runner](https://github.com/openshift-pipelines/tkn-innerloop/blob/c05878019243e611329b415954f7a769b21d1190/src/tkn_innerloop/runner.py),
  [CLI](https://github.com/openshift-pipelines/tkn-innerloop/blob/c05878019243e611329b415954f7a769b21d1190/src/tkn_innerloop/cli.py).
- Dagger:
  [client/SDK distinction](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/docs/current_docs/getting-started/api/clients-sdk.mdx),
  [Podman](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/docs/current_docs/reference/container-runtimes/podman.mdx),
  [services](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/docs/current_docs/extending/modules/services.mdx),
  [source import tests](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/core/integration/host_test.go),
  [Go module](https://github.com/dagger/dagger/blob/6ccf00943c1b4f065dea791b6d6ee956573fdabe/sdk/go/go.mod).
- Pact: `b0378a2c39f1a54aaeb39388d4a93cfe18bf221d`;
  [source transfer](https://github.com/theakshaypant/pact/blob/b0378a2c39f1a54aaeb39388d4a93cfe18bf221d/pkg/runner/source.go),
  [mutation](https://github.com/theakshaypant/pact/blob/b0378a2c39f1a54aaeb39388d4a93cfe18bf221d/pkg/runner/mutate.go),
  [execution](https://github.com/theakshaypant/pact/blob/b0378a2c39f1a54aaeb39388d4a93cfe18bf221d/pkg/runner/runner.go).
- tkn-runner: `d5aaa8eb4ce158ce0f00661031dd2d68130ac604`;
  [native resource fixture](https://github.com/waveywaves/tkn-runner/tree/d5aaa8eb4ce158ce0f00661031dd2d68130ac604/examples/resource-poc).
- act: `ac4c09993f474fc8091608f2f7cf85a69dfd012f`;
  [runner](https://github.com/nektos/act/blob/ac4c09993f474fc8091608f2f7cf85a69dfd012f/pkg/runner/run_context.go),
  [compatibility boundaries](https://github.com/nektos/act-docs/blob/35cab6107d2afb3269a61dc5de5c15c151a4574f/src/not_supported.md).
- [Kind Podman provider](https://github.com/kubernetes-sigs/kind/blob/cda67ef8588f6bfdac5358233f2471cb2149ecaf/pkg/cluster/provider.go).
