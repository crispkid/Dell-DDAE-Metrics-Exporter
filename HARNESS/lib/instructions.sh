#!/usr/bin/env bash

# Instruction-chain validation for the portable harness. This module models the
# repository-local root-to-working-directory lookup used by Codex without
# reading user-global configuration.

HARNESS_INSTRUCTION_SELECTED_COUNT=0
HARNESS_INSTRUCTION_TOTAL_BYTES=0

instructions_require_repository_file() {
  local path="$1"
  local root="$2"
  local parent physical

  [[ ! -L "$path" ]] || die "repository contract files must not be symbolic links: $path"
  [[ -f "$path" ]] || die "repository contract path is not a regular file: $path"
  parent="$(cd "$(dirname "$path")" 2>/dev/null && pwd -P)" \
    || die "repository contract parent is not accessible: $path"
  physical="$parent/$(basename "$path")"
  case "$physical" in
    "$root"/*) ;;
    *) die "repository contract file escapes the repository root: $path" ;;
  esac
}

instructions_validate_settings() {
  if ! harness_decimal_at_most "$HARNESS_INSTRUCTION_MAX_BYTES" 16777216; then
    die "HARNESS_INSTRUCTION_MAX_BYTES must be an integer from 1 to 16777216"
  fi

  [[ -n "$HARNESS_INSTRUCTION_TARGETS" ]] \
    || die "HARNESS_INSTRUCTION_TARGETS must contain at least one relative directory"

  local names=()
  local name
  if [[ -n "$HARNESS_INSTRUCTION_FALLBACK_FILENAMES" ]]; then
    IFS=':' read -r -a names <<<"$HARNESS_INSTRUCTION_FALLBACK_FILENAMES"
    for name in "${names[@]}"; do
      case "$name" in
        ""|.|..|*/*) die "instruction fallback names must be non-empty basenames: $name" ;;
      esac
    done
  fi
}

instructions_doctor_configured_targets() {
  local targets=()
  local target
  IFS=':' read -r -a targets <<<"$HARNESS_INSTRUCTION_TARGETS"
  for target in "${targets[@]}"; do
    [[ -n "$target" ]] || die "HARNESS_INSTRUCTION_TARGETS contains an empty entry"
    instructions_doctor "$target"
  done
}

instructions_select_directory() {
  local directory="$1"
  local root="$2"
  local relative="${directory#"$root"/}"
  local candidates=("AGENTS.override.md" "AGENTS.md")
  local fallbacks=()
  local candidate path selected="" name

  [[ "$directory" == "$root" ]] && relative="."
  if [[ -n "$HARNESS_INSTRUCTION_FALLBACK_FILENAMES" ]]; then
    IFS=':' read -r -a fallbacks <<<"$HARNESS_INSTRUCTION_FALLBACK_FILENAMES"
    for name in "${fallbacks[@]}"; do
      case ":AGENTS.override.md:AGENTS.md:" in
        *":$name:"*) ;;
        *) candidates+=("$name") ;;
      esac
    done
  fi

  for candidate in "${candidates[@]}"; do
    path="$directory/$candidate"
    [[ -e "$path" ]] || continue
    instructions_require_repository_file "$path" "$root"
    if [[ ! -s "$path" ]]; then
      note "empty: $relative/$candidate"
      continue
    fi
    if [[ -z "$selected" ]]; then
      selected="$candidate"
      local bytes
      bytes="$(wc -c <"$path" | tr -d '[:space:]')"
      HARNESS_INSTRUCTION_SELECTED_COUNT=$((HARNESS_INSTRUCTION_SELECTED_COUNT + 1))
      HARNESS_INSTRUCTION_TOTAL_BYTES=$((HARNESS_INSTRUCTION_TOTAL_BYTES + bytes))
      note "selected: $relative/$candidate ($bytes bytes)"
    else
      note "warning: $relative/$candidate is shadowed by $relative/$selected"
    fi
  done
}

