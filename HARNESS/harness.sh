#!/bin/bash -p
case "$-" in
  *p*) ;;
  *) printf 'Run the Harness executable directly; an unprivileged Bash startup is not supported.\n' >&2; exit 1 ;;
esac
builtin trap - DEBUG RETURN ERR
builtin unset BASH_ENV ENV CDPATH GLOBIGNORE BASH_XTRACEFD
builtin set +T +E +x
builtin set -euo pipefail
IFS=$' \t\n'

SCRIPT_PATH="${BASH_SOURCE[0]}"
case "$SCRIPT_PATH" in
  */*) SCRIPT_DIR_INPUT="${SCRIPT_PATH%/*}" ;;
  *) SCRIPT_DIR_INPUT="." ;;
esac
SCRIPT_DIR="$(builtin cd "$SCRIPT_DIR_INPUT" && builtin pwd -P)"
SCRIPT_ROOT="$(builtin cd "$SCRIPT_DIR/.." && builtin pwd -P)"
ROOT_DIR_INPUT="${HARNESS_ROOT_DIR:-$SCRIPT_ROOT}"
if [[ -d "$ROOT_DIR_INPUT" ]]; then
  ROOT_DIR="$(builtin cd "$ROOT_DIR_INPUT" && builtin pwd -P)"
else
  ROOT_DIR="$ROOT_DIR_INPUT"
fi
readonly PORTABLE_HARNESS_VERSION="1.0.0"
CONFIG_EXPLICIT="false"
ADOPTED_PROJECT="false"
if [[ -n "${HARNESS_CONFIG_FILE:-}" ]]; then
  CONFIG_EXPLICIT="true"
fi
CONFIG_FILE_INPUT="${HARNESS_CONFIG_FILE:-$ROOT_DIR/HARNESS/config.env}"
case "$CONFIG_FILE_INPUT" in
  */*) CONFIG_PARENT_INPUT="${CONFIG_FILE_INPUT%/*}"; CONFIG_BASENAME="${CONFIG_FILE_INPUT##*/}" ;;
  *) CONFIG_PARENT_INPUT="."; CONFIG_BASENAME="$CONFIG_FILE_INPUT" ;;
esac
if [[ -d "$CONFIG_PARENT_INPUT" ]]; then
  CONFIG_FILE="$(builtin cd "$CONFIG_PARENT_INPUT" && builtin pwd -P)/$CONFIG_BASENAME"
else
  CONFIG_FILE="$CONFIG_FILE_INPUT"
fi

# An adopted project must select its repository and executable policy before
# sourcing trusted Bash. This prevents caller-controlled alternate roots or
# config files from exiting early and bypassing the committed gate.
if [[ -e "$SCRIPT_ROOT/PROJECT.md" || -L "$SCRIPT_ROOT/PROJECT.md" ]]; then
  [[ "$ROOT_DIR" == "$SCRIPT_ROOT" ]] || {
    builtin printf 'An adopted Harness cannot override HARNESS_ROOT_DIR.\n' >&2
    exit 1
  }
fi
if [[ -e "$ROOT_DIR/PROJECT.md" || -L "$ROOT_DIR/PROJECT.md" ]]; then
  ADOPTED_PROJECT="true"
  [[ "$CONFIG_FILE" == "$ROOT_DIR/HARNESS/config.env" ]] || {
    builtin printf 'An adopted project must use committed HARNESS/config.env.\n' >&2
    exit 1
  }
  [[ ! -L "$ROOT_DIR/HARNESS" && -f "$CONFIG_FILE" && ! -L "$CONFIG_FILE" && -s "$CONFIG_FILE" ]] || {
    builtin printf 'An adopted project requires a non-empty, regular HARNESS/config.env in a non-symlink HARNESS directory.\n' >&2
    exit 1
  }
fi

CONFIG_VARIABLE_NAMES=(
  HARNESS_PROJECT_NAME HARNESS_REQUIRED_PATHS HARNESS_EXPECTED_BASELINE_VERSION
  HARNESS_SDD_MODE
  HARNESS_REQUIRED_STAGES HARNESS_VERIFY_STAGES
  HARNESS_GOVERNANCE_MODE HARNESS_REQUIRE_PLAN_APPROVAL
  HARNESS_ACTIVE_CHANGE_FILE HARNESS_CHANGE_ID_PATTERN
  HARNESS_SPEC_FILE HARNESS_CHANGELOG_FILE HARNESS_PLAN_FILE
  HARNESS_TEST_PLAN_FILE HARNESS_TRACE_FILE HARNESS_COVERAGE_THRESHOLD
  HARNESS_STRICT_REPO_HYGIENE HARNESS_REQUIRED_TOOLS HARNESS_ENVIRONMENT_COMMAND
  HARNESS_COMMAND_TIMEOUT_SECONDS HARNESS_INSTRUCTION_MAX_BYTES
  HARNESS_INSTRUCTION_FALLBACK_FILENAMES HARNESS_INSTRUCTION_TARGETS
  HARNESS_INSTRUCTION_REFERENCES HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE
  HARNESS_NETWORK_POLICY HARNESS_NETWORK_STAGES HARNESS_WRITE_REPORTS
  HARNESS_REPORT_DIR HARNESS_REPORT_FILE HARNESS_COMMAND_ID_PREFIX
  HARNESS_REDACT_ENV_VARS HARNESS_REDACT_MIN_BYTES
  HARNESS_EVIDENCE_MAX_STRING_BYTES HARNESS_EVIDENCE_MAX_REPORT_BYTES
  HARNESS_LINT_COMMAND HARNESS_FORMAT_COMMAND HARNESS_TYPECHECK_COMMAND
  HARNESS_TEST_COMMAND HARNESS_COVERAGE_COMMAND HARNESS_BUILD_COMMAND
  HARNESS_INTEGRATION_COMMAND HARNESS_E2E_COMMAND HARNESS_SECURITY_COMMAND
  HARNESS_DEPLOY_COMMAND HARNESS_CI_POLICY_COMMAND HARNESS_SUPPLY_CHAIN_COMMAND
  HARNESS_AGENT_EVAL_DIR HARNESS_AGENT_CASE_SCHEMA HARNESS_AGENT_RESULT_SCHEMA
  HARNESS_AGENT_CONTRACT_COMMAND HARNESS_AGENT_RESULT_DIR HARNESS_AGENT_TRIALS
  HARNESS_AGENT_REGRESSION_COMMAND HARNESS_AGENT_CAPABILITY_COMMAND
)
CONFIG_DECLARATIONS=""
load_adopted_config() {
  local marker="__PORTABLE_HARNESS_CONFIG_1_0_COMPLETE__"
  local dump
  if ! dump="$(/bin/bash -p -euo pipefail -c '
    config_file=$1
    shift
    for config_name in "$@"; do builtin unset "$config_name" 2>/dev/null || true; done
    builtin unset HARNESS_CONFIG_COMPLETE 2>/dev/null || true
    builtin source "$config_file" 1>&2
    [[ "${HARNESS_CONFIG_COMPLETE:-}" == "1.0.0" ]] || {
      printf "Committed HARNESS/config.env did not reach its required completion marker.\n" >&2
      exit 64
    }
    builtin printf "%s\n" "__PORTABLE_HARNESS_CONFIG_1_0_COMPLETE__"
    for config_name in "$@"; do
      if [[ ${!config_name+x} ]]; then
        builtin printf "%s=" "$config_name"
        builtin printf "%q" "${!config_name}"
        builtin printf "\n"
      fi
    done
  ' portable-harness-config "$CONFIG_FILE" "${CONFIG_VARIABLE_NAMES[@]}")"; then
    echo "Committed HARNESS/config.env failed isolated loading." >&2
    exit 1
  fi
  case "$dump" in
    "$marker") CONFIG_DECLARATIONS="" ;;
    "$marker"$'\n'*) CONFIG_DECLARATIONS="${dump#"$marker"$'\n'}" ;;
    *) echo "Committed HARNESS/config.env did not produce a valid completion record." >&2; exit 1 ;;
  esac
}

