# Pinned pipeline-service fixture

Repository: https://github.com/openshift-pipelines/pipeline-service
Commit: 5b34974a82703dfba477bbec73b0658d49ddc69b
Original file: .tekton/test-pipeline-service-static-code-analysis.yaml

static-checks.yaml preserves the original definition. LICENSE is copied from
the same revision. The native acceptance suite clones that revision and runs
its original helpers/image; this fixture supports the source boundary tests.
