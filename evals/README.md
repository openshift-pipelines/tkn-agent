# Evaluation specifications

These are input fixtures and reviewed-oracle proposals for a future evaluator.
They are not passing tests and cannot yet be run through tkn-agent.

`cases.json` gives each case a stable ID, source input, worktree state, and
expected observation. The expectation is defined independently of model output.
`fixtures/` contains synthetic Tekton definitions; real repository fixtures will
be imported with their original licenses and pinned source provenance.

For the original Tekton reference run, the evaluator must create an isolated
namespace and an `eval-source` PVC populated with the case's exact worktree
snapshot before submitting `worktree-check.yaml`. For the local run, it must
explicitly bind that same source workspace to the private local worktree and
record this adaptation. An empty workspace is not the clean control.

See `docs/evaluations.md` and `docs/testing.md` for gates and evidence.
