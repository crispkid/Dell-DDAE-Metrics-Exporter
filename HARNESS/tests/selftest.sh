#!/usr/bin/env bash
set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HARNESS="$HARNESS_DIR/harness.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/portable-harness.XXXXXX")"
TEST_ROOT="$(cd "$TEST_ROOT" && pwd -P)"
trap 'rm -rf "$TEST_ROOT"' EXIT
EMPTY_CONFIG="$TEST_ROOT/empty-config.env"
touch "$EMPTY_CONFIG"

expect_failure() {
  if "$@" >/dev/null 2>&1; then
    echo "Expected command at selftest line ${BASH_LINENO[0]} to fail: $*" >&2
    exit 1
  fi
}

expect_status() {
  local expected="$1"
  shift
  local actual=0
  "$@" >/dev/null 2>&1 || actual=$?
  if [[ "$actual" -ne "$expected" ]]; then
    echo "Expected exit $expected, got $actual: $*" >&2
    exit 1
  fi
}

validate_json_file() {
  local file="$1"
  if command -v python3 >/dev/null 2>&1; then
    python3 -m json.tool "$file" >/dev/null
  elif command -v python >/dev/null 2>&1; then
    python -m json.tool "$file" >/dev/null
  elif command -v node >/dev/null 2>&1; then
    node -e 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"))' "$file"
  elif command -v jq >/dev/null 2>&1; then
    jq empty "$file"
  else
    echo "note: no JSON parser available; report parser validation skipped"
  fi
}

# Keep the reusable starter inventory deliberate. Adding or renaming an
# `.example` file requires updating this manifest and the applicable adoption,
# schema, or static contract check below.
EXPECTED_EXAMPLES="$(printf '%s\n' \
  '.gitignore.example' \
  'CODE_REVIEW.md.example' \
  'HARNESS/config.env.example' \
  'HARNESS/templates/ci/GITHUB_ACTIONS.md.example' \
  'HARNESS/templates/evals/case.json.example' \
  'HARNESS/templates/evals/result.json.example' \
  'HARNESS/templates/evidence/latest-pointer.json.example' \
  'HARNESS/templates/evidence/report.json.example' \
  'HARNESS/templates/governance/ACTIVE_CHANGE.example' \
  'HARNESS/templates/governance/DEVELOPMENT_PLAN.md.example' \
  'HARNESS/templates/governance/EXECUTION_PLAN.md.example' \
  'HARNESS/templates/governance/SPECIFICATION.md.example' \
  'HARNESS/templates/governance/SPEC_CHANGELOG.md.example' \
  'HARNESS/templates/governance/TEST_PLAN.md.example' \
  'HARNESS/templates/governance/TRACEABILITY.md.example' \
  'PLANS.md.example' \
  'PROJECT.md.example')"
ACTUAL_EXAMPLES="$(cd "$HARNESS_DIR/.." && find . -type f -name '*.example' -print \
  | sed 's#^./##' | sort)"
[[ "$ACTUAL_EXAMPLES" == "$EXPECTED_EXAMPLES" ]] \
  || { printf 'The .example inventory changed without matching contract coverage.\n' >&2; exit 1; }
[[ ! -e "$HARNESS_DIR/templates/evals/case.yaml.example" \
  && -f "$HARNESS_DIR/templates/evals/YAML_COMPATIBILITY.md" ]] \
  || { printf 'YAML compatibility must be documentation, not a drifting .example descriptor.\n' >&2; exit 1; }
if grep -Eq '<(Action-oriented|What changes|Relevant paths|List every|Components|Acceptance IDs|Timestamped|Unexpected facts|Decision, rationale|Safe retry|Delivered outcome|paths)>' \
  "$HARNESS_DIR/../PLANS.md.example"; then
  printf 'PLANS.md.example contains an undetectable angle-bracket placeholder.\n' >&2
  exit 1
fi
grep -Fq -- '- Clarification Status: pending' \
  "$HARNESS_DIR/templates/governance/SPECIFICATION.md.example"
grep -Fq -- '- {{REPLACE_ME:OPEN_QUESTIONS_OR_NONE}}' \
  "$HARNESS_DIR/templates/governance/SPECIFICATION.md.example"
grep -Fq '| Test ID | Acceptance ID | Requirement ID | Test implementation |' \
  "$HARNESS_DIR/templates/governance/TEST_PLAN.md.example"
grep -Fq '| REQ-{{REPLACE_ME:CHANGE_ID}}-001 | {{REPLACE_ME:SEMVER}} |' \
  "$HARNESS_DIR/templates/governance/TRACEABILITY.md.example"
grep -Fq 'Patch: clarification or correction' \
  "$HARNESS_DIR/templates/governance/README.md"
grep -Fq 'approved or evidenced history is append-only' \
  "$HARNESS_DIR/templates/governance/README.md"
grep -Fq 'never replace a recorded digest' \
  "$HARNESS_DIR/templates/governance/README.md"
grep -Fq 'Return every impacted trace row to `Planned`' \
  "$HARNESS_DIR/templates/governance/README.md"
grep -Fq 'the named person or role' \
  "$HARNESS_DIR/templates/governance/README.md"
grep -Fq 'reuse a released or verified requirement' \
  "$HARNESS_DIR/templates/governance/SPECIFICATION.md.example"
grep -Fq 'row as append-only' \
  "$HARNESS_DIR/templates/governance/SPEC_CHANGELOG.md.example"
grep -Fq 'Never replace a digest' \
  "$HARNESS_DIR/templates/governance/DEVELOPMENT_PLAN.md.example"
grep -Fq 'Preserve useful regression tests' \
  "$HARNESS_DIR/templates/governance/TEST_PLAN.md.example"
grep -Fq 'Never carry `Verified` across an amended' \
  "$HARNESS_DIR/templates/governance/TRACEABILITY.md.example"
grep -Fq 'Isolated exploration is not' \
  "$HARNESS_DIR/../PLANS.md.example"
grep -Fq 'Isolated exploration is not' \
  "$HARNESS_DIR/templates/governance/EXECUTION_PLAN.md.example"
grep -Fq 'For a requirement amendment' \
  "$HARNESS_DIR/../CODE_REVIEW.md.example"
grep -Fq '{{REPLACE_ME:CHANGE_ID_LIFECYCLE}}' \
  "$HARNESS_DIR/../PROJECT.md.example"
grep -Fq '{{REPLACE_ME:SPEC_VERSION_POLICY}}' \
  "$HARNESS_DIR/../PROJECT.md.example"
grep -Fq '{{REPLACE_ME:REQUIREMENT_ID_POLICY}}' \
  "$HARNESS_DIR/../PROJECT.md.example"
grep -Fq '{{REPLACE_ME:EXPLORATION_POLICY}}' \
  "$HARNESS_DIR/../PROJECT.md.example"
grep -Fq 'add a stage only for a distinct execution boundary' \
  "$HARNESS_DIR/config.env.example"
grep -Fq 'test-plan, and traceability digests' "$HARNESS_DIR/../CODE_REVIEW.md.example"
grep -Fq 'Set workflow-level `GITHUB_TOKEN` permissions to read-only' \
  "$HARNESS_DIR/templates/ci/GITHUB_ACTIONS.md.example"
grep -Fq "HARNESS_CONFIG_COMPLETE='1.0.0'" "$HARNESS_DIR/config.env.example"
grep -Fq 'test-results/harness/' "$HARNESS_DIR/../.gitignore.example"
grep -Fq '# Portable AGENTS + Harness 1.0.0' "$HARNESS_DIR/../README.md"
grep -Fq '## 新增或修改需求的 SDD 流程' "$HARNESS_DIR/../README.md"
grep -Fq '## 1.0.0 - 2026-08-14' "$HARNESS_DIR/CHANGELOG.md"
grep -Fq '| Harness report schema | 1 |' "$HARNESS_DIR/HARNESS.md"
grep -Fq '"schema_version": { "const": "1" }' \
  "$HARNESS_DIR/schemas/harness-report.schema.json"
grep -Fq '"schema_version": { "const": "1" }' \
  "$HARNESS_DIR/schemas/agent-eval-case.schema.json"
[[ "$(grep -Ec '^## [0-9]+[.][0-9]+[.][0-9]+ - ' "$HARNESS_DIR/CHANGELOG.md")" -eq 1 ]] \
  || { printf 'The baseline changelog must contain exactly one release record.\n' >&2; exit 1; }

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

file_mode() {
  local mode
  if mode="$(stat -f '%Lp' "$1" 2>/dev/null)"; then
    printf '%s' "$mode"
  else
    stat -c '%a' "$1"
  fi
}

write_approved_plan() {
  mkdir -p "$1/plans"
  cat >"$1/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan
## CHG-101 Portable governance
- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Example Product Owner
- Requirements Confirmer Role: Product Owner
- Requirements Confirmed On: 2026-01-02
- Requirements Confirmation Evidence: review/requirements-CHG-101
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Example Maintainer
- Implementation Plan Approver Role: Maintainer
- Implementation Plan Approved On: 2026-01-02
- Implementation Plan Approval Evidence: review/plan-CHG-101
- Execution Plan: plans/CHG-101.md
EOF
  printf '# CHG-101 execution plan\n' >"$1/plans/CHG-101.md"
}

write_pending_plan() {
  mkdir -p "$1/plans"
  cat >"$1/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan
## CHG-101 Portable governance
- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Example Product Owner
- Requirements Confirmer Role: Product Owner
- Requirements Confirmed On: 2026-01-02
- Requirements Confirmation Evidence: review/requirements-CHG-101
- Implementation Plan Approved Status: pending
- Implementation Plan Approved By: pending
- Implementation Plan Approver Role: pending
- Implementation Plan Approved On: pending
- Implementation Plan Approval Evidence: pending
- Execution Plan: plans/CHG-101.md
EOF
  printf '# CHG-101 execution plan\n' >"$1/plans/CHG-101.md"
}

write_adopted_config() {
  mkdir -p "$1/HARNESS"
  cat >"$1/HARNESS/config.env" <<'EOF'
# committed selftest Harness policy
HARNESS_EXPECTED_BASELINE_VERSION='1.0.0'
HARNESS_SDD_MODE='off'
HARNESS_INSTRUCTION_TARGETS='.'
HARNESS_INSTRUCTION_MAX_BYTES='32768'
HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE='false'
HARNESS_CONFIG_COMPLETE='1.0.0'
EOF
}

bash -n \
  "$HARNESS" \
  "$HARNESS_DIR/lib/adapters.sh" \
  "$HARNESS_DIR/lib/governance.sh" \
  "$HARNESS_DIR/lib/instructions.sh" \
  "$HARNESS_DIR/lib/reporting.sh" \
  "$HARNESS_DIR/lib/sdd.sh" \
  "${BASH_SOURCE[0]}"

validate_json_file "$HARNESS_DIR/schemas/agent-eval-case.schema.json"
validate_json_file "$HARNESS_DIR/schemas/agent-eval-result.schema.json"
validate_json_file "$HARNESS_DIR/schemas/harness-report.schema.json"
validate_json_file "$HARNESS_DIR/schemas/latest-pointer.schema.json"
validate_json_file "$HARNESS_DIR/templates/evals/case.json.example"
validate_json_file "$HARNESS_DIR/templates/evals/result.json.example"
validate_json_file "$HARNESS_DIR/templates/evidence/report.json.example"
validate_json_file "$HARNESS_DIR/templates/evidence/latest-pointer.json.example"
HELP_OUTPUT="$($HARNESS help)"
DOCUMENTED_COMMANDS="$(sed -n '/^## Stable Commands$/,/^```$/p' "$HARNESS_DIR/HARNESS.md" | sed -n 's/^\([a-z][a-z:]*\)[[:space:]][[:space:]]*.*/\1/p' | sort)"
DISPATCHED_COMMANDS="$(printf '%s\n' "$HELP_OUTPUT" | sed -n 's/^  \([a-z][a-z:]*\)[[:space:]][[:space:]]*.*/\1/p' | sort)"
[[ "$DOCUMENTED_COMMANDS" == "$DISPATCHED_COMMANDS" ]] \
  || { echo "Stable command documentation has drifted from harness help." >&2; exit 1; }

EMPTY_ROOT="$TEST_ROOT/empty"
mkdir -p "$EMPTY_ROOT"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" doctor >/dev/null
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" governance:doctor >/dev/null
VERIFY_PRECONDITION_MARKER="$EMPTY_ROOT/verify-precondition-ran"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  HARNESS_ENVIRONMENT_COMMAND='touch verify-precondition-ran' \
  "$HARNESS" verify
[[ ! -e "$VERIFY_PRECONDITION_MARKER" ]] \
  || { echo "Verify ran environment checks before required-stage preconditions." >&2; exit 1; }
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=test HARNESS_VERIFY_STAGES=lint "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_VERIFY_STAGES=unknown "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=invalid "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=unknown "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES='   ' HARNESS_VERIFY_STAGES='   ' "$HARNESS" verify
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=',' HARNESS_VERIFY_STAGES=',' "$HARNESS" verify
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=test,test "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_VERIFY_STAGES=test,test "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_NETWORK_STAGES=unknown "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_NETWORK_STAGES=integration,integration "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COMMAND_TIMEOUT_SECONDS=invalid "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COMMAND_TIMEOUT_SECONDS=86401 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_AGENT_TRIALS=1001 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COVERAGE_THRESHOLD=100.1 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COVERAGE_THRESHOLD=101 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_INSTRUCTION_MAX_BYTES=16777217 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_EVIDENCE_MAX_STRING_BYTES=16777217 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_EVIDENCE_MAX_REPORT_BYTES=1073741825 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REDACT_MIN_BYTES=9 HARNESS_EVIDENCE_MAX_STRING_BYTES=8 "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_TOOLS=portable-harness-tool-that-does-not-exist "$HARNESS" doctor
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_PATHS=../outside "$HARNESS" doctor
mkdir -p "$TEST_ROOT/required-path-outside"
ln -s "$TEST_ROOT/required-path-outside" "$EMPTY_ROOT/escaping-required-path"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_PATHS=escaping-required-path/child "$HARNESS" doctor
printf 'outside\n' > "$TEST_ROOT/required-path-outside-file"
ln -s "$TEST_ROOT/required-path-outside-file" "$EMPTY_ROOT/escaping-required-file"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_PATHS=escaping-required-file "$HARNESS" doctor
if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_COMMAND_TIMEOUT_SECONDS=1 "$HARNESS" doctor
fi
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=none "$HARNESS" verify >/dev/null
[[ -f "$EMPTY_ROOT/test-results/harness/verify.json" ]] || { echo "Verify evidence was not written." >&2; exit 1; }
grep -Fq '"result": "passed"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"doctor"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"repo-doctor","status":"skipped"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"governance-doctor","status":"skipped"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"toolchain": [' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"network_contract_allows":false' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"network_enforcement":"declaration-only"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"schema_version": "1"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"effective_policy": {"required_stages":[],"verify_stages":[],"network_policy":"declared"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"network_policy":"declared"' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"command_timeout_seconds":0,"coverage_threshold":80' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"governance_mode":"auto","require_plan_approval":false,"strict_repo_hygiene":false' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"instruction_targets":".","instruction_max_bytes":32768,"allow_root_instruction_override":false' "$EMPTY_ROOT/test-results/harness/verify.json"
grep -Fq '"command_id":"internal:doctor"' "$EMPTY_ROOT/test-results/harness/verify.json"
if grep -Fq '"command":' "$EMPTY_ROOT/test-results/harness/verify.json"; then
  echo "Raw command text was retained in evidence." >&2
  exit 1
fi
validate_json_file "$EMPTY_ROOT/test-results/harness/verify.json"

# Configured scripts are strict by default: a failing simple command or an
# early failing pipeline element must fail the stage even if a later command
# succeeds. Projects can make an intentional exception explicitly.
STRICT_COMMAND_ROOT="$TEST_ROOT/strict-configured-command"
mkdir -p "$STRICT_COMMAND_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$STRICT_COMMAND_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND=$'false\ntrue' \
  "$HARNESS" test
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$STRICT_COMMAND_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='false | true' \
  "$HARNESS" test
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$STRICT_COMMAND_ROOT" \
HARNESS_REQUIRED_STAGES=test \
HARNESS_TEST_COMMAND=$'set +e\nfalse\ntrue' \
"$HARNESS" test >/dev/null
printf 'set +e\nfalse() { return 0; }\nexport -f false 2>/dev/null || true\n' >"$STRICT_COMMAND_ROOT/poisoned-bash-env"
expect_failure env \
  BASH_ENV="$STRICT_COMMAND_ROOT/poisoned-bash-env" \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$STRICT_COMMAND_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND=$'false\ntrue' \
  "$HARNESS" test
expect_status 2 env \
  BASH_ENV="$STRICT_COMMAND_ROOT/poisoned-bash-env" \
  "$HARNESS" unknown-command
PATH_BOOTSTRAP_BIN="$STRICT_COMMAND_ROOT/path-bootstrap-bin"
mkdir -p "$PATH_BOOTSTRAP_BIN"
cat >"$PATH_BOOTSTRAP_BIN/dirname" <<'EOF'
#!/bin/sh
printf '%s\n' /attacker/controlled/path
EOF
cat >"$PATH_BOOTSTRAP_BIN/basename" <<'EOF'
#!/bin/sh
printf '%s\n' attacker-controlled-name
EOF
chmod +x "$PATH_BOOTSTRAP_BIN/dirname" "$PATH_BOOTSTRAP_BIN/basename"
expect_status 2 env PATH="$PATH_BOOTSTRAP_BIN:$PATH" "$HARNESS" unknown-command
(
  false() { return 0; }
  export -f false
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$STRICT_COMMAND_ROOT" \
    HARNESS_REQUIRED_STAGES=test \
    HARNESS_TEST_COMMAND=$'false\ntrue' \
    "$HARNESS" test
)

# Verify executes its declared selection, not every configured command. An
# intentional `none` selection is valid only when no required stage is omitted.
VERIFY_SELECTION_ROOT="$TEST_ROOT/verify-selection"
mkdir -p "$VERIFY_SELECTION_ROOT"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$VERIFY_SELECTION_ROOT" \
HARNESS_REQUIRED_STAGES=test \
HARNESS_VERIFY_STAGES=test \
HARNESS_TEST_COMMAND='touch test-ran' \
HARNESS_BUILD_COMMAND='touch build-ran' \
"$HARNESS" verify >/dev/null
[[ -f "$VERIFY_SELECTION_ROOT/test-ran" && ! -e "$VERIFY_SELECTION_ROOT/build-ran" ]] \
  || { echo "Verify did not honor HARNESS_VERIFY_STAGES selection." >&2; exit 1; }
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$VERIFY_SELECTION_ROOT" \
HARNESS_REQUIRED_STAGES=none \
HARNESS_VERIFY_STAGES=none \
HARNESS_TEST_COMMAND='touch unselected-test-ran' \
"$HARNESS" verify >/dev/null
[[ ! -e "$VERIFY_SELECTION_ROOT/unselected-test-ran" ]] \
  || { echo "Verify ran a stage despite HARNESS_VERIFY_STAGES=none." >&2; exit 1; }

