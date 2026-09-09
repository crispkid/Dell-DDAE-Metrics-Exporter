#!/usr/bin/env bash

# Strict specification-driven-development validation. Human-readable Markdown
# remains canonical; stable headings, fields, ID grammars, and tables provide a
# portable machine contract without imposing a parser runtime on adopters.

HARNESS_SDD_CONTRACT_STATUS="disabled"
HARNESS_SDD_CHANGE_ID=""
HARNESS_SDD_SPECIFICATION_VERSION=""
HARNESS_SDD_SPECIFICATION_DIGEST=""
HARNESS_SDD_PLAN_BASIS_DIGEST=""
HARNESS_SDD_TEST_PLAN_DIGEST=""
HARNESS_SDD_TRACEABILITY_DIGEST=""

if [[ "${HARNESS_SDD_MODE:-off}" == "required" ]]; then
  HARNESS_SDD_CONTRACT_STATUS="not-run"
fi

sdd_extract_change_section_into() {
  local outvar="$1" file="$2" change_id="$3" output
  harness_private_temp_into output sdd-section || return 1
  if ! awk -v change_id="$change_id" '
    $0 ~ "^##[[:space:]]+" change_id "([[:space:]]|$)" {
      matches++
      if (matches > 1) exit 42
      active=1
    }
    active && $0 ~ "^##[[:space:]]+" && $0 !~ "^##[[:space:]]+" change_id "([[:space:]]|$)" { exit }
    active { print }
    END { if (matches != 1) exit 42 }
  ' "$file" >"$output"; then
    rm -f "$output"
    return 1
  fi
  [[ -s "$output" ]] || { rm -f "$output"; return 1; }
  printf -v "$outvar" '%s' "$output"
}

sdd_extract_plan_basis_into() {
  local outvar="$1" file="$2" output
  harness_private_temp_into output sdd-plan-basis || return 1
  if ! awk '
    $0 == "<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->" {
      begins++
      if (begins > 1 || active) exit 42
      active=1
    }
    active { print }
    $0 == "<!-- SDD_APPROVED_PLAN_BASIS_END -->" {
      ends++
      active=0
    }
    END { if (begins != 1 || ends != 1 || active) exit 42 }
  ' "$file" >"$output"; then
    rm -f "$output"
    return 1
  fi
  [[ -s "$output" ]] || { rm -f "$output"; return 1; }
  printf -v "$outvar" '%s' "$output"
}

sdd_field_into() {
  local outvar="$1" file="$2" label="$3" matches count value
  matches="$(grep -F -- "- $label:" "$file" || true)"
  count="$(printf '%s\n' "$matches" | awk 'NF { count++ } END { print count+0 }')"
  [[ "$count" -eq 1 ]] || return 1
  value="${matches#*"- $label:"}"
  value="$(printf '%s\n' "$value" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//; s/^`//; s/`$//')"
  [[ -n "$value" ]] || return 1
  printf -v "$outvar" '%s' "$value"
}

sdd_assert_adopted_text() {
  local file="$1" label="$2"
  if grep -Eq '\{\{REPLACE_ME:[^}]+\}\}|(^|[^A-Za-z])(TBD|TODO|FIXME|PENDING)([^A-Za-z]|$)' "$file"; then
    die "$label contains an unresolved SDD marker"
  fi
}

sdd_require_heading() {
  local file="$1" heading="$2" label="$3"
  grep -Fqx -- "$heading" "$file" || die "$label is missing required heading: $heading"
}

sdd_ids_into() {
  local outvar="$1" file="$2" kind="$3" change_id="$4" output duplicate count
  harness_private_temp_into output "sdd-$kind" || return 1
  awk -F'|' -v prefix="$kind-$change_id-" '
    function trim(value) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", value); return value }
    /^\|/ {
      id=trim($2)
      if (id ~ ("^" prefix "[0-9][0-9][0-9][0-9]*$")) print id
    }
  ' "$file" >"$output" || { rm -f "$output"; return 1; }
  count="$(awk 'END { print NR+0 }' "$output")"
  [[ "$count" -gt 0 ]] || { rm -f "$output"; return 1; }
  duplicate="$(sort "$output" | uniq -d | head -n 1 || true)"
  [[ -z "$duplicate" ]] || { rm -f "$output"; die "duplicate $kind ID in active SDD artifact: $duplicate"; }
  printf -v "$outvar" '%s' "$output"
}

