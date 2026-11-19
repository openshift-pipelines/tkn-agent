# tkn-innerloop-go-poc

This is a Go proof of concept for running certain checks from Tekton pipelines against your local edits before you push. It builds upon the concepts in [tkn-innerloop](https://github.com/openshift-pipelines/tkn-innerloop). The repository is tkn-innerloop-go-poc, and the executable is still tkn-agent.

Planning currently uses Google ADK Go and a model you configure with an API key. You evaluate the plan before it is executed. An unchanged, approved plan runs again without a model call. Changes to the pipeline, recorded configuration inputs, or recorded helper files require a new plan and review. Local execution requires an existing Podman runtime.

## Try it

Build with Go 1.26.10 or automatic Go toolchain selection:

```sh
git clone https://github.com/openshift-pipelines/tkn-innerloop-go-poc
cd tkn-innerloop-go-poc
make build
```

Configure `TKN_AGENT_PROVIDER` and `TKN_AGENT_MODEL`, with the provider's API key.
Supported providers are `openai`, `gemini` and `anthropic`. From the repository
you want to check, run the built executable:

```sh
/path/to/tkn-agent verify --local .tekton/checks.yaml
# Inspect the plan, then repeat with its exact approval digest:
/path/to/tkn-agent verify --local .tekton/checks.yaml --approve-plan sha256:...
```

## What has been tested

Real-container tests are provided for a pinned pipeline-service check and small fixtures. A discovery run invoked the CLI to evaluate 199 public definitions from 119 repositories and returned 162 no-local-checks and 37 blocked results, with no validators executed in that sample. Results are representational of compatibility gaps and are not passing pipeline tests.

See [the gap issue](https://github.com/openshift-pipelines/tkn-innerloop-go-poc/issues/2) and [test details](docs/testing.md). Run `make fmt-check lint test e2e real-e2e` for the implementation checks.

Apache License 2.0. See [LICENSE](LICENSE).
