# Tekton feature inventory and proposed evaluation matrix

Status: source-backed inventory and proposed oracles; no conformance claim.

Baseline: `tektoncd/pipeline@2451916543955b4525bcafb09b3b3efde4cf0c6f`,
observed on 2026-11-18. This is a main-branch revision, not a release.
All upstream paths below refer to that exact commit. The machine-readable
inventory is `evals/feature-inventory.json`.

## What "every feature" means here

Account for every documented Pipelines feature family and its current controller
and resolver flag surface. Then enumerate concrete cases within each family and
important combinations. This is a bounded baseline, not every combination of
Kubernetes fields or a promise to run every feature locally.

Every feature must eventually have an explicit capability disposition:
supported, partially supported, explicitly rejected, or awaiting assessment.
The current inventory is awaiting assessment. Specifying an eval is not passing
it. A correctly rejected feature has boundary coverage, not execution coverage.

For each family, create a supported positive, a controlled negative, and an
unsupported-backend case where applicable. Keep admission errors, runtime
failures, and unsupported backend capabilities distinct.

## Feature families

The required observations below are starting oracles, not substitutes for
individual reviewed fixtures.

| ID | Features to enumerate | Positive / negative / unsupported observations | Upstream sources |
| --- | --- | --- | --- |
| API | Task, TaskRun, Pipeline, PipelineRun; inline/named definitions; defaults, validation, conversion | Equivalent effective specs; reject conflicting/missing references; identify unsupported resource/version. | `docs/api-spec.md`, `docs/pipeline-api.md`, `pkg/apis/pipeline/v1/*_types.go`, `config/300-crds/` |
| STEP | Ordered steps, command/args/entrypoint, scripts/shebangs, workingDir, env/envFrom, stepTemplate, volumes/devices, timeouts, error policies, exit-code references, step guards, output redirection | Exact argv/environment/files and dependency order; controlled exit/timeout and `stopAndFail`/`continue`/`continueAndFail`; reject unavailable interpreter, source, mount, or alpha capability. | `docs/tasks.md`, `docs/container-contract.md`, `pkg/apis/pipeline/v1/container_types.go` |
| PARAM | String/array/object parameters, properties, defaults/merging, names, propagation, enum constraints | Exact typed values and precedence; wrong types, duplicate names, missing keys, invalid enums; no invented reference-scope propagation. | `docs/tasks.md`, `docs/taskruns.md`, `docs/pipelines.md`, `docs/pipelineruns.md`, `pkg/apis/pipeline/v1/param_types.go` |
| SUBST | Dot/bracket notation, array expansion/indexing, object access, context identifiers, retries, permitted destination fields | Preserve argv boundaries and exact values; reject invalid destinations/references; explicitly handle shell metacharacters because substitution does not escape them. | `docs/variables.md`, `docs/tasks.md` |
| RESULT | Typed step/task/pipeline results, promotion, dependency inference, missing results, extraction and size limits | Exact values and consumer graph; reject malformed/oversize/missing values; no invented promotion or storage. | `docs/tasks.md`, `docs/pipelines.md`, `docs/stepactions.md`, `pkg/apis/pipeline/v1/result_types.go` |
| DAG | runAfter, result dependencies, fan-out/fan-in, task failure policies, retries | Expected dependency partial order and attempts; reject cycles/unknown edges/invalid retry-policy combinations. | `docs/pipelines.md`, `pkg/apis/pipeline/v1/pipeline_validation.go` |
| WHEN | in/notin, conjunction, parameter/result/workspace guards, CEL | Truth tables and skipped dependents; invalid operators/empty values/nonboolean CEL; disabled feature is explicit. | `docs/pipelines.md`, `docs/tasks.md`, `pkg/apis/pipeline/v1/when_validation.go` |
| FINALLY | Success/failure finalization, results, task status/reason, aggregate status, guards, matrices | Expected final-task membership after success/failure/cancel; missing results; no assumed order among independent final tasks. | `docs/pipelines.md`, `docs/pipelines-in-pipelines.md` |
| MATRIX | Cartesian/include/include-only combinations, empty arrays, names, limits, retries, result aggregation | Exact combination set and per-combination behavior; invalid overlap/overflow; reject unsupported individual-result selection and aggregate array/object results. | `docs/matrix.md`, `pkg/apis/pipeline/v1/matrix_types.go`, `pkg/apis/pipeline/v1/pipeline_validation.go` |
| LIFE | Pending/start, retries, cancellation/stop, timeout hierarchy, child references/history, managedBy delegation | Transition predicates, retry history, cleanup/finally; invalid transitions/timeout relations; absent delegated controller cannot produce success. | `docs/taskruns.md`, `docs/pipelineruns.md`, corresponding v1 types/validation |
| WS | Bindings, optional/read-only workspaces, mountPath/subPath, mapping/propagation, isolation, PVC/templates/emptyDir/Secret/ConfigMap/projected/CSI sources | Exact visibility/persistence/isolation; missing/multiple/read-only binding failures; no cross-Task emptyDir persistence or fabricated CSI. | `docs/workspaces.md`, `pkg/apis/pipeline/v1/workspace_types.go` |
| SCHED | Affinity assistants, coschedule modes, topology/access modes, service-account precedence, PVC lifecycle | Effective affinity/ownership and cleanup; incompatible topology; local filesystem access is not scheduler/storage parity. | `docs/affinityassistants.md`, `docs/workspaces.md`, `docs/developers/affinity-assistant.md` |
| POD | Pod-template defaults/merge, per-task overrides, matrix substitution, scheduler/network/security fields | Effective PodSpec and name-keyed merge behavior; invalid/missing RuntimeClass; explicit unsupported host/network/scheduler capability. | `docs/podtemplates.md`, `docs/pipelineruns.md`, `pkg/apis/pipeline/pod/template.go` |
| RES | Step/task resources, runtime overrides, LimitRange, ResourceQuota, sidecar accounting, QoS, injected defaults | Requests/limits and admission behavior; controlled quota violations; local CPU limits are not Kubernetes accounting equivalence. | `docs/compute-resources.md`, `docs/additional-configs.md` |
| AUTH | Service accounts, Git/registry secrets, basic/SSH auth, repository selection, known_hosts, mounts/non-root access | Fixture endpoint sees intended identity; denied/missing auth; never fall back to ambient host credentials. | `docs/auth.md`, `docs/taskruns.md`, `docs/pipelineruns.md` |
| SIDE | Conventional/native/injected sidecars, readiness/probes/lifecycle, shutdown, resources, workspace visibility | Readiness gating, real service interaction, terminal state; failed readiness/OOM; explicit unsupported runtime/version/injection mechanism. | `docs/tasks.md`, `docs/taskruns.md`, `docs/additional-configs.md` |
| ACTION | Local/remote StepActions, parameter precedence, execution context, typed results/promotion | Effective step and exact results; prohibited inline fields/substitution destinations; no standalone StepAction execution claim. | `docs/stepactions.md`, `pkg/apis/pipeline/v1beta1/stepaction_types.go`, StepAction CRD |
| CUSTOM | Embedded/referenced custom tasks, params/workspaces/account, cancellation, timeouts/retries, status/results | Fixed custom controller fulfills its contract; controlled rejection/failure; missing controller is unsupported. | `docs/customruns.md`, `docs/pipelines.md`, CustomRun CRD |
| NEST | Inline/referenced child pipelines, params/workspaces, ownership, recursion, failure boundaries | Expected child and mapped inputs; recursion/missing bindings; separately reject unsupported child results/status/matrix and inherited timeout/retry assumptions. | `docs/pipelines-in-pipelines.md`, `pkg/apis/pipeline/v1/pipeline_validation.go` |
| REMOTE | ResolutionRequest, named/default/concise syntax, resource kinds, caches, auth, timeout/backoff, provenance | Fixed resolved bytes and effective resource; missing/denied/tampered content; no fabricated fallback. See resolver cases below. | `docs/resolution.md`, `docs/resolver-reference.md`, `pkg/apis/pipeline/v1/resolver_types.go` |
| TRUST | Signatures, VerificationPolicy patterns/authorities, enforce/warn, no-match policy | Signed fixture passes; tampering/wrong key fails; resolution alone is not verified provenance. | `docs/trusted-resources.md`, VerificationPolicy CRD |
| ART | Step/task artifact inputs/outputs, URI/digests/buildOutput, propagation/status | Exact artifact values; malformed/disabled-feature rejection; do not equate metadata with Chains attestations. | `docs/artifacts.md`, `pkg/apis/pipeline/v1/artifact_types.go`, `test/artifacts_test.go` |
| ENV | OS/architecture, Windows scripts/scheduling, hermetic execution, debug breakpoints, SPIRE boundaries | Supported platform output, network-denied step, breakpoint lifecycle; wrong platform/disabled field; no claimed sidecar hermeticity or unqualified SPIRE guarantee. | `docs/windows.md`, `docs/hermetic.md`, `docs/debug.md`, `docs/spire.md` |
| OBS | Metadata/display names, logs, Kubernetes events, CloudEvents, metrics/traces | Correct propagation and logical event payloads; controlled unavailable sink; no exactly-once/global-order assumptions. | `docs/labels.md`, `docs/logs.md`, `docs/events.md`, `docs/metrics.md`, `docs/developers/tracing.md` |
| OPS | Install/defaults, controller settings, HA/concurrency/backoff, cache transforms, namespace tenancy/build variants | Effective config and logical invariants across restart; invalid config/RBAC denial; no inferred FIPS certification or multi-version guarantee. | `docs/install.md`, `docs/additional-configs.md`, `docs/enabling-ha.md`, `docs/tekton-controller-flags.md`, developer tenancy/FIPS docs |
| LEGACY | Deprecated served versions, removed PipelineResources/Run/fields/flags, migration | Correct served-version boundaries/conversion; removed functionality rejected; stale examples cannot establish support. | `docs/deprecations.md`, migration docs, `config/300-crds/` |

