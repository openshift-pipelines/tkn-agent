#!/bin/sh
set -eu

: "${KUBECONFIG:?Set an explicit test-cluster kubeconfig}"
: "${KUBE_CONTEXT:?Set the explicit context in that kubeconfig}"
: "${WORKUP_NAMESPACE:?Set a dedicated, new namespace for this PoC}"

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
kube() {
  kubectl --kubeconfig="$KUBECONFIG" --context="$KUBE_CONTEXT" \
    --namespace="$WORKUP_NAMESPACE" "$@"
}

# Deliberately fail if the namespace already exists rather than adopting it.
kube create namespace "$WORKUP_NAMESPACE"
kube apply -f "$here/pipelines.yaml"

if [ -n "${MCP_NAMESPACE:-}" ] && [ -n "${MCP_SERVICE_ACCOUNT:-}" ]; then
  kube create rolebinding workup-evidence-mcp \
    --role=workup-evidence-reader \
    --serviceaccount="$MCP_NAMESPACE:$MCP_SERVICE_ACCOUNT"
fi

failed=$(kube create -f "$here/failed-build.yaml" -o jsonpath='{.metadata.name}')
control=$(kube create -f "$here/control-build.yaml" -o jsonpath='{.metadata.name}')
kube wait --for='jsonpath={.status.conditions[0].status}=False' \
  "pipelinerun/$failed" --timeout=180s
kube wait --for=condition=Succeeded=True \
  "pipelinerun/$control" --timeout=180s
uid=$(kube get "pipelinerun/$failed" -o jsonpath='{.metadata.uid}')
printf '\nFailed target: %s\nTarget UID: %s\nPassing control: %s\n' "$failed" "$uid" "$control"
printf '\nCreate your reviewed config from %s/workup.yaml.\n' "$here"
printf 'Set its skill path to %s/skills/build-failure/SKILL.md.\n' "$here"