builtin readonly SCRIPT_PATH SCRIPT_DIR_INPUT SCRIPT_DIR SCRIPT_ROOT ROOT_DIR_INPUT ROOT_DIR
builtin readonly CONFIG_FILE_INPUT CONFIG_PARENT_INPUT CONFIG_BASENAME CONFIG_FILE CONFIG_EXPLICIT ADOPTED_PROJECT

if [[ -f "$CONFIG_FILE" ]]; then
  if [[ "$ADOPTED_PROJECT" == "true" ]]; then
    load_adopted_config
    for config_name in "${CONFIG_VARIABLE_NAMES[@]}"; do
      builtin unset "$config_name" 2>/dev/null || true
    done
    builtin eval "$CONFIG_DECLARATIONS"
    builtin unset CONFIG_DECLARATIONS
  else
    # Pre-adoption maintainer fixtures may supply an alternate trusted config.
    # shellcheck source=/dev/null
    builtin source "$CONFIG_FILE"
  fi
elif [[ "$CONFIG_EXPLICIT" == "true" ]]; then
  builtin printf 'Configured harness file does not exist: %s\n' "$CONFIG_FILE" >&2
  exit 1
fi

: "${HARNESS_PROJECT_NAME:=}"
: "${HARNESS_REQUIRED_PATHS:=}"
: "${HARNESS_EXPECTED_BASELINE_VERSION:=}"
if [[ ! ${HARNESS_SDD_MODE+x} ]]; then
  if [[ "$ADOPTED_PROJECT" == "true" ]]; then
    HARNESS_SDD_MODE="required"
  else
    HARNESS_SDD_MODE="off"
  fi
fi
: "${HARNESS_REQUIRED_STAGES:=}"
: "${HARNESS_VERIFY_STAGES:=$HARNESS_REQUIRED_STAGES}"
: "${HARNESS_GOVERNANCE_MODE:=auto}"
: "${HARNESS_REQUIRE_PLAN_APPROVAL:=false}"
: "${HARNESS_ACTIVE_CHANGE_FILE:=HARNESS/ACTIVE_CHANGE}"
: "${HARNESS_CHANGE_ID_PATTERN:=[A-Z][A-Z0-9_-]*-[0-9]+}"
: "${HARNESS_SPEC_FILE:=SPECIFICATION.md}"
: "${HARNESS_CHANGELOG_FILE:=SPEC_CHANGELOG.md}"
: "${HARNESS_PLAN_FILE:=DEVELOPMENT_PLAN.md}"
: "${HARNESS_TEST_PLAN_FILE:=TEST_PLAN.md}"
: "${HARNESS_TRACE_FILE:=TRACEABILITY.md}"
: "${HARNESS_COVERAGE_THRESHOLD:=80}"
: "${HARNESS_STRICT_REPO_HYGIENE:=false}"
: "${HARNESS_REQUIRED_TOOLS:=}"
: "${HARNESS_ENVIRONMENT_COMMAND:=}"
: "${HARNESS_COMMAND_TIMEOUT_SECONDS:=0}"
: "${HARNESS_INSTRUCTION_MAX_BYTES:=32768}"
: "${HARNESS_INSTRUCTION_FALLBACK_FILENAMES:=}"
: "${HARNESS_INSTRUCTION_TARGETS:=.}"
: "${HARNESS_INSTRUCTION_REFERENCES:=}"
: "${HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE:=false}"
: "${HARNESS_NETWORK_POLICY:=declared}"
: "${HARNESS_NETWORK_STAGES:=integration,e2e,security,deploy,ci-policy,supply-chain,agent-regression,agent-capability}"
: "${HARNESS_WRITE_REPORTS:=true}"
: "${HARNESS_REPORT_DIR:=test-results/harness}"
: "${HARNESS_REPORT_FILE:=}"
: "${HARNESS_COMMAND_ID_PREFIX:=project.v1}"
: "${HARNESS_REDACT_ENV_VARS:=}"
: "${HARNESS_REDACT_MIN_BYTES:=8}"
: "${HARNESS_EVIDENCE_MAX_STRING_BYTES:=65536}"
: "${HARNESS_EVIDENCE_MAX_REPORT_BYTES:=10485760}"
: "${HARNESS_LINT_COMMAND:=}"
: "${HARNESS_FORMAT_COMMAND:=}"
: "${HARNESS_TYPECHECK_COMMAND:=}"
: "${HARNESS_TEST_COMMAND:=}"
: "${HARNESS_COVERAGE_COMMAND:=}"
: "${HARNESS_BUILD_COMMAND:=}"
: "${HARNESS_INTEGRATION_COMMAND:=}"
: "${HARNESS_E2E_COMMAND:=}"
: "${HARNESS_SECURITY_COMMAND:=}"
: "${HARNESS_DEPLOY_COMMAND:=}"
: "${HARNESS_CI_POLICY_COMMAND:=}"
: "${HARNESS_SUPPLY_CHAIN_COMMAND:=}"
: "${HARNESS_AGENT_EVAL_DIR:=HARNESS/evals}"
: "${HARNESS_AGENT_CASE_SCHEMA:=HARNESS/schemas/agent-eval-case.schema.json}"
: "${HARNESS_AGENT_RESULT_SCHEMA:=HARNESS/schemas/agent-eval-result.schema.json}"
: "${HARNESS_AGENT_CONTRACT_COMMAND:=}"
: "${HARNESS_AGENT_RESULT_DIR:=test-results/harness/agent-results}"
: "${HARNESS_AGENT_REGRESSION_COMMAND:=}"
: "${HARNESS_AGENT_CAPABILITY_COMMAND:=}"
: "${HARNESS_AGENT_TRIALS:=3}"

export HARNESS_COVERAGE_THRESHOLD
export HARNESS_AGENT_TRIALS

# shellcheck source=lib/reporting.sh
source "$SCRIPT_DIR/lib/reporting.sh"
# shellcheck source=lib/instructions.sh
source "$SCRIPT_DIR/lib/instructions.sh"
trap 'harness_private_tmpdir_cleanup' EXIT
trap 'harness_private_tmpdir_cleanup; exit 129' HUP
trap 'harness_private_tmpdir_cleanup; exit 130' INT
trap 'harness_private_tmpdir_cleanup; exit 143' TERM

section() {
  printf '\n== %s ==\n' "$1"
}

note() {
  printf '%s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "required command not found: $1"
  fi
}