## Resolver subfamilies

| ID | Required cases | Upstream sources |
| --- | --- | --- |
| REMOTE-GIT | Immutable commit/path, clone vs SCM API, provider/credential selection; conflicting URL/org/repo, wrong kind, auth denial. | `docs/git-resolver.md` |
| REMOTE-BUNDLE | Digest/name/kind, credentials, layer metadata/content; invalid media/annotations, missing resource. | `docs/bundle-resolver.md`, `docs/tekton-bundle-contracts.md` |
| REMOTE-CLUSTER | Namespace/name/kind, allow/block policy, RBAC; absent or forbidden object; no filesystem substitution. | `docs/cluster-resolver.md` |
| REMOTE-HTTP | Fixed HTTP(S) bytes, auth, SHA-256/SHA-512 verification; invalid scheme/digest/credential pair/timeout. | `docs/http-resolver.md` |
| REMOTE-HUB | Catalog/kind/version, endpoint override/fallback; missing catalog/version and invalid scheme; avoid live version selection in deterministic cases. | `docs/hub-resolver.md` |
| REMOTE-CACHE | always/never/auto, expiry/size, immutable vs mutable ref, namespace/credential isolation; controlled clock and endpoint hit/miss. | `docs/resolution.md`, resolver-specific docs |
| REMOTE-EXT | Fixed custom resolver validation/resolve/status contract; missing controller must not choose a substitute. | `docs/resolver-reference.md`, `docs/how-to-write-a-resolver.md` |