FIRST_REPORT="$(find "$EMPTY_ROOT/test-results/harness/runs" -mindepth 2 -maxdepth 2 -name report.json -type f | head -n 1)"
[[ -n "$FIRST_REPORT" ]] || { echo "Canonical run report was not written." >&2; exit 1; }
FIRST_REPORT_DIGEST="$(file_sha256 "$FIRST_REPORT")"
RUN_REPORT_COUNT_BEFORE="$(find "$EMPTY_ROOT/test-results/harness/runs" -mindepth 2 -maxdepth 2 -name report.json -type f | awk 'END {print NR+0}')"
[[ -f "${FIRST_REPORT%/report.json}/status.complete" ]] || { echo "Canonical run was not marked complete." >&2; exit 1; }
[[ "$(cat "${FIRST_REPORT%/report.json}/status.complete")" == complete ]] \
  || { echo "Canonical completion marker has the wrong content." >&2; exit 1; }
[[ -f "$EMPTY_ROOT/test-results/harness/latest/verify.json" ]] || { echo "Latest pointer was not written." >&2; exit 1; }
[[ "$(file_mode "$EMPTY_ROOT/test-results/harness")" == 700 \
  && "$(file_mode "$EMPTY_ROOT/test-results/harness/runs")" == 700 \
  && "$(file_mode "$EMPTY_ROOT/test-results/harness/latest")" == 700 \
  && "$(file_mode "${FIRST_REPORT%/report.json}")" == 700 ]] \
  || { echo "Evidence directories are not private to the invoking user." >&2; exit 1; }
[[ "$(file_mode "$FIRST_REPORT")" == 444 \
  && "$(file_mode "${FIRST_REPORT%/report.json}/status.complete")" == 444 \
  && "$(file_mode "$EMPTY_ROOT/test-results/harness/verify.json")" == 444 \
  && "$(file_mode "$EMPTY_ROOT/test-results/harness/latest/verify.json")" == 444 ]] \
  || { echo "Published evidence files are not read-only." >&2; exit 1; }
validate_json_file "$EMPTY_ROOT/test-results/harness/latest/verify.json"
if [[ "${HARNESS_RUN_SCHEMA_CONFORMANCE:-false}" == "true" ]]; then
  HARNESS_GENERATED_REPORT="$EMPTY_ROOT/test-results/harness/verify.json" \
  HARNESS_GENERATED_LATEST="$EMPTY_ROOT/test-results/harness/latest/verify.json" \
  python3 "$HARNESS_DIR/tests/schema_contract_test.py"
fi

HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" HARNESS_REQUIRED_STAGES=none "$HARNESS" verify >/dev/null
RUN_REPORT_COUNT="$(find "$EMPTY_ROOT/test-results/harness/runs" -mindepth 2 -maxdepth 2 -name report.json -type f | awk 'END {print NR+0}')"
[[ "$RUN_REPORT_COUNT" -eq "$((RUN_REPORT_COUNT_BEFORE + 1))" ]] \
  || { echo "Repeated verify did not append exactly one canonical run." >&2; exit 1; }
[[ "$(file_sha256 "$FIRST_REPORT")" == "$FIRST_REPORT_DIGEST" ]] \
  || { echo "A later run modified immutable history." >&2; exit 1; }
LATEST_REPORT_PATH="$(sed -n 's/^[[:space:]]*"report_path": "\([^"]*\)",$/\1/p' "$EMPTY_ROOT/test-results/harness/latest/verify.json")"
[[ -f "$EMPTY_ROOT/test-results/harness/$LATEST_REPORT_PATH" ]] \
  || { echo "Latest pointer target does not exist." >&2; exit 1; }
LATEST_REPORT_DIGEST="$(sed -n 's/^[[:space:]]*"report_digest": "sha256:\([0-9a-f]*\)",$/\1/p' "$EMPTY_ROOT/test-results/harness/latest/verify.json")"
LATEST_REPORT_SIZE="$(sed -n 's/^[[:space:]]*"report_size_bytes": \([0-9]*\),$/\1/p' "$EMPTY_ROOT/test-results/harness/latest/verify.json")"
[[ "$LATEST_REPORT_DIGEST" == "$(file_sha256 "$EMPTY_ROOT/test-results/harness/$LATEST_REPORT_PATH")" ]] \
  || { echo "Latest pointer digest does not match its report." >&2; exit 1; }
[[ "$LATEST_REPORT_SIZE" == "$(LC_ALL=C wc -c <"$EMPTY_ROOT/test-results/harness/$LATEST_REPORT_PATH" | tr -d '[:space:]')" ]] \
  || { echo "Latest pointer size does not match its report." >&2; exit 1; }
cmp "$EMPTY_ROOT/test-results/harness/verify.json" "$EMPTY_ROOT/test-results/harness/$LATEST_REPORT_PATH"
grep -Fq '"configuration_digest": "sha256:' "$EMPTY_ROOT/test-results/harness/$LATEST_REPORT_PATH"

if command -v git >/dev/null 2>&1; then
  UNIGNORED_EVIDENCE_ROOT="$TEST_ROOT/unignored-git-evidence"
  mkdir -p "$UNIGNORED_EVIDENCE_ROOT"
  git -C "$UNIGNORED_EVIDENCE_ROOT" init -q
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$UNIGNORED_EVIDENCE_ROOT" \
    HARNESS_REQUIRED_STAGES=none \
    "$HARNESS" verify
  [[ ! -e "$UNIGNORED_EVIDENCE_ROOT/test-results/harness/verify.json" \
    && ! -e "$UNIGNORED_EVIDENCE_ROOT/test-results/harness/latest/verify.json" ]] \
    || { echo "Unignored evidence preflight published report state." >&2; exit 1; }
  printf '.harness-ignore-probe\n' >"$UNIGNORED_EVIDENCE_ROOT/.gitignore"
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$UNIGNORED_EVIDENCE_ROOT" \
    HARNESS_REQUIRED_STAGES=none \
    "$HARNESS" verify
  [[ ! -e "$UNIGNORED_EVIDENCE_ROOT/test-results/harness/verify.json" \
    && ! -e "$UNIGNORED_EVIDENCE_ROOT/test-results/harness/latest/verify.json" ]] \
    || { echo "Probe-only ignore rule published report state." >&2; exit 1; }

  TRACKED_EVIDENCE_ROOT="$TEST_ROOT/tracked-git-evidence"
  mkdir -p "$TRACKED_EVIDENCE_ROOT/test-results/harness"
  git -C "$TRACKED_EVIDENCE_ROOT" init -q
  git -C "$TRACKED_EVIDENCE_ROOT" config user.name "Portable Harness Selftest"
  git -C "$TRACKED_EVIDENCE_ROOT" config user.email "harness@example.invalid"
  printf '/test-results/\n' >"$TRACKED_EVIDENCE_ROOT/.gitignore"
  printf 'tracked evidence\n' >"$TRACKED_EVIDENCE_ROOT/test-results/harness/tracked.txt"
  git -C "$TRACKED_EVIDENCE_ROOT" add .gitignore
  git -C "$TRACKED_EVIDENCE_ROOT" add -f test-results/harness/tracked.txt
  git -C "$TRACKED_EVIDENCE_ROOT" commit -q -m initial
  printf 'modified tracked evidence\n' >"$TRACKED_EVIDENCE_ROOT/test-results/harness/tracked.txt"
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$TRACKED_EVIDENCE_ROOT" \
    HARNESS_REQUIRED_STAGES=none \
    "$HARNESS" verify
  [[ ! -e "$TRACKED_EVIDENCE_ROOT/test-results/harness/verify.json" \
    && ! -e "$TRACKED_EVIDENCE_ROOT/test-results/harness/latest/verify.json" ]] \
    || { echo "Tracked evidence root published a misleading clean report." >&2; exit 1; }

  GIT_EVIDENCE_ROOT="$TEST_ROOT/git-evidence"
  mkdir -p "$GIT_EVIDENCE_ROOT"
  git -C "$GIT_EVIDENCE_ROOT" init -q
  git -C "$GIT_EVIDENCE_ROOT" config user.name "Portable Harness Selftest"
  git -C "$GIT_EVIDENCE_ROOT" config user.email "harness@example.invalid"
  printf '/test-results/\n' >"$GIT_EVIDENCE_ROOT/.gitignore"
  printf 'initial\n' >"$GIT_EVIDENCE_ROOT/tracked.txt"
  git -C "$GIT_EVIDENCE_ROOT" add .gitignore tracked.txt
  git -C "$GIT_EVIDENCE_ROOT" commit -q -m initial
  printf 'changed\n' >"$GIT_EVIDENCE_ROOT/tracked.txt"
  printf 'untracked-one\n' >"$GIT_EVIDENCE_ROOT/untracked.txt"
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GIT_EVIDENCE_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  GIT_REPORT_ONE="$GIT_EVIDENCE_ROOT/test-results/harness/verify.json"
  grep -Eq '"revision":"[0-9a-f]{40,128}","dirty":true,"diff_digest":"sha256:[0-9a-f]{64}"' "$GIT_REPORT_ONE"
  GIT_DIFF_ONE="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$GIT_REPORT_ONE")"
  printf 'untracked-two\n' >"$GIT_EVIDENCE_ROOT/untracked.txt"
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GIT_EVIDENCE_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  GIT_DIFF_TWO="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$GIT_EVIDENCE_ROOT/test-results/harness/verify.json")"
  [[ -n "$GIT_DIFF_ONE" && -n "$GIT_DIFF_TWO" && "$GIT_DIFF_ONE" != "$GIT_DIFF_TWO" ]] \
    || { echo "Dirty repository identity did not track changed bytes." >&2; exit 1; }
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GIT_EVIDENCE_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  GIT_DIFF_STABLE="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$GIT_EVIDENCE_ROOT/test-results/harness/verify.json")"
  [[ "$GIT_DIFF_STABLE" == "$GIT_DIFF_TWO" ]] \
    || { echo "Generated evidence contaminated repository identity." >&2; exit 1; }
  chmod +x "$GIT_EVIDENCE_ROOT/untracked.txt"
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GIT_EVIDENCE_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  GIT_DIFF_EXECUTABLE="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$GIT_EVIDENCE_ROOT/test-results/harness/verify.json")"
  [[ "$GIT_DIFF_EXECUTABLE" != "$GIT_DIFF_STABLE" ]] \
    || { echo "Dirty repository identity ignored an executable-bit change." >&2; exit 1; }

  SUBMODULE_SOURCE_ROOT="$TEST_ROOT/submodule-source"
  SUBMODULE_EVIDENCE_ROOT="$TEST_ROOT/submodule-evidence"
  mkdir -p "$SUBMODULE_SOURCE_ROOT" "$SUBMODULE_EVIDENCE_ROOT"
  git -C "$SUBMODULE_SOURCE_ROOT" init -q
  git -C "$SUBMODULE_SOURCE_ROOT" config user.name "Portable Harness Selftest"
  git -C "$SUBMODULE_SOURCE_ROOT" config user.email "harness@example.invalid"
  printf 'initial submodule bytes\n' >"$SUBMODULE_SOURCE_ROOT/data.txt"
  git -C "$SUBMODULE_SOURCE_ROOT" add data.txt
  git -C "$SUBMODULE_SOURCE_ROOT" commit -q -m initial
  git -C "$SUBMODULE_EVIDENCE_ROOT" init -q
  git -C "$SUBMODULE_EVIDENCE_ROOT" config user.name "Portable Harness Selftest"
  git -C "$SUBMODULE_EVIDENCE_ROOT" config user.email "harness@example.invalid"
  printf '/test-results/\n' >"$SUBMODULE_EVIDENCE_ROOT/.gitignore"
  git -c protocol.file.allow=always -C "$SUBMODULE_EVIDENCE_ROOT" submodule add -q \
    "$SUBMODULE_SOURCE_ROOT" module
  git -C "$SUBMODULE_EVIDENCE_ROOT" add .gitignore .gitmodules module
  git -C "$SUBMODULE_EVIDENCE_ROOT" commit -q -m 'add submodule'
  printf 'first dirty nested bytes\n' >"$SUBMODULE_EVIDENCE_ROOT/module/data.txt"
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$SUBMODULE_EVIDENCE_ROOT" \
    HARNESS_REQUIRED_STAGES=none \
    "$HARNESS" verify
  [[ ! -e "$SUBMODULE_EVIDENCE_ROOT/test-results/harness/verify.json" ]] \
    || { echo "Dirty submodule published an ambiguous repository identity." >&2; exit 1; }

  MANIFEST_COLLISION_ROOT="$TEST_ROOT/manifest-collision"
  mkdir -p "$MANIFEST_COLLISION_ROOT"
  git -C "$MANIFEST_COLLISION_ROOT" init -q
  git -C "$MANIFEST_COLLISION_ROOT" config user.name "Portable Harness Selftest"
  git -C "$MANIFEST_COLLISION_ROOT" config user.email "harness@example.invalid"
  printf '/test-results/\n' >"$MANIFEST_COLLISION_ROOT/.gitignore"
  git -C "$MANIFEST_COLLISION_ROOT" add .gitignore
  git -C "$MANIFEST_COLLISION_ROOT" commit -q -m initial
  printf 'bc\0' >"$MANIFEST_COLLISION_ROOT/a"
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$MANIFEST_COLLISION_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  MANIFEST_DIGEST_ONE="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$MANIFEST_COLLISION_ROOT/test-results/harness/verify.json")"
  printf 'b' >"$MANIFEST_COLLISION_ROOT/a"
  : >"$MANIFEST_COLLISION_ROOT/c"
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$MANIFEST_COLLISION_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
  MANIFEST_DIGEST_TWO="$(sed -n 's/.*"diff_digest":"\(sha256:[0-9a-f]*\)".*/\1/p' "$MANIFEST_COLLISION_ROOT/test-results/harness/verify.json")"
  [[ -n "$MANIFEST_DIGEST_ONE" && -n "$MANIFEST_DIGEST_TWO" \
    && "$MANIFEST_DIGEST_ONE" != "$MANIFEST_DIGEST_TWO" ]] \
    || { echo "Prefix-free repository manifest collapsed distinct file states." >&2; exit 1; }
fi

JSON_CONTROL_ROOT="$TEST_ROOT/json-control"
mkdir -p "$JSON_CONTROL_ROOT"
CONTROL_COMMAND=$'true # \001 UTF-8: 中文 quote:" slash:\\ invalid:\377'
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$JSON_CONTROL_ROOT" \
HARNESS_REQUIRED_STAGES=test \
HARNESS_TEST_COMMAND="$CONTROL_COMMAND" \
"$HARNESS" verify >/dev/null
grep -Fq '"command_id":"project.v1:test"' "$JSON_CONTROL_ROOT/test-results/harness/verify.json"
if LC_ALL=C grep -Fq 'UTF-8: 中文' "$JSON_CONTROL_ROOT/test-results/harness/verify.json"; then
  echo "Configured command text leaked into evidence." >&2
  exit 1
fi
validate_json_file "$JSON_CONTROL_ROOT/test-results/harness/verify.json"

ESCAPER_ACTUAL="$(
  ROOT_DIR="$EMPTY_ROOT"
  PORTABLE_HARNESS_VERSION="selftest"
  HARNESS_REPORT_DIR="unused"
  HARNESS_WRITE_REPORTS=false
  # shellcheck source=../lib/reporting.sh
  source "$HARNESS_DIR/lib/reporting.sh"
  trap 'harness_private_tmpdir_cleanup' EXIT
  harness_json_escape $'quote:" slash:\\ backspace:\b formfeed:\f newline:\n carriage:\r tab:\t generic:\001 delete:\177 UTF-8: 中文'
)"
ESCAPER_EXPECTED='quote:\" slash:\\ backspace:\b formfeed:\f newline:\n carriage:\r tab:\t generic:\u0001 delete:\u007f UTF-8: 中文'
[[ "$ESCAPER_ACTUAL" == "$ESCAPER_EXPECTED" ]] \
  || { echo "JSON escaping changed data: $ESCAPER_ACTUAL" >&2; exit 1; }

REDACTION_SECRET=$'long*?[secret]\\中文\nline'
REDACTION_ACTUAL="$(
  ROOT_DIR="$EMPTY_ROOT"
  PORTABLE_HARNESS_VERSION="selftest"
  CONFIG_FILE="$EMPTY_CONFIG"
  HARNESS_REPORT_DIR="unused"
  HARNESS_WRITE_REPORTS=false
  HARNESS_REDACT_ENV_VARS=REDACTION_SECRET
  HARNESS_REDACT_MIN_BYTES=8
  # shellcheck source=../lib/reporting.sh
  source "$HARNESS_DIR/lib/reporting.sh"
  trap 'harness_private_tmpdir_cleanup' EXIT
  harness_json_escape "before:$REDACTION_SECRET:after"
)"
[[ "$REDACTION_ACTUAL" == 'before:[REDACTED]:after' ]] \
  || { echo "Literal secret redaction failed." >&2; exit 1; }

REDACTION_SHORT='overlap-secret'
REDACTION_LONG='overlap-secret-with-suffix'
REDACTION_OVERLAP_ACTUAL="$(
  ROOT_DIR="$EMPTY_ROOT"
  PORTABLE_HARNESS_VERSION="selftest"
  CONFIG_FILE="$EMPTY_CONFIG"
  HARNESS_REPORT_DIR="unused"
  HARNESS_WRITE_REPORTS=false
  HARNESS_REDACT_ENV_VARS=REDACTION_SHORT,REDACTION_LONG
  HARNESS_REDACT_MIN_BYTES=8
  # shellcheck source=../lib/reporting.sh
  source "$HARNESS_DIR/lib/reporting.sh"
  trap 'harness_private_tmpdir_cleanup' EXIT
  harness_json_escape "$REDACTION_LONG"
)"
[[ "$REDACTION_OVERLAP_ACTUAL" == '[REDACTED]' ]] \
  || { echo "Overlapping secrets were not redacted longest-first." >&2; exit 1; }
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  OVERLAP_A=abcdefghX \
  OVERLAP_B=Xijklmnop \
  HARNESS_REDACT_ENV_VARS=OVERLAP_A,OVERLAP_B \
  "$HARNESS" doctor
UNICODE_SECRET='中文中文' \
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EMPTY_ROOT" \
HARNESS_REDACT_ENV_VARS=UNICODE_SECRET \
HARNESS_REDACT_MIN_BYTES=8 \
"$HARNESS" doctor >/dev/null
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  RESERVED_REDACTION='[REDACTED]' \
  HARNESS_REDACT_ENV_VARS=RESERVED_REDACTION \
  "$HARNESS" doctor

STRUCTURAL_SECRET_ROOT="$TEST_ROOT/structural-secret"
mkdir -p "$STRUCTURAL_SECRET_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$STRUCTURAL_SECRET_ROOT" \
  HARNESS_REQUIRED_STAGES=none \
  STRUCTURAL_SECRET=false \
  HARNESS_REDACT_ENV_VARS=STRUCTURAL_SECRET \
  HARNESS_REDACT_MIN_BYTES=1 \
  "$HARNESS" verify
