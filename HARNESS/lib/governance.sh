#!/usr/bin/env bash

# Repository hygiene and optional specification/approval governance. This module
# expects harness.sh to provide the configuration variables and common helpers.

repo_issue() {
  local label="$1"
  local content="$2"
  [[ -n "$content" ]] || return 0
  if is_true "$HARNESS_STRICT_REPO_HYGIENE"; then
    printf 'error: tracked %s detected:\n%s\n' "$label" "$content" >&2
    return 1
  fi
  printf 'warning: tracked %s detected:\n%s\n' "$label" "$content"
}

repo_doctor() {
  section "Repository doctor"
  if ! command -v git >/dev/null 2>&1 || ! git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    note "skip: repository is not a Git worktree"
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="repository is not a Git worktree"
    return 0
  fi

  local tracked generated sensitive
  tracked="$(git -C "$ROOT_DIR" ls-files)"
  generated="$(printf '%s\n' "$tracked" | grep -E '(^|/)(node_modules|\.venv|venv|__pycache__|\.pytest_cache|\.mypy_cache|\.ruff_cache|\.next|coverage|test-results)(/|$)|(^|/)\.DS_Store$' || true)"
  sensitive="$(printf '%s\n' "$tracked" | grep -E '(^|/)\.env$|\.(pem|p12|pfx|key)$|(^|/)(id_rsa|id_ed25519|\.npmrc|\.pypirc)$' || true)"

  repo_issue "generated or dependency paths" "$generated"
  repo_issue "secret-like files" "$sensitive"
  note "ok: repository hygiene inspection completed"
}

governance_files() {
  printf '%s\n' \
    "$HARNESS_SPEC_FILE" \
    "$HARNESS_CHANGELOG_FILE" \
    "$HARNESS_PLAN_FILE" \
    "$HARNESS_TEST_PLAN_FILE" \
    "$HARNESS_TRACE_FILE"
}

GOVERNANCE_ACTIVE="false"

