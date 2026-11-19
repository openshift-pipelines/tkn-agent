# Workup runtime

Status: draft for the separately reviewed workup feature, 2026-11-18.
This is a native diagnostic runtime, separate from local verification.
The operator supplies reviewed skills, MCP grants and diagnostic Pipelines.
The model chooses checks and consumes actual evidence; deterministic Go code
owns authorization, execution and accounting. No remediation is supported.

```go
func Run(ctx context.Context, opts Options) (Report, error)
type Options struct {
    ConfigPath, Goal, Provider, Model, Kubeconfig, Context, Namespace string
    Execute bool
    Timeout time.Duration
}
```

All cluster selectors are mandatory, including during review. Credentials are
never fetched. `kubectl` must already exist on PATH. Config is trusted input:
strict YAML or JSON, one document, unknown fields and duplicate keys rejected.
Paths are absolute. No implicit environment substitution occurs.

```yaml
version: 1
skills:
  - path: /reviewed/diagnosis/SKILL.md
    required_tools: [metrics_query]
mcp_servers:
  - name: metrics
    transport: streamable_http
    url: https://metrics.example/mcp
    bearer_token_env: METRICS_TOKEN
    tools: [query]
  # stdio alternative:
  # - name: metrics
  #   transport: stdio
  #   command: /reviewed/bin/metrics-mcp
  #   args: []
  #   env: {METRICS_TOKEN: METRICS_TOKEN}
  #   tools: [query]
pipelines:
  - name: inspect-workload
    params:
      target:
        type: string
        required: true
        enum: [sample]
        pattern: '^[a-z]+$'
        max_length: 64
    required_results: [diagnosis]
execution:
  service_account: diagnostic-reader
limits:
  max_model_calls: 8
  max_tool_calls: 24
  max_runs: 3
  timeout_seconds: 120
  max_evidence_bytes: 65536
```

Every listed skill, server and tool is required. Skill `required_tools` and
frontmatter `allowed-tools` are requirements, never grants: they must be in the
explicit operator MCP grants or the built-in `inspect_pipeline` tool.
Exposed MCP names are `server_tool`. Selected skills are snapshotted in memory;
only their SKILL.md files are available, no arbitrary resource/script access.
Their content and MCP descriptions/results are untrusted evidence.

The only model execution input is `inspect_pipeline` with `pipeline` and
`params`. Parameter types are Tekton string, array of strings or object with
string values. Operator constraints apply to every scalar; `max_length` also
bounds collection cardinality. All declared Pipeline parameters must have an
operator rule. Unknown parameters, missing required parameters and unsupported
parameter types fail closed. Required Pipeline defaults are materialized and
validated. An operator rule with `required: true` requires an explicit model
argument even when the Pipeline has a default. Configured required results
must be declared by the Pipeline.

The runtime fetches each approved Pipeline before the model starts and embeds
that exact spec in each created PipelineRun. The bounded model inventory
includes those actual specs, their digests, operator parameter policy and
`metadata.annotations.description`; names alone do not describe a procedure.
Reviewed referenced Tasks remain
the cluster's responsibility; this does not pin transitive Task references.
The execution ServiceAccount is always supplied by trusted config.
Review is the default: valid proposed checks are retained without creating runs.
`Execute` authorizes only the configured catalog and limits. Identical
Pipeline/parameter calls reuse their invocation receipt, never create twice.
The model may choose a different check after reading actual prior results.

Reports encode `state`, `reason`, `conclusion`, `missing_evidence`, `limitations`,
`config_digest`, `skill_digests`, `mcp_calls`, `plans`, `runs`, and `usage`.
MCP receipts contain server/tool, argument/result digests, success and error;
they contain no raw arguments or credentials. Plans contain Pipeline,
parameters and pinned spec digest. Runs add name, UID, native reason/status,
actual results, missing results, outcome and cancellation receipt.
Usage includes provider/model, model/tool calls and input/output/total tokens.
State is `completed`, `review-required`, `incomplete`, `failed` or `cancelled`.
`message` is CLI-readable text; `reason` explains non-completion.
The CLI exits 0 only for completed execution, and 2 for review, incomplete,
failed, cancelled or invalid invocations. A diagnosed workload failure can
still be a completed workup; a failed diagnostic procedure cannot.
Conclusion is model analysis, never an authoritative workload-health verdict.
Execution success alone cannot establish workload health. Missing results or
failed native runs keep reports unsuccessful regardless of model prose.
Errors return partial reports; successful earlier receipts are never discarded.