required_repository_path_into() {
  local outvar="$1"
  local configured="$2"
  local root parent_input basename parent physical
  case "$configured" in
    ""|/*|.|..|./*|../*|*/./*|*/.|*/../*|*/..|*[[:cntrl:]]*)
      die "required project paths must be normalized and repository-relative: $configured"
      ;;
  esac
  root="$(cd "$ROOT_DIR" && pwd -P)" \
    || die "repository root does not exist: $ROOT_DIR"
  [[ -e "$root/$configured" || -L "$root/$configured" ]] \
    || die "missing required path: $configured"
  [[ ! -L "$root/$configured" ]] \
    || die "required project paths may not be symbolic links: $configured"
  case "$configured" in
    */*) parent_input="${configured%/*}"; basename="${configured##*/}" ;;
    *) parent_input="."; basename="$configured" ;;
  esac
  parent="$(cd "$root/$parent_input" 2>/dev/null && pwd -P)" \
    || die "required project path parent is not accessible: $configured"
  physical="$parent/$basename"
  case "$physical" in
    "$root"/*) ;;
    *) die "required project path escapes the repository: $configured" ;;
  esac
  printf -v "$outvar" '%s' "$physical"
}

run_with_timeout() {
  if [[ "$HARNESS_COMMAND_TIMEOUT_SECONDS" -eq 0 ]]; then
    "$@"
    return
  fi

  if command -v timeout >/dev/null 2>&1; then
    timeout "$HARNESS_COMMAND_TIMEOUT_SECONDS" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$HARNESS_COMMAND_TIMEOUT_SECONDS" "$@"
  else
    die "HARNESS_COMMAND_TIMEOUT_SECONDS requires timeout or gtimeout"
  fi
}

run() {
  # Arguments can contain credentials or expanded configuration. The stable
  # command identity is recorded by the surrounding stage; raw argv is never
  # echoed by the shared Harness.
  printf '+ executable\n'
  run_with_timeout "$@"
}

run_in() {
  local directory="$1"
  shift
  (
    cd "$directory"
    run "$@"
  )
}

run_shell() {
  local command_text="$1"
  local command_id="${2:-project.v1:command}"
  harness_validate_command_id "$command_id"
  printf '+ command %s\n' "$command_id"
  (
    local command_script syntax_log status=0
    harness_private_temp_into command_script command || exit 1
    harness_private_temp_into syntax_log syntax || { rm -f "$command_script"; exit 1; }
    trap 'rm -f "$command_script" "$syntax_log"' EXIT
    trap 'rm -f "$command_script" "$syntax_log"; exit 129' HUP
    trap 'rm -f "$command_script" "$syntax_log"; exit 130' INT
    trap 'rm -f "$command_script" "$syntax_log"; exit 143' TERM
    printf '%s\n' "$command_text" >"$command_script" || exit 1
    if ! /bin/bash -p -n "$command_script" 2>"$syntax_log"; then
      printf 'error: configured command has invalid Bash syntax\n' >&2
      exit 2
    fi
    cd "$ROOT_DIR"
    # Runtime shell and child-process output remains project-owned. Projects
    # must avoid commands whose own diagnostics disclose command source text.
    # Fail on the first unhandled simple-command or pipeline failure. A project
    # can still opt out deliberately inside its script with an explicit
    # `set +e`, just as it can in any Bash script started with `bash -e`.
    # Privileged mode here is a startup-isolation flag: with equal real and
    # effective IDs it prevents BASH_ENV, SHELLOPTS, and exported functions
    # from rewriting the configured command contract.
    run_with_timeout /bin/bash -p -e -o pipefail "$command_script" || status=$?
    exit "$status"
  )
}

is_true() {
  case "$1" in
    true|TRUE|yes|YES|1) return 0 ;;
    *) return 1 ;;
  esac
}

validate_boolean() {
  case "$2" in
    true|TRUE|false|FALSE|yes|YES|no|NO|1|0) ;;
    *) die "$1 must be true or false, got: $2" ;;
  esac
}

stage_is_required() {
  local normalized="${HARNESS_REQUIRED_STAGES//[[:space:]]/}"
  case ",$normalized," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

report_skip() {
  local stage="$1"
  local reason="$2"
  if stage_is_required "$stage"; then
    printf 'error: %s is required but was skipped: %s\n' "$stage" "$reason" >&2
    return 1
  fi
  note "skip: $reason"
  return 0
}

stage_is_verified() {
  local normalized="${HARNESS_VERIFY_STAGES//[[:space:]]/}"
  case ",$normalized," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

csv_contains() {
  local values="${1//[[:space:]]/}"
  case ",$values," in
    *",$2,"*) return 0 ;;
    *) return 1 ;;
  esac
}

stage_network_allowed() {
  local stage="$1"
  case "$HARNESS_NETWORK_POLICY" in
    allow) return 0 ;;
    deny) return 1 ;;
    declared) csv_contains "$HARNESS_NETWORK_STAGES" "$stage" ;;
  esac
}

stage_network_guard() {
  local stage="$1"
  if [[ "$HARNESS_NETWORK_POLICY" == "deny" ]] \
    && csv_contains "$HARNESS_NETWORK_STAGES" "$stage"; then
    printf 'error: %s is declared network-dependent but HARNESS_NETWORK_POLICY=deny\n' "$stage" >&2
    return 3
  fi
  return 0
}

set_agent_network_contract() {
  local suite="$1"
  local regression_network="false"
  local capability_network="false"
  local selected_network="false"

  if stage_network_allowed agent-regression; then
    regression_network="true"
  fi
  if stage_network_allowed agent-capability; then
    capability_network="true"
  fi

  case "$suite" in
    regression) selected_network="$regression_network" ;;
    capability) selected_network="$capability_network" ;;
    all)
      # The suite-wide flag is fail-closed when individual policies differ.
      if [[ "$regression_network" == "true" && "$capability_network" == "true" ]]; then
        selected_network="true"
      fi
      ;;
    *) die "unknown agent evaluation suite: $suite" ;;
  esac

  export HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS="$regression_network"
  export HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS="$capability_network"
  export HARNESS_NETWORK_CONTRACT_ALLOWS="$selected_network"
}

validate_stage_list() {
  local setting_name="$1"
  local configured="$2"
  local stages=()
  local stage seen=","
  local normalized="${configured//[[:space:]]/}"
  [[ -n "$normalized" ]] || return 0
  [[ "$normalized" == "none" ]] && return 0
  case "$normalized" in
    ,*|*,|*,,*) die "$setting_name contains an empty stage entry" ;;
  esac
  IFS=',' read -r -a stages <<<"$configured"
  for stage in "${stages[@]}"; do
    stage="${stage//[[:space:]]/}"
    [[ -n "$stage" ]] || die "$setting_name contains an empty stage entry"
    case "$stage" in
      lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain) ;;
      *) die "unknown stage in $setting_name: $stage" ;;
    esac
    case "$seen" in
      *",$stage,"*) die "duplicate stage in $setting_name: $stage" ;;
    esac
    seen="${seen}${stage},"
  done
}

validate_stage_contract() {
  local required_stages=()
  local stage normalized_required="${HARNESS_REQUIRED_STAGES//[[:space:]]/}"
  validate_stage_list HARNESS_REQUIRED_STAGES "$HARNESS_REQUIRED_STAGES"
  validate_stage_list HARNESS_VERIFY_STAGES "$HARNESS_VERIFY_STAGES"
  [[ -z "$normalized_required" || "$normalized_required" == "none" ]] && return 0
  IFS=',' read -r -a required_stages <<<"$HARNESS_REQUIRED_STAGES"
  for stage in "${required_stages[@]}"; do
    stage="${stage//[[:space:]]/}"
    if ! stage_is_verified "$stage"; then
      die "HARNESS_REQUIRED_STAGES must be a subset of HARNESS_VERIFY_STAGES; missing: $stage"
    fi
  done
}

validate_network_stages() {
  [[ -n "$HARNESS_NETWORK_STAGES" ]] || return 0
  local stages=()
  local stage seen=","
  IFS=',' read -r -a stages <<<"$HARNESS_NETWORK_STAGES"
  for stage in "${stages[@]}"; do
    stage="${stage//[[:space:]]/}"
    case "$stage" in
      lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain|agent-regression|agent-capability) ;;
      *) die "unknown stage in HARNESS_NETWORK_STAGES: $stage" ;;
    esac
    case "$seen" in
      *",$stage,"*) die "duplicate stage in HARNESS_NETWORK_STAGES: $stage" ;;
    esac
    seen="${seen}${stage},"
  done
}

validate_settings() {
  local coverage_whole coverage_fraction=""
  if [[ -e "$ROOT_DIR/PROJECT.md" || -L "$ROOT_DIR/PROJECT.md" ]]; then
    [[ "$HARNESS_EXPECTED_BASELINE_VERSION" == "$PORTABLE_HARNESS_VERSION" ]] \
      || die "an adopted PROJECT.md requires HARNESS_EXPECTED_BASELINE_VERSION=$PORTABLE_HARNESS_VERSION"
  fi
  case "$HARNESS_GOVERNANCE_MODE" in
    auto|required|off) ;;
    *) die "HARNESS_GOVERNANCE_MODE must be auto, required, or off" ;;
  esac
  case "$HARNESS_SDD_MODE" in
    required|off) ;;
    *) die "HARNESS_SDD_MODE must be required or off" ;;
  esac
  validate_boolean HARNESS_REQUIRE_PLAN_APPROVAL "$HARNESS_REQUIRE_PLAN_APPROVAL"
  validate_boolean HARNESS_STRICT_REPO_HYGIENE "$HARNESS_STRICT_REPO_HYGIENE"
  validate_boolean HARNESS_WRITE_REPORTS "$HARNESS_WRITE_REPORTS"
  validate_boolean HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE "$HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE"
  if is_true "$HARNESS_REQUIRE_PLAN_APPROVAL" && [[ "$HARNESS_GOVERNANCE_MODE" == "off" ]]; then
    die "HARNESS_REQUIRE_PLAN_APPROVAL=true is incompatible with HARNESS_GOVERNANCE_MODE=off"
  fi
  if [[ "$HARNESS_SDD_MODE" == "required" ]]; then
    [[ "$HARNESS_GOVERNANCE_MODE" == "required" ]] \
      || die "required SDD requires HARNESS_GOVERNANCE_MODE=required"
    is_true "$HARNESS_REQUIRE_PLAN_APPROVAL" \
      || die "required SDD requires HARNESS_REQUIRE_PLAN_APPROVAL=true"
    is_true "$HARNESS_WRITE_REPORTS" \
      || die "required SDD requires HARNESS_WRITE_REPORTS=true for retained handoff evidence"
  fi
  if ! harness_decimal_at_most "$HARNESS_COMMAND_TIMEOUT_SECONDS" 86400 true; then
    die "HARNESS_COMMAND_TIMEOUT_SECONDS must be an integer from 0 to 86400"
  fi
  if [[ "$HARNESS_COMMAND_TIMEOUT_SECONDS" -ne 0 ]] \
    && ! command -v timeout >/dev/null 2>&1 \
    && ! command -v gtimeout >/dev/null 2>&1; then
    die "HARNESS_COMMAND_TIMEOUT_SECONDS requires timeout or gtimeout"
  fi
  if ! harness_decimal_at_most "$HARNESS_AGENT_TRIALS" 1000; then
    die "HARNESS_AGENT_TRIALS must be an integer from 1 to 1000"
  fi
  harness_reporting_validate_settings
  instructions_validate_settings
  validate_stage_contract
  validate_network_stages
  if [[ ! "$HARNESS_COVERAGE_THRESHOLD" =~ ^(0|[1-9][0-9]*)([.][0-9]+)?$ ]]; then
    die "HARNESS_COVERAGE_THRESHOLD must be a canonical decimal from 0 to 100"
  fi
  coverage_whole="${HARNESS_COVERAGE_THRESHOLD%%.*}"
  if [[ "$HARNESS_COVERAGE_THRESHOLD" == *.* ]]; then
    coverage_fraction="${HARNESS_COVERAGE_THRESHOLD#*.}"
  fi
  if [[ "${#coverage_whole}" -gt 3 ]] \
    || [[ "$coverage_whole" -gt 100 ]] \
    || { [[ "$coverage_whole" -eq 100 && -n "$coverage_fraction" ]] \
      && [[ ! "$coverage_fraction" =~ ^0+$ ]]; }; then
    die "HARNESS_COVERAGE_THRESHOLD must be a canonical decimal from 0 to 100"
  fi
  case "$HARNESS_NETWORK_POLICY" in
    deny|declared|allow) ;;
    *) die "HARNESS_NETWORK_POLICY must be deny, declared, or allow" ;;
  esac
  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" && "$HARNESS_EXPECTED_BASELINE_VERSION" != "$PORTABLE_HARNESS_VERSION" ]]; then
    die "harness baseline mismatch: expected $HARNESS_EXPECTED_BASELINE_VERSION, found $PORTABLE_HARNESS_VERSION"
  fi
  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" && -d "$ROOT_DIR" ]]; then
    [[ -f "$ROOT_DIR/AGENTS.md" ]] || die "AGENTS.md is required by the shared baseline version contract"
    if ! grep -Fq "Shared baseline version: **$HARNESS_EXPECTED_BASELINE_VERSION**" "$ROOT_DIR/AGENTS.md"; then
      die "AGENTS.md baseline version does not match $HARNESS_EXPECTED_BASELINE_VERSION"
    fi
  fi
}

is_ignored_path() {
  case "$1" in
    */.git/*|*/node_modules/*|*/vendor/*|*/.venv/*|*/venv/*|*/dist/*|*/build/*|*/.next/*)
      return 0
      ;;
    *) return 1 ;;
  esac
}

environment_doctor() {
  section "Environment doctor"
  note "os: $(uname -srm 2>/dev/null || uname -a)"
  note "bash: $BASH_VERSION"
  note "network policy: $HARNESS_NETWORK_POLICY"
  note "network-declared stages: ${HARNESS_NETWORK_STAGES:-none}"
  if [[ "$HARNESS_COMMAND_TIMEOUT_SECONDS" -eq 0 ]]; then
    note "command timeout: disabled"
  else
    note "command timeout: ${HARNESS_COMMAND_TIMEOUT_SECONDS}s"
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
      die "configured command timeout requires timeout or gtimeout"
    fi
  fi

  if [[ -n "$HARNESS_REQUIRED_TOOLS" ]]; then
    local tools=()
    local tool
    IFS=',' read -r -a tools <<<"$HARNESS_REQUIRED_TOOLS"
    for tool in "${tools[@]}"; do
      tool="${tool//[[:space:]]/}"
      [[ -n "$tool" ]] || continue
      require_command "$tool"
      note "ok: required tool $tool"
    done
  fi

  if [[ -n "$HARNESS_ENVIRONMENT_COMMAND" ]]; then
    local environment_command_id
    harness_command_id_into environment_command_id environment-doctor
    # Environment probes are not project stages and never receive network
    # permission. Clear potentially caller-controlled reserved values before
    # exporting a fail-closed contract for the configured probe.
    (
      unset HARNESS_AGENT_REGRESSION_NETWORK_CONTRACT_ALLOWS
      unset HARNESS_AGENT_CAPABILITY_NETWORK_CONTRACT_ALLOWS
      export HARNESS_NETWORK_CONTRACT_ALLOWS="false"
      run_shell "$HARNESS_ENVIRONMENT_COMMAND" "$environment_command_id"
    )
  fi
}

doctor() {
  section "Harness doctor"

  [[ -d "$ROOT_DIR" ]] || die "repository root does not exist: $ROOT_DIR"

  note "root: $ROOT_DIR"
  note "project: ${HARNESS_PROJECT_NAME:-$(basename "$ROOT_DIR")}" 
  if [[ -f "$CONFIG_FILE" ]]; then
    note "config: $CONFIG_FILE"
  else
    note "config: automatic detection (no HARNESS/config.env)"
  fi
  note "governance: $HARNESS_GOVERNANCE_MODE"
  note "SDD mode: $HARNESS_SDD_MODE"
  note "harness version: $PORTABLE_HARNESS_VERSION"
  note "coverage threshold: $HARNESS_COVERAGE_THRESHOLD%"
  note "evidence directory: $HARNESS_REPORT_DIR"
  if [[ -n "$HARNESS_REQUIRED_STAGES" ]]; then
    note "required stages: $HARNESS_REQUIRED_STAGES"
  else
    note "warning: no required stages declared; skipped stages will not fail"
  fi
  note "verify stages: ${HARNESS_VERIFY_STAGES:-none}"

  if [[ -n "$HARNESS_REQUIRED_PATHS" ]]; then
    local required_paths=()
    local required_path resolved_required_path
    IFS=':' read -r -a required_paths <<<"$HARNESS_REQUIRED_PATHS"
    for required_path in "${required_paths[@]}"; do
      required_repository_path_into resolved_required_path "$required_path"
      note "ok: required path $required_path"
    done
  fi

  local detected=""
  [[ -f "$ROOT_DIR/package.json" ]] && detected="${detected} Node.js"
  [[ -f "$ROOT_DIR/pyproject.toml" || -f "$ROOT_DIR/requirements.txt" ]] && detected="${detected} Python"
  [[ -f "$ROOT_DIR/go.mod" ]] && detected="${detected} Go"
  [[ -f "$ROOT_DIR/Cargo.toml" ]] && detected="${detected} Rust"
  [[ -f "$ROOT_DIR/Makefile" || -f "$ROOT_DIR/makefile" ]] && detected="${detected} Make"
  note "detected:${detected:- none at repository root}"
  harness_evidence_ignore_doctor
  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" || -e "$ROOT_DIR/PROJECT.md" || -L "$ROOT_DIR/PROJECT.md" ]]; then
    instructions_doctor_configured_targets
  fi
  environment_doctor
}

# shellcheck source=lib/governance.sh
source "$SCRIPT_DIR/lib/governance.sh"
# shellcheck source=lib/sdd.sh
source "$SCRIPT_DIR/lib/sdd.sh"
# shellcheck source=lib/adapters.sh
source "$SCRIPT_DIR/lib/adapters.sh"
execute_stage() {
  local stage="$1"
  local title="$2"
  local variable_name="$3"
  shift 3
  local configured="${!variable_name:-}"
  local network_contract_allows="false"
  local report_command_id="auto:$stage"
  local report_command_source="auto-detected"
  local status=0

  if stage_network_allowed "$stage"; then
    network_contract_allows="true"
  fi
  export HARNESS_NETWORK_CONTRACT_ALLOWS="$network_contract_allows"
  if [[ -n "$configured" ]]; then
    harness_command_id_into report_command_id "$stage"
    report_command_source="configured"
  else
    harness_validate_command_id "$report_command_id"
  fi

  section "$title"
  harness_report_stage_begin "$stage" "$report_command_id" "$report_command_source" "$network_contract_allows"
  if stage_network_guard "$stage"; then
    :
  else
    status=$?
    harness_report_stage_finish blocked "$status" "network policy blocked the stage"
    return "$status"
  fi

  if [[ -n "$configured" ]]; then
    if [[ "$configured" == "skip" ]]; then
      if report_skip "$stage" "explicitly disabled by $variable_name"; then
        harness_report_stage_finish skipped 0 "explicitly disabled by $variable_name"
        return 0
      fi
      harness_report_stage_finish failed 1 "required stage was explicitly disabled"
      return 1
    fi
    if run_shell "$configured" "$report_command_id"; then
      harness_report_stage_finish passed 0 "configured command"
      return 0
    else
      status=$?
    fi
    if [[ "$status" -eq 3 ]]; then
      harness_report_stage_finish blocked "$status" "configured command reported blocked/unavailable"
    else
      harness_report_stage_finish failed "$status" "configured command failed"
    fi
    return "$status"
  fi

  STAGE_RAN=0
  run_make_stage "$@"
  if [[ "$RUNNER_FOUND" -eq 1 ]]; then
    harness_report_stage_finish passed 0 "detected Make target"
    return 0
  fi
  run_node_stage "$@"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1
  run_python_stage "$stage"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1
  run_compiled_stage "$stage"
  [[ "$RUNNER_FOUND" -eq 1 ]] && STAGE_RAN=1

  if [[ "$STAGE_RAN" -eq 0 ]]; then
    if report_skip "$stage" "no configured or detected $stage command"; then
      harness_report_stage_finish skipped 0 "no configured or detected command"
      return 0
    fi
    harness_report_stage_finish failed 1 "required stage has no implementation"
    return 1
  fi
  harness_report_stage_finish passed 0 "auto-detected command"
}

lint_stage() {
  execute_stage lint "Lint" HARNESS_LINT_COMMAND lint
}

format_stage() {
  execute_stage format "Format check" HARNESS_FORMAT_COMMAND format-check format:check check-format check:format
}

typecheck_stage() {
  execute_stage typecheck "Type check" HARNESS_TYPECHECK_COMMAND typecheck type-check check-types check:types
}

test_stage() {
  execute_stage test "Tests" HARNESS_TEST_COMMAND test
}

coverage_stage() {
  execute_stage coverage "Coverage" HARNESS_COVERAGE_COMMAND test-coverage test:coverage coverage
}

build_stage() {
  execute_stage build "Build" HARNESS_BUILD_COMMAND build
}

integration_stage() {
  execute_stage integration "Integration tests" HARNESS_INTEGRATION_COMMAND test-integration test:integration integration-test integration:test
}

e2e_stage() {
  execute_stage e2e "End-to-end tests" HARNESS_E2E_COMMAND test-e2e test:e2e e2e
}

security_stage() {
  execute_stage security "Security" HARNESS_SECURITY_COMMAND security security-check security:check audit
}

deploy_stage() {
  execute_stage deploy "Deployment check" HARNESS_DEPLOY_COMMAND deploy-check deploy:check deploy-validate deploy:validate
}

ci_policy_stage() {
  execute_stage ci-policy "CI policy" HARNESS_CI_POLICY_COMMAND ci-policy ci:policy
}

supply_chain_stage() {
  execute_stage supply-chain "Supply-chain evidence" HARNESS_SUPPLY_CHAIN_COMMAND supply-chain supply:chain provenance sbom
}

configured_agent_stage() {
  local stage="$1"
  local title="$2"
  local variable_name="$3"
  local required="$4"
  local suite="$5"
  local configured="${!variable_name:-}"
  local network_contract_allows="false"
  local eval_directory summary_result_directory summary_result_file canonical_result_directory result_file result_schema
  local report_command_id="unconfigured:$stage" report_command_source="unconfigured"
  local runner_status=0 validator_status=0

  set_agent_network_contract "$suite"
  network_contract_allows="$HARNESS_NETWORK_CONTRACT_ALLOWS"
  if [[ -n "$configured" && "$configured" != "skip" ]]; then
    harness_command_id_into report_command_id "$stage"
    report_command_source="configured"
  else
    harness_validate_command_id "$report_command_id"
  fi

  section "$title"
  harness_report_stage_begin "$stage" "$report_command_id" "$report_command_source" "$network_contract_allows"
  if [[ -z "$configured" || "$configured" == "skip" ]]; then
    if [[ "$required" == "true" ]]; then
      printf 'error: %s requires %s\n' "$stage" "$variable_name" >&2
      harness_report_stage_finish failed 1 "required agent evaluation command is missing"
      return 1
    fi
    note "skip: $stage is not configured"
    harness_report_stage_finish skipped 0 "optional agent evaluation command is not configured"
    return 0
  fi

  if stage_network_guard "$stage"; then
    :
  else
    runner_status=$?
    harness_report_stage_finish blocked "$runner_status" "network policy blocked the stage"
    return "$runner_status"
  fi

  eval_directory="$(agent_eval_directory)" \
    || { harness_report_stage_finish failed 1 "agent evaluation directory path is invalid"; return 1; }
  summary_result_directory="$(agent_result_directory)" \
    || { harness_report_stage_finish failed 1 "agent result directory path is invalid"; return 1; }
  summary_result_file="$summary_result_directory/$suite.json"
  if [[ -L "$summary_result_file" || -d "$summary_result_file" ]]; then
    harness_report_stage_finish failed 1 "agent result summary path is unsafe"
    return 1
  fi
  result_schema="$(agent_repository_path "$HARNESS_AGENT_RESULT_SCHEMA" HARNESS_AGENT_RESULT_SCHEMA)" \
    || { harness_report_stage_finish failed 1 "agent result schema path is invalid"; return 1; }
  [[ -f "$result_schema" && -s "$result_schema" ]] \
    || { harness_report_stage_finish failed 1 "agent result schema is not a non-empty regular file"; return 1; }

  harness_run_context_init "$HARNESS_REPORT_RUN_TYPE" \
    || { harness_report_stage_finish failed 1 "agent run context could not be created"; return 1; }
  harness_ensure_evidence_subdirectory "$HARNESS_RUN_DIR" agent-results \
    || { harness_report_stage_finish failed 1 "canonical agent result directory could not be created"; return 1; }
  canonical_result_directory="$HARNESS_RUN_DIR/agent-results"
  result_file="$canonical_result_directory/$suite.json"
  if [[ -e "$result_file" || -L "$result_file" ]]; then
    harness_report_stage_finish failed 1 "canonical agent result path already exists"
    return 1
  fi

  if agent_eval_validate_cases "$suite"; then
    :
  else
    validator_status=$?
    if [[ "$validator_status" -eq 3 ]]; then
      harness_report_stage_finish blocked "$validator_status" "agent evaluation contract validator is unavailable"
    else
      harness_report_stage_finish failed "$validator_status" "agent evaluation case contract is invalid"
    fi
    return "$validator_status"
  fi

  if ! mkdir -p "$summary_result_directory"; then
    harness_report_stage_finish failed 1 "agent result directory could not be created"
    return 1
  fi
  chmod 0700 "$summary_result_directory" 2>/dev/null || true
  export HARNESS_AGENT_SUITE="$suite"
  export HARNESS_AGENT_RESULT_FILE="$result_file"

  if (
    export HARNESS_AGENT_EVAL_DIR="$eval_directory"
    run_shell "$configured" "$report_command_id"
  ); then
    runner_status=0
  else
    runner_status=$?
  fi

  if [[ ! -s "$result_file" || ! -f "$result_file" || -L "$result_file" ]]; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    printf 'error: %s did not write HARNESS_AGENT_RESULT_FILE: %s\n' "$stage" "$result_file" >&2
    if [[ "$runner_status" -eq 3 ]]; then
      harness_report_stage_finish failed 1 "agent runner reported blocked but omitted the required structured result"
      return 1
    fi
    [[ "$runner_status" -ne 0 ]] || runner_status=1
    harness_report_stage_finish failed "$runner_status" "agent evaluation result is missing or empty"
    return "$runner_status"
  fi
  # Re-establish containment before invoking the project validator. A runner
  # must not redirect that trusted validator to host-owned bytes by replacing
  # an intermediate result-directory component with a symbolic link.
  if ! harness_validate_run_owned_file "$result_file" "$canonical_result_directory"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    harness_report_stage_finish failed 1 "agent result path changed before validation"
    return 1
  fi
  export HARNESS_AGENT_RUNNER_EXIT_CODE="$runner_status"
  if run_agent_contract result "$result_schema" "$result_file" "$suite"; then
    validator_status=0
  else
    validator_status=$?
  fi
  if [[ "$validator_status" -ne 0 ]]; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    if [[ "$validator_status" -eq 3 ]]; then
      harness_report_stage_finish failed 1 "agent result validator is unavailable; result evidence is not valid"
      return 1
    else
      harness_report_stage_finish failed "$validator_status" "agent result contract is invalid"
      return "$validator_status"
    fi
  fi
  if ! harness_validate_run_owned_file "$result_file" "$canonical_result_directory"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    harness_report_stage_finish failed 1 "validated agent result path changed after runner execution"
    return 1
  fi
  chmod 0444 "$result_file" 2>/dev/null || true
  if ! harness_report_register_artifact "agent-result:$suite" "$result_file"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    harness_report_stage_finish failed 1 "validated agent result could not be registered"
    return 1
  fi
  summary_result_directory="$(agent_result_directory)" \
    || { HARNESS_REPORT_EVIDENCE_INVALID="true"; harness_report_stage_finish failed 1 "agent result directory changed after runner execution"; return 1; }
  summary_result_file="$summary_result_directory/$suite.json"
  if [[ -L "$summary_result_file" || -d "$summary_result_file" ]] \
    || ! harness_atomic_copy "$result_file" "$summary_result_file" "$ROOT_DIR"; then
    harness_report_stage_finish failed 1 "validated agent result summary could not be published"
    return 1
  fi
  if [[ "$runner_status" -eq 0 ]]; then
    harness_report_stage_finish passed 0 "configured command and result contract"
    return 0
  fi
  if [[ "$runner_status" -eq 3 ]]; then
    harness_report_stage_finish blocked 3 "agent runner was blocked; result contract validated"
  else
    harness_report_stage_finish failed "$runner_status" "agent runner failed; result contract validated"
  fi
  return "$runner_status"
}

agent_repository_path() {
  local configured="$1"
  local kind="$2"
  local root parent leaf resolved

  case "$configured" in
    ""|/*|..|../*|*/../*)
      printf 'error: %s must be a repository-relative path without parent traversal: %s\n' "$kind" "$configured" >&2
      return 1
      ;;
  esac
  root="$(cd "$ROOT_DIR" 2>/dev/null && pwd -P)" || return 1
  if [[ -L "$root/$configured" ]]; then
    printf 'error: %s must not be a symbolic link: %s\n' "$kind" "$configured" >&2
    return 1
  fi
  case "$configured" in
    */*) parent="${configured%/*}"; leaf="${configured##*/}" ;;
    *) parent="."; leaf="$configured" ;;
  esac
  if [[ -d "$root/$configured" ]]; then
    resolved="$(cd "$root/$configured" 2>/dev/null && pwd -P)" || return 1
  elif [[ -d "$root/$parent" ]]; then
    resolved="$(cd "$root/$parent" 2>/dev/null && pwd -P)/$leaf" || return 1
  else
    printf 'error: parent directory for %s does not exist: %s\n' "$kind" "$parent" >&2
    return 1
  fi
  case "$resolved" in
    "$root"/*) printf '%s\n' "$resolved" ;;
    *) printf 'error: %s escapes the repository root: %s\n' "$kind" "$configured" >&2; return 1 ;;
  esac
}

agent_eval_directory() {
  agent_repository_path "$HARNESS_AGENT_EVAL_DIR" HARNESS_AGENT_EVAL_DIR
}

agent_result_directory() {
  local configured="$HARNESS_AGENT_RESULT_DIR"
  local root current component evidence_root components=()

  case "$configured" in
    ""|.|./|/*|..|../*|*/../*)
      printf 'error: HARNESS_AGENT_RESULT_DIR must be repository-relative without parent traversal: %s\n' "$configured" >&2
      return 1
      ;;
  esac
  root="$(cd "$ROOT_DIR" 2>/dev/null && pwd -P)" || return 1
  current="$root"
  IFS='/' read -r -a components <<<"$configured"
  for component in "${components[@]}"; do
    if [[ "$component" == ".." ]]; then
      printf 'error: HARNESS_AGENT_RESULT_DIR must not contain parent traversal: %s\n' "$configured" >&2
      return 1
    fi
    [[ -n "$component" && "$component" != "." ]] || continue
    if [[ -L "$current/$component" ]]; then
      printf 'error: HARNESS_AGENT_RESULT_DIR must not traverse a symbolic link: %s\n' "$configured" >&2
      return 1
    fi
    if [[ -e "$current/$component" && ! -d "$current/$component" ]]; then
      printf 'error: HARNESS_AGENT_RESULT_DIR component is not a directory: %s\n' "$current/$component" >&2
      return 1
    fi
    current="$current/$component"
  done
  harness_report_root_into evidence_root || return 1
  case "$current" in
    "$evidence_root"|"$evidence_root/runs"|"$evidence_root/runs"/*|\
      "$evidence_root/latest"|"$evidence_root/latest"/*)
      printf 'error: HARNESS_AGENT_RESULT_DIR must not overlap reserved evidence storage: %s\n' "$configured" >&2
      return 1
      ;;
  esac
  printf '%s\n' "$current"
}

run_agent_contract() (
  local mode="$1"
  local schema="$2"
  local input="$3"
  local suite="$4"
  export HARNESS_AGENT_CONTRACT_MODE="$mode"
  export HARNESS_AGENT_CONTRACT_SCHEMA="$schema"
  export HARNESS_AGENT_CONTRACT_INPUT="$input"
  export HARNESS_AGENT_SUITE="$suite"
  export HARNESS_AGENT_EVAL_DIR="$(agent_eval_directory)"
  export HARNESS_AGENT_TRIALS
  if [[ "$mode" == "cases" ]]; then
    unset HARNESS_AGENT_RUNNER_EXIT_CODE
  fi
  local contract_command_id
  harness_command_id_into contract_command_id agent-contract
  run_shell "$HARNESS_AGENT_CONTRACT_COMMAND" "$contract_command_id"
)

HARNESS_AGENT_CASES_VALIDATED_FOR=""
HARNESS_INTERNAL_CHECK_NETWORK="false"

agent_eval_validate_cases() {
  local suite="$1"
  local eval_directory case_schema result_schema case_count status

  if [[ "$HARNESS_AGENT_CASES_VALIDATED_FOR" == "all" \
    || "$HARNESS_AGENT_CASES_VALIDATED_FOR" == "$suite" ]]; then
    return 0
  fi

  eval_directory="$(agent_eval_directory)" || return 1
  case_schema="$(agent_repository_path "$HARNESS_AGENT_CASE_SCHEMA" HARNESS_AGENT_CASE_SCHEMA)" || return 1
  result_schema="$(agent_repository_path "$HARNESS_AGENT_RESULT_SCHEMA" HARNESS_AGENT_RESULT_SCHEMA)" || return 1
  [[ -d "$eval_directory" ]] || { printf 'error: agent evaluation directory does not exist: %s\n' "$eval_directory" >&2; return 1; }
  [[ -f "$case_schema" && -s "$case_schema" ]] || { printf 'error: agent case schema is not a non-empty regular file: %s\n' "$case_schema" >&2; return 1; }
  [[ -f "$result_schema" && -s "$result_schema" ]] || { printf 'error: agent result schema is not a non-empty regular file: %s\n' "$result_schema" >&2; return 1; }
  [[ -n "$HARNESS_AGENT_CONTRACT_COMMAND" && "$HARNESS_AGENT_CONTRACT_COMMAND" != "skip" ]] \
    || { printf 'error: HARNESS_AGENT_CONTRACT_COMMAND is required for agent evaluation\n' >&2; return 1; }
  case_count="$(find "$eval_directory" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) | awk 'END { print NR+0 }')"
  [[ "$case_count" -gt 0 ]] \
    || { printf 'error: agent evaluation directory has no .yaml, .yml, or .json cases: %s\n' "$eval_directory" >&2; return 1; }

  if run_agent_contract cases "$case_schema" "$eval_directory" "$suite"; then
    HARNESS_AGENT_CASES_VALIDATED_FOR="$suite"
    note "ok: validated $case_count agent evaluation case file(s) for $suite"
    return 0
  else
    status=$?
  fi
  printf 'error: agent evaluation case validation failed for suite %s\n' "$suite" >&2
  return "$status"
}

agent_eval_doctor() {
  section "Agent evaluation doctor"
  local status result_directory validation_suite=regression
  [[ -n "$HARNESS_AGENT_REGRESSION_COMMAND" && "$HARNESS_AGENT_REGRESSION_COMMAND" != "skip" ]] \
    || die "HARNESS_AGENT_REGRESSION_COMMAND is required for agent:eval"
  if [[ -n "$HARNESS_AGENT_CAPABILITY_COMMAND" && "$HARNESS_AGENT_CAPABILITY_COMMAND" != "skip" ]]; then
    validation_suite=all
  fi
  set_agent_network_contract "$validation_suite"
  if stage_network_guard agent-regression; then
    :
  else
    status=$?
    printf 'error: agent evaluation validation is blocked by the agent-regression network policy\n' >&2
    return "$status"
  fi
  if [[ -n "$HARNESS_AGENT_CAPABILITY_COMMAND" && "$HARNESS_AGENT_CAPABILITY_COMMAND" != "skip" ]]; then
    if stage_network_guard agent-capability; then
      :
    else
      status=$?
      printf 'error: agent evaluation validation is blocked by the agent-capability network policy\n' >&2
      return "$status"
    fi
  fi
  result_directory="$(agent_result_directory)" || return 1
  if agent_eval_validate_cases "$validation_suite"; then
    HARNESS_AGENT_CASES_VALIDATED_FOR="$validation_suite"
  else
    status=$?
    return "$status"
  fi
  note "trials: $HARNESS_AGENT_TRIALS"
  note "contract validator: configured"
  note "result directory: $result_directory"
  note "regression runner: configured"
  if [[ -n "$HARNESS_AGENT_CAPABILITY_COMMAND" && "$HARNESS_AGENT_CAPABILITY_COMMAND" != "skip" ]]; then
    note "capability runner: configured"
  else
    note "capability runner: optional and not configured"
  fi
}

agent_regression_stage() {
  configured_agent_stage agent-regression "Agent regression evaluation" HARNESS_AGENT_REGRESSION_COMMAND true regression
}

agent_regression() {
  harness_report_init agent-regression
  agent_regression_stage
}

agent_capability_stage() {
  configured_agent_stage agent-capability "Agent capability evaluation" HARNESS_AGENT_CAPABILITY_COMMAND false capability
}

agent_capability() {
  harness_report_init agent-capability
  agent_capability_stage
}

reported_internal_check() {
  local stage="$1"
  local command_id="$2"
  shift 2
  HARNESS_INTERNAL_CHECK_STATUS="passed"
  HARNESS_INTERNAL_CHECK_DETAIL=""
  harness_report_stage_begin "$stage" "$command_id" internal "${HARNESS_INTERNAL_CHECK_NETWORK:-false}"
  "$@"
  harness_report_stage_finish "$HARNESS_INTERNAL_CHECK_STATUS" 0 "$HARNESS_INTERNAL_CHECK_DETAIL"
}

agent_eval() {
  local doctor_network=false validation_suite=regression
  harness_report_init agent-eval
  if [[ -n "$HARNESS_AGENT_CAPABILITY_COMMAND" && "$HARNESS_AGENT_CAPABILITY_COMMAND" != "skip" ]]; then
    validation_suite=all
  fi
  set_agent_network_contract "$validation_suite"
  doctor_network="$HARNESS_NETWORK_CONTRACT_ALLOWS"
  HARNESS_INTERNAL_CHECK_NETWORK="$doctor_network" \
    reported_internal_check agent-eval-doctor internal:agent-eval-doctor agent_eval_doctor
  agent_regression_stage
  agent_capability_stage
}

verify() {
  harness_report_init verify
  [[ -n "${HARNESS_REQUIRED_STAGES//[[:space:]]/}" ]] \
    || die "verify requires HARNESS_REQUIRED_STAGES; use 'none' only for no required product stage"
  reported_internal_check doctor internal:doctor doctor
  reported_internal_check repo-doctor internal:repo-doctor repo_doctor
  reported_internal_check governance-doctor internal:governance-doctor governance_doctor
  stage_is_verified lint && lint_stage
  stage_is_verified format && format_stage
  stage_is_verified typecheck && typecheck_stage
  stage_is_verified test && test_stage
  stage_is_verified coverage && coverage_stage
  stage_is_verified build && build_stage
  stage_is_verified integration && integration_stage
  stage_is_verified e2e && e2e_stage
  stage_is_verified ci-policy && ci_policy_stage
  stage_is_verified supply-chain && supply_chain_stage
  stage_is_verified security && security_stage
  stage_is_verified deploy && deploy_stage
  if [[ "$HARNESS_SDD_MODE" == "required" ]]; then
    reported_internal_check sdd-check internal:sdd-check sdd_contract_check observed
  fi
  return 0
}

usage() {
  cat <<'EOF'
Usage: ./HARNESS/harness.sh [command]

Commands:
  doctor                 Validate configuration and required paths
  instructions:doctor    Validate the root-to-directory instruction chain
  environment:doctor     Show and validate the execution environment
  repo:doctor            Inspect tracked generated and secret-like files
  governance:doctor      Validate configured governance records
  governance:approved    Validate active-change approval records
  sdd:digest             Calculate active SDD approval digests
  sdd:check              Validate the active strict-SDD contract
  lint                   Run lint checks
  format:check           Check formatting without rewriting files
  typecheck              Run type checks
  test                   Run unit/component tests
  coverage               Run the configured coverage threshold gate
  build                  Build project artifacts
  integration            Run real integration checks
  e2e                    Run end-to-end checks
  security               Run project security checks
  ci:policy              Run the configured CI workflow policy check
  supply-chain           Run the configured SBOM/provenance gate
  deploy:check           Validate deployment artifacts/configuration
  verify                 Run the deterministic handoff gate (default)
  agent:eval:doctor      Validate the agent evaluation dataset and runner
  agent:regression       Run required agent regression evaluations
  agent:capability       Run the optional agent capability benchmark
  agent:eval             Run configured agent regression and capability suites
  selftest               Run isolated harness fixtures
  version                Show the portable baseline version
  help                   Show this help
EOF
}

command_name="${1:-verify}"
case "$command_name" in
  version|help|-h|--help|selftest) ;;
  *) validate_settings ;;
esac
case "$command_name" in
  doctor) doctor ;;
  instructions:doctor)
    [[ "$#" -le 2 ]] || die "instructions:doctor accepts at most one relative directory"
    instructions_doctor "${2:-.}"
    ;;
  environment:doctor) environment_doctor ;;
  repo:doctor) repo_doctor ;;
  governance:doctor) governance_doctor ;;
  governance:approved) governance_approved ;;
  sdd:digest) sdd_digest ;;
  sdd:check) sdd_check ;;
  lint) lint_stage ;;
  format:check) format_stage ;;
  typecheck) typecheck_stage ;;
  test) test_stage ;;
  coverage) coverage_stage ;;
  build) build_stage ;;
  integration) integration_stage ;;
  e2e) e2e_stage ;;
  security) security_stage ;;
  ci:policy) ci_policy_stage ;;
  supply-chain) supply_chain_stage ;;
  deploy:check) deploy_stage ;;
  verify) verify ;;
  agent:eval:doctor) agent_eval_doctor ;;
  agent:regression) agent_regression ;;
  agent:capability) agent_capability ;;
  agent:eval) agent_eval ;;
  selftest) run "$SCRIPT_DIR/tests/selftest.sh" ;;
  version) printf '%s\n' "$PORTABLE_HARNESS_VERSION" ;;
  help|-h|--help) usage ;;
  *)
    printf 'Unknown harness command: %s\n' "$command_name" >&2
    usage >&2
    exit 2
    ;;
esac