[[ ! -e "$STRUCTURAL_SECRET_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "Latest advanced after structural evidence redaction." >&2; exit 1; }
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$STRUCTURAL_SECRET_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  STRUCTURAL_SOURCE_SECRET=configured \
  HARNESS_REDACT_ENV_VARS=STRUCTURAL_SOURCE_SECRET \
  HARNESS_TEST_COMMAND='touch structural-stage-ran' \
  "$HARNESS" verify
[[ ! -e "$STRUCTURAL_SECRET_ROOT/structural-stage-ran" ]] \
  || { echo "Stage ran before structural evidence validation failed." >&2; exit 1; }

NETWORK_INJECTION_ROOT="$TEST_ROOT/network-injection"
mkdir -p "$NETWORK_INJECTION_ROOT"
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$NETWORK_INJECTION_ROOT" \
  HARNESS_INTERNAL_CHECK_NETWORK='true,"injected":true' \
  HARNESS_REQUIRED_STAGES=none \
  "$HARNESS" verify >/dev/null
validate_json_file "$NETWORK_INJECTION_ROOT/test-results/harness/verify.json"
if grep -Fq injected "$NETWORK_INJECTION_ROOT/test-results/harness/verify.json"; then
  echo "Caller-controlled internal network evidence was retained." >&2
  exit 1
fi

ENVIRONMENT_NETWORK_ROOT="$TEST_ROOT/environment-network-contract"
mkdir -p "$ENVIRONMENT_NETWORK_ROOT"
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$ENVIRONMENT_NETWORK_ROOT" \
  HARNESS_NETWORK_CONTRACT_ALLOWS=true \
  HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS=true \
  HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS=true \
  HARNESS_ENVIRONMENT_COMMAND='test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = false && test -z "${HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS+x}" && test -z "${HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS+x}"' \
  "$HARNESS" environment:doctor >/dev/null

(
  ROOT_DIR="$EMPTY_ROOT"
  PORTABLE_HARNESS_VERSION="selftest"
  CONFIG_FILE="$EMPTY_CONFIG"
  HARNESS_REPORT_DIR="unused"
  HARNESS_WRITE_REPORTS=false
  # shellcheck source=../lib/reporting.sh
  source "$HARNESS_DIR/lib/reporting.sh"
  trap 'harness_private_tmpdir_cleanup' EXIT
  harness_utf8_validate valid $'UTF-8 中文 \364\217\277\277'
  if harness_utf8_validate invalid $'bad\377' >/dev/null 2>&1; then exit 1; fi
  if harness_utf8_validate invalid $'bad\300\200' >/dev/null 2>&1; then exit 1; fi
  if harness_utf8_validate invalid $'bad\355\240\200' >/dev/null 2>&1; then exit 1; fi
  NUL_CAPTURE="$TEST_ROOT/nul-capture"
  printf 'a\0b' >"$NUL_CAPTURE"
  if harness_utf8_validate_file invalid-capture "$NUL_CAPTURE" >/dev/null 2>&1; then exit 1; fi
)

REDACTION_ROOT="$TEST_ROOT/redaction"
mkdir -p "$REDACTION_ROOT/bin"
cat >"$REDACTION_ROOT/bin/node" <<'EOF'
#!/usr/bin/env bash
printf 'node UTF-8: 中文 secret:%s\n' "$TEST_EVIDENCE_SECRET"
EOF
chmod +x "$REDACTION_ROOT/bin/node"
TEST_EVIDENCE_SECRET='token-with-*?[glob]-and-length' \
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$REDACTION_ROOT" \
HARNESS_REQUIRED_STAGES=test \
HARNESS_REDACT_ENV_VARS=TEST_EVIDENCE_SECRET \
HARNESS_TEST_COMMAND='true # token-with-*?[glob]-and-length' \
PATH="$REDACTION_ROOT/bin:$PATH" \
"$HARNESS" verify >"$REDACTION_ROOT/run.log"
grep -Fq '[REDACTED]' "$REDACTION_ROOT/test-results/harness/verify.json"
grep -Fq 'UTF-8: 中文' "$REDACTION_ROOT/test-results/harness/verify.json"
if grep -Fq 'token-with-*?[glob]-and-length' "$REDACTION_ROOT/test-results/harness/verify.json" "$REDACTION_ROOT/run.log"; then
  echo "Registered secret or raw configured command leaked." >&2
  exit 1
fi
validate_json_file "$REDACTION_ROOT/test-results/harness/verify.json"

SYNTAX_SECRET='syntax-secret-token-123456'
SYNTAX_SECRET_ROOT="$TEST_ROOT/syntax-secret"
mkdir -p "$SYNTAX_SECRET_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$SYNTAX_SECRET_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_REDACT_ENV_VARS=SYNTAX_SECRET \
  SYNTAX_SECRET="$SYNTAX_SECRET" \
  HARNESS_TEST_COMMAND="echo $SYNTAX_SECRET; )" \
  "$HARNESS" verify >"$SYNTAX_SECRET_ROOT/run.log" 2>&1
if grep -Fq "$SYNTAX_SECRET" "$SYNTAX_SECRET_ROOT/run.log" "$SYNTAX_SECRET_ROOT/test-results/harness/verify.json"; then
  echo "A configured command parse error leaked raw command text." >&2
  exit 1
fi

expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  HARNESS_REDACT_ENV_VARS='NOT-A-NAME' \
  "$HARNESS" doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  HARNESS_REDACT_ENV_VARS=$'FIRST_SECRET\nSECOND_SECRET' \
  FIRST_SECRET=first-secret-value \
  SECOND_SECRET=second-secret-value \
  "$HARNESS" doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  SHORT_SECRET=short \
  HARNESS_REDACT_ENV_VARS=SHORT_SECRET \
  "$HARNESS" doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  ID_SECRET=project.v1 \
  HARNESS_REDACT_ENV_VARS=ID_SECRET \
  "$HARNESS" doctor

BAD_UTF8_ROOT="$TEST_ROOT/bad-utf8"
mkdir -p "$BAD_UTF8_ROOT/bin"
cat >"$BAD_UTF8_ROOT/bin/node" <<'EOF'
#!/usr/bin/env bash
printf '\377\n'
EOF
chmod +x "$BAD_UTF8_ROOT/bin/node"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$BAD_UTF8_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  PATH="$BAD_UTF8_ROOT/bin:$PATH" \
  "$HARNESS" verify
[[ ! -e "$BAD_UTF8_ROOT/stage-ran" ]] || { echo "Stage ran after invalid evidence metadata." >&2; exit 1; }
[[ ! -e "$BAD_UTF8_ROOT/test-results/harness/verify.json" ]] || { echo "Invalid UTF-8 report was published." >&2; exit 1; }
[[ ! -e "$BAD_UTF8_ROOT/test-results/harness/latest/verify.json" ]] || { echo "Latest pointer advanced after invalid UTF-8." >&2; exit 1; }

FAILED_TOOL_ROOT="$TEST_ROOT/failed-tool-version"
mkdir -p "$FAILED_TOOL_ROOT/bin"
cat >"$FAILED_TOOL_ROOT/bin/node" <<'EOF'
#!/usr/bin/env sh
exit 42
EOF
chmod +x "$FAILED_TOOL_ROOT/bin/node"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$FAILED_TOOL_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  PATH="$FAILED_TOOL_ROOT/bin:$PATH" \
  "$HARNESS" verify
[[ ! -e "$FAILED_TOOL_ROOT/stage-ran" \
  && ! -e "$FAILED_TOOL_ROOT/test-results/harness/verify.json" \
  && ! -e "$FAILED_TOOL_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "A failed metadata probe did not fail before evidence or project work." >&2; exit 1; }

METADATA_TIMEOUT_ROOT="$TEST_ROOT/metadata-timeout"
mkdir -p "$METADATA_TIMEOUT_ROOT/bin"
cat >"$METADATA_TIMEOUT_ROOT/bin/timeout" <<'EOF'
#!/usr/bin/env sh
test "$1" = 1 || exit 97
shift
case "$1" in
  */node|node) exit 124 ;;
  *) exec "$@" ;;
esac
EOF
cat >"$METADATA_TIMEOUT_ROOT/bin/node" <<'EOF'
#!/usr/bin/env sh
sleep 5
printf 'unexpected node version\n'
EOF
chmod +x "$METADATA_TIMEOUT_ROOT/bin/timeout" "$METADATA_TIMEOUT_ROOT/bin/node"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$METADATA_TIMEOUT_ROOT" \
  HARNESS_COMMAND_TIMEOUT_SECONDS=1 \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  PATH="$METADATA_TIMEOUT_ROOT/bin:$PATH" \
  "$HARNESS" verify
[[ ! -e "$METADATA_TIMEOUT_ROOT/stage-ran" \
  && ! -e "$METADATA_TIMEOUT_ROOT/test-results/harness/verify.json" \
  && ! -e "$METADATA_TIMEOUT_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "Timed-out evidence metadata did not fail before publication." >&2; exit 1; }

ATOMIC_SOURCE_ROOT="$TEST_ROOT/atomic-source-root"
mkdir -p "$ATOMIC_SOURCE_ROOT/evidence" "$ATOMIC_SOURCE_ROOT/outside"
printf 'outside-source\n' >"$ATOMIC_SOURCE_ROOT/outside/source.json"
(
  ROOT_DIR="$ATOMIC_SOURCE_ROOT"
  PORTABLE_HARNESS_VERSION=selftest
  CONFIG_FILE="$EMPTY_CONFIG"
  HARNESS_REPORT_DIR=evidence
  HARNESS_WRITE_REPORTS=true
  # shellcheck source=../lib/reporting.sh
  source "$HARNESS_DIR/lib/reporting.sh"
  HARNESS_EVIDENCE_ROOT="$ATOMIC_SOURCE_ROOT/evidence"
  HARNESS_RUN_DIR=""
  if harness_atomic_copy \
    "$ATOMIC_SOURCE_ROOT/outside/source.json" \
    "$ATOMIC_SOURCE_ROOT/evidence/copied.json" \
    "$ATOMIC_SOURCE_ROOT/evidence" >/dev/null 2>&1; then
    exit 1
  fi
)
[[ ! -e "$ATOMIC_SOURCE_ROOT/evidence/copied.json" ]] \
  || { echo "Atomic publication accepted an unset source trust root." >&2; exit 1; }

FAKE_AWK_ROOT="$TEST_ROOT/fake-awk"
mkdir -p "$FAKE_AWK_ROOT/bin"
cat >"$FAKE_AWK_ROOT/bin/awk" <<'EOF'
#!/usr/bin/env sh
exit 42
EOF
chmod +x "$FAKE_AWK_ROOT/bin/awk"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$FAKE_AWK_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  PATH="$FAKE_AWK_ROOT/bin:$PATH" \
  "$HARNESS" verify
[[ -e "$FAKE_AWK_ROOT/stage-ran" ]] \
  || { echo "Fake-awk fixture did not reach report serialization." >&2; exit 1; }
[[ ! -e "$FAKE_AWK_ROOT/test-results/harness/verify.json" \
  && ! -e "$FAKE_AWK_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "A failed report serializer published incomplete evidence." >&2; exit 1; }

FAKE_GIT_ROOT="$TEST_ROOT/fake-git"
mkdir -p "$FAKE_GIT_ROOT/bin"
cat >"$FAKE_GIT_ROOT/bin/git" <<'EOF'
#!/usr/bin/env sh
case "$*" in
  *"rev-parse --is-inside-work-tree"*) printf 'true\n' ;;
  *"rev-parse HEAD"*) printf '1111111111111111111111111111111111111111\n' ;;
  *"status --porcelain=v1"*) printf ' M tracked.txt\0' ;;
  *" diff "*) exit 42 ;;
  *"ls-files --others"*) exit 0 ;;
  *"--version"*) printf 'git version selftest\n' ;;
  *) exit 42 ;;
esac
EOF
chmod +x "$FAKE_GIT_ROOT/bin/git"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$FAKE_GIT_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  PATH="$FAKE_GIT_ROOT/bin:$PATH" \
  "$HARNESS" verify
