# Determinism and strong evaluations

Status: draft contract and specified fixtures. No evaluation runner exists in
this repository yet. Expected outcomes below are requirements, not run results.

## What can be deterministic

Separate two properties:

- **Planning quality:** can a model propose a correct adaptation from a fixed
  definition, repository snapshot, and allowed context?
- **Execution repeatability:** given an accepted immutable plan and identical
  effective inputs, does the runner execute the same scope with the same
  commands and preserve the expected outcomes?

Do not run a model inside the second loop. Temperature zero, a fixed seed, or
reusing a prompt does not establish deterministic model output. Validate every
generated plan against an independent contract.

## Execution inputs

An execution receipt must identify the plan/schema version, definition and
helper hashes, complete worktree snapshot, parameter values, explicitly allowed
environment, source image digest, runtime/platform, resource/service inputs,
and cache mode. Secret values must never appear in an artifact; record only
their declared bindings and safe identifiers.

For deterministic evaluation, use digest-pinned images, fixed synthetic
inputs, no network during checks, a known architecture, fixed locale/timezone,
and an empty isolated cache. Add warm-cache cases separately and require the
same verdict. Do not normalize away missing checks or meaningful differences.

Stable IDs, declared ordering/dependencies, and a canonical report projection
make replay comparisons meaningful. Timing, random invocation names, and host
temporary paths are evidence but do not belong in that projection.

Production checks may intentionally depend on network services, time, mutable
packages, or credentials. Report those dependencies and the weaker
reproducibility claim; do not advertise universal deterministic outcomes.

## An independent oracle for every case

Before model execution, a reviewer specifies:

1. The pinned source and local inputs.
2. The check IDs that must run, may be excluded, or must remain unsupported.
3. Required images/commands and allowed semantic adaptations.
4. Observable outputs, file/result values, exit outcomes, and forbidden effects.
5. A mutation that changes the expected verdict and a harmless control.

Use actual original Tekton behavior as the semantic reference when claiming
compatibility. For synthetic cases, expected outputs follow directly from the
small source command and input, and should also be confirmed against Tekton.
An LLM can propose fixtures and mutations; it cannot be their sole judge.

## Coverage and pass rules

Track feature-family coverage, fixture coverage, required execution coverage,
and mutation detection independently. Count feature combinations separately
from individual features. A parser recognizing a field is not execution support.

Required checks must have receipts. Required mutations must execute their target
and cause the independently expected failure. A missing target, blocked run,
skipped mutation, timeout, malformed plan, or absent report prevents a passing
execution evaluation. An unsupported-feature case can pass its rejection
assertion without counting as executable feature coverage.

Use hard correctness gates for the reviewed suite: zero false passes, no
unaccounted required checks, no prohibited effects, and expected unsupported
outcomes. Show numerator and denominator for each gate. Also report false
failures and useful executed scope; rejecting everything cannot satisfy the
clean positive cases.

Repeated live-model trials measure planning reliability, not mathematical
determinism. Preserve model/provider identifiers, prompt/tool/spec revisions,
plan rejections, attempts, token use, latency, and every trial outcome.
Do not execute only the best generated plan and call the batch successful.

## Existing real pipelines

Start from the pinned tkn-innerloop corpus at
`c05878019243e611329b415954f7a769b21d1190`. Its 18 cases span PaC, Konflux,
Pipelines, Chains release definitions, and plumbing examples.

Strengthen existing PaC clean/mutated cases before adding copies. Add
staged/unstaged/untracked variants and prerequisite failures. Add one substantive
Konflux validator and one shared downstream build-wrapper family only after
recording their exact source inputs and requirements.

Real pipeline fixtures test realism. Synthetic fixtures isolate semantics.
Neither replaces the other. Preserve source pins and licenses when importing
corpus data; do not publish internal repository content or credentials.

## Initial synthetic cases

`evals/cases.json` defines requirements for a future runner. It is deliberately
not an executable test manifest until the runner contract is implemented.

The baseline PipelineRun checks whether `message.txt` contains exactly `valid`.
The clean and broken cases differ only in file contents; the staged, unstaged,
and untracked variants must exercise the actual worktree ingestion path.
The unsupported sidecar case requires explicit blocking before execution.
Review-policy cases distinguish an unreviewed noninteractive plan from an
accepted plan replayed without provider access. An invalid interpreter override
must not pass merely because the host can syntax-check that interpreter.

The image is an OCI index resolved from Docker Hub's `library/alpine:3.22` on
2026-11-18 and pinned by digest. The tag is provenance, not an execution input.
Check execution should require no network after image retrieval.

## Reference evaluation finding, 2026-11-18

One live planning trial each for the pinned PaC linter and Go cases passed the
existing structural/fixture expectations, using
`openai-codex/gpt-5.6-sol:low`. Generation took 64.6 and 27.6 seconds respectively.
Those two observations do not establish a model reliability rate.

The subsequent four-check linter run on Linux/arm64 Podman passed codespell,
shellcheck, and the clean test-convention control, but failed to start yamllint:
the generated plan selected `bash`, which the source image did not contain.
The source script had no shebang; its image did provide `sh` with `pipefail`
and the yamllint executable. Host shell syntax validation had not caught this
image/interpreter mismatch.

The original run remains failed. This motivates the interpreter eval above;
changing the image or silently replacing the plan would erase the evidence.
The clean convention control reported no changed test files. A subsequent run
with untracked spelling, unchecked-`cd`, and test-name defects made each of the
three corresponding checks fail for its intended reason, using the same plan.

The Go check passed 4,884 tests on the clean source. Adding an untracked failing
test made the same saved plan report 4,885 tests with exactly one failure, in
the planted test. All four temporary defect files were then removed; no fixture
commits or history resets were used.

Across the original five mutation targets, four executed and were detected.
The YAML-key mutation remains blocked by the interpreter mismatch and stays in
the denominator: this is incomplete evaluation, not a five-of-five success.
The three linter defects were present together, so independent single-defect
attribution is not yet established for every interaction. A paired Tekton
comparison and repeated-model reliability trials remain unverified.

Generation time, runner time, image retrieval, package installation, and cached
Go execution have different boundaries here. These observations are not a cold
or warm latency benchmark and do not establish deterministic runtime behavior.

## Building further cases with AI

Give the agent a feature ID, authoritative source revision, and a real example.
Ask for a minimal positive fixture, a single-fault negative variant, independent
expected outputs, and relevant interactions. Review these before adding them.

Prioritize substitutions with spaces/quotes, empty and missing values,
dependencies and cycles, results, conditions, matrices, failure continuation,
workspace paths, platform differences, cancellation, and context isolation.
Do not generate an unreviewable Cartesian product. Pairwise combinations are a
starting point; add higher-order cases when semantics require them.

The feature inventory must track every feature as supported, partially
supported, explicitly rejected, or awaiting assessment. New upstream features
require classification before any expanded compatibility claim.