sdd_file_references_id() {
  local file="$1" id="$2"
  grep -Eq "(^|[^A-Za-z0-9_-])${id}([^A-Za-z0-9_-]|$)" "$file"
}

sdd_table_cell_into() {
  local outvar="$1" file="$2" id="$3" column="$4" value
  value="$(awk -F'|' -v wanted="$id" -v column="$column" '
    function trim(text) {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", text)
      if (length(text) >= 2 && substr(text, 1, 1) == "`" && substr(text, length(text), 1) == "`") {
        text=substr(text, 2, length(text)-2)
      }
      return text
    }
    /^\|/ { key=trim($2); if (key == wanted) { found++; result=trim($(column+1)) } }
    END { if (found != 1 || result == "") exit 42; print result }
  ' "$file")" || return 1
  printf -v "$outvar" '%s' "$value"
}

sdd_list_has_exact_id() {
  local file="$1" id="$2"
  grep -Fqx -- "$id" "$file"
}

sdd_cell_references_id() {
  local cell="$1" id="$2"
  grep -Eq "(^|[^A-Za-z0-9_-])${id}([^A-Za-z0-9_-]|$)" <<<"$cell"
}

sdd_cell_ids_into() {
  local outvar="$1" cell="$2" kind="$3" output
  harness_private_temp_into output "sdd-cell-$kind" || return 1
  printf '%s\n' "$cell" \
    | grep -Eo "${kind}-[A-Z][A-Z0-9_-]*-[0-9]+" \
    | sort -u >"$output" || true
  [[ -s "$output" ]] || { rm -f "$output"; return 1; }
  printf -v "$outvar" '%s' "$output"
}

sdd_cell_ids_are_known() {
  local cell="$1" kind="$2" known_ids="$3" label="$4" extracted id
  sdd_cell_ids_into extracted "$cell" "$kind" \
    || die "$label must contain at least one $kind ID"
  while IFS= read -r id; do
    sdd_list_has_exact_id "$known_ids" "$id" \
      || die "$label references unknown $kind ID $id"
  done <"$extracted"
}

sdd_table_column_references_id() {
  local file="$1" column="$2" wanted="$3"
  awk -F'|' -v column="$column" -v wanted="$wanted" '
    function trim(text) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", text); return text }
    /^\|/ {
      value=trim($(column+1))
      before="(^|[^A-Za-z0-9_-])"
      after="([^A-Za-z0-9_-]|$)"
      if (value ~ (before wanted after)) found=1
    }
    END { exit(found ? 0 : 42) }
  ' "$file"
}

sdd_table_has_value_in_column() {
  local file="$1" column="$2" wanted="$3"
  awk -F'|' -v column="$column" -v wanted="$wanted" '
    function trim(text) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", text); return text }
    /^\|/ { if (trim($(column+1)) == wanted) found=1 }
    END { exit(found ? 0 : 42) }
  ' "$file"
}

sdd_validate_open_questions() {
  local spec_section="$1" questions
  questions="$(awk '
    $0 == "### Open Questions" { active=1; found++; next }
    active && /^### / { active=0 }
    active && $0 !~ /^[[:space:]]*$/ { print }
    END { if (found != 1) exit 42 }
  ' "$spec_section")" || die "active specification needs exactly one Open Questions section"
  [[ "$questions" == '- None.' ]] \
    || die "active specification must close every open question with exactly '- None.'"
}

sdd_validate_normative_requirements() {
  local spec_section="$1" change_id="$2"
  awk -F'|' -v prefix="REQ-$change_id-" '
    function trim(value) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", value); return value }
    /^\|/ {
      id=trim($2)
      if (id ~ ("^" prefix "[0-9][0-9][0-9][0-9]*$")) {
        statement=trim($4)
        if (statement !~ /(^|[^A-Z])(MUST|SHOULD|MAY)([^A-Z]|$)/) exit 42
      }
    }
  ' "$spec_section" || die "every requirement must contain normative MUST, SHOULD, or MAY language"
}