[[ ! -e "$FAKE_GIT_ROOT/stage-ran" \
  && ! -e "$FAKE_GIT_ROOT/test-results/harness/verify.json" \
  && ! -e "$FAKE_GIT_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "A failed repository snapshot did not fail closed." >&2; exit 1; }

TERM_ROOT="$TEST_ROOT/term-cleanup"
TERM_TMP="$TERM_ROOT/tmp"
mkdir -p "$TERM_TMP"
TERM_SECRET='termination-secret-token-123456'
env \
  TMPDIR="$TERM_TMP" \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$TERM_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_REDACT_ENV_VARS=TERM_SECRET \
  TERM_SECRET="$TERM_SECRET" \
  HARNESS_TEST_COMMAND="sleep 1 # $TERM_SECRET" \
  "$HARNESS" verify >"$TERM_ROOT/run.log" 2>&1 &
TERM_PID=$!
TERM_SEEN=false
TERM_POLL=0
while [[ "$TERM_POLL" -lt 200 ]]; do
  if find "$TERM_TMP" -name 'command.*' -type f | grep -q .; then
    TERM_SEEN=true
    break
  fi
  kill -0 "$TERM_PID" 2>/dev/null || break
  sleep 0.01
  TERM_POLL=$((TERM_POLL + 1))
done
[[ "$TERM_SEEN" == true ]] || { echo "Termination fixture did not observe a command temporary." >&2; exit 1; }
kill -TERM "$TERM_PID"
TERM_STATUS=0
wait "$TERM_PID" || TERM_STATUS=$?
[[ "$TERM_STATUS" -eq 143 ]] || { echo "Terminated Harness returned $TERM_STATUS instead of 143." >&2; exit 1; }
if find "$TERM_TMP" -name 'portable-harness-process.*' -print | grep -q .; then
  echo "Catchable termination left the private Harness process directory." >&2
  exit 1
fi
[[ ! -e "$TERM_ROOT/test-results/harness/verify.json" \
  && ! -e "$TERM_ROOT/test-results/harness/latest/verify.json" ]] \
  || { echo "Terminated Harness published passing or authoritative evidence." >&2; exit 1; }

REPORT_TRAVERSAL_ROOT="$TEST_ROOT/report-traversal"
mkdir -p "$REPORT_TRAVERSAL_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$REPORT_TRAVERSAL_ROOT" \
  HARNESS_REPORT_DIR=../outside-reports \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  "$HARNESS" verify
[[ ! -e "$REPORT_TRAVERSAL_ROOT/stage-ran" ]] || { echo "Stage ran with an escaping report directory." >&2; exit 1; }
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$REPORT_TRAVERSAL_ROOT" \
  HARNESS_REPORT_FILE=../outside.json \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch report-file-stage-ran' \
  "$HARNESS" verify
[[ ! -e "$REPORT_TRAVERSAL_ROOT/report-file-stage-ran" ]] \
  || { echo "Stage ran with an escaping compatibility report filename." >&2; exit 1; }

REPORT_SYMLINK_ROOT="$TEST_ROOT/report-symlink"
REPORT_SYMLINK_OUTSIDE="$TEST_ROOT/report-symlink-outside"
mkdir -p "$REPORT_SYMLINK_ROOT/test-results" "$REPORT_SYMLINK_OUTSIDE"
printf 'preserve\n' >"$REPORT_SYMLINK_OUTSIDE/sentinel"
ln -s "$REPORT_SYMLINK_OUTSIDE" "$REPORT_SYMLINK_ROOT/test-results/harness"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$REPORT_SYMLINK_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  "$HARNESS" verify
grep -Fq preserve "$REPORT_SYMLINK_OUTSIDE/sentinel"
[[ ! -e "$REPORT_SYMLINK_ROOT/stage-ran" ]] || { echo "Stage ran through a symlink report directory." >&2; exit 1; }

SUMMARY_SYMLINK_ROOT="$TEST_ROOT/summary-symlink"
mkdir -p "$SUMMARY_SYMLINK_ROOT/test-results/harness/latest"
printf 'preserve\n' >"$TEST_ROOT/summary-sentinel"
ln -s "$TEST_ROOT/summary-sentinel" "$SUMMARY_SYMLINK_ROOT/test-results/harness/verify.json"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$SUMMARY_SYMLINK_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  "$HARNESS" verify
grep -Fq preserve "$TEST_ROOT/summary-sentinel"
[[ ! -e "$SUMMARY_SYMLINK_ROOT/stage-ran" ]] || { echo "Stage ran before unsafe summary evidence failed." >&2; exit 1; }

LATEST_SYMLINK_ROOT="$TEST_ROOT/latest-symlink"
mkdir -p "$LATEST_SYMLINK_ROOT/test-results/harness/latest"
printf 'preserve-latest\n' >"$TEST_ROOT/latest-sentinel"
ln -s "$TEST_ROOT/latest-sentinel" "$LATEST_SYMLINK_ROOT/test-results/harness/latest/verify.json"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$LATEST_SYMLINK_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='touch stage-ran' \
  "$HARNESS" verify
grep -Fq preserve-latest "$TEST_ROOT/latest-sentinel"
[[ ! -e "$LATEST_SYMLINK_ROOT/stage-ran" ]] \
  || { echo "Stage ran before unsafe latest evidence failed." >&2; exit 1; }

LATE_SUMMARY_ROOT="$TEST_ROOT/late-summary-symlink"
LATE_SUMMARY_SENTINEL="$TEST_ROOT/late-summary-sentinel"
mkdir -p "$LATE_SUMMARY_ROOT"
printf 'preserve-late-summary\n' >"$LATE_SUMMARY_SENTINEL"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$LATE_SUMMARY_ROOT" \
HARNESS_REQUIRED_STAGES=none \
"$HARNESS" verify >/dev/null
LATE_SUMMARY_LATEST_BEFORE="$(file_sha256 "$LATE_SUMMARY_ROOT/test-results/harness/latest/verify.json")"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$LATE_SUMMARY_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  LATE_SUMMARY_SENTINEL="$LATE_SUMMARY_SENTINEL" \
  HARNESS_TEST_COMMAND='rm -f test-results/harness/verify.json; ln -s "$LATE_SUMMARY_SENTINEL" test-results/harness/verify.json' \
  "$HARNESS" verify
grep -Fq preserve-late-summary "$LATE_SUMMARY_SENTINEL"
[[ "$(file_sha256 "$LATE_SUMMARY_ROOT/test-results/harness/latest/verify.json")" == "$LATE_SUMMARY_LATEST_BEFORE" ]] \
  || { echo "Latest advanced after a late unsafe summary path." >&2; exit 1; }

PARALLEL_REPORT_ROOT="$TEST_ROOT/parallel-reports"
mkdir -p "$PARALLEL_REPORT_ROOT"
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$PARALLEL_REPORT_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='sleep 1' \
  "$HARNESS" verify >"$PARALLEL_REPORT_ROOT/run-1.log" &
PARALLEL_PID_ONE=$!
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$PARALLEL_REPORT_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND='sleep 1' \
  "$HARNESS" verify >"$PARALLEL_REPORT_ROOT/run-2.log" &
PARALLEL_PID_TWO=$!
wait "$PARALLEL_PID_ONE"
wait "$PARALLEL_PID_TWO"
PARALLEL_REPORT_COUNT="$(find "$PARALLEL_REPORT_ROOT/test-results/harness/runs" -mindepth 2 -maxdepth 2 -name report.json -type f | awk 'END {print NR+0}')"
[[ "$PARALLEL_REPORT_COUNT" -eq 2 ]] || { echo "Parallel verify runs shared canonical storage." >&2; exit 1; }
while IFS= read -r PARALLEL_REPORT; do
  validate_json_file "$PARALLEL_REPORT"
done < <(find "$PARALLEL_REPORT_ROOT/test-results/harness/runs" -mindepth 2 -maxdepth 2 -name report.json -type f)
validate_json_file "$PARALLEL_REPORT_ROOT/test-results/harness/verify.json"
validate_json_file "$PARALLEL_REPORT_ROOT/test-results/harness/latest/verify.json"
PARALLEL_LATEST="$PARALLEL_REPORT_ROOT/test-results/harness/latest/verify.json"
PARALLEL_LATEST_RUN_ID="$(sed -n 's/^[[:space:]]*"run_id": "\([^"]*\)",$/\1/p' "$PARALLEL_LATEST")"
PARALLEL_LATEST_PATH="$(sed -n 's/^[[:space:]]*"report_path": "\([^"]*\)",$/\1/p' "$PARALLEL_LATEST")"
PARALLEL_LATEST_DIGEST="$(sed -n 's/^[[:space:]]*"report_digest": "sha256:\([0-9a-f]*\)",$/\1/p' "$PARALLEL_LATEST")"
PARALLEL_LATEST_SIZE="$(sed -n 's/^[[:space:]]*"report_size_bytes": \([0-9]*\),$/\1/p' "$PARALLEL_LATEST")"
PARALLEL_LATEST_REPORT="$PARALLEL_REPORT_ROOT/test-results/harness/$PARALLEL_LATEST_PATH"
[[ -n "$PARALLEL_LATEST_RUN_ID" \
  && "$PARALLEL_LATEST_PATH" == "runs/$PARALLEL_LATEST_RUN_ID/report.json" \
  && -f "$PARALLEL_LATEST_REPORT" \
  && -f "${PARALLEL_LATEST_REPORT%/report.json}/status.complete" \
  && "$PARALLEL_LATEST_DIGEST" == "$(file_sha256 "$PARALLEL_LATEST_REPORT")" \
  && "$PARALLEL_LATEST_SIZE" == "$(LC_ALL=C wc -c <"$PARALLEL_LATEST_REPORT" | tr -d '[:space:]')" ]] \
  || { echo "Parallel latest pointer does not identify one complete canonical report." >&2; exit 1; }
grep -Fq '"run_type": "verify"' "$PARALLEL_LATEST"

FAIL_REPORT_ROOT="$TEST_ROOT/failure-report"
mkdir -p "$FAIL_REPORT_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$FAIL_REPORT_ROOT" \
  HARNESS_REQUIRED_STAGES=test \
  HARNESS_TEST_COMMAND=false \
  "$HARNESS" verify
[[ -f "$FAIL_REPORT_ROOT/test-results/harness/verify.json" ]] || { echo "Failure evidence was not written." >&2; exit 1; }
grep -Fq '"result": "failed"' "$FAIL_REPORT_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"test","status":"failed"' "$FAIL_REPORT_ROOT/test-results/harness/verify.json"

BASELINE_ROOT="$TEST_ROOT/baseline"
mkdir -p "$BASELINE_ROOT"
write_adopted_config "$BASELINE_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **invalid**\n' >"$BASELINE_ROOT/AGENTS.md"
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This fixture tests baseline adoption without a product change.\n- SDD exemption owner: Harness maintainers\n- SDD exemption follow-up: Re-enable SDD when this fixture covers a product change.\n' >"$BASELINE_ROOT/PROJECT.md"
cp "$BASELINE_ROOT/HARNESS/config.env" "$BASELINE_ROOT/HARNESS/config.valid"
printf '%s\n' 'exit 0' >"$BASELINE_ROOT/HARNESS/config.env"
expect_failure env HARNESS_ROOT_DIR="$BASELINE_ROOT" "$HARNESS" unknown-command
mv "$BASELINE_ROOT/HARNESS/config.valid" "$BASELINE_ROOT/HARNESS/config.env"
expect_failure env HARNESS_CONFIG_FILE="$BASELINE_ROOT/HARNESS/config.env" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 "$HARNESS" doctor
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$BASELINE_ROOT/AGENTS.md"
HARNESS_CONFIG_FILE="$BASELINE_ROOT/HARNESS/config.env" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 "$HARNESS" doctor >/dev/null
cp "$BASELINE_ROOT/HARNESS/config.env" "$BASELINE_ROOT/HARNESS/config.saved"
printf '%s\n' "HARNESS_EXPECTED_BASELINE_VERSION=''" "HARNESS_CONFIG_COMPLETE='1.0.0'" >"$BASELINE_ROOT/HARNESS/config.env"
expect_failure env HARNESS_ROOT_DIR="$BASELINE_ROOT" "$HARNESS" doctor
mv "$BASELINE_ROOT/HARNESS/config.saved" "$BASELINE_ROOT/HARNESS/config.env"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 "$HARNESS" doctor
printf '# Project Context\n\n- Shared baseline version: `invalid`\n' >"$BASELINE_ROOT/PROJECT.md"
expect_failure env HARNESS_CONFIG_FILE="$BASELINE_ROOT/HARNESS/config.env" HARNESS_ROOT_DIR="$BASELINE_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 "$HARNESS" doctor
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This fixture tests baseline adoption without a product change.\n- SDD exemption owner: Harness maintainers\n- SDD exemption follow-up: Re-enable SDD when this fixture covers a product change.\n' >"$BASELINE_ROOT/PROJECT.md"

MISSING_PROJECT_ROOT="$TEST_ROOT/missing-project"
mkdir -p "$MISSING_PROJECT_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$MISSING_PROJECT_ROOT/AGENTS.md"
MISSING_PROJECT_MARKER="$MISSING_PROJECT_ROOT/environment-ran"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$MISSING_PROJECT_ROOT" \
  HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 \
  HARNESS_ENVIRONMENT_COMMAND='touch environment-ran' \
  "$HARNESS" doctor
[[ ! -e "$MISSING_PROJECT_MARKER" ]] \
  || { echo "Environment command ran before project contract validation." >&2; exit 1; }

INVALID_PROJECT_ROOT="$TEST_ROOT/invalid-project"
mkdir -p "$INVALID_PROJECT_ROOT/PROJECT.md"
printf '# AGENTS.md\n' >"$INVALID_PROJECT_ROOT/AGENTS.md"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$INVALID_PROJECT_ROOT" "$HARNESS" doctor
rm -rf "$INVALID_PROJECT_ROOT/PROJECT.md"
ln -s "$INVALID_PROJECT_ROOT/missing-target" "$INVALID_PROJECT_ROOT/PROJECT.md"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$INVALID_PROJECT_ROOT" "$HARNESS" doctor

TEMPLATE_PROJECT_ROOT="$TEST_ROOT/template-project"
mkdir -p "$TEMPLATE_PROJECT_ROOT"
write_adopted_config "$TEMPLATE_PROJECT_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$TEMPLATE_PROJECT_ROOT/AGENTS.md"
printf '# Project\n\nOwner: {{REPLACE_ME:owner}}\n' >"$TEMPLATE_PROJECT_ROOT/PROJECT.md"
expect_failure env HARNESS_CONFIG_FILE="$TEMPLATE_PROJECT_ROOT/HARNESS/config.env" HARNESS_ROOT_DIR="$TEMPLATE_PROJECT_ROOT" HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 "$HARNESS" doctor

MISSING_CONFIG_ROOT="$TEST_ROOT/missing-adopted-config"
mkdir -p "$MISSING_CONFIG_ROOT/HARNESS"
printf '# AGENTS.md\n' >"$MISSING_CONFIG_ROOT/AGENTS.md"
printf '# Project Context\n' >"$MISSING_CONFIG_ROOT/PROJECT.md"
expect_failure env \
  HARNESS_ROOT_DIR="$MISSING_CONFIG_ROOT" \
  "$HARNESS" doctor

EXAMPLE_CONFIG_ROOT="$TEST_ROOT/example-config"
mkdir -p "$EXAMPLE_CONFIG_ROOT/HARNESS"
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$EXAMPLE_CONFIG_ROOT/AGENTS.md"
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `required`\n- SDD exemption reason: Not applicable while SDD mode is required.\n' >"$EXAMPLE_CONFIG_ROOT/PROJECT.md"
printf '# Harness contract\n' >"$EXAMPLE_CONFIG_ROOT/HARNESS/HARNESS.md"
cp "$HARNESS_DIR/config.env.example" "$EXAMPLE_CONFIG_ROOT/HARNESS/config.env"
HARNESS_CONFIG_FILE="$EXAMPLE_CONFIG_ROOT/HARNESS/config.env" HARNESS_ROOT_DIR="$EXAMPLE_CONFIG_ROOT" "$HARNESS" doctor >/dev/null

STRICT_SDD_ROOT="$TEST_ROOT/strict-sdd"
mkdir -p "$STRICT_SDD_ROOT/HARNESS" "$STRICT_SDD_ROOT/plans"
cat >"$STRICT_SDD_ROOT/HARNESS/config.env" <<'EOF'
HARNESS_EXPECTED_BASELINE_VERSION='1.0.0'
HARNESS_SDD_MODE='required'
HARNESS_REQUIRED_STAGES='test'
HARNESS_VERIFY_STAGES='test'
HARNESS_GOVERNANCE_MODE='required'
HARNESS_REQUIRE_PLAN_APPROVAL='true'
HARNESS_ACTIVE_CHANGE_FILE='HARNESS/ACTIVE_CHANGE'
HARNESS_SPEC_FILE='SPECIFICATION.md'
HARNESS_CHANGELOG_FILE='SPEC_CHANGELOG.md'
HARNESS_PLAN_FILE='DEVELOPMENT_PLAN.md'
HARNESS_TEST_PLAN_FILE='TEST_PLAN.md'
HARNESS_TRACE_FILE='TRACEABILITY.md'
HARNESS_TEST_COMMAND='test -f src/sdd-contract.sh && test -f src/reporting-contract.sh && test -f tests/sdd-contract.test'
HARNESS_CONFIG_COMPLETE='1.0.0'
EOF
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$STRICT_SDD_ROOT/AGENTS.md"
cat >"$STRICT_SDD_ROOT/PROJECT.md" <<'EOF'
# Project Context

- Shared baseline version: `1.0.0`
- SDD mode: `required`
- SDD exemption reason: Not applicable while SDD mode is required.
EOF
printf 'CHG-101\n' >"$STRICT_SDD_ROOT/HARNESS/ACTIVE_CHANGE"
cat >"$STRICT_SDD_ROOT/SPECIFICATION.md" <<'EOF'
# Specification

## CHG-101 Strict SDD fixture

### Specification Metadata
- Specification Version: 1.0.0
- Status: active
- Owner: Example Product Owner
- Clarification Status: resolved

### Problem Statement
The handoff must prove one observable behavior.

### Goals
- Provide a deterministic verified outcome.

### Non-goals
- Production deployment is outside this fixture.

### Actors and Scenarios
- A maintainer verifies the portable SDD contract.

### Requirement Index
| Requirement ID | Priority | Normative requirement | Acceptance IDs |
|---|---|---|---|
| REQ-CHG-101-001 | must | The Harness MUST validate the fixture. | AC-CHG-101-001 |
| REQ-CHG-101-002 | should | The Harness SHOULD retain SDD identity evidence. | AC-CHG-101-002 |

### Acceptance Criteria
| Acceptance ID | Requirement ID | Given | When | Then |
|---|---|---|---|---|
| AC-CHG-101-001 | REQ-CHG-101-001 | an approved fixture | verify runs | the contract passes |
| AC-CHG-101-002 | REQ-CHG-101-002 | a verified contract | a report is published | SDD identities are retained |

### Interfaces, Data, and Failure Behavior
The fixture uses repository-local Markdown and fails closed.

### Quality Attributes
Validation is deterministic and does not use secrets or network access.

### Compatibility and Migration
This baseline contract has no production migration.

### Assumptions
- SHA-256 tooling is available.

### Open Questions
- None.
EOF
cat >"$STRICT_SDD_ROOT/SPEC_CHANGELOG.md" <<'EOF'
# Specification Changelog

| Date | Change ID | Specification version | Spec section | Reason | Compatibility/migration | Test impact |
|---|---|---|---|---|---|---|
| 2026-08-13 | CHG-101 | 1.0.0 | Strict SDD fixture | Establish contract | Baseline fixture | TEST-CHG-101-001 |
EOF
cat >"$STRICT_SDD_ROOT/TEST_PLAN.md" <<'EOF'
# Test Plan

## CHG-101 Test Design

| Test ID | Acceptance ID | Requirement ID | Test implementation | Level | Fixture/data | Expected pre-implementation state | Harness stage |
|---|---|---|---|---|---|---|---|
| TEST-CHG-101-001 | AC-CHG-101-001 | REQ-CHG-101-001 | path:tests/sdd-contract.test | contract | local fixture | unsupported before adoption | test |
| TEST-CHG-101-002 | AC-CHG-101-002 | REQ-CHG-101-002 | path:tests/sdd-contract.test | schema | generated report | SDD identity absent before adoption | test |

### Failure and Boundary Coverage
- Missing IDs, unresolved questions, stale digests, and unverified traces fail.
EOF
cat >"$STRICT_SDD_ROOT/TRACEABILITY.md" <<'EOF'
# Traceability

## CHG-101 Requirement Trace

| Requirement ID | Specification version | Acceptance IDs | Design decision | Task IDs | Implementation | Test IDs | Harness stage | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| REQ-CHG-101-001 | 1.0.0 | AC-CHG-101-001 | strict Markdown contract | TASK-CHG-101-001 | path:src/sdd-contract.sh | TEST-CHG-101-001 | test | stage:test | Planned |
| REQ-CHG-101-002 | 1.0.0 | AC-CHG-101-002 | report identity contract | TASK-CHG-101-002 | path:src/reporting-contract.sh | TEST-CHG-101-002 | test | stage:test | Planned |
EOF
cat >"$STRICT_SDD_ROOT/plans/CHG-101.md" <<'EOF'
# Verify CHG-101 strict SDD

## Purpose and Observable Outcome
The Harness rejects incomplete SDD state and accepts this complete fixture.

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->
## Approved Plan Basis

### Requirement Coverage
REQ-CHG-101-001 is satisfied by TASK-CHG-101-001.
REQ-CHG-101-002 is satisfied by TASK-CHG-101-002.

### Technical Design
Use strict repository-contained Markdown records and fail-closed ID mapping.

### Task Breakdown
| Task ID | Requirement IDs | Dependencies | Files/components | Verification |
|---|---|---|---|---|
| TASK-CHG-101-001 | REQ-CHG-101-001 | none | HARNESS/lib/sdd.sh | TEST-CHG-101-001 |
| TASK-CHG-101-002 | REQ-CHG-101-002 | TASK-CHG-101-001 | HARNESS/lib/reporting.sh | TEST-CHG-101-002 |

### Verification Basis
AC-CHG-101-001 will be verified by TEST-CHG-101-001 in test.
AC-CHG-101-002 will be verified by TEST-CHG-101-002 in test.
<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress
- [x] Fixture completed with retained selftest evidence.

## Surprises and Discoveries
- None.

## Decision Log
- Keep the human-readable contract portable to Bash 3.2.

## Recovery and Idempotence
The fixture is isolated and safely repeatable.

## Outcomes and Retrospective
The strict SDD contract is covered by positive and negative tests.
EOF
cat >"$STRICT_SDD_ROOT/DEVELOPMENT_PLAN.md" <<'EOF'
# Development Plan

## CHG-101 Strict SDD fixture

### Approval Record
- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Example Product Owner
- Requirements Confirmer Role: Product Owner
- Requirements Confirmed On: 2026-08-13
- Requirements Confirmation Evidence: review/requirements-CHG-101
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Example Maintainer
- Implementation Plan Approver Role: Maintainer
- Implementation Plan Approved On: 2026-08-13
- Implementation Plan Approval Evidence: review/plan-CHG-101
- Approved Specification Version: pending
- Approved Specification Digest: pending
- Approved Plan Basis Digest: pending
- Approved Test Plan Digest: pending

### Execution Plan Reference
- Execution Plan: plans/CHG-101.md
EOF
STRICT_SDD_DIGESTS="$(HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:digest)"
STRICT_SDD_VERSION="$(printf '%s\n' "$STRICT_SDD_DIGESTS" | sed -n 's/^Approved Specification Version: //p')"
STRICT_SDD_SPEC_DIGEST="$(printf '%s\n' "$STRICT_SDD_DIGESTS" | sed -n 's/^Approved Specification Digest: //p')"
STRICT_SDD_PLAN_DIGEST="$(printf '%s\n' "$STRICT_SDD_DIGESTS" | sed -n 's/^Approved Plan Basis Digest: //p')"
STRICT_SDD_TEST_DIGEST="$(printf '%s\n' "$STRICT_SDD_DIGESTS" | sed -n 's/^Approved Test Plan Digest: //p')"
[[ -n "$STRICT_SDD_VERSION" && "$STRICT_SDD_SPEC_DIGEST" == sha256:* \
  && "$STRICT_SDD_PLAN_DIGEST" == sha256:* && "$STRICT_SDD_TEST_DIGEST" == sha256:* ]] \
  || { echo "Strict SDD digests were not generated." >&2; exit 1; }
sed -e "s/Approved Specification Version: pending/Approved Specification Version: $STRICT_SDD_VERSION/" \
  -e "s/Approved Specification Digest: pending/Approved Specification Digest: $STRICT_SDD_SPEC_DIGEST/" \
  -e "s/Approved Plan Basis Digest: pending/Approved Plan Basis Digest: $STRICT_SDD_PLAN_DIGEST/" \
  -e "s/Approved Test Plan Digest: pending/Approved Test Plan Digest: $STRICT_SDD_TEST_DIGEST/" \
  "$STRICT_SDD_ROOT/DEVELOPMENT_PLAN.md" >"$STRICT_SDD_ROOT/DEVELOPMENT_PLAN.updated"
mv "$STRICT_SDD_ROOT/DEVELOPMENT_PLAN.updated" "$STRICT_SDD_ROOT/DEVELOPMENT_PLAN.md"
HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" governance:approved >/dev/null
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mkdir -p "$STRICT_SDD_ROOT/src" "$STRICT_SDD_ROOT/tests"
printf '%s\n' '# implementation fixture' >"$STRICT_SDD_ROOT/src/sdd-contract.sh"
printf '%s\n' '# reporting fixture' >"$STRICT_SDD_ROOT/src/reporting-contract.sh"
printf '%s\n' '# executable acceptance-test fixture' >"$STRICT_SDD_ROOT/tests/sdd-contract.test"
sed 's/| Planned |/| Verified |/' "$STRICT_SDD_ROOT/TRACEABILITY.md" \
  >"$STRICT_SDD_ROOT/TRACEABILITY.updated"
mv "$STRICT_SDD_ROOT/TRACEABILITY.updated" "$STRICT_SDD_ROOT/TRACEABILITY.md"
HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check >/dev/null
HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" verify >/dev/null
grep -Fq '"schema_version": "1"' "$STRICT_SDD_ROOT/test-results/harness/verify.json"
grep -Fq '"sdd": {"mode":"required","contract_status":"verified","change_id":"CHG-101"' \
  "$STRICT_SDD_ROOT/test-results/harness/verify.json"
grep -Fq "\"specification_digest\":\"$STRICT_SDD_SPEC_DIGEST\"" \
  "$STRICT_SDD_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"test","status":"passed"' \
  "$STRICT_SDD_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"sdd-check","status":"passed"' \
  "$STRICT_SDD_ROOT/test-results/harness/verify.json"

mv "$STRICT_SDD_ROOT/src/sdd-contract.sh" "$STRICT_SDD_ROOT/src/sdd-contract.missing"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/src/sdd-contract.missing" "$STRICT_SDD_ROOT/src/sdd-contract.sh"
mv "$STRICT_SDD_ROOT/tests/sdd-contract.test" "$STRICT_SDD_ROOT/tests/sdd-contract.missing"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/tests/sdd-contract.missing" "$STRICT_SDD_ROOT/tests/sdd-contract.test"
mv "$STRICT_SDD_ROOT/tests/sdd-contract.test" "$STRICT_SDD_ROOT/tests/sdd-contract.saved"
mkdir "$STRICT_SDD_ROOT/tests/sdd-contract.test"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
rmdir "$STRICT_SDD_ROOT/tests/sdd-contract.test"
mv "$STRICT_SDD_ROOT/tests/sdd-contract.saved" "$STRICT_SDD_ROOT/tests/sdd-contract.test"
cp "$STRICT_SDD_ROOT/TRACEABILITY.md" "$STRICT_SDD_ROOT/TRACEABILITY.valid"
sed 's/| test | stage:test | Verified |/| made-up-stage | stage:made-up-stage | Verified |/' \
  "$STRICT_SDD_ROOT/TRACEABILITY.valid" >"$STRICT_SDD_ROOT/TRACEABILITY.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/TRACEABILITY.valid" "$STRICT_SDD_ROOT/TRACEABILITY.md"
cp "$STRICT_SDD_ROOT/TRACEABILITY.md" "$STRICT_SDD_ROOT/TRACEABILITY.valid"
sed 's/| test | stage:test | Verified |/| test | stage:build | Verified |/' \
  "$STRICT_SDD_ROOT/TRACEABILITY.valid" >"$STRICT_SDD_ROOT/TRACEABILITY.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/TRACEABILITY.valid" "$STRICT_SDD_ROOT/TRACEABILITY.md"
cp "$STRICT_SDD_ROOT/HARNESS/config.env" "$STRICT_SDD_ROOT/HARNESS/config.valid"
cp "$STRICT_SDD_ROOT/TRACEABILITY.md" "$STRICT_SDD_ROOT/TRACEABILITY.valid"
sed "s/HARNESS_VERIFY_STAGES='test'/HARNESS_VERIFY_STAGES='test,integration'/" \
  "$STRICT_SDD_ROOT/HARNESS/config.valid" >"$STRICT_SDD_ROOT/HARNESS/config.env"
sed 's/| test | stage:test | Verified |/| test,integration | stage:test,stage:integration | Verified |/' \
  "$STRICT_SDD_ROOT/TRACEABILITY.valid" >"$STRICT_SDD_ROOT/TRACEABILITY.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/HARNESS/config.valid" "$STRICT_SDD_ROOT/HARNESS/config.env"
mv "$STRICT_SDD_ROOT/TRACEABILITY.valid" "$STRICT_SDD_ROOT/TRACEABILITY.md"
mv "$STRICT_SDD_ROOT/tests/sdd-contract.test" "$STRICT_SDD_ROOT/tests/sdd-contract.missing"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" verify
mv "$STRICT_SDD_ROOT/tests/sdd-contract.missing" "$STRICT_SDD_ROOT/tests/sdd-contract.test"

cp "$STRICT_SDD_ROOT/SPECIFICATION.md" "$STRICT_SDD_ROOT/SPECIFICATION.valid"
sed 's/- None\./- Which behavior should be selected?/' "$STRICT_SDD_ROOT/SPECIFICATION.valid" \
  >"$STRICT_SDD_ROOT/SPECIFICATION.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/SPECIFICATION.valid" "$STRICT_SDD_ROOT/SPECIFICATION.md"
cp "$STRICT_SDD_ROOT/SPEC_CHANGELOG.md" "$STRICT_SDD_ROOT/SPEC_CHANGELOG.valid"
sed 's/| 1.0.0 | Strict SDD/| invalid | Strict SDD/' "$STRICT_SDD_ROOT/SPEC_CHANGELOG.valid" \
  >"$STRICT_SDD_ROOT/SPEC_CHANGELOG.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/SPEC_CHANGELOG.valid" "$STRICT_SDD_ROOT/SPEC_CHANGELOG.md"
cp "$STRICT_SDD_ROOT/TEST_PLAN.md" "$STRICT_SDD_ROOT/TEST_PLAN.valid"
sed 's/Missing IDs, unresolved questions/Missing IDs, duplicate IDs, unresolved questions/' \
  "$STRICT_SDD_ROOT/TEST_PLAN.valid" >"$STRICT_SDD_ROOT/TEST_PLAN.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/TEST_PLAN.valid" "$STRICT_SDD_ROOT/TEST_PLAN.md"
cp "$STRICT_SDD_ROOT/TRACEABILITY.md" "$STRICT_SDD_ROOT/TRACEABILITY.valid"
sed 's/| Verified |/| Implemented |/' "$STRICT_SDD_ROOT/TRACEABILITY.valid" \
  >"$STRICT_SDD_ROOT/TRACEABILITY.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" verify
mv "$STRICT_SDD_ROOT/TRACEABILITY.valid" "$STRICT_SDD_ROOT/TRACEABILITY.md"
cp "$STRICT_SDD_ROOT/TRACEABILITY.md" "$STRICT_SDD_ROOT/TRACEABILITY.valid"
sed -e 's/TASK-CHG-101-001/TASK-CHG-101-TEMP/' \
  -e 's/TASK-CHG-101-002/TASK-CHG-101-001/' \
  -e 's/TASK-CHG-101-TEMP/TASK-CHG-101-002/' \
  "$STRICT_SDD_ROOT/TRACEABILITY.valid" >"$STRICT_SDD_ROOT/TRACEABILITY.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/TRACEABILITY.valid" "$STRICT_SDD_ROOT/TRACEABILITY.md"
cp "$STRICT_SDD_ROOT/plans/CHG-101.md" "$STRICT_SDD_ROOT/plans/CHG-101.valid"
sed 's/strict repository-contained Markdown records/changed approved contract/' "$STRICT_SDD_ROOT/plans/CHG-101.valid" \
  >"$STRICT_SDD_ROOT/plans/CHG-101.md"
expect_failure env HARNESS_ROOT_DIR="$STRICT_SDD_ROOT" "$HARNESS" sdd:check
mv "$STRICT_SDD_ROOT/plans/CHG-101.valid" "$STRICT_SDD_ROOT/plans/CHG-101.md"

SDD_OFF_REASON_ROOT="$TEST_ROOT/sdd-off-reason"
mkdir -p "$SDD_OFF_REASON_ROOT"
write_adopted_config "$SDD_OFF_REASON_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$SDD_OFF_REASON_ROOT/AGENTS.md"
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: None\n- SDD exemption owner: Harness maintainers\n- SDD exemption follow-up: Review after this negative fixture.\n' >"$SDD_OFF_REASON_ROOT/PROJECT.md"
expect_failure env HARNESS_ROOT_DIR="$SDD_OFF_REASON_ROOT" "$HARNESS" doctor
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This repository is documentation-only.\n- SDD exemption owner: None\n- SDD exemption follow-up: Review the exception annually.\n' >"$SDD_OFF_REASON_ROOT/PROJECT.md"
expect_failure env HARNESS_ROOT_DIR="$SDD_OFF_REASON_ROOT" "$HARNESS" doctor
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This repository is documentation-only.\n- SDD exemption owner: Documentation maintainers\n- SDD exemption follow-up: Pending\n' >"$SDD_OFF_REASON_ROOT/PROJECT.md"
expect_failure env HARNESS_ROOT_DIR="$SDD_OFF_REASON_ROOT" "$HARNESS" doctor

INSTRUCTION_ROOT="$TEST_ROOT/instructions"
INSTRUCTION_TARGET="$INSTRUCTION_ROOT/packages/path with spaces/src"
mkdir -p "$INSTRUCTION_TARGET"
printf '# Root instructions\n' >"$INSTRUCTION_ROOT/AGENTS.md"
touch "$INSTRUCTION_ROOT/AGENTS.override.md"
printf '# Shadowed package instructions\n' >"$INSTRUCTION_ROOT/packages/AGENTS.md"
printf '# Package override\n' >"$INSTRUCTION_ROOT/packages/AGENTS.override.md"
printf '# Fallback instructions\n' >"$INSTRUCTION_TARGET/TEAM.md"
INSTRUCTION_OUTPUT="$TEST_ROOT/instructions.out"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
HARNESS_INSTRUCTION_FALLBACK_FILENAMES=TEAM.md \
"$HARNESS" instructions:doctor "packages/path with spaces/src" >"$INSTRUCTION_OUTPUT"
grep -Fq 'selected: ./AGENTS.md' "$INSTRUCTION_OUTPUT"
grep -Fq 'empty: ./AGENTS.override.md' "$INSTRUCTION_OUTPUT"
grep -Fq 'selected: packages/AGENTS.override.md' "$INSTRUCTION_OUTPUT"
grep -Fq 'warning: packages/AGENTS.md is shadowed' "$INSTRUCTION_OUTPUT"
grep -Fq 'selected: packages/path with spaces/src/TEAM.md' "$INSTRUCTION_OUTPUT"
ROOT_AGENT_BYTES="$(wc -c <"$INSTRUCTION_ROOT/AGENTS.md" | tr -d '[:space:]')"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
HARNESS_INSTRUCTION_MAX_BYTES="$ROOT_AGENT_BYTES" \
"$HARNESS" instructions:doctor >/dev/null
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
  HARNESS_INSTRUCTION_MAX_BYTES="$((ROOT_AGENT_BYTES - 1))" \
  "$HARNESS" instructions:doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
  HARNESS_INSTRUCTION_MAX_BYTES=10 \
  "$HARNESS" instructions:doctor "packages/path with spaces/src"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
  HARNESS_INSTRUCTION_FALLBACK_FILENAMES='../TEAM.md' \
  "$HARNESS" instructions:doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
  HARNESS_INSTRUCTION_REFERENCES=missing.md \
  "$HARNESS" instructions:doctor
if ln -s "$EMPTY_ROOT" "$INSTRUCTION_ROOT/outside" 2>/dev/null; then
  expect_failure env \
    HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
    HARNESS_ROOT_DIR="$INSTRUCTION_ROOT" \
    "$HARNESS" instructions:doctor outside
fi

SYMLINK_CONTRACT_ROOT="$TEST_ROOT/symlink-contract"
mkdir -p "$SYMLINK_CONTRACT_ROOT"
printf '# External instructions\n' >"$TEST_ROOT/external-instructions.md"
printf '# AGENTS.md\n' >"$SYMLINK_CONTRACT_ROOT/AGENTS.real.md"
ln -s "$TEST_ROOT/external-instructions.md" "$SYMLINK_CONTRACT_ROOT/AGENTS.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$SYMLINK_CONTRACT_ROOT" \
  "$HARNESS" instructions:doctor
rm "$SYMLINK_CONTRACT_ROOT/AGENTS.md"
mv "$SYMLINK_CONTRACT_ROOT/AGENTS.real.md" "$SYMLINK_CONTRACT_ROOT/AGENTS.md"
ln -s "$TEST_ROOT/external-instructions.md" "$SYMLINK_CONTRACT_ROOT/EXTERNAL.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$SYMLINK_CONTRACT_ROOT" \
  HARNESS_INSTRUCTION_REFERENCES=EXTERNAL.md \
  "$HARNESS" instructions:doctor
ln -s "$TEST_ROOT/external-instructions.md" "$SYMLINK_CONTRACT_ROOT/PROJECT.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$SYMLINK_CONTRACT_ROOT" \
  "$HARNESS" instructions:doctor

NESTED_LIMIT_ROOT="$TEST_ROOT/nested-limit"
mkdir -p "$NESTED_LIMIT_ROOT/apps/deep"
write_adopted_config "$NESTED_LIMIT_ROOT"
cat >"$NESTED_LIMIT_ROOT/HARNESS/config.env" <<'EOF'
HARNESS_EXPECTED_BASELINE_VERSION='1.0.0'
HARNESS_SDD_MODE='off'
HARNESS_INSTRUCTION_TARGETS='apps/deep'
HARNESS_INSTRUCTION_MAX_BYTES='20'
HARNESS_CONFIG_COMPLETE='1.0.0'
EOF
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$NESTED_LIMIT_ROOT/AGENTS.md"
printf '# Nested content larger than configured chain limit\n' >"$NESTED_LIMIT_ROOT/apps/deep/AGENTS.md"
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This fixture tests nested instruction limits only.\n- SDD exemption owner: Harness maintainers\n- SDD exemption follow-up: Re-enable SDD outside this isolated fixture.\n' >"$NESTED_LIMIT_ROOT/PROJECT.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$NESTED_LIMIT_ROOT/HARNESS/config.env" \
  HARNESS_ROOT_DIR="$NESTED_LIMIT_ROOT" \
  HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 \
  HARNESS_INSTRUCTION_TARGETS=apps/deep \
  HARNESS_INSTRUCTION_MAX_BYTES=20 \
  "$HARNESS" doctor

ROOT_OVERRIDE_ROOT="$TEST_ROOT/root-override"
mkdir -p "$ROOT_OVERRIDE_ROOT"
write_adopted_config "$ROOT_OVERRIDE_ROOT"
printf '# AGENTS.md\n\nShared baseline version: **1.0.0**\n' >"$ROOT_OVERRIDE_ROOT/AGENTS.md"
printf '# Temporary root override\n' >"$ROOT_OVERRIDE_ROOT/AGENTS.override.md"
printf '# Project Context\n\n- Shared baseline version: `1.0.0`\n- SDD mode: `off`\n- SDD exemption reason: This fixture tests instruction override policy only.\n- SDD exemption owner: Harness maintainers\n- SDD exemption follow-up: Re-enable SDD outside this isolated fixture.\n' >"$ROOT_OVERRIDE_ROOT/PROJECT.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$ROOT_OVERRIDE_ROOT/HARNESS/config.env" \
  HARNESS_ROOT_DIR="$ROOT_OVERRIDE_ROOT" \
  HARNESS_EXPECTED_BASELINE_VERSION=1.0.0 \
  "$HARNESS" doctor
expect_failure env \
  HARNESS_ROOT_DIR="$ROOT_OVERRIDE_ROOT" \
  HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE=true \
  "$HARNESS" doctor
sed 's/HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE=.false./HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE='"'"'true'"'"'/' \
  "$ROOT_OVERRIDE_ROOT/HARNESS/config.env" >"$ROOT_OVERRIDE_ROOT/HARNESS/config.updated"
mv "$ROOT_OVERRIDE_ROOT/HARNESS/config.updated" "$ROOT_OVERRIDE_ROOT/HARNESS/config.env"
HARNESS_ROOT_DIR="$ROOT_OVERRIDE_ROOT" "$HARNESS" doctor >/dev/null

NO_INSTRUCTION_ROOT="$TEST_ROOT/no-instructions"
mkdir -p "$NO_INSTRUCTION_ROOT"
expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NO_INSTRUCTION_ROOT" "$HARNESS" instructions:doctor

CONFIG_ROOT="$TEST_ROOT/configured"
mkdir -p "$CONFIG_ROOT"
touch "$CONFIG_ROOT/expected"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$CONFIG_ROOT" \
HARNESS_REQUIRED_STAGES=test,integration,ci-policy,supply-chain \
  HARNESS_TEST_COMMAND='test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = false && test -f expected' \
  HARNESS_INTEGRATION_COMMAND='test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = true && test -f expected && touch integration-ran' \
HARNESS_CI_POLICY_COMMAND='test -f expected && touch ci-policy-ran' \
HARNESS_SUPPLY_CHAIN_COMMAND='test -f expected && touch supply-chain-ran' \
"$HARNESS" verify >/dev/null
[[ -f "$CONFIG_ROOT/integration-ran" ]] || { echo "Required integration stage did not run." >&2; exit 1; }
[[ -f "$CONFIG_ROOT/ci-policy-ran" ]] || { echo "Required CI policy stage did not run." >&2; exit 1; }
[[ -f "$CONFIG_ROOT/supply-chain-ran" ]] || { echo "Required supply-chain stage did not run." >&2; exit 1; }

EVAL_ROOT="$TEST_ROOT/agent-eval"
mkdir -p \
  "$EVAL_ROOT/HARNESS/evals" \
  "$EVAL_ROOT/HARNESS/schemas" \
  "$EVAL_ROOT/fixtures/agent-scope-001" \
  "$EVAL_ROOT/fixtures/agent-capability-001" \
  "$EVAL_ROOT/evals/graders" \
  "$EVAL_ROOT/evals/solutions"
cp "$HARNESS_DIR/templates/evals/case.json.example" "$EVAL_ROOT/HARNESS/evals/regression.json"
cp "$HARNESS_DIR/schemas/agent-eval-case.schema.json" "$EVAL_ROOT/HARNESS/schemas/agent-eval-case.schema.json"
cp "$HARNESS_DIR/schemas/agent-eval-result.schema.json" "$EVAL_ROOT/HARNESS/schemas/agent-eval-result.schema.json"
sed \
  -e 's/"id": "agent-scope-001"/"id": "agent-capability-001"/' \
  -e 's/"suite": "regression"/"suite": "capability"/' \
  -e 's/"count": 3/"count": 1/' \
  -e '/{ "name": "pass_at", "k": 3 }/d' \
  -e '/{ "name": "pass_k", "k": 3 }/d' \
  -e 's#fixtures/agent-scope-001#fixtures/agent-capability-001#' \
  "$HARNESS_DIR/templates/evals/case.json.example" >"$EVAL_ROOT/HARNESS/evals/capability.json"
sed \
  -e 's/"count": 3/"count": 1/' \
  -e '/{ "name": "pass_at", "k": 3 }/d' \
  -e '/{ "name": "pass_k", "k": 3 }/d' \
  "$EVAL_ROOT/HARNESS/evals/regression.json" >"$EVAL_ROOT/HARNESS/evals/regression.tmp"
mv "$EVAL_ROOT/HARNESS/evals/regression.tmp" "$EVAL_ROOT/HARNESS/evals/regression.json"
printf '{}\n' >"$EVAL_ROOT/evals/agent-config.json"
printf '{"files":["../grade-agent-scope-001.sh"]}\n' >"$EVAL_ROOT/evals/graders/outcome-and-scope.manifest.json"
printf '#!/usr/bin/env bash\nexit 0\n' >"$EVAL_ROOT/evals/grade-agent-scope-001.sh"
printf '#!/usr/bin/env bash\nexit 0\n' >"$EVAL_ROOT/evals/solutions/agent-scope-001.sh"
cat >"$EVAL_ROOT/contract.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
sha_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf 'sha256:%s' "$(sha256sum "$1" | awk '{print $1}')"
  else
    printf 'sha256:%s' "$(shasum -a 256 "$1" | awk '{print $1}')"
  fi
}
size_file() { LC_ALL=C wc -c <"$1" | tr -d '[:space:]'; }
test -n "$HARNESS_AGENT_CONTRACT_MODE"
test -f "$HARNESS_AGENT_CONTRACT_SCHEMA"
test -n "$HARNESS_AGENT_CONTRACT_INPUT"
test -n "$HARNESS_AGENT_SUITE"
test -d "$HARNESS_AGENT_EVAL_DIR"
test "$HARNESS_AGENT_TRIALS" = 1
case "$HARNESS_AGENT_CONTRACT_MODE" in
  cases)
    case "$HARNESS_AGENT_CONTRACT_SCHEMA" in *agent-eval-case.schema.json) ;; *) exit 1 ;; esac
    test -d "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq '"schema_version": "1"' "$HARNESS_AGENT_CONTRACT_INPUT"/*.json
    test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = "${EXPECT_CONTRACT_NETWORK:-$HARNESS_NETWORK_CONTRACT_ALLOWS}"
    test "$HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS" = \
      "${EXPECT_REGRESSION_CONTRACT_NETWORK:-$HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS}"
    test "$HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS" = \
      "${EXPECT_CAPABILITY_CONTRACT_NETWORK:-$HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS}"
    [[ "${REJECT_CASES:-false}" != "true" ]] || exit 1
    [[ "${BLOCK_CASES:-false}" != "true" ]] || exit 3
    touch "$HARNESS_AGENT_CONTRACT_INPUT/.validated-$HARNESS_AGENT_SUITE"
    ;;
  result)
    case "$HARNESS_AGENT_CONTRACT_SCHEMA" in *agent-eval-result.schema.json) ;; *) exit 1 ;; esac
    test -s "$HARNESS_AGENT_CONTRACT_INPUT"
    test "$HARNESS_AGENT_RUNNER_EXIT_CODE" -ge 0
    grep -Fq '"schema_version": "1"' "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq "\"run_id\": \"$HARNESS_RUN_ID\"" "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq '"provenance": {' "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq '"case_digest": "sha256:' "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq '"digest":"sha256:' "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq "\"suite\": \"$HARNESS_AGENT_SUITE\"" "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq "\"total\": $HARNESS_AGENT_TRIALS" "$HARNESS_AGENT_CONTRACT_INPUT"
    grep -Fq "\"configuration_digest\": \"$HARNESS_CONFIG_DIGEST\"" "$HARNESS_AGENT_CONTRACT_INPUT"
    case_id=agent-scope-001
    [[ "$HARNESS_AGENT_SUITE" == "regression" ]] || case_id=agent-capability-001
    artifact_file="$(dirname "$HARNESS_AGENT_CONTRACT_INPUT")/artifacts/$case_id/trial-1/diff.patch"
    test -f "$artifact_file"
    test ! -L "$artifact_file"
    artifact_digest="$(sha_file "$artifact_file")"
    artifact_size="$(size_file "$artifact_file")"
    grep -Fq "\"diff.patch\":{\"path\":\"artifacts/$case_id/trial-1/diff.patch\",\"digest\":\"$artifact_digest\",\"size_bytes\":$artifact_size}" "$HARNESS_AGENT_CONTRACT_INPUT"
    [[ "${REJECT_RESULT:-false}" != "true" ]] || exit 1
    [[ "${BLOCK_RESULT:-false}" != "true" ]] || exit 3
    printf '%s\n' "$HARNESS_RUN_ID" >"$HARNESS_AGENT_EVAL_DIR/.validated-result-$HARNESS_AGENT_SUITE"
    ;;
  *) exit 1 ;;
