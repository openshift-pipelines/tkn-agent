#!/bin/sh
set -eu

: "${KUBECONFIG:?Set the explicit test-cluster kubeconfig}"
: "${KUBE_CONTEXT:?Set the explicit context}"
: "${WORKUP_NAMESPACE:?Set the dedicated PoC namespace}"
: "${WORKUP_CONFIG:?Set the absolute path of your reviewed workup config}"
: "${TKN_AGENT_MODEL:?Set an explicit provider model}"

if [ "$#" -ne 1 ]; then
  printf 'Usage: sh examples/workup/run.sh "Investigation goal including target name and UID"\n' >&2
  exit 2
fi
here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
exec "$here/../../bin/tkn-agent" workup "$1" \
  --config="$WORKUP_CONFIG" \
  --kubeconfig="$KUBECONFIG" \
  --context="$KUBE_CONTEXT" \
  --namespace="$WORKUP_NAMESPACE" \
  --provider="${TKN_AGENT_PROVIDER:-openai}" \
  --model="$TKN_AGENT_MODEL" \
  --execute --format=json
