# tkn-agent

A Tekton CLI plugin for coding-agent workflows.

The first planned capability brings selected checks from a repository's
existing Tekton pipelines into the local edit loop. It should preserve the
organization's validation commands, explain necessary adaptations, and report
exactly what was checked. Cluster CI remains authoritative.

## Current state

This repository starts with a specification for review. There is no executable,
installation procedure, or implemented test suite yet. The commands below
describe the accepted entry point, not available features.

```sh
tkn-agent verify --local .tekton/checks.yaml
# Equivalent invocation when installed as a Tekton CLI plugin:
tkn agent verify --local .tekton/checks.yaml
```

Go is the intended implementation language. An ADK will support plan generation;
the particular SDK, model provider, and authentication contract remain decisions.
New or changed plans require review before execution. Repeated execution of an
accepted unchanged plan requires neither another prompt nor a model call.

## Start here

- [Behavior specification](docs/spec.md): scope, guarantees, command contract,
  acceptance scenarios, and open decisions.
- [Test strategy](docs/testing.md): how scenarios become unit, integration,
  end-to-end, and model evaluation checks.
- [Determinism and evaluations](docs/evaluations.md): immutable execution inputs,
  independent expected outcomes, and coverage accounting.
- [Tekton feature matrix](docs/tekton-feature-matrix.md): the pinned feature
  inventory, flag surface, and proposed evaluation oracles.
- [Specification workflow](docs/spec-workflow.md): how humans and AI revise the
  specification and implement one verified behavior at a time.
- [Agent guidance](AGENTS.md): instructions for contributors and coding agents.

The existing `openshift-pipelines/tkn-innerloop` project supplies reference
behavior and a pipeline corpus. Reuse its findings and preserve attribution
when adapting implementation or fixtures; do not assume every prototype
behavior is part of this project's contract.
