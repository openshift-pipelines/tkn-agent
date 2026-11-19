# Evaluation specifications

`cases.json` contains reviewed-oracle proposals for the broader evaluator.
Those cases are not passing compatibility tests merely because they are listed.
The implemented synthetic mixed-check fixture is exercised by
`test/e2e/verify_test.go`.

`cases.json` gives each case a stable ID, source input, worktree state, and
expected observation. The expectation is defined independently of model output.
`fixtures/` contains synthetic Tekton definitions; real repository fixtures will
be imported with their original licenses and pinned source provenance.

`inventory/openshift-pipelines-20261118.json` records all 101 public repository
default-branch pins, including absent directories and placeholder-only cases.
`real-repositories/pipeline-service-static-checks.json` defines a pinned real
check scope, preserved commands, explicit replay adaptations and independent
defect oracles. Its direct container replay is separate from native CLI coverage:
the original unadapted PipelineRun returns `no-local-checks`. The explicit
checkout/parameter adaptation is exercised through the actual CLI by
`make real-e2e`; the historical component experiment remains separate evidence.

See `docs/repository-evaluation.md` for observed outcomes and timings. Runtime
receipts remain under ignored `artifacts/real-repository-eval/`; the case
descriptor does not declare a passed test or a completed live-model evaluation.

For the original Tekton reference run, the evaluator must create an isolated
namespace and an `eval-source` PVC populated with the case's exact worktree
snapshot before submitting `worktree-check.yaml`. For the local run, it must
explicitly bind that same source workspace to the private local worktree and
record this adaptation. An empty workspace is not the clean control.

See `docs/evaluations.md` and `docs/testing.md` for gates and evidence.
