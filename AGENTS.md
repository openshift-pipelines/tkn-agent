# Contributor and coding-agent guidance

## Read first

Read `README.md`, `docs/spec.md`, and `docs/testing.md` before changing behavior.
Use `docs/spec-workflow.md` when proposing or implementing a specification change.
The first executable boundary is documented in `docs/first-slice.md`.
Never describe broader proposed commands or planned scenarios as available or passing.

## Scope and decisions

- Build a maintained Go CLI, beginning with local CI-derived verification.
- Keep proposed decisions marked as proposed until a maintainer accepts them.
- Record accepted behavior in the specification before implementing it.
- Do not expand into general pipeline generation, deployment, MCP, RAG, or a
  Kubernetes replacement without a separately reviewed specification change.
- Prefer existing Tekton and tkn-innerloop behavior and fixtures over a parallel
  design. Preserve licenses, source revisions, and attribution when reusing them.
- Keep public APIs private until a real caller requires an exported name.

## Implementation

- Keep model-based planning separate from deterministic validation and execution.
- Parse untrusted definitions and plans at their boundaries. A model response
  cannot authorize execution, establish semantic equivalence, or prove safety.
- Preserve source images, commands, flags, inputs, and prerequisites, or disclose
  each adaptation. Unsupported requirements must remain explicit.
- Never fetch credentials automatically, inherit unrelated host secrets, or
  operate on the ambient cluster as part of local verification.
- Do not silently skip requested checks or infer passing coverage from exit zero.
- Use the standard library and existing dependencies before adding a framework.
- Keep each change focused. Do not create unused extension points or stubs.

## Tests and evidence

- Name the specification scenario IDs affected by each behavior change.
- Start with a failing observable acceptance case, then add the smallest unit
  coverage needed for the underlying rule. Avoid duplicate assertions.
- Deterministic tests use fixed model responses; provider evaluations are a
  separate suite. Mocking a container process does not verify container behavior.
- Require execution receipts for every expected check and mutation. Blocked,
  skipped, and unexecuted cases cannot count as passing evaluation coverage.
- Use `evals/feature-inventory.json` and `docs/tekton-feature-matrix.md` when
  adding compatibility claims. Every feature needs an assessed capability and
  an independent positive, negative, or explicit-unsupported oracle.
- Introduce real build, unit, lint, and end-to-end commands with their owning
  implementation. Do not add always-green placeholder CI or claim planned checks
  were run.
- Once Go implementation exists, run formatting, the project's actual
  golangci-lint configuration, and relevant tests, including the race detector.
- Report exact checks, runtime prerequisites, and anything unverified.

## Changes outside the checkout

Do not commit, push, publish releases, create PRs, or send external messages
without explicit authorization. Use the identity and publication rules supplied
by the maintainer's current environment. Never publish credentials or internal
tracking context.

Do not add automated AI co-author trailers to commits.
