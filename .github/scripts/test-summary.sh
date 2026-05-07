#!/usr/bin/env bash
# test-summary.sh
#
# Post-run command for gotestsum (--post-run-command).
# Called automatically after all test runs and reruns complete.
#
# Responsibilities:
#   1. Detect flaky tests: tests that failed at least once but ultimately passed
#      on rerun. These are identified by having both "fail" and "pass" actions
#      for the same test name in the JSON output.
#   2. Expose outputs to subsequent GitHub Actions steps via $GITHUB_OUTPUT:
#      - has_flaky: "true" if any flaky tests were detected, "false" otherwise
#      - flaky_count: number of flaky tests
#   3. Append a summary to the GitHub Actions job summary ($GITHUB_STEP_SUMMARY).
#
# Environment variables provided by gotestsum:
#   GOTESTSUM_JSONFILE  path to the JSON file with all test events (test2json format)
#   TESTS_FAILED        number of tests that ultimately failed
#   TESTS_TOTAL         total number of tests run
#   GOTESTSUM_ELAPSED   total elapsed time (e.g. "12.34s")

set -eo pipefail

# Extract names of flaky tests: those that appear with both a "fail" and a "pass"
# action. These passed only after being rerun, making them flaky.
FLAKY_NAMES=$(jq -rs '[
  [.[] | select(.Test != null)] |
  group_by(.Test)[] |
  select(any(.[]; .Action == "fail") and any(.[]; .Action == "pass")) |
  "- `\(.[0].Package)/\(.[0].Test)`"
] | join("\n")' "$GOTESTSUM_JSONFILE")

FLAKY=$(echo "$FLAKY_NAMES" | grep -c "^-" || true)

# Determine the overall status line based on final failure count.
if [ "$TESTS_FAILED" -gt 0 ]; then
  STATUS="❌ $TESTS_FAILED test(s) failed"
else
  STATUS="✅ All tests passed!"
fi

# Expose flaky test info to subsequent steps in the same job.
# flaky_names uses the multiline delimiter syntax required by GitHub Actions.
echo "has_flaky=$([ "$FLAKY" -gt 0 ] && echo true || echo false)" >> "$GITHUB_OUTPUT"
echo "flaky_count=$FLAKY" >> "$GITHUB_OUTPUT"
echo "tests_total=$TESTS_TOTAL" >> "$GITHUB_OUTPUT"
echo "elapsed=$GOTESTSUM_ELAPSED" >> "$GITHUB_OUTPUT"
{
  echo "flaky_names<<EOF"
  echo "$FLAKY_NAMES"
  echo "EOF"
} >> "$GITHUB_OUTPUT"

# Write a summary to the GitHub Actions job summary page.
{
  echo "## Test Results"
  echo "$STATUS | $TESTS_TOTAL tests in $GOTESTSUM_ELAPSED"
  if [ "$FLAKY" -gt 0 ]; then
    printf '\n> ⚠️ %s test(s) were flaky (failed then passed on rerun)\n\n' "$FLAKY"
    echo "$FLAKY_NAMES"
  fi
} >> "$GITHUB_STEP_SUMMARY"