## Feature flags

The pinned controller ConfigMap contains 27 active entries and documents optional
`max-result-size`, for 28 keys to account for. The resolver ConfigMap contains
five keys. Defaults and explicit values are separate test inputs.

For each operative flag, test omitted/default, each permitted value, invalid
values according to its actual parser, and an unavailable backend capability.
Do not assume all parsers reject invalid input.

| Key | Shipped/default value | Oracle focus |
| --- | --- | --- |
| enable-api-fields | beta | stable/beta/alpha admission, independent flags |
| coschedule | workspaces | all modes, placement and PVC lifecycle |
| disable-creds-init | false | injected vs explicit-mount credentials |
| await-sidecar-readiness | true | step gating, including injected sidecars |
| running-in-environment-with-injected-sidecars | true | declared/injected sidecar combinations |
| require-git-ssh-secret-known-hosts | false | known_hosts requirement |
| enable-tekton-oci-bundles | false | compatibility key, not restored legacy fields |
| send-cloudevents-for-runs | true | deprecated CustomRun behavior |
| trusted-resources-verification-no-match-policy | ignore | ignore/warn/fail |
| enable-provenance-in-status | true | provenance presence, unchanged outputs |
| enforce-nonfalsifiability | none | none/spire, infrastructure qualification |
| results-from | termination-message | equivalent typed results via each extraction path |
| max-result-size | 4096, optional | sidecar extraction and actual size/parser boundaries |
| set-security-context | true | injected contexts, not user-container guarantees |
| set-security-context-read-only-root-filesystem | false | context interaction and writable mounts |
| keep-pod-on-cancel | false | retained/deleted Pod and stopped processes |
| enable-cel-in-whenexpression | false | enabled truth tables, disabled rejection |
| enable-artifacts | false | capture/substitution and disabled rejection |
| enable-param-enum | false | enum/default/subset validation |
| disable-inline-spec | empty string | each scope and combinations |
| enable-concise-resolver-syntax | false | explicit/concise equivalence and off boundary |
| enable-kubernetes-sidecar | false | lifecycle and Kubernetes capability |
| enable-step-actions | true | compatibility no-op; false does not disable stable feature |
| enable-wait-exponential-backoff | false | controlled failures and fake-clock schedule |
| enable-termination-message-compression | false | decoding/capacity, extraction-path interaction |
| surface-pod-events | false | current-Pod warnings and bounded event evidence |
| enable-informer-cache-transforms | true | stripped cached metadata, unchanged API object, restart |
| keep-status-spec-descriptions | false | snapshot descriptions, unchanged definitions |
| enable-bundles-resolver | true | enabled resolution and disabled boundary |
| enable-git-resolver | true | enabled resolution and disabled boundary |
| enable-hub-resolver | true | enabled resolution and disabled boundary |
| enable-cluster-resolver | true | enabled resolution and disabled boundary |
| enable-http-resolver | true | enabled resolution and disabled boundary |

