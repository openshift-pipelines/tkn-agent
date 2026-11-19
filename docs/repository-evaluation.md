# Real repository discovery and validation

Status: observed evidence, 2026-11-18. The original discovery result is retained
below; the bounded CLI integration is now implemented.

## Implemented follow-up

The pinned case now runs through the actual CLI using
`--bind-checkout fetch-repository --param target_branch=main`. Recorded provider
responses drive the real ADK tool loop; the planner inspects original helpers,
the YAML configuration, the `.shellcheckrc` link and an absent optional config.

Native acceptance observed:

- Initial review: no execution and no shared file written.
- Approved clean source: both checks passed in 21.83 seconds.
- Credential-free replay with both untracked defects: both failed with their
  expected diagnostics in 21.42 seconds, without another model call.
- Repaired credential-free replay: both passed in 22.08 seconds.
- An inspected helper change required renewed review without executing checks.
- Normal replay left the saved shared plan unchanged.

This is one real pipeline family and two selected checks, not support for every
entry or a measured cluster-CI speedup. Live provider selection quality and the
hosted CI job remain unverified. See `real-slice.md` and `plan-state.md` for the
supported adaptation and state contract.

## Repository inventory

A complete public/default-branch inventory inspected 101 repositories in
`openshift-pipelines`. Exactly 24 contain either directory: 19 contain `.tekton`,
16 contain `.konflux`, and 11 contain both. Seven `.tekton` directories contain
only `.placeholder`; 12 contain YAML files directly under `.tekton`. One matching
repository is archived. Forks remain listed and do not count as independent
validation families. Release branches and private repositories were not inventoried.

The full positive/negative denominator, default-branch commit pins, directory
object IDs and first-level listings are in
[`openshift-pipelines-20261118.json`](../evals/inventory/openshift-pipelines-20261118.json).
A directory containing packaging configuration is not proof of a runnable check.

| Repository | `.tekton` | `.konflux` | Notes |
| --- | --- | --- | --- |
| ecosystem-images | YAML | yes | - |
| hack | no | yes | - |
| lightspeed-rag-content | YAML | no | fork |
| operator | YAML | yes | - |
| operator-tooling | YAML | no | archived |
| p12n-console-plugin | placeholder | yes | - |
| p12n-console-plugin-pf5 | placeholder | yes | - |
| p12n-manual-approval-gate | placeholder | yes | - |
| p12n-opc | no | yes | - |
| pac-downstream | placeholder | yes | fork |
| pipeline-service | YAML | no | - |
| pipeline-service-exporter | YAML | no | - |
| pipelines-tutorial | YAML | no | fork |
| release-tests | YAML | no | - |
| tekton-kueue | no | yes | - |
| tektoncd-chains | placeholder | yes | fork |
| tektoncd-cli | no | yes | fork |
| tektoncd-git-clone | YAML | yes | fork |
| tektoncd-hub | placeholder | yes | fork |
| tektoncd-pipeline | YAML | yes | fork |
| tektoncd-results | no | yes | fork |
| tektoncd-results-for-konflux | YAML | no | fork |
| tektoncd-triggers | placeholder | yes | fork |
| tkn-innerloop | YAML | no | - |

## First selected real case

Selected `openshift-pipelines/pipeline-service` at
`5b34974a82703dfba477bbec73b0658d49ddc69b`, with
`.tekton/test-pipeline-service-static-code-analysis.yaml`. Its original
ShellCheck and yamllint helpers are small, independent source validations.
The two checks are one pipeline family. The Apache-2.0 license is retained in
the private source checkout; no upstream implementation was copied into this CLI.

This is the current default-branch pin, whose last commit is dated August 14,
2024. It is a historical real-workload control, not evidence of current team
adoption or current CI execution. The pinned commit has no public GitHub check
runs from which to derive an outer-loop timing baseline.

Other inspected candidates included `pipeline-service-exporter/.tekton/unit-test.yaml`
and `release-tests/.tekton/go-lint-pipelinerun.yaml`. They introduce clone,
workspace, toolchain or credential prerequisites that also require assessment.
The archived `operator-tooling` case is not the adoption target.

## Original CLI result before the adapter

`tkn-agent verify --local` against the untouched source PipelineRun returned
exit 2, `no-local-checks`: zero selected, zero executed, six unresolved entries.
Its approximately 2.35-second blocked return is not a successful validation time.

Observed report reasons include the unavailable clone Task, `imagePullPolicy`
and workspace mappings. Source inspection also shows `runAfter`, target-branch
image parameters and the checkout subdirectory that a supported adapter must
preserve. Removing one parser rejection does not satisfy those prerequisites.