sdd_validate_changelog() {
  local changelog="$1" change_id="$2" version="$3" date
  date="$(awk -F'|' -v wanted="$change_id" -v expected="$version" '
    function trim(value) { gsub(/^[[:space:]`]+|[[:space:]`]+$/, "", value); return value }
    /^\|/ {
      id=trim($3)
      if (id == wanted) {
        found++
        actual=trim($4)
        if (actual != expected) exit 42
        result=trim($2)
      }
    }
    END { if (found != 1 || result == "") exit 42; print result }
  ' "$changelog")" || die "specification changelog needs exactly one $change_id row for version $version"
  governance_valid_iso_date "$date" \
    || die "specification changelog date for $change_id must be a real ISO date"
}

sdd_trace_requirement_has_valid_status() {
  local trace_section="$1" requirement_id="$2" version="$3" phase="$4"
  awk -F'|' -v wanted="$requirement_id" -v expected_version="$version" -v phase="$phase" '
    function trim(value) {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      if (length(value) >= 2 && substr(value, 1, 1) == "`" && substr(value, length(value), 1) == "`") {
        value=substr(value, 2, length(value)-2)
      }
      return value
    }
    /^\|/ {
      id=trim($2)
      if (id == wanted) {
        found++
        version=trim($3)
        status=trim($(NF-1))
        if (version != expected_version) exit 42
        if ((phase == "handoff" || phase == "observed") && status != "Verified") exit 42
        if (phase == "approval" && status != "Planned" && status != "Implemented" && status != "Verified") exit 42
      }
    }
    END { if (found != 1) exit 42 }
  ' "$trace_section"
}

sdd_validate_product_stage() {
  local stage="$1" label="$2"
  case "$stage" in
    lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain) ;;
    *) die "$label must name a portable product stage: $stage" ;;
  esac
  stage_is_verified "$stage" \
    || die "$label is not scheduled by HARNESS_VERIFY_STAGES: $stage"
}

sdd_normalize_product_stages_into() {
  local outvar="$1" configured="$2" label="$3" normalized stages=() stage seen="," result=""
  normalized="${configured//[[:space:]]/}"
  [[ -n "$normalized" ]] || die "$label must contain at least one product stage"
  case "$normalized" in
    ,*|*,|*,,*) die "$label contains an empty stage entry" ;;
  esac
  IFS=',' read -r -a stages <<<"$normalized"
  for stage in "${stages[@]}"; do
    sdd_validate_product_stage "$stage" "$label"
    case "$seen" in
      *",$stage,"*) die "$label contains duplicate stage $stage" ;;
    esac
    seen="${seen}${stage},"
    result="${result:+$result,}$stage"
  done
  printf -v "$outvar" '%s' "$result"
}

sdd_validate_repository_reference() {
  local reference="$1" label="$2" phase="$3" kind="${4:-path}" configured path
  case "$reference" in
    path:*) configured="${reference#path:}" ;;
    *) die "$label must use path:<repository-relative-path>" ;;
  esac
  governance_repository_path_into path "$configured" "$label"
  if [[ "$phase" == "handoff" || "$phase" == "observed" ]]; then
    if [[ "$kind" == "file" ]]; then
      [[ -f "$path" && ! -L "$path" ]] \
        || die "$label does not resolve to a regular repository file: $configured"
    else
      [[ -e "$path" && ! -L "$path" ]] \
        || die "$label does not resolve to an existing repository path: $configured"
    fi
  fi
}

sdd_validate_stage_evidence() {
  local evidence="$1" stages="$2" requirement="$3" phase="$4"
  local stage expected="" values=()
  IFS=',' read -r -a values <<<"$stages"
  for stage in "${values[@]}"; do
    expected="${expected:+$expected,}stage:$stage"
  done
  [[ "${evidence//[[:space:]]/}" == "$expected" ]] \
    || die "$requirement evidence must be exactly $expected"
  if [[ "$phase" == "observed" ]]; then
    for stage in "${values[@]}"; do
      harness_report_stage_passed_current_run "$stage" \
        || die "$requirement cannot be Verified because stage $stage did not pass in the current verify run"
    done
  fi
}