instructions_check_project_contract() {
  local root="$1"
  local project_file="$root/PROJECT.md"
  local canonical_config="$root/HARNESS/config.env"
  local sdd_reason_line sdd_reason sdd_owner_line sdd_owner
  local sdd_follow_up_line sdd_follow_up

  if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" ]]; then
    [[ -e "$root/AGENTS.md" ]] || die "AGENTS.md is required by the shared baseline version contract"
    [[ -e "$project_file" ]] || die "PROJECT.md is required by the shared baseline version contract"
    instructions_require_repository_file "$root/AGENTS.md" "$root"
    instructions_require_repository_file "$project_file" "$root"
    if [[ -s "$root/AGENTS.override.md" ]] && ! is_true "$HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE"; then
      die "root AGENTS.override.md shadows the adopted baseline; set HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE=true only after explicit review"
    fi
  fi

  if [[ -e "$project_file" || -L "$project_file" ]]; then
    instructions_require_repository_file "$project_file" "$root"
    [[ -s "$project_file" ]] || die "PROJECT.md must be non-empty"
    [[ "$CONFIG_FILE" == "$canonical_config" ]] \
      || die "an adopted PROJECT.md must use committed HARNESS/config.env; alternate HARNESS_CONFIG_FILE values are not authoritative"
    [[ ! -L "$root/HARNESS" ]] \
      || die "the canonical HARNESS directory must not be a symbolic link"
    instructions_require_repository_file "$canonical_config" "$root"
    [[ -s "$canonical_config" ]] || die "committed HARNESS/config.env must be non-empty"
    if grep -Fq '<!-- PORTABLE_PROJECT_TEMPLATE:' "$project_file" \
      || grep -Fq '{{REPLACE_ME:' "$project_file"; then
      die "PROJECT.md still contains portable template markers or placeholders"
    fi
    if [[ -n "$HARNESS_EXPECTED_BASELINE_VERSION" ]] \
      && ! grep -Fq "Shared baseline version: \`$HARNESS_EXPECTED_BASELINE_VERSION\`" "$project_file"; then
      die "PROJECT.md baseline version does not match $HARNESS_EXPECTED_BASELINE_VERSION"
    fi
    if [[ "$HARNESS_SDD_MODE" == "required" ]]; then
      grep -Fqx -- '- SDD mode: `required`' "$project_file" \
        || die "PROJECT.md must declare '- SDD mode: `required`' to match committed policy"
    else
      grep -Fqx -- '- SDD mode: `off`' "$project_file" \
        || die "PROJECT.md must declare '- SDD mode: `off`' to match committed policy"
      sdd_reason_line="$(grep -E '^- SDD exemption reason:[[:space:]]*[^[:space:]].*$' "$project_file" || true)"
      [[ "$(printf '%s\n' "$sdd_reason_line" | awk 'NF { count++ } END { print count+0 }')" -eq 1 ]] \
        || die "PROJECT.md must contain exactly one non-empty SDD exemption reason"
      sdd_reason="${sdd_reason_line#*- SDD exemption reason:}"
      sdd_reason="${sdd_reason#${sdd_reason%%[![:space:]]*}}"
      [[ -n "$sdd_reason" ]] \
        && ! grep -Eqi '^(none|n/a|not applicable|tbd|todo|pending)[.]?$' <<<"$sdd_reason" \
        || die "PROJECT.md SDD exemption reason must be concrete and accountable"
      sdd_owner_line="$(grep -E '^- SDD exemption owner:[[:space:]]*[^[:space:]].*$' "$project_file" || true)"
      [[ "$(printf '%s\n' "$sdd_owner_line" | awk 'NF { count++ } END { print count+0 }')" -eq 1 ]] \
        || die "PROJECT.md must contain exactly one non-empty SDD exemption owner"
      sdd_owner="${sdd_owner_line#*- SDD exemption owner:}"
      sdd_owner="${sdd_owner#${sdd_owner%%[![:space:]]*}}"
      [[ -n "$sdd_owner" ]] \
        && grep -Eq '[[:alnum:]]' <<<"$sdd_owner" \
        && ! grep -Eqi '^(none|n/a|not applicable|tbd|todo|pending)[.]?$' <<<"$sdd_owner" \
        || die "PROJECT.md SDD exemption owner must name an accountable person or role"
      sdd_follow_up_line="$(grep -E '^- SDD exemption follow-up:[[:space:]]*[^[:space:]].*$' "$project_file" || true)"
      [[ "$(printf '%s\n' "$sdd_follow_up_line" | awk 'NF { count++ } END { print count+0 }')" -eq 1 ]] \
        || die "PROJECT.md must contain exactly one non-empty SDD exemption follow-up"
      sdd_follow_up="${sdd_follow_up_line#*- SDD exemption follow-up:}"
      sdd_follow_up="${sdd_follow_up#${sdd_follow_up%%[![:space:]]*}}"
      [[ -n "$sdd_follow_up" ]] \
        && ! grep -Eqi '^(none|n/a|not applicable|tbd|todo|pending)[.]?$' <<<"$sdd_follow_up" \
        || die "PROJECT.md SDD exemption follow-up must state a concrete action or review policy"
    fi
    note "ok: PROJECT.md is present and adopted"
  else
    note "note: PROJECT.md is not present; this repository has not declared the project contract"
  fi
}