The original experiment made no product changes to bypass these checks. The
implemented follow-up adds explicit validated adaptations and approval instead.
Live ADK selection was not run: no supported provider credential or explicit
model was configured.

## Direct original-check experiment

The original image reference, bound to target branch `main`, resolved to
`quay.io/redhat-pipeline-service/static-checks@sha256:0d44abed716eec3e9566c4f9b6789e011f5985154869eff0e70e0e616789e113`.
Its image ID is `ee59890bcdbb6ef994a0367e17cd8652397a09f6158c2d0e36c09ec1271ce948`.
It is a roughly 444 MB linux/amd64 image, executed on the existing arm64 Podman
VM. The initial image pull took about 55 seconds. Cross-platform execution and
this cold setup must not be hidden in a warm performance claim.

The direct replay binds an invocation-owned pinned checkout instead of running
the cluster clone Task and runs the original helpers in its checkout root.
It retains the original image, bash interpreter, scripts, configuration and
validation flags. This component experiment is separate from native CLI support.

| Check | Clean control | Seeded untracked defect | Defect result | Repaired control |
| --- | --- | --- | --- | --- |
| ShellCheck 0.10.0 | exit 0 | unquoted `$1` in a new shell file | SC2086, exit 123 through `xargs` | exit 0 |
| yamllint 1.35.1 | exit 0 | duplicate YAML mapping key | key-duplicates, exit 1 | exit 0 |

Component mutation denominator: 2 expected, 2 executed, 2 detected, 0 blocked,
0 skipped. Native tkn-agent mutation execution remains zero; no native mutation
replay was performed after its blocked baseline. Four other source entries
were outside the direct component scope and are not passing coverage.

Initial concurrent clean controls took 15.59 seconds and 12.86 seconds. After
repair, three sequential alternating warm measurements per check were:

| Check | Warm samples, seconds | Median |
| --- | --- | --- |
| ShellCheck | 10.00, 9.86, 9.93 | 9.93 s |
| yamllint | 7.78, 7.54, 7.74 | 7.74 s |

These measurements include container startup and the original helper. They
exclude checkout preparation, tkn-agent source transfer, plan generation and
cluster queue/runtime. There is no measured CI speedup or statistically broad
performance claim. They demonstrate that these two actual validators can catch
known defects locally with feedback measured in seconds.

The two injected files were removed from the private checkout after repair
measurements. All ten invocation-owned containers were verified absent. The
pulled image remains cached; unrelated containers and caches were untouched.

## Reproduce and inspect

- Case and independent mutation oracles:
  [`pipeline-service-static-checks.json`](../evals/real-repositories/pipeline-service-static-checks.json).
- Full observed CLI report, image/platform and clean results:
  `artifacts/real-repository-eval/baseline.json` (ignored local runtime evidence).
- Mutation counts and warm samples:
  `artifacts/real-repository-eval/results.json` (ignored local runtime evidence).
- Private pinned source checkout:
  `artifacts/real-repository-eval/pipeline-service`.

To repeat, use a new private checkout at the pinned source commit, the pinned
image digest, and the commands/oracles in the case descriptor. Run the clean
controls first; create only the named mutation files; require their specific
diagnostics; repair them and require passing controls again. Keep actual CLI
results and direct component results in separate denominators.

## Small next implementation tranche

1. Specify an explicit self-checkout binding for this exact case. Establish
   repository/revision identity before substituting current local source; do
   not skip arbitrary Tasks merely because their name resembles git-clone.
2. Support its workspace aliases and checkout subdirectory, bounded literal
   parameter binding, and original image pull policy, with provenance in the plan.
3. Drive the same clean/defect/repair controls through the actual CLI, required
   review and warm hook replay. Keep the other checks unresolved until supported.
4. Evaluate live ADK selection once provider/model configuration is supplied.

This bounded tranche is now implemented and covered by `make real-e2e`. The
other source checks remain intentionally outside the selected acceptance scope.

## Repository-tracked plan history

The implemented state design is in [`plan-state.md`](plan-state.md): retain a
shared plan and explicit definition/helper hashes beside source CI, use Git for
history, exclude that state from its own invalidation key, and keep local
approval and runtime receipts outside the repository. Application edits rerun
checks without regenerating the plan. Referenced helper/configuration changes
can invalidate it even when `.tekton` is unchanged. Per-user cache state retains
pending candidates and local acceptance separately from the shared file.