sdd_prepare_artifacts() {
  local change_id="$1"
  local spec_path changelog_path test_path trace_path plan_record_path execution_plan execution_plan_path
  governance_repository_path_into spec_path "$HARNESS_SPEC_FILE" "SDD specification path"
  governance_repository_path_into changelog_path "$HARNESS_CHANGELOG_FILE" "SDD changelog path"
  governance_repository_path_into test_path "$HARNESS_TEST_PLAN_FILE" "SDD test-plan path"
  governance_repository_path_into trace_path "$HARNESS_TRACE_FILE" "SDD traceability path"
  governance_repository_path_into plan_record_path "$HARNESS_PLAN_FILE" "SDD approval-record path"
  SDD_SPEC_PATH="$spec_path"
  SDD_CHANGELOG_PATH="$changelog_path"
  SDD_TEST_PATH="$test_path"
  SDD_TRACE_PATH="$trace_path"
  SDD_PLAN_RECORD_PATH="$plan_record_path"

  sdd_extract_change_section_into SDD_SPEC_SECTION "$spec_path" "$change_id" \
    || die "$HARNESS_SPEC_FILE must contain exactly one section for $change_id"
  sdd_extract_change_section_into SDD_TEST_SECTION "$test_path" "$change_id" \
    || die "$HARNESS_TEST_PLAN_FILE must contain exactly one section for $change_id"
  sdd_extract_change_section_into SDD_TRACE_SECTION "$trace_path" "$change_id" \
    || die "$HARNESS_TRACE_FILE must contain exactly one section for $change_id"
  sdd_extract_change_section_into SDD_APPROVAL_SECTION "$plan_record_path" "$change_id" \
    || die "$HARNESS_PLAN_FILE must contain exactly one section for $change_id"
  sdd_field_into execution_plan "$SDD_APPROVAL_SECTION" "Execution Plan" \
    || die "$HARNESS_PLAN_FILE needs one Execution Plan field for $change_id"
  [[ "$execution_plan" == "plans/$change_id.md" ]] \
    || die "active SDD execution plan must be plans/$change_id.md"
  governance_repository_path_into execution_plan_path "$execution_plan" "SDD execution-plan path"
  [[ -f "$execution_plan_path" && ! -L "$execution_plan_path" && -s "$execution_plan_path" ]] \
    || die "active SDD execution plan is missing or unsafe: $execution_plan"
  SDD_EXECUTION_PLAN_PATH="$execution_plan_path"
  sdd_extract_plan_basis_into SDD_PLAN_BASIS "$execution_plan_path" \
    || die "execution plan needs exactly one complete approved plan-basis block"
}

sdd_calculate_identities() {
  local version
  sdd_field_into version "$SDD_SPEC_SECTION" "Specification Version" \
    || die "active specification needs exactly one Specification Version"
  [[ "$version" =~ ^[0-9]+[.][0-9]+[.][0-9]+$ ]] \
    || die "active specification version must use semantic version syntax"
  HARNESS_SDD_SPECIFICATION_VERSION="$version"
  harness_sha256_file_into HARNESS_SDD_SPECIFICATION_DIGEST "$SDD_SPEC_SECTION" || die "cannot hash active specification"
  harness_sha256_file_into HARNESS_SDD_PLAN_BASIS_DIGEST "$SDD_PLAN_BASIS" || die "cannot hash approved plan basis"
  harness_sha256_file_into HARNESS_SDD_TEST_PLAN_DIGEST "$SDD_TEST_SECTION" || die "cannot hash active test plan"
  harness_sha256_file_into HARNESS_SDD_TRACEABILITY_DIGEST "$SDD_TRACE_SECTION" || die "cannot hash active traceability"
}

