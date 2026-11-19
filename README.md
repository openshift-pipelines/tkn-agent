# tkn-agent

A Tekton CLI plugin for coding-agent workflows.

The first capability brings selected checks from a repository's
existing Tekton pipelines into the local edit loop. It preserves the
organization's validation commands, explains necessary adaptations, and reports
exactly what was checked. Cluster CI remains authoritative.

## Current state

The Go CLI implements a deliberately small workflow: select supported independent
inline checks, review their exact plan, and replay it through existing Podman.
Explicit reviewed self-checkout bindings support the pinned real pipeline case.
General remote execution and Kubernetes-dependent execution remain unsupported.

Prerequisites: Go 1.26.10 or automatic Go toolchain selection, Git, reachable
Podman, and an explicitly configured model for first-time planning.

```sh
make build
go install ./cmd/tkn-agent
```

Configure the planner provider and model using `TKN_AGENT_PROVIDER` and
`TKN_AGENT_MODEL`, plus the provider's API credentials. Then run:

```sh
tkn-agent verify --local .tekton/checks.yaml
# Inspect the emitted plan, then accept its exact digest:
tkn-agent verify --local .tekton/checks.yaml --approve-plan sha256:...
# Equivalent invocation when installed as a Tekton CLI plugin:
tkn agent verify --local .tekton/checks.yaml
```

Google ADK Go supplies the bounded planning loop. Provider options are
`openai`, `gemini`, and `anthropic`; the default provider is `openai`, and the
model name is required when generating a plan. The planner can only read bounded,
repository-relative helper files. It cannot execute commands.
OpenAI uses `OPENAI_API_KEY`; Gemini uses `GOOGLE_API_KEY` or `GEMINI_API_KEY`;
Anthropic uses `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN`.
New or changed plans require review before execution. Repeated execution of an
accepted unchanged plan requires neither another prompt nor a model call.

Shared plans live beside their source definitions: a `.tekton/ci.yaml` plan is
saved as `.tekton/.tkn-agent/ci.yaml.plan.json`. `.konflux` sources use the
corresponding `.konflux/.tkn-agent` directory. Other sources use `.tkn-agent`.
Git provides the shared plan's history. Pending candidates and local approval
live under `$XDG_CACHE_HOME/tkn-agent/plans` or the OS user cache directory.
`--refresh` requests fresh analysis. Definition or inspected-helper changes,
resolved image changes, and timeout changes require review again. Execution
always uses a fresh snapshot of tracked and non-ignored untracked source files.
Git metadata and submodules remain outside the snapshot capability. Safe relative
links to included repository files are preserved; escaping and directory links
are rejected. Inspected links and their targets both participate in invalidation.
Checks use the image's interpreter, a writable private workspace, bridge networking,
and declared literal environment; host provider credentials and proxy variables
are not forwarded. Cluster CI remains required.

Exit codes: `0` for completed successful selected scope, `1` for a failed check,
and `2` for review-required, unavailable, unsupported, or incomplete execution.
Use `--format=json` for check receipts, scope counts, and source/plan digests.

For a Git hook, set `TKN_AGENT_PIPELINE` and invoke `sh examples/pre-commit`
from your existing hook. `.pre-commit-hooks.yaml` also exposes a system hook;
configure its pipeline argument and install the binary separately. A warm hook
needs neither a model name nor provider credentials.

For an explicitly reviewed source checkout, use:

```sh
tkn-agent verify --local .tekton/test-pipeline-service-static-code-analysis.yaml \
  --bind-checkout fetch-repository --param target_branch=main
```

The binding validates the GitHub repository identity, current-event revision
contract, output workspace and safe subdirectory. It is reported as provided,
not executed or validated. String parameters remain scoped to their Tasks.
Source image pull policy is preserved. Other dependencies remain unsupported.

A shared plan never grants execution approval to another user or context.
It can be loaded without provider access, but needs one local digest approval.
Only explicit approval writes shared state; normal replay leaves it unchanged.
Reserved state cannot become its own planning input. Recorded optional input
absence is invalidated when the configuration appears.

Run `make fmt-check lint test e2e` to check formatting, golangci-lint, race-enabled
unit tests, and real Podman acceptance tests. The latter use a local recorded
provider response, not a live paid model.
`make real-e2e` also drives the pinned pipeline-service clean/defect/repair
workflow through the actual CLI and original image. It needs GitHub/registry
access and linux/amd64 execution support. `TKN_AGENT_REAL_SOURCE` can point to an
existing checkout as a clone source; tests mutate only a fresh private clone.

## Start here

- [Behavior specification](docs/spec.md): scope, guarantees, command contract,
  acceptance scenarios, and open decisions.
- [First Go workflow](docs/first-slice.md): automatic selection of smaller checks,
  Podman execution, plan review/replay, and pre-commit behavior.
- [Real repository assessment](docs/repository-evaluation.md): the pinned
  OpenShift Pipelines inventory, actual validator experiments, and CLI gaps.
- [Plan history](docs/plan-state.md): repository-tracked plans, Git history,
  separate per-user acceptance and explicit updates.
- [Real workflow contract](docs/real-slice.md): the bounded supported checkout,
  parameter, workspace and shared-plan behavior.
- [Test strategy](docs/testing.md): how scenarios become unit, integration,
  end-to-end, and model evaluation checks.
- [Determinism and evaluations](docs/evaluations.md): immutable execution inputs,
  independent expected outcomes, and coverage accounting.
- [Tekton feature matrix](docs/tekton-feature-matrix.md): the pinned feature
  inventory, flag surface, and proposed evaluation oracles.
- [Gap analysis and existing projects](docs/gap-analysis.md): the upstream
  backlog, Dagger/native-Tekton alternatives, and experiments before choosing
  an execution backend.
- [Specification workflow](docs/spec-workflow.md): how humans and AI revise the
  specification and implement one verified behavior at a time.
- [Agent guidance](AGENTS.md): instructions for contributors and coding agents.

The existing `openshift-pipelines/tkn-innerloop` project supplies reference
behavior and a pipeline corpus. Reuse its findings and preserve attribution
when adapting implementation or fixtures; do not assume every prototype
behavior is part of this project's contract.