esac
EOF
cat >"$EVAL_ROOT/run-agent-result.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
expected_suite="$1"
marker="$2"
runner_exit="${3:-0}"
case_id=agent-scope-001
[[ "$expected_suite" == "regression" ]] || case_id=agent-capability-001
result_status=passed
grader_status=passed
threshold_met=true
score=1
passed=1
failed=0
blocked=0
suite_failure=""
case_failure=""
trial_failure=""
grader_failure=""
if [[ "$runner_exit" -eq 3 ]]; then
  result_status=blocked
  grader_status=blocked
  threshold_met=null
  score=0
  passed=0
  blocked=1
  suite_failure=', "failure": {"phase":"suite-execution","code":"runner-blocked","detail":"The suite runner reported a blocked prerequisite."}'
  case_failure=', "failure": {"phase":"case-execution","code":"runner-blocked","detail":"The case could not complete because its runner was blocked."}'
  trial_failure=', "failure": {"phase":"trial-execution","code":"runner-blocked","detail":"The trial runner reported a blocked prerequisite."}'
  grader_failure=', "failure": {"phase":"grader-execution","code":"runner-blocked","detail":"The grader could not complete after the runner was blocked."}'
elif [[ "$runner_exit" -ne 0 ]]; then
  result_status=failed
  grader_status=failed
  threshold_met=false
  score=0
  passed=0
  failed=1
