# Diagnostic workup PoC

This example tests the agent-to-Tekton direction: a goal enters `tkn-agent`,
the ADK loop obtains MCP evidence and chooses registered diagnostic Pipelines,
and real PipelineRun results return to the model. It is separate from local
`verify --local` and requires no Podman.

## Prerequisites and scope

- An explicit kubeconfig and context for a test cluster with Tekton v1 APIs.
- Existing `kubectl`, Go as pinned by `go.mod`, and an explicit provider/model.
- An authenticated, reachable evidence MCP server exposing the selected
  `get_kubernetes_resource` and `get_pipelinerun_logs` tools. Its Kubernetes identity must have read-only
  access to this fixture namespace.
- A reviewed workup configuration and the supplied example skill. The example
  is new diagnostic guidance, not a claim that an organization's existing
  skills, SOPs or knowledge integrations are present.

The model receives neither kubeconfig nor credential values. It can propose only
registered Pipeline names and constrained parameters. The client uses the
configured execution ServiceAccount for diagnostic Tasks. The fixture account
can read workload evidence; it cannot read Secrets or create workloads.

The launcher account can create PipelineRuns. Kubernetes RBAC alone cannot
restrict their spec fields; the reviewed config, deterministic runtime validation
and trusted launcher process form an additional boundary. A trusted operator
must install the reviewed Pipelines and control who can edit their definitions.

## Prepare a fresh fixture namespace

```sh
make build
export KUBECONFIG=/absolute/path/to/test-kubeconfig
export KUBE_CONTEXT=explicit-test-context
export WORKUP_NAMESPACE=workup-poc-unique
# Optional: grant an existing evidence MCP identity access to this new namespace.
export MCP_NAMESPACE=mcp-system
export MCP_SERVICE_ACCOUNT=evidence-reader
sh examples/workup/setup.sh
```

The script creates only the explicitly named new namespace and its fixtures; it
fails rather than adopting an existing namespace. Record the printed failed
target, UID and passing control. The failed run must terminate with exit 42
because `BUILD_TARGET` is empty. The control supplies a value and must succeed.

The diagnostic Pipelines examine actual run state and effective build input.
They emit bounded JSON as `evidence`. The input-comparison Pipeline also accepts
the passing control name and emits required `control-evidence` containing its
UID, input, actual status and bounded logs. Successful diagnostic execution does
not mean the inspected build succeeded.

## Supply skills, MCP configuration and provider credentials

Create a private working copy of `workup.yaml`. Set its skill path to the
absolute `examples/workup/skills/build-failure/SKILL.md` in your checkout.
Set the server URL and reviewed tools to match your evidence MCP deployment.
For local testing, forward its service to `127.0.0.1:18080`; the supplied HTTP
URL is intended only for that loopback forward. Use authenticated TLS for a
remote connection.

Provide `WORKUP_MCP_TOKEN` through your credential manager/environment. The
configuration contains the environment variable's name, never its value.
Configure `TKN_AGENT_PROVIDER`, `TKN_AGENT_MODEL` and that provider's credential
variables using the same provider contract as local verification.

For execution, supply a separate kubeconfig using the `workup-launcher`
ServiceAccount and explicit context. Its Role is installed by setup. An operator
can provision a short-lived token for that identity; the application never
requests or fetches it. Do not use a cluster-admin kubeconfig for the workup.

```sh
export WORKUP_CONFIG=/absolute/path/to/private/workup.yaml
export TKN_AGENT_PROVIDER=openai
export TKN_AGENT_MODEL=explicit-model-name
```

Do not check provider keys, bearer tokens, kubeconfig files or private endpoints
into the repository. Never forward those credentials to diagnostic Task steps.
Stdio MCP alternatives are documented in `docs/workup.md`; each subprocess
receives only its explicitly configured environment references.

## Review, execute and inspect

First run without `--execute`; this may obtain read-only MCP evidence and call
the model, but must not create diagnostic PipelineRuns:

```sh
bin/tkn-agent workup \
  "Investigate FAILED_TARGET, UID EXPECTED_UID, source kubernetes, namespace WORKUP_NAMESPACE. Establish where it failed and test whether build configuration explains it. State evidence gaps." \
  --config="$WORKUP_CONFIG" \
  --kubeconfig="$KUBECONFIG" --context="$KUBE_CONTEXT" \
  --namespace="$WORKUP_NAMESPACE" \
  --provider="$TKN_AGENT_PROVIDER" --model="$TKN_AGENT_MODEL" --format=json
```

Replace the uppercase placeholders with the actual names. Review the configured
catalog, parameter constraints, execution identity and limits. The next invocation
performs fresh model selection within that policy; it is not an exact replay of
the preceding model's proposed plan.

Include the setup's passing control name and UID in the goal and constrain the
config's `control.enum` to that control. A required comparison belongs in the
reviewed Pipeline's inputs/results; it cannot be satisfied by a model claiming
that the unperformed comparison is an out-of-scope limitation.

Authorize that bounded execution:

```sh
sh examples/workup/run.sh \
  "Investigate FAILED_TARGET, UID EXPECTED_UID, source kubernetes, namespace WORKUP_NAMESPACE. Establish where it failed and test whether build configuration explains it. State evidence gaps."
```

Inspect the report's MCP receipts, selected plans, actual native run names/UIDs,
required results, outcome and provider usage. Inspect the created PipelineRuns
with the native CLI as an independent execution record. The model should use
actual failed-step and build-input evidence; names and descriptions are not
evidence. Required missing evidence produces an incomplete result and exit 2;
explicitly out-of-scope capabilities are preserved separately as `limitations`.
The report cannot move a required log, SOP or knowledge gap into limitations to
claim success. It must not rerun or modify the failed target.

Run a separate workup against the printed passing control. It must not report
the missing-input defect for that control. Missing tools, denied reads, failed
diagnostics or absent required results must remain incomplete/failed evidence.

## Completion boundary

This proves a bounded configured runtime and test fixture, not production
incident readiness. Organizational knowledge/SOPs, incident and memory servers,
archive fallback, fleet reporting, scheduling, deployment ownership and retention
must be supplied and verified for their selected deployment. Do not equate this
example skill with the complete organizational investigation catalog.

Keep reports in access-controlled local/CI artifacts. Sanitize native logs and
endpoints before attaching evidence to a public PR. The example does not create
an additional report store or deploy an AgenticRun adapter.

The explicit live-model evaluation requires the built binary, reviewed config,
the provider/MCP credentials, the scoped kubeconfig and the setup's target/UID:

```sh
export WORKUP_KUBECONFIG="$KUBECONFIG"
export WORKUP_CONTEXT="$KUBE_CONTEXT"
export WORKUP_TARGET=the-printed-failed-target
export WORKUP_TARGET_UID=the-printed-failed-target-uid
export WORKUP_CONTROL=the-printed-passing-control
export WORKUP_CONTROL_UID=the-passing-control-uid-from-the-native-api
export WORKUP_EVIDENCE_DIR=/absolute/path/to/private/evidence
make workup-live
```

This suite fails on absent prerequisites. It independently checks native run
UIDs and results, both expected diagnostic executions, target identity, actual
resource and log MCP use and measured model usage. Its goal supplies the control
for read-only comparison; it checks the native control result's UID, successful
status, present input and configuration log. Diagnostic execution remains scoped
to the failed target, with comparison read inside the approved input-check
Pipeline. Tighten the config's `target.enum` to that failed target for this run.
It is separate from deterministic CI and
incurs provider requests.

For an authorization-boundary evaluation, select the `evidence-boundary` skill
instead and name an evidence namespace that this MCP identity cannot access.
Keep the execution namespace and catalog unchanged. The expected result is
`incomplete`, exit 2, an explicit denied-evidence gap and no diagnostic runs.
Do not grant more access to make that negative evaluation pass.

After inspecting the receipts, an operator can delete the dedicated namespace
created by this PoC. The setup and run scripts never delete a namespace or prune
unrelated resources automatically.