# Resolve a configured governance path without following symbolic links. Keep
# this helper local to the module: governance.sh can also be sourced by focused
# tests that do not load the instruction or reporting modules first.
governance_repository_path_into() {
  local outvar="$1"
  local configured="$2"
  local label="$3"
  local root current component
  local components=()

  case "$configured" in
    ""|/*|.|..|./*|../*|*/./*|*/.|*/../*|*/..|*[[:cntrl:]]*)
      die "$label must be a normalized repository-relative path: $configured"
      ;;
  esac
  root="$(cd "$ROOT_DIR" 2>/dev/null && pwd -P)" \
    || die "repository root does not exist: $ROOT_DIR"
  current="$root"
  IFS='/' read -r -a components <<<"$configured"
  for component in "${components[@]}"; do
    [[ -n "$component" ]] || die "$label contains an empty path component: $configured"
    [[ ! -L "$current/$component" ]] \
      || die "$label must not traverse a symbolic link: $configured"
    current="$current/$component"
  done
  case "$current" in
    "$root"/*) ;;
    *) die "$label escapes the repository root: $configured" ;;
  esac
  printf -v "$outvar" '%s' "$current"
}

governance_validate_configured_paths() {
  local file path existing
  local resolved=()
  while IFS= read -r file; do
    governance_repository_path_into path "$file" "governance file path"
    if [[ "${#resolved[@]}" -gt 0 ]]; then
      for existing in "${resolved[@]}"; do
        [[ "$path" != "$existing" ]] \
          || die "each governance contract role must use a distinct file: $file"
      done
    fi
    resolved+=("$path")
    if [[ -e "$path" || -L "$path" ]]; then
      [[ -f "$path" && ! -L "$path" ]] \
        || die "governance control file must be a regular non-symlink file: $file"
    fi
  done < <(governance_files)

  governance_repository_path_into path "$HARNESS_ACTIVE_CHANGE_FILE" \
    "HARNESS_ACTIVE_CHANGE_FILE"
  if [[ -e "$path" || -L "$path" ]]; then
    [[ -f "$path" && ! -L "$path" ]] \
      || die "active change file must be a regular non-symlink file: $HARNESS_ACTIVE_CHANGE_FILE"
  fi
}

governance_file_references_change() {
  local file="$1"
  local change_id="$2"
  # Change IDs contain only the portable grammar accepted by
  # validate_change_id, so the value is safe to use literally in this bounded
  # ERE. Token boundaries reject both CHG-10 inside CHG-101 and prose such as
  # prefixCHG-10suffix.
  grep -Eq "(^|[^A-Za-z0-9_-])${change_id}([^A-Za-z0-9_-]|$)" "$file"
}

governance_file_has_template_markers() {
  grep -Eq 'PORTABLE_PLAN_TEMPLATE|\{\{REPLACE_ME:[^}]+\}\}|`<(confirmed understanding|resolved decisions and remaining blockers|observable requirement|precondition|action|observable result|YYYY-MM-DD|reason|impact|tests|evidence reference|unit/integration/e2e|cases|stage IDs from HARNESS/config.env|commands generated or selected for those stages|path|test|report/artifact reference)>`' "$1"
}

governance_has_explicit_activation() {
  local active_path
  governance_repository_path_into active_path "$HARNESS_ACTIVE_CHANGE_FILE" \
    "HARNESS_ACTIVE_CHANGE_FILE"
  [[ -f "$active_path" && ! -L "$active_path" ]]
}

governance_doctor_base() {
  GOVERNANCE_ACTIVE="false"
  governance_validate_configured_paths

  if [[ "$HARNESS_GOVERNANCE_MODE" == "off" ]]; then
    if is_true "$HARNESS_REQUIRE_PLAN_APPROVAL"; then
      die "HARNESS_REQUIRE_PLAN_APPROVAL=true is incompatible with HARNESS_GOVERNANCE_MODE=off"
    fi
    note "skip: governance checks are disabled"
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="governance checks are disabled"
    return 0
  fi

  local file path present=0 total=0
  while IFS= read -r file; do
    total=$((total + 1))
    governance_repository_path_into path "$file" "governance file path"
    [[ -f "$path" && ! -L "$path" ]] && present=$((present + 1))
  done < <(governance_files)

  if [[ "$HARNESS_GOVERNANCE_MODE" == "auto" ]] \
    && ! is_true "$HARNESS_REQUIRE_PLAN_APPROVAL" \
    && ! governance_has_explicit_activation; then
    note "skip: no explicit governance activation marker or active change ID"
    if [[ "$present" -gt 0 ]]; then
      note "note: $present governance-named file(s) are inactive without an explicit activation marker"
    fi
    HARNESS_INTERNAL_CHECK_STATUS="skipped"
    HARNESS_INTERNAL_CHECK_DETAIL="no explicit governance activation"
    return 0
  fi
  if [[ "$present" -ne "$total" ]]; then
    while IFS= read -r file; do
      governance_repository_path_into path "$file" "governance file path"
      [[ -f "$path" && ! -L "$path" ]] || printf 'missing: %s\n' "$file" >&2
    done < <(governance_files)
    die "governance file set is incomplete"
  fi

  GOVERNANCE_ACTIVE="true"
  local change_id
  change_id="$(active_change_id)"
  while IFS= read -r file; do
    governance_repository_path_into path "$file" "governance file path"
    [[ -s "$path" ]] || die "$file must be non-empty"
    if governance_file_has_template_markers "$path"; then
      die "$file still contains governance template placeholders"
    fi
    if ! governance_file_references_change "$path" "$change_id"; then
      die "$file does not reference active change $change_id"
    fi
  done < <(governance_files)
  note "ok: active change $change_id has exact references in all governance files"
}

active_change_id() {
  local value active_path active_line_count

  governance_repository_path_into active_path "$HARNESS_ACTIVE_CHANGE_FILE" \
    "HARNESS_ACTIVE_CHANGE_FILE"
  [[ -f "$active_path" && ! -L "$active_path" ]] \
    || die "active change is undefined; create $HARNESS_ACTIVE_CHANGE_FILE"
  active_line_count="$(awk '!/^[[:space:]]*(#|$)/ { count++ } END { print count+0 }' "$active_path")"
  [[ "$active_line_count" -eq 1 ]] \
    || die "$HARNESS_ACTIVE_CHANGE_FILE must contain exactly one active change ID"
  value="$(awk '!/^[[:space:]]*(#|$)/ { gsub(/^[[:space:]]+|[[:space:]]+$/, ""); print }' "$active_path")"
  validate_change_id "$value"

  printf '%s\n' "$value"
}

validate_change_id() {
  local value="$1"
  if [[ ! "$value" =~ ^[A-Z][A-Z0-9_-]{0,63}-[0-9]{1,18}$ ]] \
    || ! grep -Eq "^(${HARNESS_CHANGE_ID_PATTERN})$" <<<"$value"; then
    die "invalid active change ID: $value"
  fi
}

extract_approval_field() {
  local section_text="$1"
  local label_pattern="$2"
  local matches count
  matches="$(printf '%s\n' "$section_text" \
    | grep -Ei "^[[:space:]]*-[[:space:]]*(${label_pattern})[[:space:]]*:" || true)"
  count="$(printf '%s\n' "$matches" | awk 'NF { count++ } END { print count+0 }')"
  [[ "$count" -le 1 ]] \
    || die "active change approval field appears more than once: $label_pattern"
  printf '%s\n' "$matches" \
    | sed -E 's/^[^:]+:[[:space:]]*//; s/[[:space:]]+$//'
}

