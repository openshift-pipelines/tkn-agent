# First supported Go workflow

Status: implementation contract, 2026-11-18. This narrows the first capability;
it does not claim that the entire feature inventory is implemented.

## User-visible outcome

Given one pipeline containing a small source check and an image-build task,
`tkn-agent verify --local` analyzes both, proposes the source check for local
execution, explains why the image build stays in CI, and requires plan review.
After acceptance, the same command checks current edits from a Git pre-commit
hook without another model call while its planning inputs stay unchanged.

The first acceptance fixture uses a clean input and an untracked defect.
The local source check must actually execute and distinguish them; the image
build must never execute. Zero selected checks cannot produce a passing verdict.

## Selection and execution boundaries

- Use Go ADK for the bounded planning/tool loop. Provider/model configuration
  is explicit; missing credentials are a configuration error, not a fabricated
  local selection. Tests use a deterministic local provider endpoint.
- Supply the parsed source inventory, supported capabilities, and local-scope
  goal. The selector accounts for every source step as run, skip, or unresolved.
- The initial plan selects existing scripts, images, environment and order.
  It cannot introduce replacement commands, change check flags, or silently
  weaken a check. Unsupported substitutions and prerequisites remain explicit.
- Publishing, deployment, signing, image-building and cluster-administration
  commands are outside the initial local scope, even if a model selects them.
- Execute supported independent source checks through existing Podman.
  No Dagger dependency, Kubernetes provisioning, host-shell fallback, or
  inherited host-secret forwarding is part of this slice.
- Give each invocation isolated source/scratch state. Do not write generated
  check files into the contributor's checkout. Report unsupported filesystem
  cases instead of silently omitting required input.
- Bound execution and cancel invocation-owned containers. Distinguish a failed
  source check from a container that never started.

Remote references, result dependencies, conditions, matrices, sidecars and
additional resource workspaces may be explicitly unsupported in this slice.
Their presence outside the accepted scope does not imply they passed.

## Review and replay

The initial command emits the plan and its digest, then returns a review-required
outcome without executing checks. `--approve-plan <digest>` accepts exactly that
plan. Acceptance is stored outside the source repository.

Application edits produce a fresh source snapshot without invalidating acceptance.
Changes to definitions, inspected helpers/configuration, scope, images or execution
policy require a new digest and review. Source images and all adaptations remain
visible. Machine-readable output reports coverage and the terminal local outcome.

## Pre-commit use

Ship an ordinary Git hook example that invokes the same command. A new or stale
plan blocks the commit until reviewed. An accepted plan runs unattended and
blocks the commit when a selected check fails or cannot execute completely.
The hook does not trigger the outer pipeline or mark cluster CI as satisfied.

## Evidence required before handoff

1. A recorded-provider end-to-end test drives automatic mixed-pipeline selection.
2. The first unreviewed plan executes no checks.
3. Acceptance runs the small check and leaves the heavy task unexecuted.
4. An untracked defect fails the same accepted scope without another model call.
5. A changed definition/helper requires review again.
6. A model-selected heavy task, unknown step, or unsupported prerequisite cannot run.
7. A missing image interpreter yields unsuccessful coverage, never a passed check.
8. Unit/race/lint checks and real Podman execution pass, with provider-live testing
   reported separately if credentials are unavailable.
