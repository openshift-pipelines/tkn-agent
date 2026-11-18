# How we create and iterate on the specification

The specification is a reviewed description of what users can rely on. AI helps
write and challenge it; maintainers decide its meaning. Tests provide evidence
that the implementation meets it. None of these substitutes for the others.

## Creating the first draft

1. Start with a concrete user story and the failure it should prevent.
2. Inspect existing implementations and real pipeline examples. Separate source
   behavior, observed runtime evidence, assumptions, and desired changes.
3. State the supported inputs, observable outputs, side effects, exclusions, and
   unresolved decisions. Mark proposals explicitly.
4. Write clean, failing, missing-prerequisite, and cancellation scenarios with
   stable IDs. Decide how each result can be observed independently.
5. Review the command contract and execution policy with maintainers before
   implementing them. A draft is useful without pretending these are settled.

The initial draft follows this process using tkn-innerloop's source and corpus,
Tekton CLI plugin discovery, and the requested local verification workflow.
It deliberately makes no measured compatibility or speed claim.

## Revising with AI

Ask the agent to propose a change to the existing spec, not regenerate the whole
document. The proposal should show the user example, affected scenario IDs,
observable behavior changes, trade-offs, and questions that require a human.
Keep IDs stable when wording improves. Add a new ID for a new contract and
record why any existing guarantee is removed or changed.

Example request:

> Read AGENTS.md and docs/spec.md. I want a coding agent to check my uncommitted
> changes without silently skipping a required task. Propose the smallest
> specification change and acceptance examples. Identify any execution-policy
> decision I need to make. Do not implement the proposal yet.

Review that diff. Accepted decisions become requirements; rejected alternatives
need only a short rationale. Add a separate decision record when a substantial
trade-off needs a durable explanation, not for every wording change.

## Implementing one accepted slice

1. Name the accepted spec revision and scenarios for the slice.
2. Check existing tests and reference behavior before creating new coverage.
3. Write the smallest failing acceptance case, then implement its path.
4. Add focused unit checks for underlying rules where existing coverage is
   insufficient. Keep the runtime and planner boundaries explicit.
5. Run deterministic checks and actual container tests. Evaluate a live model
   separately when the slice includes planning.
6. Review code, test results, limitations, and any required spec changes together.

Example implementation request:

> Implement LOCAL-001 and LOCAL-002 from the accepted spec revision. Begin with
> a deterministic planner fixture and the actual CLI/container path. Preserve
> the source command and image. Show the initial failing test and final results.
> Do not add unrelated commands or claim live-model planning was evaluated.

## When implementation and specification disagree

First determine whether the implementation is wrong, the test is wrong, or the
requirement is ambiguous. Use the original user example and independent source
contract to decide. Report ambiguity rather than changing all three to agree.

Behavior changes require review of the spec diff. A passing generated test does
not prove its expected result was correct. Missing runtime evidence remains
unverified, even when static validation passes.