fi
test "$HARNESS_AGENT_SUITE" = "$expected_suite"
case "$HARNESS_AGENT_SUITE" in
  regression) test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = "$HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS" ;;
  capability) test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = "$HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS" ;;
  *) exit 1 ;;
esac
test "$HARNESS_NETWORK_CONTRACT_ALLOWS" = "${EXPECT_RUNNER_NETWORK:-$HARNESS_NETWORK_CONTRACT_ALLOWS}"
test -d "$HARNESS_AGENT_EVAL_DIR"
test -n "$HARNESS_RUN_ID"
test -n "$HARNESS_CONFIG_DIGEST"
case "$HARNESS_AGENT_RESULT_FILE" in /*) ;; *) exit 1 ;; esac
case "$HARNESS_AGENT_RESULT_FILE" in */runs/"$HARNESS_RUN_ID"/agent-results/"$HARNESS_AGENT_SUITE".json) ;; *) exit 1 ;; esac
run_dir="$(dirname "$(dirname "$HARNESS_AGENT_RESULT_FILE")")"
artifact_dir="$(dirname "$HARNESS_AGENT_RESULT_FILE")/artifacts/$case_id/trial-1"
mkdir -p "$artifact_dir/verifier"
if [[ "${PRECREATE_COMPLETE_DIR:-false}" == "true" ]]; then
  mkdir "$run_dir/status.complete"
fi
printf 'diff\n' >"$artifact_dir/diff.patch"
printf '{"passed":true}\n' >"$artifact_dir/outcome.json"
printf '{"messages":[]}\n' >"$artifact_dir/trajectory.json"
printf 'verified\n' >"$artifact_dir/verifier/stdout.txt"
: >"$artifact_dir/verifier/stderr.txt"
sha_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf 'sha256:%s' "$(sha256sum "$1" | awk '{print $1}')"
  else
    printf 'sha256:%s' "$(shasum -a 256 "$1" | awk '{print $1}')"
  fi
}
size_file() { LC_ALL=C wc -c <"$1" | tr -d '[:space:]'; }
diff_digest="$(sha_file "$artifact_dir/diff.patch")"
outcome_digest="$(sha_file "$artifact_dir/outcome.json")"
trajectory_digest="$(sha_file "$artifact_dir/trajectory.json")"
stdout_digest="$(sha_file "$artifact_dir/verifier/stdout.txt")"
stderr_digest="$(sha_file "$artifact_dir/verifier/stderr.txt")"
output_run_id="$HARNESS_RUN_ID"
if [[ "${BAD_RUN_ID:-false}" == "true" ]]; then
  output_run_id="20260813T010000Z-agent-regression.BadRun"
fi
if [[ "${BAD_ARTIFACT_DIGEST:-false}" == "true" ]]; then
  diff_digest="sha256:0000000000000000000000000000000000000000000000000000000000000000"
fi
result_tmp="$(mktemp "$(dirname "$HARNESS_AGENT_RESULT_FILE")/.result.XXXXXX")"
cat >"$result_tmp" <<JSON
{
  "schema_version": "1",
  "run_id": "$output_run_id",
  "suite": "$HARNESS_AGENT_SUITE",
  "status": "$result_status",
  "started_at": "2026-08-13T01:00:00Z",
  "finished_at": "2026-08-13T01:00:01Z"$suite_failure,
  "provenance": {
    "repository_revision": null,
    "repository_dirty": null,
    "diff_digest": null,
    "configuration_digest": "$HARNESS_CONFIG_DIGEST",
    "dataset_digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111"
  },
  "cases": [{
    "case_id": "$case_id",
    "case_version": 1,
    "status": "$result_status",
    "threshold_met": $threshold_met$case_failure,
    "inputs": {
      "case_digest": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
      "fixture_revision": "fixture-v1",
      "fixture_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "agent_config_digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "environment_identity": "example/eval@sha256:selftest",
      "reference_solution_digest": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
      "graders": [{"grader_id":"outcome-and-scope","config_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}]
    },
    "trials": [{
      "trial_index": 1,
      "status": "$result_status",
      "started_at": "2026-08-13T01:00:00Z",
      "finished_at": "2026-08-13T01:00:01Z",
      "duration_seconds": 1$trial_failure,
      "graders": [{"grader_id": "outcome-and-scope", "status": "$grader_status", "score": $score$grader_failure}],
      "artifacts": {
        "diff.patch":{"path":"artifacts/$case_id/trial-1/diff.patch","digest":"$diff_digest","size_bytes":$(size_file "$artifact_dir/diff.patch")},
        "outcome.json":{"path":"artifacts/$case_id/trial-1/outcome.json","digest":"$outcome_digest","size_bytes":$(size_file "$artifact_dir/outcome.json")},
        "trajectory.json":{"path":"artifacts/$case_id/trial-1/trajectory.json","digest":"$trajectory_digest","size_bytes":$(size_file "$artifact_dir/trajectory.json")},
        "verifier/stdout.txt":{"path":"artifacts/$case_id/trial-1/verifier/stdout.txt","digest":"$stdout_digest","size_bytes":$(size_file "$artifact_dir/verifier/stdout.txt")},
        "verifier/stderr.txt":{"path":"artifacts/$case_id/trial-1/verifier/stderr.txt","digest":"$stderr_digest","size_bytes":$(size_file "$artifact_dir/verifier/stderr.txt")}
      }
    }],
    "summary": {"total": 1, "passed": $passed, "failed": $failed, "blocked": $blocked, "errors": 0, "pass_rate": $passed},
    "metrics": [
      {"name": "pass_at", "k": 1, "value": $passed},
      {"name": "duration_seconds", "value": 1},
      {"name": "tool_calls", "value": 0},
      {"name": "tokens", "value": 0}
    ]
  }]
}
JSON
mv "$result_tmp" "$HARNESS_AGENT_RESULT_FILE"
if [[ "${TAMPER_ARTIFACT:-false}" == "true" ]]; then
  printf 'tampered\n' >>"$artifact_dir/diff.patch"
fi
if [[ "${SWAP_RESULT_PARENT:-false}" == "true" ]]; then
  outside_parent="${SWAP_RESULT_PARENT_TARGET:?}"
  mkdir -p "$outside_parent"
  mv "$(dirname "$HARNESS_AGENT_RESULT_FILE")" "$run_dir/agent-results-original"
  ln -s "$outside_parent" "$(dirname "$HARNESS_AGENT_RESULT_FILE")"
  cp -R "$run_dir/agent-results-original/." "$outside_parent/"
fi
touch "$marker"
exit "$runner_exit"
EOF
chmod +x \
  "$EVAL_ROOT/contract.sh" \
  "$EVAL_ROOT/run-agent-result.sh" \
  "$EVAL_ROOT/evals/grade-agent-scope-001.sh" \
  "$EVAL_ROOT/evals/solutions/agent-scope-001.sh"

HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EVAL_ROOT" \
EXPECT_CONTRACT_NETWORK=true \
EXPECT_REGRESSION_CONTRACT_NETWORK=true \
EXPECT_CAPABILITY_CONTRACT_NETWORK=true \
HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression regression-ran' \
HARNESS_AGENT_CAPABILITY_COMMAND='./run-agent-result.sh capability capability-ran' \
HARNESS_AGENT_TRIALS=1 \
"$HARNESS" agent:eval >/dev/null
[[ -f "$EVAL_ROOT/regression-ran" ]] || { echo "Agent regression suite did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/capability-ran" ]] || { echo "Agent capability suite did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/test-results/harness/agent-eval.json" ]] || { echo "Agent eval evidence was not written." >&2; exit 1; }
grep -Fq '"stage":"agent-regression","status":"passed"' "$EVAL_ROOT/test-results/harness/agent-eval.json"
grep -Fq '"network_contract_allows":true' "$EVAL_ROOT/test-results/harness/agent-eval.json"
grep -Fq '"stage":"agent-eval-doctor","status":"passed"' "$EVAL_ROOT/test-results/harness/agent-eval.json"
grep -Fq '"name":"agent-result:regression"' "$EVAL_ROOT/test-results/harness/agent-eval.json"
grep -Fq '"name":"agent-result:capability"' "$EVAL_ROOT/test-results/harness/agent-eval.json"
validate_json_file "$EVAL_ROOT/test-results/harness/agent-eval.json"
validate_json_file "$EVAL_ROOT/test-results/harness/agent-results/regression.json"
validate_json_file "$EVAL_ROOT/test-results/harness/agent-results/capability.json"
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-all" ]] || { echo "Agent case contract did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression" ]] || { echo "Regression result contract did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-result-capability" ]] || { echo "Capability result contract did not run." >&2; exit 1; }
FIRST_AGENT_RESULT="$(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f | head -n 1)"
[[ -n "$FIRST_AGENT_RESULT" ]] || { echo "Canonical agent result was not retained." >&2; exit 1; }
FIRST_AGENT_RESULT_DIGEST="$(file_sha256 "$FIRST_AGENT_RESULT")"
FIRST_AGENT_RUN_ID="$(basename "$(dirname "$(dirname "$FIRST_AGENT_RESULT")")")"
grep -Fq "\"run_id\": \"$FIRST_AGENT_RUN_ID\"" "$FIRST_AGENT_RESULT"
grep -Fq "\"digest\":\"sha256:$FIRST_AGENT_RESULT_DIGEST\"" "$EVAL_ROOT/test-results/harness/agent-eval.json"
if [[ "${HARNESS_RUN_SCHEMA_CONFORMANCE:-false}" == "true" ]]; then
  HARNESS_GENERATED_AGENT_RESULT="$FIRST_AGENT_RESULT" \
  python3 "$HARNESS_DIR/tests/schema_contract_test.py"
fi

rm -f "$EVAL_ROOT/HARNESS/evals/.validated-all"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EVAL_ROOT" \
HARNESS_NETWORK_POLICY=declared \
HARNESS_NETWORK_STAGES=agent-capability \
EXPECT_CONTRACT_NETWORK=false \
EXPECT_REGRESSION_CONTRACT_NETWORK=false \
EXPECT_CAPABILITY_CONTRACT_NETWORK=true \
HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression mixed-regression-ran' \
HARNESS_AGENT_CAPABILITY_COMMAND='./run-agent-result.sh capability mixed-capability-ran' \
HARNESS_AGENT_TRIALS=1 \
"$HARNESS" agent:eval >/dev/null
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-all" ]] \
  || { echo "Mixed-network all-suite contract did not run." >&2; exit 1; }
[[ -f "$EVAL_ROOT/mixed-regression-ran" && -f "$EVAL_ROOT/mixed-capability-ran" ]] \
  || { echo "Mixed-network suite runners did not receive their individual contracts." >&2; exit 1; }
[[ "$(file_sha256 "$FIRST_AGENT_RESULT")" == "$FIRST_AGENT_RESULT_DIGEST" ]] \
  || { echo "A later agent run modified prior canonical evidence." >&2; exit 1; }

rm -f "$EVAL_ROOT/HARNESS/evals/.validated-regression"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EVAL_ROOT" \
HARNESS_NETWORK_POLICY=deny \
HARNESS_NETWORK_STAGES=agent-capability \
EXPECT_CONTRACT_NETWORK=false \
EXPECT_RUNNER_NETWORK=false \
HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression regression-only-ran' \
HARNESS_AGENT_TRIALS=1 \
"$HARNESS" agent:eval >/dev/null
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-regression" ]] \
  || { echo "Regression-only agent doctor used the wrong suite scope." >&2; exit 1; }
[[ -f "$EVAL_ROOT/regression-only-ran" ]] \
  || { echo "Regression-only agent eval did not run." >&2; exit 1; }

NO_REPORT_RESULT_COUNT_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f | awk 'END {print NR+0}')"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$EVAL_ROOT" \
HARNESS_WRITE_REPORTS=false \
HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression no-wrapper-report-ran' \
HARNESS_AGENT_TRIALS=1 \
"$HARNESS" agent:regression >/dev/null
NO_REPORT_RESULT_COUNT_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f | awk 'END {print NR+0}')"
[[ "$NO_REPORT_RESULT_COUNT_AFTER" -eq "$((NO_REPORT_RESULT_COUNT_BEFORE + 1))" ]] \
  || { echo "Report-disabled agent run did not retain a unique canonical result." >&2; exit 1; }
[[ ! -e "$EVAL_ROOT/test-results/harness/latest/agent-regression.json" ]] \
  || { echo "Report-disabled agent run unexpectedly published a wrapper pointer." >&2; exit 1; }
NO_REPORT_RUN_ID="$(cat "$EVAL_ROOT/HARNESS/evals/.validated-result-regression")"
NO_REPORT_RUN_DIR="$EVAL_ROOT/test-results/harness/runs/$NO_REPORT_RUN_ID"
[[ -f "$NO_REPORT_RUN_DIR/agent-results/regression.json" \
  && ! -e "$NO_REPORT_RUN_DIR/report.json" \
  && "$(cat "$NO_REPORT_RUN_DIR/status.complete")" == complete \
  && "$(file_mode "$NO_REPORT_RUN_DIR")" == 700 \
  && "$(file_mode "$NO_REPORT_RUN_DIR/agent-results")" == 700 \
  && "$(file_mode "$NO_REPORT_RUN_DIR/agent-results/regression.json")" == 444 \
  && "$(file_mode "$NO_REPORT_RUN_DIR/status.complete")" == 444 ]] \
  || { echo "Report-disabled valid evidence was not completed with safe modes." >&2; exit 1; }

NO_REPORT_INCOMPLETE_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
NO_REPORT_LEGACY_DIGEST="$(file_sha256 "$EVAL_ROOT/test-results/harness/agent-results/regression.json")"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_WRITE_REPORTS=false \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
NO_REPORT_INCOMPLETE_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
[[ "$NO_REPORT_INCOMPLETE_AFTER" -eq "$((NO_REPORT_INCOMPLETE_BEFORE + 1))" \
  && "$(file_sha256 "$EVAL_ROOT/test-results/harness/agent-results/regression.json")" == "$NO_REPORT_LEGACY_DIGEST" ]] \
  || { echo "Report-disabled invalid evidence was completed or published." >&2; exit 1; }