sdd_validate_structure_and_trace() {
  local change_id="$1" phase="$2" status owner clarification
  local requirement_ids acceptance_ids task_ids test_ids
  local requirement acceptance acceptance_requirement acceptance_cell
  local test_id test_acceptance test_requirement test_implementation test_stage task task_requirements
  local trace_acceptance trace_task trace_test trace_design trace_implementation trace_stage trace_stages trace_evidence
  local stage stage_found stage_values=()

  for heading in \
    '### Specification Metadata' '### Problem Statement' '### Goals' '### Non-goals' \
    '### Actors and Scenarios' '### Requirement Index' '### Acceptance Criteria' \
    '### Interfaces, Data, and Failure Behavior' '### Quality Attributes' \
    '### Compatibility and Migration' '### Assumptions' '### Open Questions'; do
    sdd_require_heading "$SDD_SPEC_SECTION" "$heading" "active specification"
  done
  sdd_require_heading "$SDD_PLAN_BASIS" '### Requirement Coverage' "approved plan basis"
  sdd_require_heading "$SDD_PLAN_BASIS" '### Technical Design' "approved plan basis"
  sdd_require_heading "$SDD_PLAN_BASIS" '### Task Breakdown' "approved plan basis"
  sdd_require_heading "$SDD_PLAN_BASIS" '### Verification Basis' "approved plan basis"
  sdd_require_heading "$SDD_TEST_SECTION" '### Failure and Boundary Coverage' "active test plan"

  sdd_assert_adopted_text "$SDD_SPEC_SECTION" "active specification"
  sdd_assert_adopted_text "$SDD_PLAN_BASIS" "approved plan basis"
  sdd_assert_adopted_text "$SDD_TEST_SECTION" "active test plan"
  sdd_assert_adopted_text "$SDD_TRACE_SECTION" "active traceability"
  sdd_validate_open_questions "$SDD_SPEC_SECTION"

  sdd_field_into status "$SDD_SPEC_SECTION" "Status" || die "active specification needs one Status"
  [[ "$status" == "active" ]] || die "active specification Status must be active"
  sdd_field_into owner "$SDD_SPEC_SECTION" "Owner" || die "active specification needs one Owner"
  approval_value_is_filled "$owner" || die "active specification Owner is not filled"
  sdd_field_into clarification "$SDD_SPEC_SECTION" "Clarification Status" \
    || die "active specification needs one Clarification Status"
  [[ "$clarification" == "resolved" ]] \
    || die "active specification Clarification Status must be resolved"
  sdd_validate_changelog "$SDD_CHANGELOG_PATH" "$change_id" "$HARNESS_SDD_SPECIFICATION_VERSION"

  sdd_ids_into requirement_ids "$SDD_SPEC_SECTION" REQ "$change_id" \
    || die "active specification needs at least one unique REQ-$change_id-NNN row"
  sdd_ids_into acceptance_ids "$SDD_SPEC_SECTION" AC "$change_id" \
    || die "active specification needs at least one unique AC-$change_id-NNN row"
  sdd_ids_into task_ids "$SDD_PLAN_BASIS" TASK "$change_id" \
    || die "approved plan basis needs at least one unique TASK-$change_id-NNN row"
  sdd_ids_into test_ids "$SDD_TEST_SECTION" TEST "$change_id" \
    || die "active test plan needs at least one unique TEST-$change_id-NNN row"
  sdd_validate_normative_requirements "$SDD_SPEC_SECTION" "$change_id"

  while IFS= read -r requirement; do
    sdd_table_cell_into acceptance_cell "$SDD_SPEC_SECTION" "$requirement" 4 \
      || die "$requirement needs acceptance IDs in its requirement row"
    sdd_cell_ids_are_known "$acceptance_cell" AC "$acceptance_ids" "$requirement acceptance mapping"
    while IFS= read -r acceptance; do
      if sdd_cell_references_id "$acceptance_cell" "$acceptance"; then
        sdd_table_cell_into acceptance_requirement "$SDD_SPEC_SECTION" "$acceptance" 2 \
          || die "$acceptance needs exactly one requirement mapping"
        [[ "$acceptance_requirement" == "$requirement" ]] \
          || die "$requirement lists acceptance criterion $acceptance owned by $acceptance_requirement"
      fi
    done <"$acceptance_ids"
    sdd_table_column_references_id "$SDD_PLAN_BASIS" 2 "$requirement" \
      || die "$requirement has no task in the approved plan basis"
    sdd_table_has_value_in_column "$SDD_TEST_SECTION" 3 "$requirement" \
      || die "$requirement has no active test-plan row"
    sdd_trace_requirement_has_valid_status "$SDD_TRACE_SECTION" "$requirement" \
      "$HARNESS_SDD_SPECIFICATION_VERSION" "$phase" \
      || die "$requirement needs exactly one traceability row with a valid $phase status"

    sdd_table_cell_into trace_acceptance "$SDD_TRACE_SECTION" "$requirement" 3 \
      || die "$requirement trace row needs acceptance IDs"
    sdd_cell_ids_are_known "$trace_acceptance" AC "$acceptance_ids" "$requirement trace acceptance mapping"
    while IFS= read -r acceptance; do
      sdd_table_cell_into acceptance_requirement "$SDD_SPEC_SECTION" "$acceptance" 2 \
        || die "$acceptance needs exactly one specification mapping"
      if [[ "$acceptance_requirement" == "$requirement" ]]; then
        sdd_cell_references_id "$trace_acceptance" "$acceptance" \
          || die "$requirement trace row is missing $acceptance"
      elif sdd_cell_references_id "$trace_acceptance" "$acceptance"; then
        die "$requirement trace row contains acceptance criterion $acceptance owned by $acceptance_requirement"
      fi
    done <"$acceptance_ids"

    sdd_table_cell_into trace_task "$SDD_TRACE_SECTION" "$requirement" 5 \
      || die "$requirement trace row needs task IDs"
    sdd_cell_ids_are_known "$trace_task" TASK "$task_ids" "$requirement trace task mapping"
    while IFS= read -r task; do
      sdd_table_cell_into task_requirements "$SDD_PLAN_BASIS" "$task" 2 \
        || die "$task needs requirement IDs in its task row"
      if sdd_cell_references_id "$task_requirements" "$requirement"; then
        sdd_cell_references_id "$trace_task" "$task" \
          || die "$requirement trace row is missing $task"
      elif sdd_cell_references_id "$trace_task" "$task"; then
        die "$requirement trace row contains $task that does not map to it"
      fi
    done <"$task_ids"

    sdd_table_cell_into trace_test "$SDD_TRACE_SECTION" "$requirement" 7 \
      || die "$requirement trace row needs test IDs"
    sdd_cell_ids_are_known "$trace_test" TEST "$test_ids" "$requirement trace test mapping"
    while IFS= read -r test_id; do
      sdd_table_cell_into test_requirement "$SDD_TEST_SECTION" "$test_id" 3 \
        || die "$test_id needs exactly one requirement mapping"
      if [[ "$test_requirement" == "$requirement" ]]; then
        sdd_cell_references_id "$trace_test" "$test_id" \
          || die "$requirement trace row is missing $test_id"
        sdd_table_cell_into test_implementation "$SDD_TEST_SECTION" "$test_id" 4 \
          || die "$test_id needs one test implementation reference"
        sdd_validate_repository_reference "$test_implementation" "$test_id test implementation" "$phase" file
        sdd_table_cell_into test_stage "$SDD_TEST_SECTION" "$test_id" 8 \
          || die "$test_id needs one Harness stage"
        sdd_validate_product_stage "$test_stage" "$test_id Harness stage"
      elif sdd_cell_references_id "$trace_test" "$test_id"; then
        die "$requirement trace row contains $test_id owned by $test_requirement"
      fi
    done <"$test_ids"

    sdd_table_cell_into trace_design "$SDD_TRACE_SECTION" "$requirement" 4 \
      || die "$requirement trace row needs one design decision"
    approval_value_is_filled "$trace_design" \
      || die "$requirement trace design decision is not filled"
    sdd_table_cell_into trace_implementation "$SDD_TRACE_SECTION" "$requirement" 6 \
      || die "$requirement trace row needs one implementation reference"
    sdd_validate_repository_reference "$trace_implementation" "$requirement implementation" "$phase"
    sdd_table_cell_into trace_stage "$SDD_TRACE_SECTION" "$requirement" 8 \
      || die "$requirement trace row needs one Harness stage"
    sdd_normalize_product_stages_into trace_stages "$trace_stage" "$requirement Harness stages"
    while IFS= read -r test_id; do
      sdd_table_cell_into test_requirement "$SDD_TEST_SECTION" "$test_id" 3 \
        || die "$test_id needs exactly one requirement mapping"
      if [[ "$test_requirement" == "$requirement" ]]; then
        sdd_table_cell_into test_stage "$SDD_TEST_SECTION" "$test_id" 8 \
          || die "$test_id needs one Harness stage"
        csv_contains "$trace_stages" "$test_stage" \
          || die "$test_id stage $test_stage is missing from $requirement trace stages $trace_stages"
      fi
    done <"$test_ids"
    IFS=',' read -r -a stage_values <<<"$trace_stages"
    for stage in "${stage_values[@]}"; do
      stage_found="false"
      while IFS= read -r test_id; do
        sdd_table_cell_into test_requirement "$SDD_TEST_SECTION" "$test_id" 3 \
          || die "$test_id needs exactly one requirement mapping"
        if [[ "$test_requirement" == "$requirement" ]]; then
          sdd_table_cell_into test_stage "$SDD_TEST_SECTION" "$test_id" 8 \
            || die "$test_id needs one Harness stage"
          [[ "$test_stage" == "$stage" ]] && stage_found="true"
        fi
      done <"$test_ids"
      [[ "$stage_found" == "true" ]] \
        || die "$requirement trace stage $stage has no mapped Test Plan row"
    done
    sdd_table_cell_into trace_evidence "$SDD_TRACE_SECTION" "$requirement" 9 \
      || die "$requirement trace row needs one evidence reference"
    sdd_validate_stage_evidence "$trace_evidence" "$trace_stages" "$requirement" "$phase"
  done <"$requirement_ids"

  while IFS= read -r acceptance; do
    sdd_table_cell_into acceptance_requirement "$SDD_SPEC_SECTION" "$acceptance" 2 \
      || die "$acceptance needs exactly one requirement mapping"
    sdd_list_has_exact_id "$requirement_ids" "$acceptance_requirement" \
      || die "$acceptance references unknown requirement $acceptance_requirement"
    sdd_file_references_id "$SDD_TEST_SECTION" "$acceptance" \
      || die "$acceptance is missing from the active test plan"
    sdd_file_references_id "$SDD_TRACE_SECTION" "$acceptance" \
      || die "$acceptance is missing from active traceability"
  done <"$acceptance_ids"

  while IFS= read -r task; do
    sdd_table_cell_into task_requirements "$SDD_PLAN_BASIS" "$task" 2 \
      || die "$task needs requirement IDs in its task row"
    sdd_cell_ids_are_known "$task_requirements" REQ "$requirement_ids" "$task requirement mapping"
  done <"$task_ids"

  while IFS= read -r test_id; do
    sdd_table_cell_into test_acceptance "$SDD_TEST_SECTION" "$test_id" 2 \
      || die "$test_id needs exactly one acceptance mapping"
    sdd_table_cell_into test_requirement "$SDD_TEST_SECTION" "$test_id" 3 \
      || die "$test_id needs exactly one requirement mapping"
    sdd_list_has_exact_id "$acceptance_ids" "$test_acceptance" \
      || die "$test_id references unknown acceptance criterion $test_acceptance"
    sdd_list_has_exact_id "$requirement_ids" "$test_requirement" \
      || die "$test_id references unknown requirement $test_requirement"
    sdd_table_cell_into acceptance_requirement "$SDD_SPEC_SECTION" "$test_acceptance" 2 \
      || die "$test_acceptance needs one specification mapping"
    [[ "$test_requirement" == "$acceptance_requirement" ]] \
      || die "$test_id maps $test_acceptance to the wrong requirement"
    sdd_table_cell_into test_implementation "$SDD_TEST_SECTION" "$test_id" 4 \
      || die "$test_id needs one test implementation reference"
    sdd_validate_repository_reference "$test_implementation" "$test_id test implementation" "$phase" file
    sdd_table_cell_into test_stage "$SDD_TEST_SECTION" "$test_id" 8 \
      || die "$test_id needs one Harness stage"
    sdd_validate_product_stage "$test_stage" "$test_id Harness stage"
  done <"$test_ids"
}