Sources: `config/config-feature-flags.yaml`,
`config/resolvers/config-feature-flags.yaml`,
`pkg/apis/config/feature_flags.go`,
`pkg/apis/config/featureflags_validation.go`,
`pkg/apis/config/resolver/feature_flags.go`,
`internal/reconciler/cachetransform/setup.go`.

Defaults, events, observability, registry certificates, resolver configuration,
controller flags, and build tags are not all feature flags. Their family
coverage remains required.

## Required interactions

1. Results x producer failure x error continuation x missing values.
2. Retries x result reset/history x retry-count x cancellation.
3. Finally x failure/skip x missing results x cancellation/timeout modes.
4. Matrix x includes/empty arrays x retries x aggregation x taskRunSpecs.
5. Inline/reference scope x propagation x inline-spec policy.
6. PVC/subPath x multi-task access x topology/coschedule x cleanup.
7. Readiness/injection x native sidecars x cancellation x result sidecar.
8. Security context x non-root credentials x read-only root x admission.
9. Resource overrides x LimitRange/Quota x injected defaults x sidecars.
10. Resolver x immutable ref x credentials/namespace x cache x trust.
11. StepAction x typed results x promotion x guards.
12. Child pipeline x inputs x failure x recursion and each known limitation.
13. Pending x timeout clock x start/cancel x delegated management.
14. Restart/duplicate reconciliation x events x cache/status transformations.
15. Windows/hermetic/debug x gates and backend capabilities.

Use constrained pairwise generation plus these targeted interactions. Compare
dependency partial orders rather than exact parallel schedules. Use explicit
barriers/fake clocks where possible; real-cluster tests assert bounded outcomes.
Normalize volatile IDs/timestamps while preserving reference relationships.

## Source discrepancies needing qualification

- HTTP is the fifth resolver despite older prose listing four.
- Artifacts have flagged implementation/tests despite an outdated unimplemented
  comment in the feature ConfigMap.
- SPIRE documentation says support is not yet functional; configuration alone
  cannot establish its runtime guarantees.
- Matrix docs contain conflicting prose; verify aggregation against validation
  and execution before promising it.
- keep-pod-on-cancel maturity and hostNetwork documentation conflict with other
  source declarations. Test the exact pinned implementation.
- Derive served API versions from CRDs, not Go type names. StepAction, CustomRun,
  ResolutionRequest, and VerificationPolicy do not share core v1's versioning.

## Ecosystem scope

PaC event selection/provenance, Chains signing, Results persistence, and Triggers
are distinct components needing separate pinned inventories if supported later.
The initial local-verification corpus may consume their pipeline definitions
without claiming to emulate those services.