PARALLEL_AGENT_COUNT_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f | awk 'END {print NR+0}')"
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression parallel-agent-one' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression >"$EVAL_ROOT/parallel-agent-one.log" &
PARALLEL_AGENT_PID_ONE=$!
env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression parallel-agent-two' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression >"$EVAL_ROOT/parallel-agent-two.log" &
PARALLEL_AGENT_PID_TWO=$!
wait "$PARALLEL_AGENT_PID_ONE"
wait "$PARALLEL_AGENT_PID_TWO"
PARALLEL_AGENT_COUNT_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f | awk 'END {print NR+0}')"
[[ "$PARALLEL_AGENT_COUNT_AFTER" -eq "$((PARALLEL_AGENT_COUNT_BEFORE + 2))" ]] \
  || { echo "Parallel agent runs shared a result file." >&2; exit 1; }
while IFS= read -r PARALLEL_AGENT_RESULT; do
  validate_json_file "$PARALLEL_AGENT_RESULT"
done < <(find "$EVAL_ROOT/test-results/harness/runs" -path '*/agent-results/regression.json' -type f)

OUTSIDE_AGENT_RESULTS="$TEST_ROOT/outside-agent-results"
mkdir -p "$OUTSIDE_AGENT_RESULTS"
printf 'preserve\n' >"$OUTSIDE_AGENT_RESULTS/regression.json"
rm -f "$EVAL_ROOT/HARNESS/evals/.invalid-result-path-validated"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=../outside-agent-results \
  HARNESS_AGENT_CONTRACT_COMMAND='touch HARNESS/evals/.invalid-result-path-validated' \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
grep -Fq preserve "$OUTSIDE_AGENT_RESULTS/regression.json"
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.invalid-result-path-validated" ]] \
  || { echo "Case contract ran before invalid result path failed." >&2; exit 1; }

printf 'preserve-dot-dot\n' >"$TEST_ROOT/regression.json"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=./.. \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
grep -Fq preserve-dot-dot "$TEST_ROOT/regression.json"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=nested/.. \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=../outside-agent-results \
  HARNESS_AGENT_CONTRACT_COMMAND='touch HARNESS/evals/.invalid-result-path-validated' \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:eval:doctor
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.invalid-result-path-validated" ]] \
  || { echo "Agent eval doctor ran the contract before invalid result path failed." >&2; exit 1; }

RESULT_SYMLINK_TARGET="$TEST_ROOT/result-symlink-target"
mkdir -p "$RESULT_SYMLINK_TARGET"
ln -s "$RESULT_SYMLINK_TARGET" "$EVAL_ROOT/symlink-results"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=symlink-results \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=symlink-results \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:eval:doctor

rm -f "$EVAL_ROOT/HARNESS/evals/.reserved-result-path-validated"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=test-results/harness/runs \
  HARNESS_AGENT_CONTRACT_COMMAND='touch HARNESS/evals/.reserved-result-path-validated' \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.reserved-result-path-validated" ]] \
  || { echo "Case contract ran with an agent result directory in reserved storage." >&2; exit 1; }
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=test-results/harness/latest/custom \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:eval:doctor

SUMMARY_AGENT_RESULT_DIR="$EVAL_ROOT/summary-agent-result-symlink"
mkdir -p "$SUMMARY_AGENT_RESULT_DIR"
printf 'preserve-agent-result\n' >"$TEST_ROOT/agent-result-sentinel"
ln -s "$TEST_ROOT/agent-result-sentinel" "$SUMMARY_AGENT_RESULT_DIR/regression.json"
rm -f "$EVAL_ROOT/HARNESS/evals/.summary-result-path-validated"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_RESULT_DIR=summary-agent-result-symlink \
  HARNESS_AGENT_CONTRACT_COMMAND='touch HARNESS/evals/.summary-result-path-validated' \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
grep -Fq preserve-agent-result "$TEST_ROOT/agent-result-sentinel"
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.summary-result-path-validated" ]] \
  || { echo "Case contract ran before an unsafe agent summary path failed." >&2; exit 1; }

expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA="$HARNESS_DIR/schemas/agent-eval-case.schema.json" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:eval:doctor

MISSING_CONTRACT_MARKER="$EVAL_ROOT/missing-contract-ran"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_REGRESSION_COMMAND="./run-agent-result.sh regression $MISSING_CONTRACT_MARKER" \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ ! -e "$MISSING_CONTRACT_MARKER" ]] || { echo "Runner ran without a contract validator." >&2; exit 1; }

REJECTED_CASE_MARKER="$EVAL_ROOT/rejected-case-ran"
expect_failure env \
  REJECT_CASES=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND="./run-agent-result.sh regression $REJECTED_CASE_MARKER" \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ ! -e "$REJECTED_CASE_MARKER" ]] || { echo "Runner ran after case contract rejection." >&2; exit 1; }

STALE_RESULT="$EVAL_ROOT/test-results/harness/agent-results/regression.json"
STALE_RESULT_DIGEST="$(file_sha256 "$STALE_RESULT")"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ "$(file_sha256 "$STALE_RESULT")" == "$STALE_RESULT_DIGEST" ]] \
  || { echo "A missing runner result replaced the latest validated current summary." >&2; exit 1; }

expect_failure env \
  REJECT_RESULT=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression rejected-result-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression

expect_failure env \
  BAD_RUN_ID=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression bad-run-id-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ -f "$EVAL_ROOT/bad-run-id-ran" ]] \
  || { echo "Bad-run-ID fixture did not reach post-run validation." >&2; exit 1; }

expect_failure env \
  BAD_ARTIFACT_DIGEST=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression bad-artifact-digest-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
expect_failure env \
  TAMPER_ARTIFACT=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression tampered-artifact-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ "$(file_sha256 "$STALE_RESULT")" == "$STALE_RESULT_DIGEST" ]] \
  || { echo "Invalid integrity evidence replaced the validated current summary." >&2; exit 1; }

SWAPPED_RESULT_TARGET="$TEST_ROOT/swapped-result-parent"
SWAPPED_LATEST_DIGEST="$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")"
SWAPPED_LEGACY_DIGEST="$(file_sha256 "$STALE_RESULT")"
SWAPPED_INCOMPLETE_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
rm -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression"
expect_failure env \
  SWAP_RESULT_PARENT=true \
  SWAP_RESULT_PARENT_TARGET="$SWAPPED_RESULT_TARGET" \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression swapped-parent-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ -f "$SWAPPED_RESULT_TARGET/regression.json" ]] \
  || { echo "Swapped-parent fixture did not execute." >&2; exit 1; }
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.validated-result-regression" ]] \
  || { echo "Result validator followed a runner-replaced parent symlink." >&2; exit 1; }
SWAPPED_INCOMPLETE_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
[[ "$SWAPPED_INCOMPLETE_AFTER" -eq "$((SWAPPED_INCOMPLETE_BEFORE + 1))" \
  && "$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")" == "$SWAPPED_LATEST_DIGEST" \
  && "$(file_sha256 "$STALE_RESULT")" == "$SWAPPED_LEGACY_DIGEST" ]] \
  || { echo "Swapped-parent evidence was completed or published." >&2; exit 1; }

LATEST_AGENT_POINTER_DIGEST="$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")"
COMPLETE_MARKERS_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.complete -type f | awk 'END {print NR+0}')"
INCOMPLETE_MARKERS_BEFORE="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
expect_failure env \
  PRECREATE_COMPLETE_DIR=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression bad-complete-marker-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ "$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")" == "$LATEST_AGENT_POINTER_DIGEST" ]] \
  || { echo "Latest advanced after an unsafe completion marker." >&2; exit 1; }
COMPLETE_MARKERS_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.complete -type f | awk 'END {print NR+0}')"
INCOMPLETE_MARKERS_AFTER="$(find "$EVAL_ROOT/test-results/harness/runs" -name status.incomplete -type f | awk 'END {print NR+0}')"
[[ "$COMPLETE_MARKERS_AFTER" -eq "$COMPLETE_MARKERS_BEFORE" \
  && "$INCOMPLETE_MARKERS_AFTER" -eq "$((INCOMPLETE_MARKERS_BEFORE + 1))" ]] \
  || { echo "Unsafe completion-marker directory produced a completed run." >&2; exit 1; }

rm -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression"
expect_status 7 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression failed-runner-ran 7' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression" ]] \
  || { echo "Failed runner result was not contract-validated." >&2; exit 1; }

rm -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression"
expect_status 3 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression blocked-runner-ran 3' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
[[ -f "$EVAL_ROOT/HARNESS/evals/.validated-result-regression" ]] \
  || { echo "Blocked runner result was not contract-validated." >&2; exit 1; }
grep -Fq '"status": "blocked"' "$EVAL_ROOT/test-results/harness/agent-results/regression.json"
grep -Fq '"code":"runner-blocked"' "$EVAL_ROOT/test-results/harness/agent-results/regression.json"

# Once a runner starts, exit 3 is only a valid blocked outcome when it also
# writes the required structured result. Missing evidence is a contract failure.
expect_status 1 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='exit 3' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression

REPORTLESS_FAILED_LATEST_BEFORE="$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")"
expect_status 7 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_WRITE_REPORTS=false \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression reportless-failed-ran 7' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression
REPORTLESS_FAILED_RUN_ID="$(cat "$EVAL_ROOT/HARNESS/evals/.validated-result-regression")"
REPORTLESS_FAILED_RUN_DIR="$EVAL_ROOT/test-results/harness/runs/$REPORTLESS_FAILED_RUN_ID"
[[ -f "$REPORTLESS_FAILED_RUN_DIR/status.complete" \
  && "$(cat "$REPORTLESS_FAILED_RUN_DIR/status.complete")" == complete \
  && ! -e "$REPORTLESS_FAILED_RUN_DIR/report.json" \
  && "$(file_sha256 "$EVAL_ROOT/test-results/harness/latest/agent-regression.json")" == "$REPORTLESS_FAILED_LATEST_BEFORE" ]] \
  || { echo "Validated report-disabled failed evidence was not retained correctly." >&2; exit 1; }

expect_status 1 env \
  BLOCK_RESULT=true \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND='./run-agent-result.sh regression blocked-result-ran' \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:regression

rm -f "$EVAL_ROOT/HARNESS/evals/.validated-all"
expect_status 3 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_NETWORK_POLICY=deny \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  "$HARNESS" agent:regression
expect_status 3 env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EVAL_ROOT" \
  HARNESS_NETWORK_POLICY=deny \
  HARNESS_AGENT_CASE_SCHEMA=HARNESS/schemas/agent-eval-case.schema.json \
  HARNESS_AGENT_RESULT_SCHEMA=HARNESS/schemas/agent-eval-result.schema.json \
  HARNESS_AGENT_CONTRACT_COMMAND=./contract.sh \
  HARNESS_AGENT_REGRESSION_COMMAND=true \
  HARNESS_AGENT_TRIALS=1 \
  "$HARNESS" agent:eval
[[ ! -e "$EVAL_ROOT/HARNESS/evals/.validated-all" ]] \
  || { echo "Agent contract ran after network policy blocked agent:eval." >&2; exit 1; }
grep -Fq '"stage":"agent-eval-doctor","status":"blocked","exit_code":3' "$EVAL_ROOT/test-results/harness/agent-eval.json"

if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
  NODE_ROOT="$TEST_ROOT/node"
  mkdir -p "$NODE_ROOT"
  cat >"$NODE_ROOT/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(0)\""
  }
}
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  cat >"$NODE_ROOT/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(9)\""
  }
}
EOF
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

  NESTED_NODE_ROOT="$TEST_ROOT/nested-node"
  mkdir -p "$NESTED_NODE_ROOT/apps/web"
  cat >"$NESTED_NODE_ROOT/apps/web/package.json" <<'EOF'
{
  "private": true,
  "scripts": {
    "test": "node -e \"process.exit(0)\""
  }
}
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$NESTED_NODE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  MANAGER_ROOT="$TEST_ROOT/package-manager"
  FAKE_NODE_BIN="$TEST_ROOT/fake-node-bin"
  mkdir -p "$MANAGER_ROOT" "$FAKE_NODE_BIN"
  cat >"$MANAGER_ROOT/package.json" <<'EOF'
{
  "private": true,
  "packageManager": "pnpm@9.0.0",
  "scripts": {
    "test": "unused"
  }
}
EOF
  cat >"$FAKE_NODE_BIN/pnpm" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  cat >"$FAKE_NODE_BIN/npm" <<'EOF'
#!/usr/bin/env bash
exit 9
EOF
  chmod +x "$FAKE_NODE_BIN/pnpm" "$FAKE_NODE_BIN/npm"
  PATH="$FAKE_NODE_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MANAGER_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
fi

if command -v make >/dev/null 2>&1; then
  MAKE_ROOT="$TEST_ROOT/make"
  mkdir -p "$MAKE_ROOT"
  cat >"$MAKE_ROOT/Makefile" <<'EOF'
test:
	@true
EOF
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MAKE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null

  cat >"$MAKE_ROOT/Makefile" <<'EOF'
test:
	@exit 7
EOF
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$MAKE_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test
fi

PYTHON_ROOT="$TEST_ROOT/python"
mkdir -p "$PYTHON_ROOT/tests" "$PYTHON_ROOT/.venv/bin"
cat >"$PYTHON_ROOT/pyproject.toml" <<'EOF'
[project]
name = "harness-selftest"
version = "0.0.0"
EOF
cat >"$PYTHON_ROOT/.venv/bin/python" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == "-m pytest" ]]; then
  exit "${FAKE_PYTHON_STATUS:-0}"
fi
exit 8
EOF
chmod +x "$PYTHON_ROOT/.venv/bin/python"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PYTHON_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env FAKE_PYTHON_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PYTHON_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

PYTHON_MULTI_CONFIG_ROOT="$TEST_ROOT/python-multi-config"
PYTHON_TOOL_MARKER="$PYTHON_MULTI_CONFIG_ROOT/mypy-ran"
mkdir -p "$PYTHON_MULTI_CONFIG_ROOT/.venv/bin"
cat >"$PYTHON_MULTI_CONFIG_ROOT/pyproject.toml" <<'EOF'
[project]
name = "harness-multi-config-selftest"
version = "0.0.0"
EOF
cat >"$PYTHON_MULTI_CONFIG_ROOT/setup.cfg" <<'EOF'
[mypy]
strict = True
EOF
cat >"$PYTHON_MULTI_CONFIG_ROOT/.venv/bin/mypy" <<'EOF'
#!/usr/bin/env bash
touch "$PYTHON_TOOL_MARKER"
EOF
chmod +x "$PYTHON_MULTI_CONFIG_ROOT/.venv/bin/mypy"
PYTHON_TOOL_MARKER="$PYTHON_TOOL_MARKER" \
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$PYTHON_MULTI_CONFIG_ROOT" \
HARNESS_REQUIRED_STAGES=typecheck \
"$HARNESS" typecheck >/dev/null
[[ -f "$PYTHON_TOOL_MARKER" ]] || { echo "Python split configuration was not detected." >&2; exit 1; }

PYTHON_COVERAGE_ROOT="$TEST_ROOT/python-coverage-config"
PYTHON_COVERAGE_MARKER="$PYTHON_COVERAGE_ROOT/coverage-ran"
mkdir -p "$PYTHON_COVERAGE_ROOT/.venv/bin"
cat >"$PYTHON_COVERAGE_ROOT/pyproject.toml" <<'EOF'
[project]
name = "harness-coverage-config-selftest"
version = "0.0.0"
EOF
cat >"$PYTHON_COVERAGE_ROOT/setup.cfg" <<'EOF'
[coverage:run]
branch = True
EOF
cat >"$PYTHON_COVERAGE_ROOT/.venv/bin/python" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == "-c import pytest_cov" ]]; then
  exit 0
fi
if [[ "$*" == -m\ pytest\ --cov=* ]]; then
  touch "$PYTHON_COVERAGE_MARKER"
  exit 0
fi
exit 8
EOF
chmod +x "$PYTHON_COVERAGE_ROOT/.venv/bin/python"
PYTHON_COVERAGE_MARKER="$PYTHON_COVERAGE_MARKER" \
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$PYTHON_COVERAGE_ROOT" \
HARNESS_REQUIRED_STAGES=coverage \
"$HARNESS" coverage >/dev/null
[[ -f "$PYTHON_COVERAGE_MARKER" ]] || { echo "Python coverage split configuration was not detected." >&2; exit 1; }

COMPILED_BIN="$TEST_ROOT/compiled-bin"
mkdir -p "$COMPILED_BIN"
cat >"$COMPILED_BIN/go" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_GO_STATUS:-0}"
EOF
cat >"$COMPILED_BIN/cargo" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_CARGO_STATUS:-0}"
EOF
chmod +x "$COMPILED_BIN/go" "$COMPILED_BIN/cargo"

GO_ROOT="$TEST_ROOT/go"
mkdir -p "$GO_ROOT"
printf 'module example.invalid/selftest\n' >"$GO_ROOT/go.mod"
PATH="$COMPILED_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$GO_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env PATH="$COMPILED_BIN:$PATH" FAKE_GO_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$GO_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

RUST_ROOT="$TEST_ROOT/rust"
mkdir -p "$RUST_ROOT"
cat >"$RUST_ROOT/Cargo.toml" <<'EOF'
[package]
name = "harness-selftest"
version = "0.0.0"
EOF
PATH="$COMPILED_BIN:$PATH" HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$RUST_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test >/dev/null
expect_failure env PATH="$COMPILED_BIN:$PATH" FAKE_CARGO_STATUS=7 HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$RUST_ROOT" HARNESS_REQUIRED_STAGES=test "$HARNESS" test

UNSUPPORTED_COMPILED_ROOT="$TEST_ROOT/unsupported-compiled-stage"
MINIMAL_BIN="$TEST_ROOT/minimal-bin"
mkdir -p "$UNSUPPORTED_COMPILED_ROOT" "$MINIMAL_BIN"
printf 'module example.invalid/unsupported\n' >"$UNSUPPORTED_COMPILED_ROOT/go.mod"
printf '[workspace]\nmembers = []\n' >"$UNSUPPORTED_COMPILED_ROOT/Cargo.toml"
  ln -s "$(command -v dirname)" "$MINIMAL_BIN/dirname"
  ln -s "$(command -v basename)" "$MINIMAL_BIN/basename"
ln -s "$(command -v mktemp)" "$MINIMAL_BIN/mktemp"
ln -s "$(command -v chmod)" "$MINIMAL_BIN/chmod"
ln -s "$(command -v rm)" "$MINIMAL_BIN/rm"
env \
  PATH="$MINIMAL_BIN" \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$UNSUPPORTED_COMPILED_ROOT" \
  HARNESS_WRITE_REPORTS=false \
  "$HARNESS" coverage >/dev/null

GOV_ROOT="$TEST_ROOT/governed"
mkdir -p "$GOV_ROOT"
cat >"$GOV_ROOT/SPECIFICATION.md" <<'EOF'
# Specification
## CHG-101 Portable governance
EOF
cat >"$GOV_ROOT/SPEC_CHANGELOG.md" <<'EOF'
# Specification Changelog
Historical example: CHG-999
| Date | Change ID | Status |
|---|---|---|
| 2026-01-02 | CHG-101 | Gate 4 approved |
EOF
mkdir -p "$GOV_ROOT/HARNESS"
printf 'CHG-101\n' >"$GOV_ROOT/HARNESS/ACTIVE_CHANGE"
write_approved_plan "$GOV_ROOT"
cat >"$GOV_ROOT/TEST_PLAN.md" <<'EOF'
# Test Plan
CHG-101 is covered by the harness self-test.
EOF
cat >"$GOV_ROOT/TRACEABILITY.md" <<'EOF'
| Requirement | Change | Test | Status |
|---|---|---|---|
| GOV-101 | CHG-101 | selftest | Pass |
EOF

HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=required \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
"$HARNESS" governance:doctor >/dev/null

write_pending_plan "$GOV_ROOT"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor

write_approved_plan "$GOV_ROOT"
sed 's/CHG-101/CHG-102/g' "$GOV_ROOT/TRACEABILITY.md" >"$GOV_ROOT/TRACEABILITY.wrong"
mv "$GOV_ROOT/TRACEABILITY.wrong" "$GOV_ROOT/TRACEABILITY.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor
sed 's/CHG-102/CHG-101/g' "$GOV_ROOT/TRACEABILITY.md" >"$GOV_ROOT/TRACEABILITY.fixed"
mv "$GOV_ROOT/TRACEABILITY.fixed" "$GOV_ROOT/TRACEABILITY.md"

# Approval policy activates governance even in auto mode, and change references
# are exact IDs.
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=auto \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
"$HARNESS" governance:doctor >/dev/null

cp "$GOV_ROOT/DEVELOPMENT_PLAN.md" "$GOV_ROOT/DEVELOPMENT_PLAN.saved"
sed 's/2026-01-02/2026-02-29/g' "$GOV_ROOT/DEVELOPMENT_PLAN.saved" >"$GOV_ROOT/DEVELOPMENT_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:approved
sed 's/2026-02-29/2028-02-29/g' "$GOV_ROOT/DEVELOPMENT_PLAN.md" >"$GOV_ROOT/DEVELOPMENT_PLAN.leap"
mv "$GOV_ROOT/DEVELOPMENT_PLAN.leap" "$GOV_ROOT/DEVELOPMENT_PLAN.md"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
HARNESS_ROOT_DIR="$GOV_ROOT" \
HARNESS_GOVERNANCE_MODE=required \
HARNESS_REQUIRE_PLAN_APPROVAL=true \
"$HARNESS" governance:approved >/dev/null
mv "$GOV_ROOT/DEVELOPMENT_PLAN.saved" "$GOV_ROOT/DEVELOPMENT_PLAN.md"

# Duplicate governance roles, active sections, or approval fields are
# ambiguous to reviewers and must fail rather than accepting the first value.
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_TEST_PLAN_FILE=SPECIFICATION.md \
  "$HARNESS" governance:doctor
cp "$GOV_ROOT/DEVELOPMENT_PLAN.md" "$GOV_ROOT/DEVELOPMENT_PLAN.saved"
printf '%s\n' '- Requirements Confirmed Status: pending' >>"$GOV_ROOT/DEVELOPMENT_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor
mv "$GOV_ROOT/DEVELOPMENT_PLAN.saved" "$GOV_ROOT/DEVELOPMENT_PLAN.md"
cp "$GOV_ROOT/DEVELOPMENT_PLAN.md" "$GOV_ROOT/DEVELOPMENT_PLAN.saved"
printf '%s\n' '## CHG-101 Duplicate section' >>"$GOV_ROOT/DEVELOPMENT_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor
mv "$GOV_ROOT/DEVELOPMENT_PLAN.saved" "$GOV_ROOT/DEVELOPMENT_PLAN.md"
mkdir -p "$GOV_ROOT/HARNESS"
printf 'CHG-101\n' >"$GOV_ROOT/HARNESS/ACTIVE_CHANGE"
sed 's/CHG-101/CHG-10/g' "$GOV_ROOT/TEST_PLAN.md" >"$GOV_ROOT/TEST_PLAN.wrong"
mv "$GOV_ROOT/TEST_PLAN.wrong" "$GOV_ROOT/TEST_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor
sed 's/CHG-10/CHG-101/g' "$GOV_ROOT/TEST_PLAN.md" >"$GOV_ROOT/TEST_PLAN.fixed"
mv "$GOV_ROOT/TEST_PLAN.fixed" "$GOV_ROOT/TEST_PLAN.md"
printf '{{REPLACE_ME:TESTS}}\n' >>"$GOV_ROOT/TEST_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor
sed '/{{REPLACE_ME:TESTS}}/d' "$GOV_ROOT/TEST_PLAN.md" >"$GOV_ROOT/TEST_PLAN.fixed"
mv "$GOV_ROOT/TEST_PLAN.fixed" "$GOV_ROOT/TEST_PLAN.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_ACTIVE_CHANGE_FILE=../outside-active-change \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor
mv "$GOV_ROOT/SPECIFICATION.md" "$GOV_ROOT/SPECIFICATION.real.md"
ln -s SPECIFICATION.real.md "$GOV_ROOT/SPECIFICATION.md"
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor
rm "$GOV_ROOT/SPECIFICATION.md"
mv "$GOV_ROOT/SPECIFICATION.real.md" "$GOV_ROOT/SPECIFICATION.md"

TEMPLATE_GOV_ROOT="$TEST_ROOT/template-governance"
TEMPLATE_GOV_DIR="$HARNESS_DIR/templates/governance"
mkdir -p "$TEMPLATE_GOV_ROOT/HARNESS" "$TEMPLATE_GOV_ROOT/plans"
cp "$HARNESS_DIR/../AGENTS.md" "$TEMPLATE_GOV_ROOT/AGENTS.md"
sed -E \
  -e '/PORTABLE_PROJECT_TEMPLATE/d' \
  -e 's/\{\{REPLACE_ME:[^}]+\}\}/Example adopted value/g' \
  "$HARNESS_DIR/../PROJECT.md.example" >"$TEMPLATE_GOV_ROOT/PROJECT.md"
cp "$HARNESS_DIR/HARNESS.md" "$TEMPLATE_GOV_ROOT/HARNESS/HARNESS.md"
sed \
  -e "s/HARNESS_PROJECT_NAME=''/HARNESS_PROJECT_NAME='Example template adoption'/" \
  -e "s/HARNESS_REQUIRED_STAGES='lint,test,build'/HARNESS_REQUIRED_STAGES='test'/" \
  -e "s/HARNESS_VERIFY_STAGES='lint,test,build'/HARNESS_VERIFY_STAGES='test'/" \
  -e "s#HARNESS_TEST_COMMAND=''#HARNESS_TEST_COMMAND='test -f src/governance.sh \&\& test -f tests/governance.test'#" \
  "$HARNESS_DIR/config.env.example" >"$TEMPLATE_GOV_ROOT/HARNESS/config.env"
sed 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  "$TEMPLATE_GOV_DIR/ACTIVE_CHANGE.example" >"$TEMPLATE_GOV_ROOT/HARNESS/ACTIVE_CHANGE"
sed \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's/{{REPLACE_ME:CHANGE_TITLE}}/Adopt portable governance/g' \
  -e 's/{{REPLACE_ME:SEMVER}}/1.0.0/g' \
  -e 's/{{REPLACE_ME:SPEC_OWNER}}/Example Product Owner/g' \
  -e 's/{{REPLACE_ME:PROBLEM_AND_OBSERVABLE_IMPACT}}/Governance state needs deterministic validation/g' \
  -e 's/{{REPLACE_ME:GOAL}}/Validate the active governance contract/g' \
  -e 's/{{REPLACE_ME:NON_GOAL}}/Change project product behavior/g' \
  -e 's/{{REPLACE_ME:ACTOR}}/maintainer/g' \
  -e 's/{{REPLACE_ME:SCENARIO_AND_OUTCOME}}/a deterministic governance result/g' \
  -e 's/{{REPLACE_ME:NORMATIVE_BEHAVIOR}}/validate active governance records deterministically/g' \
  -e 's/{{REPLACE_ME:PRECONDITION}}/the governed files exist/g' \
  -e 's/{{REPLACE_ME:ACTION}}/the governance doctor runs/g' \
  -e 's/{{REPLACE_ME:OBSERVABLE_RESULT}}/every contract is validated/g' \
  -e 's/{{REPLACE_ME:INTERFACES_DATA_ERRORS_RECOVERY}}/Markdown files are validated and failures are actionable/g' \
  -e 's/{{REPLACE_ME:SECURITY_PERFORMANCE_ACCESSIBILITY_OBSERVABILITY}}/Validation is local and deterministic/g' \
  -e 's/{{REPLACE_ME:COMPATIBILITY_MIGRATION_ROLLBACK}}/No compatibility impact/g' \
  -e 's/{{REPLACE_ME:ASSUMPTION}}/The active change ID is canonical/g' \
  -e 's/Clarification Status: pending/Clarification Status: resolved/' \
  -e 's/{{REPLACE_ME:OPEN_QUESTIONS_OR_NONE}}/None./g' \
  "$TEMPLATE_GOV_DIR/SPECIFICATION.md.example" >"$TEMPLATE_GOV_ROOT/SPECIFICATION.md"
sed \
  -e 's/{{REPLACE_ME:YYYY-MM-DD}}/2026-01-02/g' \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's/{{REPLACE_ME:SEMVER}}/1.0.0/g' \
  -e 's/{{REPLACE_ME:CHANGE_TITLE}}/Adopt portable governance/g' \
  -e 's/{{REPLACE_ME:REASON}}/Make governance deterministic/g' \
  -e 's/{{REPLACE_ME:IMPACT}}/No compatibility impact/g' \
  -e 's/{{REPLACE_ME:TESTS}}/Harness selftest/g' \
  "$TEMPLATE_GOV_DIR/SPEC_CHANGELOG.md.example" >"$TEMPLATE_GOV_ROOT/SPEC_CHANGELOG.md"
sed \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's/{{REPLACE_ME:TEST_PATH}}/tests\/governance.test/g' \
  -e 's/{{REPLACE_ME:TEST_LEVEL}}/unit/g' \
  -e 's/{{REPLACE_ME:FIXTURE}}/isolated governance repository/g' \
  -e 's/{{REPLACE_ME:BASELINE_EVIDENCE}}/governance contract is unavailable/g' \
  -e 's/{{REPLACE_ME:STAGE_ID}}/test/g' \
  -e 's/{{REPLACE_ME:FAILURE_PERMISSION_BOUNDARY_CASES}}/missing and conflicting selectors/g' \
  "$TEMPLATE_GOV_DIR/TEST_PLAN.md.example" >"$TEMPLATE_GOV_ROOT/TEST_PLAN.md"
sed \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's/{{REPLACE_ME:SEMVER}}/1.0.0/g' \
  -e 's/{{REPLACE_ME:DESIGN_DECISION}}/strict governance validation/g' \
  -e 's/{{REPLACE_ME:IMPLEMENTATION_PATH}}/src\/governance.sh/g' \
  -e 's/{{REPLACE_ME:STAGE_ID}}/test/g' \
  "$TEMPLATE_GOV_DIR/TRACEABILITY.md.example" >"$TEMPLATE_GOV_ROOT/TRACEABILITY.md"
sed \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's/{{REPLACE_ME:CHANGE_TITLE}}/Adopt portable governance/g' \
  -e 's/Requirements Confirmed Status: pending/Requirements Confirmed Status: confirmed/' \
  -e 's/Requirements Confirmed By: pending/Requirements Confirmed By: Example Product Owner/' \
  -e 's/Requirements Confirmer Role: pending/Requirements Confirmer Role: Product Owner/' \
  -e 's/Requirements Confirmed On: pending/Requirements Confirmed On: 2026-01-02/' \
  -e 's#Requirements Confirmation Evidence: pending#Requirements Confirmation Evidence: review/requirements-CHG-001#' \
  -e 's/Implementation Plan Approved Status: pending/Implementation Plan Approved Status: approved/' \
  -e 's/Implementation Plan Approved By: pending/Implementation Plan Approved By: Example Maintainer/' \
  -e 's/Implementation Plan Approver Role: pending/Implementation Plan Approver Role: Maintainer/' \
  -e 's/Implementation Plan Approved On: pending/Implementation Plan Approved On: 2026-01-02/' \
  -e 's#Implementation Plan Approval Evidence: pending#Implementation Plan Approval Evidence: review/plan-CHG-001#' \
  "$TEMPLATE_GOV_DIR/DEVELOPMENT_PLAN.md.example" >"$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.md"
sed \
  -e 's/{{REPLACE_ME:ACTION_ORIENTED_OUTCOME}}/Adopt portable governance/g' \
  -e 's/{{REPLACE_ME:PURPOSE_AND_OBSERVABLE_OUTCOME}}/The completed template passes strict SDD verification/g' \
  -e 's/{{REPLACE_ME:CONTEXT_AND_CONSTRAINTS}}/Use only repository-contained portable contracts/g' \
  -e 's/{{REPLACE_ME:REQUIREMENT_COVERAGE}}/REQ-CHG-001-001 is satisfied by TASK-CHG-001-001./g' \
  -e 's/{{REPLACE_ME:TECHNICAL_DESIGN}}/Validate Markdown identities and current-run stage evidence./g' \
  -e 's/{{REPLACE_ME:CHANGE_ID}}/CHG-001/g' \
  -e 's#{{REPLACE_ME:FILES_OR_COMPONENTS}}#src/governance.sh#g' \
  -e 's/{{REPLACE_ME:VERIFICATION_BASIS}}/AC-CHG-001-001 is verified by TEST-CHG-001-001 in test./g' \
  -e 's/{{REPLACE_ME:TIMESTAMPED_PROGRESS}}/2026-08-14 - complete template adoption/g' \
  -e 's/{{REPLACE_ME:SURPRISES_AND_DISCOVERIES}}/None./g' \
  -e 's/{{REPLACE_ME:DECISION_LOG}}/Use the portable strict-SDD contract; 2026-08-14; Harness maintainers./g' \
  -e 's/{{REPLACE_ME:RECOVERY_AND_IDEMPOTENCE}}/The isolated fixture is safe to recreate./g' \
  -e 's/{{REPLACE_ME:OUTCOMES_AND_RETROSPECTIVE}}/Template adoption is verified by the Harness selftest./g' \
  "$TEMPLATE_GOV_DIR/EXECUTION_PLAN.md.example" >"$TEMPLATE_GOV_ROOT/plans/CHG-001.md"

TEMPLATE_SDD_DIGESTS="$(HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" "$HARNESS" sdd:digest)"
TEMPLATE_SDD_VERSION="$(printf '%s\n' "$TEMPLATE_SDD_DIGESTS" | sed -n 's/^Approved Specification Version: //p')"
TEMPLATE_SDD_SPEC_DIGEST="$(printf '%s\n' "$TEMPLATE_SDD_DIGESTS" | sed -n 's/^Approved Specification Digest: //p')"
TEMPLATE_SDD_PLAN_DIGEST="$(printf '%s\n' "$TEMPLATE_SDD_DIGESTS" | sed -n 's/^Approved Plan Basis Digest: //p')"
TEMPLATE_SDD_TEST_DIGEST="$(printf '%s\n' "$TEMPLATE_SDD_DIGESTS" | sed -n 's/^Approved Test Plan Digest: //p')"
sed \
  -e "s/Approved Specification Version: pending/Approved Specification Version: $TEMPLATE_SDD_VERSION/" \
  -e "s/Approved Specification Digest: pending/Approved Specification Digest: $TEMPLATE_SDD_SPEC_DIGEST/" \
  -e "s/Approved Plan Basis Digest: pending/Approved Plan Basis Digest: $TEMPLATE_SDD_PLAN_DIGEST/" \
  -e "s/Approved Test Plan Digest: pending/Approved Test Plan Digest: $TEMPLATE_SDD_TEST_DIGEST/" \
  "$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.md" >"$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.updated"
mv "$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.updated" "$TEMPLATE_GOV_ROOT/DEVELOPMENT_PLAN.md"
HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" "$HARNESS" governance:approved >/dev/null
mkdir -p "$TEMPLATE_GOV_ROOT/src" "$TEMPLATE_GOV_ROOT/tests"
printf '%s\n' '# adopted implementation' >"$TEMPLATE_GOV_ROOT/src/governance.sh"
printf '%s\n' '# adopted executable test' >"$TEMPLATE_GOV_ROOT/tests/governance.test"
sed 's/| Planned |/| Verified |/' "$TEMPLATE_GOV_ROOT/TRACEABILITY.md" \
  >"$TEMPLATE_GOV_ROOT/TRACEABILITY.updated"
mv "$TEMPLATE_GOV_ROOT/TRACEABILITY.updated" "$TEMPLATE_GOV_ROOT/TRACEABILITY.md"
HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" "$HARNESS" sdd:check >/dev/null
HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" "$HARNESS" verify >/dev/null
grep -Fq '"stage":"test","status":"passed"' \
  "$TEMPLATE_GOV_ROOT/test-results/harness/verify.json"
grep -Fq '"stage":"sdd-check","status":"passed"' \
  "$TEMPLATE_GOV_ROOT/test-results/harness/verify.json"

rm "$TEMPLATE_GOV_ROOT/plans/CHG-001.md"
expect_failure env \
  HARNESS_ROOT_DIR="$TEMPLATE_GOV_ROOT" \
  "$HARNESS" governance:doctor

PARTIAL_GOV_ROOT="$TEST_ROOT/partial-governance"
mkdir -p "$PARTIAL_GOV_ROOT"
printf '# Specification\nCHG-101\n' >"$PARTIAL_GOV_ROOT/SPECIFICATION.md"
HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$PARTIAL_GOV_ROOT" "$HARNESS" governance:doctor >/dev/null
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$PARTIAL_GOV_ROOT" \
  HARNESS_GOVERNANCE_MODE=required \
  "$HARNESS" governance:doctor

expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  HARNESS_GOVERNANCE_MODE=auto \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor
expect_failure env \
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" \
  HARNESS_ROOT_DIR="$EMPTY_ROOT" \
  HARNESS_GOVERNANCE_MODE=off \
  HARNESS_REQUIRE_PLAN_APPROVAL=true \
  "$HARNESS" governance:doctor

if command -v git >/dev/null 2>&1; then
  REPO_ROOT="$TEST_ROOT/repository"
  mkdir -p "$REPO_ROOT"
  git -C "$REPO_ROOT" init -q
  printf 'SECRET=placeholder\n' >"$REPO_ROOT/.env"
  git -C "$REPO_ROOT" add -f .env
  HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$REPO_ROOT" "$HARNESS" repo:doctor >/dev/null
  expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$REPO_ROOT" HARNESS_STRICT_REPO_HYGIENE=true "$HARNESS" repo:doctor
fi

expect_failure env HARNESS_CONFIG_FILE="$EMPTY_CONFIG" HARNESS_ROOT_DIR="$EMPTY_ROOT" "$HARNESS" unknown-command

if command -v shellcheck >/dev/null 2>&1; then
  shellcheck \
    "$HARNESS" \
    "$HARNESS_DIR/lib/adapters.sh" \
    "$HARNESS_DIR/lib/governance.sh" \
    "$HARNESS_DIR/lib/instructions.sh" \
    "$HARNESS_DIR/lib/reporting.sh" \
    "${BASH_SOURCE[0]}"
else
  echo "note: shellcheck unavailable; syntax and behavioral self-tests still ran"
fi

echo "ok: portable harness self-tests passed"