sdd_validate_approval_identities() {
  local approved_version approved_spec approved_basis approved_test
  sdd_field_into approved_version "$SDD_APPROVAL_SECTION" "Approved Specification Version" \
    || die "$HARNESS_PLAN_FILE needs one Approved Specification Version"
  sdd_field_into approved_spec "$SDD_APPROVAL_SECTION" "Approved Specification Digest" \
    || die "$HARNESS_PLAN_FILE needs one Approved Specification Digest"
  sdd_field_into approved_basis "$SDD_APPROVAL_SECTION" "Approved Plan Basis Digest" \
    || die "$HARNESS_PLAN_FILE needs one Approved Plan Basis Digest"
  sdd_field_into approved_test "$SDD_APPROVAL_SECTION" "Approved Test Plan Digest" \
    || die "$HARNESS_PLAN_FILE needs one Approved Test Plan Digest"
  [[ "$approved_version" == "$HARNESS_SDD_SPECIFICATION_VERSION" ]] \
    || die "approved specification version does not match active specification"
  [[ "$approved_spec" == "$HARNESS_SDD_SPECIFICATION_DIGEST" ]] \
    || die "approved specification digest does not match active specification"
  [[ "$approved_basis" == "$HARNESS_SDD_PLAN_BASIS_DIGEST" ]] \
    || die "approved plan-basis digest does not match active execution plan"
  [[ "$approved_test" == "$HARNESS_SDD_TEST_PLAN_DIGEST" ]] \
    || die "approved test-plan digest does not match active test plan"
}