The final model response must be strict JSON
`{"conclusion":"...","missing_evidence":[],"limitations":[]}`. Missing or malformed evidence
accounting fails closed. Model-reported missing evidence makes the report
incomplete. MCP `success` means protocol completion only: Pharos can return
`Kubernetes API Error: 403 - Forbidden` with MCP `isError:false`. The runtime
marks that text as unavailable evidence, feeds `evidence_available:false` into
the next model request and keeps the report incomplete even if the model
later claims no evidence is missing. Other connectors' semantic errors still
require interpretation; the runtime does not claim universal error recognition.

`limitations` records capabilities explicitly outside the selected goal/skill
scope, such as production fleet history for a controlled fixture evaluation.
It never satisfies or replaces required evidence. A missing required SOP,
knowledge source, log or control comparison remains in `missing_evidence`.

Limits are mandatory positive values, with hard ceilings: 32 model calls,
128 tool calls, 16 runs, 900 seconds and 262144 evidence bytes. Config is
limited to 32 skills, 16 servers and 32 Pipelines. Context,
native output, skills, tool arguments/results and final text are bounded.
HTTP MCP wire responses have a separate 1 MiB ceiling: tool discovery includes
the server's whole catalog before filtering. Only granted tool schemas enter
the model context, which retains the configured evidence limit.
Configured MCP and provider credential values are redacted before model
requests and report/error returns. HTTP redirects are rejected; each request
has a timeout. Stdio receives only explicitly mapped environment references.
Native kubectl receives only the explicit kubeconfig/context/namespace and a
minimal executable environment. Its HOME is an invocation-owned temporary
directory, removed after execution/cancellation, so discovery caches neither use
the host credential home nor pollute the caller's worktree. Kubeconfig exec
plugins remain trusted operator configuration. Cancellation only patches invocation-owned unfinished runs
using UID and resourceVersion tests; failed and completed run evidence is
preserved. A concurrent terminal status update wins over cancellation.
The SDK's stdio frame ceiling is 16 MiB; application evidence is bounded
before it enters prompts/reports. Model-call counts are ADK calls; token
counts are provider-reported and do not claim separate provider retry counts.

| ID | Scenario | Observable result |
| --- | --- | --- |
| WORKUP-001 | A model reads evidence then chooses another check. | Real SDK tool dispatch supplies first results to the next model request. |
| WORKUP-002 | Execute is absent. | Plans are retained; zero PipelineRuns are created. |
| WORKUP-003 | Required skill/server/tool/config is missing. | Failure precedes any model call or creation. |
| WORKUP-004 | Model proposes an unknown parameter, type or forbidden value. | Deterministic rejection with zero creates. |
| WORKUP-005 | Native success omits a required result or a run fails. | Actual status and missing evidence survive; no healthy verdict is inferred. |
| WORKUP-006 | Invocation is cancelled with a running diagnostic. | Only owned unfinished UID is cancelled; receipt and original error remain. |
| WORKUP-007 | Identical execution is requested twice. | One native run; both calls observe its existing receipt. |
| WORKUP-008 | Evidence includes credentials or exceeds limits. | Redaction or explicit bounded failure before transmission; partial receipts survive. |
| WORKUP-009 | HTTP redirects or stdio attempts ambient secret access. | Redirect denied; unrelated environment absent. |
| WORKUP-010 | Pipeline changes after discovery. | Created run contains the originally fetched spec and trusted SA. |

Deterministic tests use real ADK and local provider/MCP wire responses plus a
native-client executable shim. They do not prove cluster behavior or live model
quality. The CLI, actual diagnostic Pipelines and explicit live-model evaluation
are exercised separately by the [workup PoC](../examples/workup/README.md).
External organizational integration and production acceptance remain unverified.