approval_status_is() {
  local action="$1"
  local value="$2"
  case "$action" in
    confirmed) grep -Eqi '^(confirmed|已確認)$' <<<"$value" ;;
    approved) grep -Eqi '^(approved|已核准)$' <<<"$value" ;;
    *) return 1 ;;
  esac
}

approval_value_is_filled() {
  [[ -n "$1" ]] \
    && grep -Eq '[[:alnum:]]' <<<"$1" \
    && ! grep -Eqi '^(TBD|TODO|PENDING|N/A|NA|None|未定|待確認|待核准)$' <<<"$1"
}

governance_valid_iso_date() {
  local value="$1" year month day maximum
  [[ "$value" =~ ^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$ ]] \
    || return 1
  year=$((10#${value:0:4}))
  month=$((10#${value:5:2}))
  day=$((10#${value:8:2}))
  [[ "$year" -ge 1 ]] || return 1
  case "$month" in
    1|3|5|7|8|10|12) maximum=31 ;;
    4|6|9|11) maximum=30 ;;
    2)
      maximum=28
      if (( year % 400 == 0 || (year % 4 == 0 && year % 100 != 0) )); then
        maximum=29
      fi
      ;;
    *) return 1 ;;
  esac
  [[ "$day" -le "$maximum" ]]
}

extract_change_section() {
  local change_id="$1"
  awk -v change_id="$change_id" '
    $0 ~ "^##[[:space:]]+" change_id "([[:space:]]|$)" {
      matches++
      if (matches > 1) exit 42
      active=1
      found=1
    }
    active && found && $0 ~ "^##[[:space:]]+" && $0 !~ "^##[[:space:]]+" change_id "([[:space:]]|$)" {
      exit
    }
    active { print }
    END { if (matches > 1) exit 42 }
  ' "$ROOT_DIR/$HARNESS_PLAN_FILE"
}

governance_approval_base() {
  local change_id section_text
  local requirements_status requirements_by requirements_role requirements_on requirements_evidence
  local plan_status plan_by plan_role plan_on plan_evidence execution_plan execution_plan_path
  change_id="$(active_change_id)"
  section_text="$(extract_change_section "$change_id")" \
    || die "$HARNESS_PLAN_FILE must contain exactly one level-two section for $change_id"
  [[ -n "$section_text" ]] || die "$HARNESS_PLAN_FILE needs a level-two section beginning with $change_id"

  requirements_status="$(extract_approval_field "$section_text" 'Requirements[[:space:]]+Confirmed[[:space:]]+Status')"
  requirements_by="$(extract_approval_field "$section_text" 'Requirements[[:space:]]+Confirmed[[:space:]]+By')"
  requirements_role="$(extract_approval_field "$section_text" 'Requirements[[:space:]]+Confirmer[[:space:]]+Role')"
  requirements_on="$(extract_approval_field "$section_text" 'Requirements[[:space:]]+Confirmed[[:space:]]+On')"
  requirements_evidence="$(extract_approval_field "$section_text" 'Requirements[[:space:]]+Confirmation[[:space:]]+Evidence')"
  plan_status="$(extract_approval_field "$section_text" 'Implementation[[:space:]]+Plan[[:space:]]+Approved[[:space:]]+Status')"
  plan_by="$(extract_approval_field "$section_text" 'Implementation[[:space:]]+Plan[[:space:]]+Approved[[:space:]]+By')"
  plan_role="$(extract_approval_field "$section_text" 'Implementation[[:space:]]+Plan[[:space:]]+Approver[[:space:]]+Role')"
  plan_on="$(extract_approval_field "$section_text" 'Implementation[[:space:]]+Plan[[:space:]]+Approved[[:space:]]+On')"
  plan_evidence="$(extract_approval_field "$section_text" 'Implementation[[:space:]]+Plan[[:space:]]+Approval[[:space:]]+Evidence')"
  execution_plan="$(extract_approval_field "$section_text" 'Execution[[:space:]]+Plan')"

  approval_status_is confirmed "$requirements_status" \
    || die "active change $change_id must set 'Requirements Confirmed Status' to confirmed"
  approval_value_is_filled "$requirements_by" \
    || die "active change $change_id must fill 'Requirements Confirmed By'"
  approval_value_is_filled "$requirements_role" \
    || die "active change $change_id must fill 'Requirements Confirmer Role'"
  governance_valid_iso_date "$requirements_on" \
    || die "active change $change_id must fill 'Requirements Confirmed On' with an ISO date"
  approval_value_is_filled "$requirements_evidence" \
    || die "active change $change_id must fill 'Requirements Confirmation Evidence'"

  approval_status_is approved "$plan_status" \
    || die "active change $change_id must set 'Implementation Plan Approved Status' to approved"
  approval_value_is_filled "$plan_by" \
    || die "active change $change_id must fill 'Implementation Plan Approved By'"
  approval_value_is_filled "$plan_role" \
    || die "active change $change_id must fill 'Implementation Plan Approver Role'"
  governance_valid_iso_date "$plan_on" \
    || die "active change $change_id must fill 'Implementation Plan Approved On' with an ISO date"
  approval_value_is_filled "$plan_evidence" \
    || die "active change $change_id must fill 'Implementation Plan Approval Evidence'"
  [[ "$execution_plan" == "plans/$change_id.md" ]] \
    || die "active change $change_id must link 'Execution Plan' to plans/$change_id.md"
  governance_repository_path_into execution_plan_path "$execution_plan" "execution plan path"
  [[ -f "$execution_plan_path" && ! -L "$execution_plan_path" && -s "$execution_plan_path" ]] \
    || die "active change $change_id execution plan must be a non-empty regular file: $execution_plan"
  governance_file_has_template_markers "$execution_plan_path" \
    && die "active change $change_id execution plan still contains template placeholders"
  governance_file_references_change "$execution_plan_path" "$change_id" \
    || die "active change $change_id execution plan must reference its change ID"

  note "ok: requirements were confirmed by $requirements_by on $requirements_on"
  note "ok: implementation plan was approved by $plan_by on $plan_on"
}

governance_doctor() {
  section "Governance doctor"
  governance_doctor_base
  if [[ "$GOVERNANCE_ACTIVE" == "true" ]] && is_true "$HARNESS_REQUIRE_PLAN_APPROVAL"; then
    governance_approval_base
  fi
  if [[ "$HARNESS_SDD_MODE" == "required" ]]; then
    [[ "$GOVERNANCE_ACTIVE" == "true" ]] || die "required SDD needs active governance"
    sdd_approval_contract_check
  fi
}

governance_approved() {
  section "Governance approval"
  governance_doctor_base
  if [[ "$GOVERNANCE_ACTIVE" != "true" ]]; then
    die "no active governance record is available for approval validation"
  fi
  governance_approval_base
  if [[ "$HARNESS_SDD_MODE" == "required" ]]; then
    sdd_approval_contract_check
  fi
}