sdd_contract_check() {
  local mode="${1:-structural}" change_id phase=handoff
  [[ "$HARNESS_SDD_MODE" == "required" ]] || {
    HARNESS_SDD_CONTRACT_STATUS="disabled"
    return 0
  }
  HARNESS_SDD_CONTRACT_STATUS="failed"
  instructions_check_project_contract "$ROOT_DIR"
  change_id="$(active_change_id)"
  HARNESS_SDD_CHANGE_ID="$change_id"
  sdd_prepare_artifacts "$change_id"
  sdd_calculate_identities
  if [[ "$mode" == "observed" ]]; then
    phase=observed
  elif [[ "$mode" != "structural" ]]; then
    die "unknown SDD contract-check mode: $mode"
  fi
  sdd_validate_structure_and_trace "$change_id" "$phase"
  sdd_validate_approval_identities
  if [[ "$mode" == "observed" ]]; then
    HARNESS_SDD_CONTRACT_STATUS="verified"
    note "ok: strict SDD contract and current-run stage evidence verified for $change_id"
  else
    HARNESS_SDD_CONTRACT_STATUS="approved"
    note "ok: strict SDD handoff structure is consistent for $change_id; run verify for observed stage closure"
  fi
}

sdd_approval_contract_check() {
  local change_id
  [[ "$HARNESS_SDD_MODE" == "required" ]] || return 0
  HARNESS_SDD_CONTRACT_STATUS="failed"
  instructions_check_project_contract "$ROOT_DIR"
  change_id="$(active_change_id)"
  HARNESS_SDD_CHANGE_ID="$change_id"
  sdd_prepare_artifacts "$change_id"
  sdd_calculate_identities
  sdd_validate_structure_and_trace "$change_id" approval
  sdd_validate_approval_identities
  HARNESS_SDD_CONTRACT_STATUS="approved"
  note "ok: strict SDD approval basis verified for $change_id"
}

sdd_digest() {
  local change_id
  section "SDD approval digests"
  [[ "$HARNESS_SDD_MODE" == "required" ]] \
    || die "sdd:digest requires HARNESS_SDD_MODE=required"
  instructions_check_project_contract "$ROOT_DIR"
  governance_doctor_base
  [[ "$GOVERNANCE_ACTIVE" == "true" ]] || die "strict SDD needs active governance"
  change_id="$(active_change_id)"
  sdd_prepare_artifacts "$change_id"
  sdd_calculate_identities
  sdd_validate_structure_and_trace "$change_id" approval
  printf '%s\n' \
    "Approved Specification Version: $HARNESS_SDD_SPECIFICATION_VERSION" \
    "Approved Specification Digest: $HARNESS_SDD_SPECIFICATION_DIGEST" \
    "Approved Plan Basis Digest: $HARNESS_SDD_PLAN_BASIS_DIGEST" \
    "Approved Test Plan Digest: $HARNESS_SDD_TEST_PLAN_DIGEST"
}

sdd_check() {
  [[ "$HARNESS_SDD_MODE" == "required" ]] \
    || die "sdd:check requires HARNESS_SDD_MODE=required"
  governance_approved
  sdd_contract_check
}