instructions_check_references() {
  local root="$1"
  local references=()
  local reference

  [[ -n "$HARNESS_INSTRUCTION_REFERENCES" ]] || return 0
  IFS=':' read -r -a references <<<"$HARNESS_INSTRUCTION_REFERENCES"
  for reference in "${references[@]}"; do
    case "$reference" in
      ""|/*|..|../*|*/../*) die "instruction references must stay within the repository: $reference" ;;
    esac
    [[ -e "$root/$reference" ]] || die "missing referenced instruction file: $reference"
    instructions_require_repository_file "$root/$reference" "$root"
    note "ok: referenced instruction file $reference"
  done
}

instructions_doctor() {
  local requested_target="${1:-.}"
  local root target relative current component
  local components=()

  case "$requested_target" in
    /*) die "instruction target must be relative to the repository root" ;;
  esac
  root="$(cd "$ROOT_DIR" 2>/dev/null && pwd -P)" \
    || die "repository root does not exist: $ROOT_DIR"
  target="$(cd "$ROOT_DIR/$requested_target" 2>/dev/null && pwd -P)" \
    || die "instruction target is not an accessible directory: $requested_target"
  case "$target/" in
    "$root/"*) ;;
    *) die "instruction target escapes the repository root: $requested_target" ;;
  esac

  section "Instruction doctor"
  note "root: $root"
  if [[ "$target" == "$root" ]]; then
    note "target: ."
  else
    note "target: ${target#"$root"/}"
  fi

  HARNESS_INSTRUCTION_SELECTED_COUNT=0
  HARNESS_INSTRUCTION_TOTAL_BYTES=0
  instructions_select_directory "$root" "$root"

  if [[ "$target" != "$root" ]]; then
    relative="${target#"$root"/}"
    IFS='/' read -r -a components <<<"$relative"
    current="$root"
    for component in "${components[@]}"; do
      current="$current/$component"
      instructions_select_directory "$current" "$root"
    done
  fi

  [[ "$HARNESS_INSTRUCTION_SELECTED_COUNT" -gt 0 ]] \
    || die "no non-empty repository instruction file was selected"
  note "selected files: $HARNESS_INSTRUCTION_SELECTED_COUNT"
  note "combined bytes: $HARNESS_INSTRUCTION_TOTAL_BYTES / $HARNESS_INSTRUCTION_MAX_BYTES"
  if [[ "$HARNESS_INSTRUCTION_TOTAL_BYTES" -gt "$HARNESS_INSTRUCTION_MAX_BYTES" ]]; then
    die "selected instruction chain exceeds HARNESS_INSTRUCTION_MAX_BYTES"
  fi

  instructions_check_project_contract "$root"
  instructions_check_references "$root"
  HARNESS_INTERNAL_CHECK_DETAIL="instruction_files=$HARNESS_INSTRUCTION_SELECTED_COUNT; instruction_bytes=$HARNESS_INSTRUCTION_TOTAL_BYTES"
}
