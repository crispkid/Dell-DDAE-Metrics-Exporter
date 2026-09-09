#!/usr/bin/env bash

# Machine-readable evidence support for the portable harness. This file expects
# ROOT_DIR, PORTABLE_HARNESS_VERSION, HARNESS_REPORT_DIR, HARNESS_REPORT_FILE,
# HARNESS_WRITE_REPORTS, CONFIG_FILE, and the evidence settings to be set by
# harness.sh. Canonical run directories are append-only from the Harness point
# of view; fixed paths are non-authoritative current summaries.

HARNESS_REPORT_ACTIVE="false"
HARNESS_REPORT_FINALIZED="false"
HARNESS_REPORT_CURRENT_STAGE=""
HARNESS_REPORT_CURRENT_COMMAND_ID=""
HARNESS_REPORT_CURRENT_COMMAND_SOURCE=""
HARNESS_REPORT_CURRENT_NETWORK="false"
HARNESS_REPORT_CURRENT_STARTED=0
HARNESS_REPORT_STARTED_AT=""
HARNESS_REPORT_RUN_TYPE=""
HARNESS_REPORT_RECORDS=""
HARNESS_REPORT_ARTIFACT_RECORDS=""
HARNESS_REPORT_OUTPUT=""
HARNESS_REPORT_SUMMARY_OUTPUT=""
HARNESS_REPORT_OS=""
HARNESS_REPORT_BASH_VERSION=""
HARNESS_REPORT_TOOLCHAIN_JSON=""
HARNESS_REPORT_EVIDENCE_INVALID="false"

HARNESS_RUN_CONTEXT_ACTIVE="false"
HARNESS_RUN_CONTEXT_HAS_VALIDATED_EVIDENCE="false"
HARNESS_RUN_ID=""
HARNESS_RUN_DIR=""
HARNESS_RUN_RELATIVE_DIR=""
HARNESS_EVIDENCE_ROOT=""
HARNESS_REPOSITORY_REVISION=""
HARNESS_REPOSITORY_DIRTY="unknown"
HARNESS_REPOSITORY_DIFF_DIGEST=""
HARNESS_CONFIG_DIGEST=""

HARNESS_REDACTION_INITIALIZED="false"
HARNESS_REDACTION_VALUES=()
HARNESS_REDACTION_COUNT=0
HARNESS_PRIVATE_TMPDIR=""

harness_decimal_at_most() {
  local _h_value="$1"
  local _h_maximum="$2"
  local _h_allow_zero="${3:-false}"
  local LC_ALL=C
  if [[ "$_h_allow_zero" == "true" && "$_h_value" == "0" ]]; then
    return 0
  fi
  [[ "$_h_value" =~ ^[1-9][0-9]*$ ]] || return 1
  if [[ "${#_h_value}" -gt "${#_h_maximum}" \
    || ( "${#_h_value}" -eq "${#_h_maximum}" && "$_h_value" > "$_h_maximum" ) ]]; then
    return 1
  fi
}

harness_private_tmpdir_init() {
  [[ -z "$HARNESS_PRIVATE_TMPDIR" ]] || return 0
  HARNESS_PRIVATE_TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/portable-harness-process.XXXXXX")" || return 1
  chmod 0700 "$HARNESS_PRIVATE_TMPDIR" 2>/dev/null || true
}

harness_private_temp_into() {
  local _h_outvar="$1"
  local _h_name="$2"
  local _h_temp
  harness_private_tmpdir_init || return 1
  _h_temp="$(mktemp "$HARNESS_PRIVATE_TMPDIR/${_h_name}.XXXXXX")" || return 1
  chmod 0600 "$_h_temp" 2>/dev/null || true
  printf -v "$_h_outvar" '%s' "$_h_temp"
}

harness_private_tmpdir_cleanup() {
  if [[ -n "$HARNESS_PRIVATE_TMPDIR" && -d "$HARNESS_PRIVATE_TMPDIR" && ! -L "$HARNESS_PRIVATE_TMPDIR" ]]; then
    rm -rf "$HARNESS_PRIVATE_TMPDIR"
  fi
  HARNESS_PRIVATE_TMPDIR=""
}

harness_registered_secrets_overlap() {
  local _h_i _h_j _h_left _h_right _h_length _h_k _h_overlap
  local LC_ALL=C
  for ((_h_i = 0; _h_i < HARNESS_REDACTION_COUNT; _h_i++)); do
    _h_left="${HARNESS_REDACTION_VALUES[_h_i]}"
    for ((_h_j = _h_i + 1; _h_j < HARNESS_REDACTION_COUNT; _h_j++)); do
      _h_right="${HARNESS_REDACTION_VALUES[_h_j]}"
      # Full containment is safe because replacement is longest-first. Reject
      # partial suffix/prefix overlaps, whose replacement order could expose a
      # fragment of the other registered value.
      if [[ "$_h_left" == *"$_h_right"* || "$_h_right" == *"$_h_left"* ]]; then
        continue
      fi
      _h_length="${#_h_left}"
      [[ "${#_h_right}" -lt "$_h_length" ]] && _h_length="${#_h_right}"
      for ((_h_k = 1; _h_k < _h_length; _h_k++)); do
        _h_overlap="${_h_left:${#_h_left}-_h_k:_h_k}"
        if [[ "$_h_overlap" == "${_h_right:0:_h_k}" ]]; then return 0; fi
        _h_overlap="${_h_right:${#_h_right}-_h_k:_h_k}"
        if [[ "$_h_overlap" == "${_h_left:0:_h_k}" ]]; then return 0; fi
      done
    done
  done
  return 1
}

harness_redaction_init() {
  [[ "$HARNESS_REDACTION_INITIALIZED" == "false" ]] || return 0

  local _h_raw_names="${HARNESS_REDACT_ENV_VARS:-}"
  local _h_names=()
  local _h_name _h_value _h_existing _h_duplicate _h_i _h_j _h_tmp
  local LC_ALL=C
  if [[ -z "$_h_raw_names" ]]; then
    HARNESS_REDACTION_INITIALIZED="true"
    return 0
  fi
  case "$_h_raw_names" in
    *$'\n'*|*$'\r'*)
      printf 'error: HARNESS_REDACT_ENV_VARS must be a single comma-separated line\n' >&2
      return 1
      ;;
  esac
  IFS=',' read -r -a _h_names <<<"$_h_raw_names"
  for _h_name in "${_h_names[@]}"; do
    [[ -n "$_h_name" ]] || continue
    if [[ ! "$_h_name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      printf 'error: HARNESS_REDACT_ENV_VARS contains an invalid environment variable name\n' >&2
      return 1
    fi
    _h_value="${!_h_name-}"
    [[ -n "$_h_value" ]] || continue
    if [[ "$_h_value" == "[REDACTED]" ]]; then
      printf 'error: the literal [REDACTED] cannot be registered as a secret value\n' >&2
      return 1
    fi
    if [[ "${#_h_value}" -lt "${HARNESS_REDACT_MIN_BYTES:-8}" ]]; then
      printf 'error: a registered redaction value is shorter than HARNESS_REDACT_MIN_BYTES\n' >&2
      return 1
    fi
    _h_duplicate="false"
    if [[ "$HARNESS_REDACTION_COUNT" -gt 0 ]]; then
      for _h_existing in "${HARNESS_REDACTION_VALUES[@]}"; do
        if [[ "$_h_existing" == "$_h_value" ]]; then
          _h_duplicate="true"
          break
        fi
      done
    fi
    if [[ "$_h_duplicate" != "true" ]]; then
      HARNESS_REDACTION_VALUES+=("$_h_value")
      HARNESS_REDACTION_COUNT=$((HARNESS_REDACTION_COUNT + 1))
    fi
  done

  # Replace longer values first so an overlapping short value cannot expose a
  # suffix of a longer secret. Indexed arrays keep this compatible with Bash 3.2.
  for ((_h_i = 0; _h_i < HARNESS_REDACTION_COUNT; _h_i++)); do
    for ((_h_j = _h_i + 1; _h_j < HARNESS_REDACTION_COUNT; _h_j++)); do
      if [[ "${#HARNESS_REDACTION_VALUES[_h_j]}" -gt "${#HARNESS_REDACTION_VALUES[_h_i]}" ]]; then
        _h_tmp="${HARNESS_REDACTION_VALUES[_h_i]}"
        HARNESS_REDACTION_VALUES[_h_i]="${HARNESS_REDACTION_VALUES[_h_j]}"
        HARNESS_REDACTION_VALUES[_h_j]="$_h_tmp"
      fi
    done
  done
  if harness_registered_secrets_overlap; then
    printf 'error: registered redaction values overlap ambiguously\n' >&2
    return 1
  fi
  HARNESS_REDACTION_INITIALIZED="true"
}

harness_redact_into() {
  local _h_outvar="$1"
  local _h_remaining="$2"
  local _hr_accumulator=""
  local _hr_result=""
  local _h_prefix _h_secret _h_offset
  local LC_ALL=C

  harness_redaction_init || return 1
  if [[ "$HARNESS_REDACTION_COUNT" -gt 0 ]]; then
    for _h_secret in "${HARNESS_REDACTION_VALUES[@]}"; do
      _hr_accumulator=""
      while [[ "$_h_remaining" == *"$_h_secret"* ]]; do
        _h_prefix="${_h_remaining%%"$_h_secret"*}"
        _hr_accumulator="${_hr_accumulator}${_h_prefix}[REDACTED]"
        _h_offset=$((${#_h_prefix} + ${#_h_secret}))
        _h_remaining="${_h_remaining:$_h_offset}"
      done
      _h_remaining="${_hr_accumulator}${_h_remaining}"
    done
  fi
  _hr_result="$_h_remaining"
  printf -v "$_h_outvar" '%s' "$_hr_result"
}

harness_byte_code_into() {
  local _h_outvar="$1"
  local _h_character="$2"
  local _hb_numeric
  printf -v _hb_numeric '%d' "'$_h_character"
  if [[ "$_hb_numeric" -lt 0 ]]; then
    _hb_numeric=$((_hb_numeric + 256))
  fi
  printf -v "$_h_outvar" '%s' "$_hb_numeric"
}

harness_utf8_validate() {
  local _h_field="$1"
  local _h_value="$2"
  local _h_i=0 _h_length _h_code _h_need=0 _h_min=128 _h_max=191
  local LC_ALL=C
  _h_length="${#_h_value}"

  while [[ "$_h_i" -lt "$_h_length" ]]; do
    harness_byte_code_into _h_code "${_h_value:$_h_i:1}"
    if [[ "$_h_need" -gt 0 ]]; then
      if [[ "$_h_code" -lt "$_h_min" || "$_h_code" -gt "$_h_max" ]]; then
        printf 'error: evidence field %s is not valid UTF-8\n' "$_h_field" >&2
        return 1
      fi
      _h_need=$((_h_need - 1))
      _h_min=128
      _h_max=191
      _h_i=$((_h_i + 1))
      continue
    fi

    if [[ "$_h_code" -eq 0 ]]; then
      printf 'error: evidence field %s contains NUL\n' "$_h_field" >&2
      return 1
    elif [[ "$_h_code" -le 127 ]]; then
      :
    elif [[ "$_h_code" -ge 194 && "$_h_code" -le 223 ]]; then
      _h_need=1
    elif [[ "$_h_code" -eq 224 ]]; then
      _h_need=2; _h_min=160; _h_max=191
    elif [[ "$_h_code" -ge 225 && "$_h_code" -le 236 ]]; then
      _h_need=2
    elif [[ "$_h_code" -eq 237 ]]; then
      _h_need=2; _h_min=128; _h_max=159
    elif [[ "$_h_code" -ge 238 && "$_h_code" -le 239 ]]; then
      _h_need=2
    elif [[ "$_h_code" -eq 240 ]]; then
      _h_need=3; _h_min=144; _h_max=191
    elif [[ "$_h_code" -ge 241 && "$_h_code" -le 243 ]]; then
      _h_need=3
    elif [[ "$_h_code" -eq 244 ]]; then
      _h_need=3; _h_min=128; _h_max=143
    else
      printf 'error: evidence field %s is not valid UTF-8\n' "$_h_field" >&2
      return 1
    fi
    _h_i=$((_h_i + 1))
  done

  if [[ "$_h_need" -ne 0 ]]; then
    printf 'error: evidence field %s is truncated UTF-8\n' "$_h_field" >&2
    return 1
  fi
}

harness_utf8_validate_file() {
  local _h_field="$1"
  local _h_file="$2"
  local _h_limit="${3:-${HARNESS_EVIDENCE_MAX_REPORT_BYTES:-10485760}}"
  local _h_size _h_od_file _h_line _h_code _h_need=0 _h_min=128 _h_max=191

  _h_size="$(LC_ALL=C wc -c <"$_h_file" | tr -d '[:space:]')"
  if [[ ! "$_h_size" =~ ^[0-9]+$ || "$_h_size" -gt "$_h_limit" ]]; then
    printf 'error: evidence field %s exceeds its byte limit\n' "$_h_field" >&2
    return 1
  fi
  harness_private_temp_into _h_od_file od || return 1
  if ! LC_ALL=C od -An -v -t u1 "$_h_file" >"$_h_od_file"; then
    rm -f "$_h_od_file"
    return 1
  fi
  while IFS= read -r _h_line || [[ -n "$_h_line" ]]; do
    for _h_code in $_h_line; do
      if [[ "$_h_need" -gt 0 ]]; then
        if [[ "$_h_code" -lt "$_h_min" || "$_h_code" -gt "$_h_max" ]]; then
          rm -f "$_h_od_file"
          printf 'error: evidence field %s is not valid UTF-8\n' "$_h_field" >&2
          return 1
        fi
        _h_need=$((_h_need - 1)); _h_min=128; _h_max=191
        continue
      fi
      if [[ "$_h_code" -eq 0 ]]; then
        rm -f "$_h_od_file"
        printf 'error: evidence field %s contains NUL\n' "$_h_field" >&2
        return 1
      elif [[ "$_h_code" -le 127 ]]; then
        :
      elif [[ "$_h_code" -ge 194 && "$_h_code" -le 223 ]]; then
        _h_need=1
      elif [[ "$_h_code" -eq 224 ]]; then
        _h_need=2; _h_min=160; _h_max=191
      elif [[ "$_h_code" -ge 225 && "$_h_code" -le 236 ]]; then
        _h_need=2
      elif [[ "$_h_code" -eq 237 ]]; then
        _h_need=2; _h_min=128; _h_max=159
      elif [[ "$_h_code" -ge 238 && "$_h_code" -le 239 ]]; then
        _h_need=2
      elif [[ "$_h_code" -eq 240 ]]; then
        _h_need=3; _h_min=144; _h_max=191
      elif [[ "$_h_code" -ge 241 && "$_h_code" -le 243 ]]; then
        _h_need=3
      elif [[ "$_h_code" -eq 244 ]]; then
        _h_need=3; _h_min=128; _h_max=143
      else
        rm -f "$_h_od_file"
        printf 'error: evidence field %s is not valid UTF-8\n' "$_h_field" >&2
        return 1
      fi
    done
  done <"$_h_od_file"
  rm -f "$_h_od_file"
  if [[ "$_h_need" -ne 0 ]]; then
    printf 'error: evidence field %s is truncated UTF-8\n' "$_h_field" >&2
    return 1
  fi
}

harness_json_escape_raw_into() {
  local _h_outvar="$1"
  local _h_value="$2"
  local _hj_buffer=""
  local _h_character _h_code _hj_control_escape _h_index
  local LC_ALL=C

  for ((_h_index = 0; _h_index < ${#_h_value}; _h_index++)); do
    _h_character="${_h_value:$_h_index:1}"
    case "$_h_character" in
      '"') _hj_buffer="${_hj_buffer}\\\"" ;;
      '\') _hj_buffer="${_hj_buffer}\\\\" ;;
      $'\b') _hj_buffer="${_hj_buffer}\\b" ;;
      $'\f') _hj_buffer="${_hj_buffer}\\f" ;;
      $'\n') _hj_buffer="${_hj_buffer}\\n" ;;
      $'\r') _hj_buffer="${_hj_buffer}\\r" ;;
      $'\t') _hj_buffer="${_hj_buffer}\\t" ;;
      [[:cntrl:]])
        harness_byte_code_into _h_code "$_h_character"
        printf -v _hj_control_escape '\\u%04x' "$_h_code"
        _hj_buffer="${_hj_buffer}${_hj_control_escape}"
        ;;
      *) _hj_buffer="${_hj_buffer}${_h_character}" ;;
    esac
  done
  printf -v "$_h_outvar" '%s' "$_hj_buffer"
}

harness_json_string_into() {
  local _h_outvar="$1"
  local _h_field="$2"
  local _h_raw="$3"
  local _hjs_sanitized
  local LC_ALL=C

  harness_redact_into _hjs_sanitized "$_h_raw" || return 1
  harness_utf8_validate "$_h_field" "$_hjs_sanitized" || return 1
  if [[ "${#_hjs_sanitized}" -gt "${HARNESS_EVIDENCE_MAX_STRING_BYTES:-65536}" ]]; then
    printf 'error: evidence field %s exceeds HARNESS_EVIDENCE_MAX_STRING_BYTES\n' "$_h_field" >&2
    return 1
  fi
  harness_json_escape_raw_into "$_h_outvar" "$_hjs_sanitized"
}

harness_json_structural_string_into() {
  local _h_outvar="$1"
  local _h_field="$2"
  local _h_raw="$3"
  local _hjs_structural
  harness_redact_into _hjs_structural "$_h_raw" || return 1
  if [[ "$_hjs_structural" != "$_h_raw" ]]; then
    printf 'error: structural evidence field %s overlaps a registered secret\n' "$_h_field" >&2
    return 1
  fi
  harness_json_string_into "$_h_outvar" "$_h_field" "$_h_raw"
}

harness_assert_structural_value() {
  local _h_field="$1"
  local _h_value="$2"
  local _h_checked
  harness_redact_into _h_checked "$_h_value" || return 1
  if [[ "$_h_checked" != "$_h_value" ]]; then
    printf 'error: structural evidence field %s overlaps a registered secret\n' "$_h_field" >&2
    return 1
  fi
}

# Compatibility helper used by the baseline self-test. Report generation uses
# the into form so failures cannot be hidden by command substitution.
harness_json_escape() {
  local _h_escaped
  harness_json_string_into _h_escaped json-string "$1" || return 1
  printf '%s' "$_h_escaped"
}

harness_capture_utf8_first_line_into() {
  local _h_outvar="$1"
  local _h_field="$2"
  shift 2
  local _h_capture _hc_line=""

  harness_private_temp_into _h_capture capture || return 1
  if ! run_with_timeout "$@" >"$_h_capture" 2>&1; then
    rm -f "$_h_capture"
    printf 'error: evidence metadata command failed for %s\n' "$_h_field" >&2
    return 1
  fi
  if ! harness_utf8_validate_file "$_h_field" "$_h_capture" "${HARNESS_EVIDENCE_MAX_STRING_BYTES:-65536}"; then
    rm -f "$_h_capture"
    return 1
  fi
  IFS= read -r _hc_line <"$_h_capture" || true
  rm -f "$_h_capture"
  printf -v "$_h_outvar" '%s' "$_hc_line"
}

harness_sha256_file_into() {
  local _h_outvar="$1"
  local _h_file="$2"
  local _h_output _hs_digest _h_rest
  harness_private_temp_into _h_output sha || return 1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$_h_file" >"$_h_output" || { rm -f "$_h_output"; return 1; }
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$_h_file" >"$_h_output" || { rm -f "$_h_output"; return 1; }
  else
    rm -f "$_h_output"
    printf 'error: evidence integrity requires sha256sum or shasum\n' >&2
    return 1
  fi
  IFS=' ' read -r _hs_digest _h_rest <"$_h_output" || true
  rm -f "$_h_output"
  if [[ ! "$_hs_digest" =~ ^[0-9a-f]{64}$ ]]; then
    printf 'error: SHA-256 tool returned an invalid digest\n' >&2
    return 1
  fi
  printf -v "$_h_outvar" 'sha256:%s' "$_hs_digest"
}

harness_file_size_into() {
  local _h_outvar="$1"
  local _h_file="$2"
  local _hz_value
  _hz_value="$(LC_ALL=C wc -c <"$_h_file" | tr -d '[:space:]')"
  [[ "$_hz_value" =~ ^[0-9]+$ ]] || return 1
  printf -v "$_h_outvar" '%s' "$_hz_value"
}

harness_report_is_enabled() {
  case "${HARNESS_WRITE_REPORTS:-true}" in
    true|TRUE|yes|YES|1) return 0 ;;
    *) return 1 ;;
  esac
}

harness_report_root_into() {
  local _h_outvar="$1"
  local _h_configured="${HARNESS_REPORT_DIR:-test-results/harness}"
  local _hp_physical_root _h_current _h_component
  local _h_components=()
  local LC_ALL=C

  harness_utf8_validate HARNESS_REPORT_DIR "$_h_configured" || return 1
  case "$_h_configured" in
    ""|.|./|/*|..|../*|*/../*|*[[:cntrl:]]*)
      printf 'error: HARNESS_REPORT_DIR must be a safe repository-relative path\n' >&2
      return 1
      ;;
  esac
  _hp_physical_root="$(cd "$ROOT_DIR" 2>/dev/null && pwd -P)" || return 1
  _h_current="$_hp_physical_root"
  IFS='/' read -r -a _h_components <<<"$_h_configured"
  for _h_component in "${_h_components[@]}"; do
    [[ -n "$_h_component" && "$_h_component" != "." ]] || continue
    if [[ "$_h_component" == ".." || -L "$_h_current/$_h_component" ]]; then
      printf 'error: HARNESS_REPORT_DIR must not traverse parent or symbolic-link components\n' >&2
      return 1
    fi
    if [[ -e "$_h_current/$_h_component" && ! -d "$_h_current/$_h_component" ]]; then
      printf 'error: a HARNESS_REPORT_DIR component is not a directory\n' >&2
      return 1
    fi
    _h_current="$_h_current/$_h_component"
  done
  printf -v "$_h_outvar" '%s' "$_h_current"
}

harness_path_has_no_symlink_components() {
  local _h_path="$1"
  local _h_expected_root="$2"
  local _h_physical_expected _h_relative _h_current _h_component
  local _h_components=()

  _h_physical_expected="$(cd "$_h_expected_root" 2>/dev/null && pwd -P)" || return 1
  case "$_h_path" in
    "$_h_physical_expected") return 0 ;;
    "$_h_physical_expected"/*) _h_relative="${_h_path#"$_h_physical_expected"/}" ;;
    *) return 1 ;;
  esac
  _h_current="$_h_physical_expected"
  IFS='/' read -r -a _h_components <<<"$_h_relative"
  for _h_component in "${_h_components[@]}"; do
    [[ -n "$_h_component" && "$_h_component" != "." ]] || continue
    [[ "$_h_component" != ".." && ! -L "$_h_current/$_h_component" ]] || return 1
    _h_current="$_h_current/$_h_component"
  done
}

harness_validate_run_owned_file() {
  local _h_path="$1"
  local _h_expected_parent="${2:-$HARNESS_RUN_DIR}"
  local _h_parent
  [[ -n "$HARNESS_RUN_DIR" && -d "$HARNESS_RUN_DIR" && ! -L "$HARNESS_RUN_DIR" ]] || return 1
  harness_path_has_no_symlink_components "$_h_expected_parent" "$HARNESS_RUN_DIR" || return 1
  _h_parent="$(dirname "$_h_path")"
  [[ "$_h_parent" == "$_h_expected_parent" ]] || return 1
  harness_path_has_no_symlink_components "$_h_path" "$HARNESS_RUN_DIR" || return 1
  [[ -f "$_h_path" && ! -L "$_h_path" ]]
}

harness_complete_run_marker() {
  local _h_incomplete="$HARNESS_RUN_DIR/status.incomplete"
  local _h_complete="$HARNESS_RUN_DIR/status.complete"
  harness_path_has_no_symlink_components "$HARNESS_RUN_DIR" "$HARNESS_EVIDENCE_ROOT" || return 1
  if [[ ! -f "$_h_incomplete" || -L "$_h_incomplete" \
    || -e "$_h_complete" || -L "$_h_complete" ]]; then
    printf 'error: run completion marker is missing or unsafe\n' >&2
    return 1
  fi
  printf 'complete\n' >"$_h_incomplete" || return 1
  mv "$_h_incomplete" "$_h_complete" || return 1
  if [[ ! -f "$_h_complete" || -L "$_h_complete" ]]; then
    printf 'error: completed run marker is not a regular file\n' >&2
    return 1
  fi
  chmod 0444 "$_h_complete" 2>/dev/null || true
}

harness_validate_command_id() {
  local _h_id="$1"
  local _h_redacted
  if [[ "${#_h_id}" -gt 128 || ! "$_h_id" =~ ^[a-z0-9][a-z0-9._:/@+-]*$ ]]; then
    printf 'error: command ID must be 1-128 portable ASCII characters\n' >&2
    return 1
  fi
  harness_redact_into _h_redacted "$_h_id" || return 1
  if [[ "$_h_redacted" != "$_h_id" ]]; then
    printf 'error: command ID overlaps a registered secret\n' >&2
    return 1
  fi
}

harness_command_id_into() {
  local _h_outvar="$1"
  local _h_suffix="$2"
  local _h_id="${HARNESS_COMMAND_ID_PREFIX:-project.v1}:$_h_suffix"
  harness_validate_command_id "$_h_id" || return 1
  printf -v "$_h_outvar" '%s' "$_h_id"
}

harness_reporting_validate_settings() {
  local _h_root _h_file="${HARNESS_REPORT_FILE:-}"
  if ! harness_decimal_at_most "${HARNESS_REDACT_MIN_BYTES:-8}" 1048576; then
    printf 'error: HARNESS_REDACT_MIN_BYTES must be an integer from 1 to 1048576\n' >&2
    return 1
  fi
  harness_redaction_init || return 1
  if ! harness_decimal_at_most "${HARNESS_EVIDENCE_MAX_STRING_BYTES:-65536}" 16777216 \
    || ! harness_decimal_at_most "${HARNESS_EVIDENCE_MAX_REPORT_BYTES:-10485760}" 1073741824; then
    printf 'error: evidence byte limits are outside their portable ranges\n' >&2
    return 1
  fi
  if [[ "${HARNESS_REDACT_MIN_BYTES:-8}" -gt "${HARNESS_EVIDENCE_MAX_STRING_BYTES:-65536}" ]]; then
    printf 'error: HARNESS_REDACT_MIN_BYTES exceeds HARNESS_EVIDENCE_MAX_STRING_BYTES\n' >&2
    return 1
  fi
  harness_report_root_into _h_root || return 1
  harness_private_tmpdir_init || return 1
  if [[ -n "$_h_file" ]]; then
    if [[ "${#_h_file}" -gt 128 || ! "$_h_file" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*[.]json$ ]]; then
      printf 'error: HARNESS_REPORT_FILE must be a portable JSON basename\n' >&2
      return 1
    fi
  fi
  if [[ "${#HARNESS_COMMAND_ID_PREFIX}" -gt 64 \
    || ! "$HARNESS_COMMAND_ID_PREFIX" =~ ^[a-z0-9][a-z0-9._/@+-]*$ ]]; then
    printf 'error: HARNESS_COMMAND_ID_PREFIX must be 1-64 portable ASCII characters\n' >&2
    return 1
  fi
  harness_validate_command_id "${HARNESS_COMMAND_ID_PREFIX}:test" || return 1
}

# Canonicalize a validated repository-relative path for Git pathspecs. Empty
# and dot components are removed so spelling differences cannot bypass an
# evidence exclusion or ignore check.
harness_normalize_repository_relative_path_into() {
  local _h_outvar="$1"
  local _h_raw="$2"
  local _hn_component _hn_result=""
  local _hn_components=()
  case "$_h_raw" in
    ""|.|./|/*|..|../*|*/../*|*[[:cntrl:]]*) return 1 ;;
  esac
  IFS='/' read -r -a _hn_components <<<"$_h_raw"
  for _hn_component in "${_hn_components[@]}"; do
    [[ -n "$_hn_component" && "$_hn_component" != "." ]] || continue
    [[ "$_hn_component" != ".." ]] || return 1
    if [[ -n "$_hn_result" ]]; then
      _hn_result="$_hn_result/$_hn_component"
    else
      _hn_result="$_hn_component"
    fi
  done
  [[ -n "$_hn_result" ]] || return 1
  printf -v "$_h_outvar" '%s' "$_hn_result"
}

# Add repository-contained generated paths as literal Git pathspec exclusions.
# This keeps Harness output from changing the identity of the source snapshot
# that the output describes. The caller receives a Bash 3.2 indexed array.
harness_repository_exclusion_pathspecs() {
  local _h_outvar="$1"
  local _h_path _h_normalized _h_existing _h_duplicate
  local _h_paths=("${HARNESS_REPORT_DIR:-test-results/harness}" "${HARNESS_AGENT_RESULT_DIR:-test-results/harness/agent-results}")
  local _h_result=()
  for _h_path in "${_h_paths[@]}"; do
    harness_normalize_repository_relative_path_into _h_normalized "$_h_path" || continue
    _h_duplicate="false"
    if [[ "${#_h_result[@]}" -gt 0 ]]; then
      for _h_existing in "${_h_result[@]}"; do
        if [[ "$_h_existing" == ":(exclude,top,literal)$_h_normalized" ]]; then
          _h_duplicate="true"
          break
        fi
      done
    fi
    [[ "$_h_duplicate" == "true" ]] || _h_result+=(":(exclude,top,literal)$_h_normalized")
  done
  # Every generated element is controlled by the validated repository-relative
  # path settings above. Assignment by nameref is unavailable in Bash 3.2.
  eval "$_h_outvar=(\"\${_h_result[@]}\")"
}

# Return success only when every generated evidence root is ignored by Git.
# Snapshot capture excludes these paths independently; this helper lets doctor
# surface the repository-hygiene defect before generated bytes accumulate.
harness_evidence_paths_are_ignored() {
  local _h_path _h_normalized _h_probe _h_existing _h_duplicate _h_expected_root _h_tracked
  local _h_paths=("${HARNESS_REPORT_DIR:-test-results/harness}" "${HARNESS_AGENT_RESULT_DIR:-test-results/harness/agent-results}")
  local _h_checked=()
  command -v git >/dev/null 2>&1 || return 2
  git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 2
  for _h_path in "${_h_paths[@]}"; do
    if ! harness_normalize_repository_relative_path_into _h_normalized "$_h_path"; then
      printf 'error: generated evidence path is not a safe repository-relative directory: %s\n' "$_h_path" >&2
      return 1
    fi
    _h_duplicate="false"
    if [[ "${#_h_checked[@]}" -gt 0 ]]; then
      for _h_existing in "${_h_checked[@]}"; do
        [[ "$_h_existing" == "$_h_normalized" ]] && { _h_duplicate="true"; break; }
      done
    fi
    [[ "$_h_duplicate" == "true" ]] && continue
    _h_checked+=("$_h_normalized")
    # Generated roots must contain no tracked entries. Otherwise excluding the
    # root from source identity could hide a real tracked modification while a
    # force-added ignored file remains in the index.
    harness_private_temp_into _h_tracked evidence-tracked || return 1
    if ! git -C "$ROOT_DIR" ls-files -z -- ":(top,literal)$_h_normalized" >"$_h_tracked"; then
      rm -f "$_h_tracked"
      printf 'error: cannot inspect tracked entries under generated evidence directory: %s\n' "$_h_normalized" >&2
      return 1
    fi
    if [[ -s "$_h_tracked" ]]; then
      rm -f "$_h_tracked"
      printf 'error: generated evidence directory contains tracked repository entries: %s\n' "$_h_normalized" >&2
      return 1
    fi
    rm -f "$_h_tracked"
    # Require the directory itself to be ignored. Matching a handful of known
    # files is insufficient because run IDs and artifact names are dynamic.
    _h_probe="$_h_normalized/"
    if ! git -C "$ROOT_DIR" check-ignore --no-index -q -- "$_h_probe"; then
      printf 'error: generated evidence directory is not ignored by repository rules: %s\n' "$_h_normalized" >&2
      return 1
    fi
    _h_expected_root="$(git -C "$ROOT_DIR" check-ignore --no-index -v -- "$_h_probe" 2>/dev/null \
      | awk -F: 'NR == 1 { print $1 }')"
    case "$_h_expected_root" in
      .gitignore|*/.gitignore|.git/info/exclude) ;;
      *)
        printf 'error: generated evidence ignore must come from repository-local ignore rules: %s\n' "$_h_normalized" >&2
        return 1
        ;;
    esac
  done
}

harness_evidence_ignore_doctor() {
  local _h_status
  if harness_evidence_paths_are_ignored; then
    note "ok: generated evidence paths are ignored by Git"
  else
    _h_status=$?
    if [[ "$_h_status" -eq 2 ]]; then
      note "skip: evidence ignore check requires a Git worktree"
      return 0
    fi
    die "ignore HARNESS_REPORT_DIR and any separate HARNESS_AGENT_RESULT_DIR before running the Harness"
  fi
}

harness_capture_file_manifest_record() {
  local _h_manifest="$1"
  local _h_type="$2"
  local _h_path="$3"
  local _h_source="$4"
  local _h_digest _h_size _h_path_bytes _h_executable="false"
  harness_sha256_file_into _h_digest "$_h_source" || return 1
  harness_file_size_into _h_size "$_h_source" || return 1
  _h_path_bytes="$(LC_ALL=C printf '%s' "$_h_path" | wc -c | tr -d '[:space:]')"
  [[ "$_h_path_bytes" =~ ^[0-9]+$ ]] || return 1
  [[ -x "$_h_source" ]] && _h_executable="true"
  # Decimal lengths make every field prefix-free; hashes stand in for arbitrary
  # bytes, so no path or file content can be mistaken for framing metadata.
  printf 'entry type=%s executable=%s path-bytes=%s content-bytes=%s content-sha256=%s\n' \
    "$_h_type" "$_h_executable" "$_h_path_bytes" "$_h_size" "$_h_digest" >>"$_h_manifest" || return 1
  printf '%s\n' "$_h_path" >>"$_h_manifest"
}

harness_capture_symlink_manifest_record() {
  local _h_manifest="$1"
  local _h_path="$2"
  local _h_link_file _h_status
  harness_private_temp_into _h_link_file symlink || return 1
  # -n preserves trailing newlines in the link payload (command substitution
  # would discard them), so distinct targets cannot collapse to one digest.
  readlink -n "$ROOT_DIR/$_h_path" >"$_h_link_file" \
    || { rm -f "$_h_link_file"; return 1; }
  harness_capture_file_manifest_record "$_h_manifest" symlink "$_h_path" "$_h_link_file"
  _h_status=$?
  rm -f "$_h_link_file"
  return "$_h_status"
}

harness_capture_repository_state() {
  local _h_status _h_snapshot _h_untracked _h_tracked_diff _h_submodule_state _h_path _h_digest _h_revision=""
  local _h_status_digest _h_status_size _h_diff_digest _h_diff_size
  local _h_git_pathspecs=()
  local LC_ALL=C
  HARNESS_REPOSITORY_REVISION=""
  HARNESS_REPOSITORY_DIRTY="unknown"
  HARNESS_REPOSITORY_DIFF_DIGEST=""
  HARNESS_CONFIG_DIGEST=""

  if [[ -f "$CONFIG_FILE" ]]; then
    harness_sha256_file_into HARNESS_CONFIG_DIGEST "$CONFIG_FILE" || return 1
  fi
  if ! command -v git >/dev/null 2>&1 \
    || ! git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    export HARNESS_REPOSITORY_REVISION HARNESS_REPOSITORY_DIRTY
    export HARNESS_REPOSITORY_DIFF_DIGEST HARNESS_CONFIG_DIGEST
    return 0
  fi

  _h_revision="$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || true)"
  if [[ -n "$_h_revision" && ! "$_h_revision" =~ ^[0-9a-f]{40,128}$ ]]; then
    printf 'error: Git returned an invalid repository revision\n' >&2
    return 1
  fi
  HARNESS_REPOSITORY_REVISION="$_h_revision"
  harness_private_temp_into _h_status status || return 1
  harness_private_temp_into _h_snapshot diff || { rm -f "$_h_status"; return 1; }
  harness_private_temp_into _h_untracked untracked || { rm -f "$_h_status" "$_h_snapshot"; return 1; }
  harness_private_temp_into _h_tracked_diff tracked-diff \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked"; return 1; }
  harness_private_temp_into _h_submodule_state submodule-state \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  # A superproject diff represents every dirty submodule with the same
  # `<gitlink>-dirty` sentinel, which is not a byte identity for nested source.
  # Fail closed rather than publishing two distinct nested states under one
  # repository digest. Recursive foreach also catches dirty nested submodules.
  if ! git -C "$ROOT_DIR" submodule foreach --quiet --recursive \
    'if [ -n "$(git status --porcelain=v1 --untracked-files=all)" ]; then printf "%s\n" "$sm_path"; fi' \
    >"$_h_submodule_state"; then
    rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff" "$_h_submodule_state"
    return 1
  fi
  if [[ -s "$_h_submodule_state" ]]; then
    rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff" "$_h_submodule_state"
    printf 'error: dirty submodules cannot be represented by the portable repository identity\n' >&2
    return 1
  fi
  rm -f "$_h_submodule_state"
  harness_repository_exclusion_pathspecs _h_git_pathspecs
  if ! git -C "$ROOT_DIR" status --porcelain=v1 -z --untracked-files=all -- . "${_h_git_pathspecs[@]}" >"$_h_status"; then
    rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"
    return 1
  fi
  if [[ ! -s "$_h_status" ]]; then
    HARNESS_REPOSITORY_DIRTY="false"
    rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"
    export HARNESS_REPOSITORY_REVISION HARNESS_REPOSITORY_DIRTY
    export HARNESS_REPOSITORY_DIFF_DIGEST HARNESS_CONFIG_DIGEST
    return 0
  fi

  HARNESS_REPOSITORY_DIRTY="true"
  printf 'portable-harness-repository-manifest-v1\n' >"$_h_snapshot" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  harness_sha256_file_into _h_status_digest "$_h_status" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  harness_file_size_into _h_status_size "$_h_status" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  printf 'status bytes=%s sha256=%s\n' "$_h_status_size" "$_h_status_digest" >>"$_h_snapshot" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  if [[ -n "$HARNESS_REPOSITORY_REVISION" ]]; then
    git -C "$ROOT_DIR" diff --binary --no-ext-diff HEAD -- . "${_h_git_pathspecs[@]}" >"$_h_tracked_diff" \
      || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  else
    git -C "$ROOT_DIR" diff --binary --no-ext-diff --cached -- . "${_h_git_pathspecs[@]}" >"$_h_tracked_diff" \
      || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
    git -C "$ROOT_DIR" diff --binary --no-ext-diff -- . "${_h_git_pathspecs[@]}" >>"$_h_tracked_diff" \
      || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  fi
  harness_sha256_file_into _h_diff_digest "$_h_tracked_diff" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  harness_file_size_into _h_diff_size "$_h_tracked_diff" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  printf 'tracked-diff bytes=%s sha256=%s\n' "$_h_diff_size" "$_h_diff_digest" >>"$_h_snapshot" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  if ! git -C "$ROOT_DIR" ls-files --others --exclude-standard -z -- . "${_h_git_pathspecs[@]}" >"$_h_untracked"; then
    rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"
    return 1
  fi
  while IFS= read -r -d '' _h_path; do
    if [[ -L "$ROOT_DIR/$_h_path" ]]; then
      harness_capture_symlink_manifest_record "$_h_snapshot" "$_h_path" \
        || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
    elif [[ -f "$ROOT_DIR/$_h_path" ]]; then
      harness_capture_file_manifest_record "$_h_snapshot" file "$_h_path" "$ROOT_DIR/$_h_path" \
        || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
    else
      rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"
      printf 'error: repository changed while evidence identity was captured\n' >&2
      return 1
    fi
  done <"$_h_untracked"
  harness_sha256_file_into _h_digest "$_h_snapshot" \
    || { rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"; return 1; }
  HARNESS_REPOSITORY_DIFF_DIGEST="$_h_digest"
  rm -f "$_h_status" "$_h_snapshot" "$_h_untracked" "$_h_tracked_diff"
  export HARNESS_REPOSITORY_REVISION HARNESS_REPOSITORY_DIRTY
  export HARNESS_REPOSITORY_DIFF_DIGEST HARNESS_CONFIG_DIGEST
}

harness_report_capture_environment() {
  local _h_tool _h_path _h_version _h_name_json _h_path_json _h_version_json _h_first="true"
  local _h_tools=(git make node npm python3 python go rustc cargo)

  harness_capture_utf8_first_line_into HARNESS_REPORT_OS environment.os uname -srm || return 1
  HARNESS_REPORT_BASH_VERSION="$BASH_VERSION"
  harness_utf8_validate environment.bash "$HARNESS_REPORT_BASH_VERSION" || return 1
  harness_private_temp_into HARNESS_REPORT_TOOLCHAIN_JSON toolchain || return 1
  printf '[' >"$HARNESS_REPORT_TOOLCHAIN_JSON"
  for _h_tool in "${_h_tools[@]}"; do
    command -v "$_h_tool" >/dev/null 2>&1 || continue
    if [[ "$_h_tool" == "python" ]] && command -v python3 >/dev/null 2>&1; then
      continue
    fi
    _h_path="$(command -v "$_h_tool")"
    harness_utf8_validate toolchain.path "$_h_path" || return 1
    case "$_h_tool" in
      go) harness_capture_utf8_first_line_into _h_version toolchain.version "$_h_tool" version || return 1 ;;
      *) harness_capture_utf8_first_line_into _h_version toolchain.version "$_h_tool" --version || return 1 ;;
    esac
    harness_json_string_into _h_name_json toolchain.name "$_h_tool" || return 1
    harness_json_string_into _h_path_json toolchain.path "$_h_path" || return 1
    harness_json_string_into _h_version_json toolchain.version "$_h_version" || return 1
    if [[ "$_h_first" == "true" ]]; then
      _h_first="false"
    else
      printf ',' >>"$HARNESS_REPORT_TOOLCHAIN_JSON"
    fi
    printf '{"name":"%s","path":"%s","version":"%s"}' \
      "$_h_name_json" "$_h_path_json" "$_h_version_json" >>"$HARNESS_REPORT_TOOLCHAIN_JSON"
  done
  printf ']' >>"$HARNESS_REPORT_TOOLCHAIN_JSON"
}

harness_ensure_evidence_subdirectory() {
  local _h_parent="$1"
  local _h_name="$2"
  if [[ -L "$_h_parent/$_h_name" \
    || ( -e "$_h_parent/$_h_name" && ! -d "$_h_parent/$_h_name" ) ]]; then
    printf 'error: evidence storage contains an unsafe reserved path\n' >&2
    return 1
  fi
  mkdir -p "$_h_parent/$_h_name" || return 1
  chmod 0700 "$_h_parent/$_h_name" 2>/dev/null || true
}

harness_run_context_init() {
  local _h_run_type="$1"
  local _h_root _h_runs _h_latest _h_timestamp _h_run_dir _h_summary_filename _h_ignore_status
  if [[ "$HARNESS_RUN_CONTEXT_ACTIVE" == "true" ]]; then
    [[ "$HARNESS_REPORT_RUN_TYPE" == "$_h_run_type" ]] \
      || { printf 'error: one Harness process cannot own multiple run contexts\n' >&2; return 1; }
    return 0
  fi
  if [[ ! "$_h_run_type" =~ ^[a-z][a-z-]{0,31}$ ]]; then
    printf 'error: invalid Harness run type\n' >&2
    return 1
  fi
  harness_report_root_into _h_root || return 1
  if harness_evidence_paths_are_ignored; then
    :
  else
    _h_ignore_status=$?
    if [[ "$_h_ignore_status" -ne 2 ]]; then
      printf 'error: generated evidence paths must be ignored before a run starts\n' >&2
      return 1
    fi
  fi
  harness_capture_repository_state || return 1
  mkdir -p "$_h_root" || return 1
  chmod 0700 "$_h_root" 2>/dev/null || true
  harness_report_root_into _h_root || return 1
  harness_ensure_evidence_subdirectory "$_h_root" runs || return 1
  harness_ensure_evidence_subdirectory "$_h_root" latest || return 1
  _h_runs="$_h_root/runs"
  _h_latest="$_h_root/latest"
  if harness_report_is_enabled; then
    _h_summary_filename="${HARNESS_REPORT_FILE:-${_h_run_type}.json}"
    if [[ -L "$_h_root/$_h_summary_filename" || -d "$_h_root/$_h_summary_filename" \
      || -L "$_h_latest/${_h_run_type}.json" || -d "$_h_latest/${_h_run_type}.json" ]]; then
      printf 'error: evidence publication path is an unsafe symbolic link or directory\n' >&2
      return 1
    fi
  fi
  _h_timestamp="$(date -u '+%Y%m%dT%H%M%SZ')" || return 1
  _h_run_dir="$(mktemp -d "$_h_runs/${_h_timestamp}-${_h_run_type}.XXXXXX")" || return 1
  chmod 0700 "$_h_run_dir" 2>/dev/null || true

  HARNESS_REPORT_RUN_TYPE="$_h_run_type"
  HARNESS_EVIDENCE_ROOT="$_h_root"
  HARNESS_RUN_DIR="$_h_run_dir"
  HARNESS_RUN_ID="${_h_run_dir##*/}"
  HARNESS_RUN_RELATIVE_DIR="runs/$HARNESS_RUN_ID"
  HARNESS_REPORT_OUTPUT="$_h_run_dir/report.json"
  printf 'incomplete\n' >"$_h_run_dir/status.incomplete" || return 1
  HARNESS_RUN_CONTEXT_ACTIVE="true"
  export HARNESS_RUN_ID
  trap 'harness_report_on_exit' EXIT
}

harness_report_path() {
  local _h_root _h_filename="${HARNESS_REPORT_FILE:-${HARNESS_REPORT_RUN_TYPE}.json}"
  harness_report_root_into _h_root || return 1
  printf '%s/%s\n' "$_h_root" "$_h_filename"
}

harness_report_init() {
  local _h_run_type="$1"
  HARNESS_REPORT_RUN_TYPE="$_h_run_type"
  HARNESS_REPORT_STARTED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  HARNESS_REPORT_FINALIZED="false"
  HARNESS_REPORT_EVIDENCE_INVALID="false"
  harness_report_is_enabled || return 0

  harness_run_context_init "$_h_run_type" || return 1
  harness_report_capture_environment || return 1
  harness_private_temp_into HARNESS_REPORT_RECORDS stages || return 1
  harness_private_temp_into HARNESS_REPORT_ARTIFACT_RECORDS artifacts || return 1
  HARNESS_REPORT_SUMMARY_OUTPUT="$(harness_report_path)" || return 1
  HARNESS_REPORT_ACTIVE="true"
}

harness_report_stage_begin() {
  local _h_stage="$1"
  local _h_command_id="$2"
  local _h_command_source="$3"
  local _h_network_allowed="${4:-false}"
  local _h_guard
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  case "$_h_network_allowed" in
    true|false) ;;
    *) HARNESS_REPORT_EVIDENCE_INVALID="true"; printf 'error: invalid stage network evidence value\n' >&2; return 1 ;;
  esac
  if ! harness_json_structural_string_into _h_guard stage.name "$_h_stage" \
    || ! harness_json_structural_string_into _h_guard stage.command_id "$_h_command_id" \
    || ! harness_json_structural_string_into _h_guard stage.command_source "$_h_command_source" \
    || ! harness_json_structural_string_into _h_guard stage.network "$_h_network_allowed"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    return 1
  fi
  if ! harness_validate_command_id "$_h_command_id"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    return 1
  fi

  HARNESS_REPORT_CURRENT_STAGE="$_h_stage"
  HARNESS_REPORT_CURRENT_COMMAND_ID="$_h_command_id"
  HARNESS_REPORT_CURRENT_COMMAND_SOURCE="$_h_command_source"
  HARNESS_REPORT_CURRENT_NETWORK="$_h_network_allowed"
  HARNESS_REPORT_CURRENT_STARTED="$(date '+%s')"
}

harness_report_stage_finish() {
  local _h_status="$1"
  local _h_exit_code="$2"
  local _h_detail="${3:-}"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  [[ -n "$HARNESS_REPORT_CURRENT_STAGE" ]] || return 0

  local _h_finished _h_duration _h_stage_raw _h_command_id_raw _h_source_raw _h_network_raw
  local _h_stage _h_command_id _h_source _h_network_guard _h_detail_json _h_status_json
  _h_finished="$(date '+%s')"
  _h_duration=$((_h_finished - HARNESS_REPORT_CURRENT_STARTED))
  _h_stage_raw="$HARNESS_REPORT_CURRENT_STAGE"
  _h_command_id_raw="$HARNESS_REPORT_CURRENT_COMMAND_ID"
  _h_source_raw="$HARNESS_REPORT_CURRENT_COMMAND_SOURCE"
  _h_network_raw="$HARNESS_REPORT_CURRENT_NETWORK"
  HARNESS_REPORT_CURRENT_STAGE=""
  HARNESS_REPORT_CURRENT_COMMAND_ID=""
  HARNESS_REPORT_CURRENT_COMMAND_SOURCE=""
  HARNESS_REPORT_CURRENT_NETWORK="false"
  HARNESS_REPORT_CURRENT_STARTED=0

  if ! harness_json_structural_string_into _h_stage stage.name "$_h_stage_raw" \
    || ! harness_json_structural_string_into _h_command_id stage.command_id "$_h_command_id_raw" \
    || ! harness_json_structural_string_into _h_source stage.command_source "$_h_source_raw" \
    || ! harness_json_structural_string_into _h_status_json stage.status "$_h_status" \
    || ! harness_json_structural_string_into _h_network_guard stage.network "$_h_network_raw" \
    || ! harness_json_string_into _h_detail_json stage.detail "$_h_detail"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    return 1
  fi

  printf '{"stage":"%s","status":"%s","exit_code":%s,"duration_seconds":%s,"network_contract_allows":%s,"network_enforcement":"declaration-only","network_allowed":%s,"command_id":"%s","command_source":"%s","detail":"%s"}\n' \
    "$_h_stage" "$_h_status_json" "$_h_exit_code" "$_h_duration" "$_h_network_raw" "$_h_network_raw" \
    "$_h_command_id" "$_h_source" "$_h_detail_json" >>"$HARNESS_REPORT_RECORDS"
}

# Return success only when the named stage has exactly one passing record in
# the active run. SDD closure uses this after product stages execute; parsing a
# previous report or accepting a configured command would not prove that the
# current run observed the required behavior.
harness_report_stage_passed_current_run() {
  local _h_stage="$1"
  local _h_expected _h_count
  [[ "$HARNESS_REPORT_ACTIVE" == "true" && -n "$HARNESS_REPORT_RECORDS" \
    && -f "$HARNESS_REPORT_RECORDS" && ! -L "$HARNESS_REPORT_RECORDS" ]] || return 1
  case "$_h_stage" in
    lint|format|typecheck|test|coverage|build|integration|e2e|security|deploy|ci-policy|supply-chain) ;;
    *) return 1 ;;
  esac
  _h_expected="{\"stage\":\"$_h_stage\",\"status\":\"passed\",\"exit_code\":0,"
  _h_count="$(awk -v expected="$_h_expected" 'index($0, expected) == 1 { count++ } END { print count+0 }' \
    "$HARNESS_REPORT_RECORDS")" || return 1
  [[ "$_h_count" -eq 1 ]]
}

harness_report_register_artifact() {
  local _h_name="$1"
  local _h_path="$2"
  local _h_digest _h_size _h_relative _h_name_json _h_path_json
  if ! harness_validate_run_owned_file "$_h_path" "$(dirname "$_h_path")"; then
    printf 'error: validated evidence artifact is not a regular non-symlink file\n' >&2
    return 1
  fi
  case "$_h_path" in
    "$HARNESS_EVIDENCE_ROOT"/*) _h_relative="${_h_path#"$HARNESS_EVIDENCE_ROOT"/}" ;;
    *) printf 'error: evidence artifact escapes HARNESS_REPORT_DIR\n' >&2; return 1 ;;
  esac
  harness_sha256_file_into _h_digest "$_h_path" || return 1
  harness_file_size_into _h_size "$_h_path" || return 1
  harness_assert_structural_value artifact.digest "$_h_digest" || return 1
  HARNESS_RUN_CONTEXT_HAS_VALIDATED_EVIDENCE="true"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  if ! harness_json_structural_string_into _h_name_json artifact.name "$_h_name" \
    || ! harness_json_structural_string_into _h_path_json artifact.path "$_h_relative"; then
    HARNESS_REPORT_EVIDENCE_INVALID="true"
    return 1
  fi
  printf '{"name":"%s","path":"%s","digest":"%s","size_bytes":%s}\n' \
    "$_h_name_json" "$_h_path_json" "$_h_digest" "$_h_size" >>"$HARNESS_REPORT_ARTIFACT_RECORDS"
}

harness_report_render_array_file() {
  local _h_file="$1"
  awk 'NR > 1 { printf ",\n" } { printf "    %s", $0 } END { if (NR > 0) printf "\n" }' "$_h_file"
}

harness_report_render_stage_list() {
  local _h_configured="$1"
  local _h_values=()
  local _h_value _h_json _h_first="true"
  local _h_normalized="${_h_configured//[[:space:]]/}"
  printf '['
  if [[ -n "$_h_normalized" && "$_h_normalized" != "none" ]]; then
    IFS=',' read -r -a _h_values <<<"$_h_configured"
    for _h_value in "${_h_values[@]}"; do
      _h_value="${_h_value//[[:space:]]/}"
      [[ -n "$_h_value" ]] || continue
      harness_json_structural_string_into _h_json effective_policy.stage "$_h_value" || return 1
      if [[ "$_h_first" == "true" ]]; then _h_first="false"; else printf ','; fi
      printf '"%s"' "$_h_json"
    done
  fi
  printf ']'
}

harness_report_render_sdd() {
  local _h_mode="${HARNESS_SDD_MODE:-off}"
  local _h_status="${HARNESS_SDD_CONTRACT_STATUS:-disabled}"
  local _h_mode_json _h_status_json _h_value_json _h_field _h_value
  harness_json_structural_string_into _h_mode_json sdd.mode "$_h_mode" || return 1
  harness_json_structural_string_into _h_status_json sdd.contract_status "$_h_status" || return 1
  printf '  "sdd": {"mode":"%s","contract_status":"%s"' "$_h_mode_json" "$_h_status_json"
  if [[ -n "${HARNESS_SDD_CHANGE_ID:-}" ]]; then
    harness_json_structural_string_into _h_value_json sdd.change_id "$HARNESS_SDD_CHANGE_ID" || return 1
    printf ',"change_id":"%s"' "$_h_value_json"
  else
    printf ',"change_id":null'
  fi
  if [[ -n "${HARNESS_SDD_SPECIFICATION_VERSION:-}" ]]; then
    harness_json_structural_string_into _h_value_json sdd.specification_version "$HARNESS_SDD_SPECIFICATION_VERSION" || return 1
    printf ',"specification_version":"%s"' "$_h_value_json"
  else
    printf ',"specification_version":null'
  fi
  for _h_field in specification_digest plan_basis_digest test_plan_digest traceability_digest; do
    case "$_h_field" in
      specification_digest) _h_value="${HARNESS_SDD_SPECIFICATION_DIGEST:-}" ;;
      plan_basis_digest) _h_value="${HARNESS_SDD_PLAN_BASIS_DIGEST:-}" ;;
      test_plan_digest) _h_value="${HARNESS_SDD_TEST_PLAN_DIGEST:-}" ;;
      traceability_digest) _h_value="${HARNESS_SDD_TRACEABILITY_DIGEST:-}" ;;
    esac
    if [[ -n "$_h_value" ]]; then
      harness_assert_structural_value "sdd.$_h_field" "$_h_value" || return 1
      printf ',"%s":"%s"' "$_h_field" "$_h_value"
    else
      printf ',"%s":null' "$_h_field"
    fi
  done
  printf '},\n'
}

harness_report_render() {
  local _h_exit_code="$1"
  local _h_finished="$2"
  local _h_result="$3"
  local _h_run_id _h_version _h_run_type _h_started _h_finished_json _h_revision
  local _h_os _h_bash _h_result_json _h_storage _h_config

  harness_json_structural_string_into _h_run_id run.id "$HARNESS_RUN_ID" || return 1
  harness_json_structural_string_into _h_version harness.version "$PORTABLE_HARNESS_VERSION" || return 1
  harness_json_structural_string_into _h_run_type run.type "$HARNESS_REPORT_RUN_TYPE" || return 1
  harness_json_structural_string_into _h_started run.started_at "$HARNESS_REPORT_STARTED_AT" || return 1
  harness_json_structural_string_into _h_finished_json run.finished_at "$_h_finished" || return 1
  harness_json_string_into _h_os environment.os "$HARNESS_REPORT_OS" || return 1
  harness_json_string_into _h_bash environment.bash "$HARNESS_REPORT_BASH_VERSION" || return 1
  harness_json_structural_string_into _h_result_json run.result "$_h_result" || return 1
  harness_json_structural_string_into _h_storage storage.path "$HARNESS_RUN_RELATIVE_DIR/report.json" || return 1

  printf '{\n'
  printf '  "schema_version": "1",\n'
  printf '  "run_id": "%s",\n' "$_h_run_id"
  printf '  "harness_version": "%s",\n' "$_h_version"
  printf '  "run_type": "%s",\n' "$_h_run_type"
  printf '  "started_at": "%s",\n' "$_h_started"
  printf '  "finished_at": "%s",\n' "$_h_finished_json"
  printf '  "repository": {'
  if [[ -n "$HARNESS_REPOSITORY_REVISION" ]]; then
    harness_json_structural_string_into _h_revision repository.revision "$HARNESS_REPOSITORY_REVISION" || return 1
    printf '"revision":"%s",' "$_h_revision"
  else
    printf '"revision":null,'
  fi
  case "$HARNESS_REPOSITORY_DIRTY" in
    true|false) printf '"dirty":%s,' "$HARNESS_REPOSITORY_DIRTY" ;;
    *) printf '"dirty":null,' ;;
  esac
  if [[ -n "$HARNESS_REPOSITORY_DIFF_DIGEST" ]]; then
    harness_assert_structural_value repository.diff_digest "$HARNESS_REPOSITORY_DIFF_DIGEST" || return 1
    printf '"diff_digest":"%s"},\n' "$HARNESS_REPOSITORY_DIFF_DIGEST"
  else
    printf '"diff_digest":null},\n'
  fi
  if [[ -n "$HARNESS_CONFIG_DIGEST" ]]; then
    harness_json_structural_string_into _h_config configuration.digest "$HARNESS_CONFIG_DIGEST" || return 1
    printf '  "configuration_digest": "%s",\n' "$_h_config"
  else
    printf '  "configuration_digest": null,\n'
  fi
  printf '  "effective_policy": {"required_stages":'
  harness_report_render_stage_list "${HARNESS_REQUIRED_STAGES:-}" || return 1
  printf ',"verify_stages":'
  harness_report_render_stage_list "${HARNESS_VERIFY_STAGES:-${HARNESS_REQUIRED_STAGES:-}}" || return 1
  harness_json_structural_string_into _h_config effective_policy.network_policy "${HARNESS_NETWORK_POLICY:-declared}" || return 1
  printf ',"network_policy":"%s","network_stages":' "$_h_config"
  harness_report_render_stage_list "${HARNESS_NETWORK_STAGES:-}" || return 1
  printf ',"command_timeout_seconds":%s,"coverage_threshold":%s,' \
    "${HARNESS_COMMAND_TIMEOUT_SECONDS:-0}" "${HARNESS_COVERAGE_THRESHOLD:-80}"
  harness_json_structural_string_into _h_config effective_policy.sdd_mode "${HARNESS_SDD_MODE:-off}" || return 1
  printf '"sdd_mode":"%s",' "$_h_config"
  harness_json_structural_string_into _h_config effective_policy.governance_mode "${HARNESS_GOVERNANCE_MODE:-auto}" || return 1
  printf '"governance_mode":"%s"' "$_h_config"
  if is_true "${HARNESS_REQUIRE_PLAN_APPROVAL:-false}"; then
    printf ',"require_plan_approval":true'
  else
    printf ',"require_plan_approval":false'
  fi
  if is_true "${HARNESS_STRICT_REPO_HYGIENE:-false}"; then
    printf ',"strict_repo_hygiene":true'
  else
    printf ',"strict_repo_hygiene":false'
  fi
  harness_json_structural_string_into _h_config effective_policy.instruction_targets "${HARNESS_INSTRUCTION_TARGETS:-.}" || return 1
  printf ',"instruction_targets":"%s","instruction_max_bytes":%s' \
    "$_h_config" "${HARNESS_INSTRUCTION_MAX_BYTES:-32768}"
  if is_true "${HARNESS_ALLOW_ROOT_INSTRUCTION_OVERRIDE:-false}"; then
    printf ',"allow_root_instruction_override":true},\n'
  else
    printf ',"allow_root_instruction_override":false},\n'
  fi
  harness_report_render_sdd || return 1
  printf '  "environment": {"os":"%s","bash":"%s"},\n' "$_h_os" "$_h_bash"
  printf '  "toolchain": '
  cat "$HARNESS_REPORT_TOOLCHAIN_JSON" || return 1
  printf ',\n'
  printf '  "redaction": {"policy":"registered-env-exact-v1","scope":"harness-metadata"},\n'
  printf '  "storage": {"layout_version":"1","canonical_path":"%s"},\n' "$_h_storage"
  printf '  "result": "%s",\n' "$_h_result_json"
  printf '  "exit_code": %s,\n' "$_h_exit_code"
  printf '  "artifacts": [\n'
  harness_report_render_array_file "$HARNESS_REPORT_ARTIFACT_RECORDS" || return 1
  printf '  ],\n'
  printf '  "stages": [\n'
  harness_report_render_array_file "$HARNESS_REPORT_RECORDS" || return 1
  printf '  ]\n'
  printf '}\n'
}

harness_atomic_copy() {
  local _h_source="$1"
  local _h_destination="$2"
  local _h_trusted_root="${3:-$HARNESS_EVIDENCE_ROOT}"
  local _h_source_trusted_root="${4:-$HARNESS_RUN_DIR}"
  local _h_directory _h_basename _h_temporary
  _h_directory="$(dirname "$_h_destination")"
  _h_basename="$(basename "$_h_destination")"
  if [[ ! -d "$_h_directory" || -L "$_h_directory" \
    || ! -f "$_h_source" || -L "$_h_source" \
    || -L "$_h_destination" || -d "$_h_destination" ]] \
    || ! harness_path_has_no_symlink_components "$_h_directory" "$_h_trusted_root"; then
    printf 'error: refusing to replace an unsafe evidence summary path\n' >&2
    return 1
  fi
  if [[ -z "$_h_source_trusted_root" ]] \
    || ! harness_path_has_no_symlink_components "$_h_source" "$_h_source_trusted_root"; then
    printf 'error: refusing to publish an unsafe canonical evidence source\n' >&2
    return 1
  fi
  _h_temporary="$(mktemp "$_h_directory/.${_h_basename}.XXXXXX")" || return 1
  chmod 0600 "$_h_temporary" 2>/dev/null || true
  if ! cp "$_h_source" "$_h_temporary"; then
    rm -f "$_h_temporary"
    return 1
  fi
  chmod 0444 "$_h_temporary" 2>/dev/null || true
  if ! mv -f "$_h_temporary" "$_h_destination"; then
    rm -f "$_h_temporary"
    return 1
  fi
}

harness_publish_latest_pointer() {
  local _h_report_digest="$1"
  local _h_report_size="$2"
  local _h_result="$3"
  local _h_finished="$4"
  local _h_latest_dir="$HARNESS_EVIDENCE_ROOT/latest"
  local _h_destination="$_h_latest_dir/$HARNESS_REPORT_RUN_TYPE.json"
  local _h_temporary _h_published _h_run_id _h_run_type _h_path _h_result_json _h_finished_json _h_published_json
  if [[ ! -d "$_h_latest_dir" || -L "$_h_latest_dir" \
    || -L "$_h_destination" || -d "$_h_destination" ]] \
    || ! harness_path_has_no_symlink_components "$_h_latest_dir" "$HARNESS_EVIDENCE_ROOT"; then
    printf 'error: refusing to replace an unsafe latest pointer path\n' >&2
    return 1
  fi
  _h_temporary="$(mktemp "$_h_latest_dir/.${HARNESS_REPORT_RUN_TYPE}.XXXXXX")" || return 1
  chmod 0600 "$_h_temporary" 2>/dev/null || true
  _h_published="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  harness_json_structural_string_into _h_run_id latest.run_id "$HARNESS_RUN_ID" || { rm -f "$_h_temporary"; return 1; }
  harness_json_structural_string_into _h_run_type latest.run_type "$HARNESS_REPORT_RUN_TYPE" || { rm -f "$_h_temporary"; return 1; }
  harness_json_structural_string_into _h_path latest.report_path "$HARNESS_RUN_RELATIVE_DIR/report.json" || { rm -f "$_h_temporary"; return 1; }
  harness_json_structural_string_into _h_result_json latest.result "$_h_result" || { rm -f "$_h_temporary"; return 1; }
  harness_json_structural_string_into _h_finished_json latest.finished_at "$_h_finished" || { rm -f "$_h_temporary"; return 1; }
  harness_json_structural_string_into _h_published_json latest.published_at "$_h_published" || { rm -f "$_h_temporary"; return 1; }
  {
    printf '{\n'
    printf '  "schema_version": "1",\n'
    printf '  "run_id": "%s",\n' "$_h_run_id"
    printf '  "run_type": "%s",\n' "$_h_run_type"
    printf '  "report_path": "%s",\n' "$_h_path"
    printf '  "report_digest": "%s",\n' "$_h_report_digest"
    printf '  "report_size_bytes": %s,\n' "$_h_report_size"
    printf '  "result": "%s",\n' "$_h_result_json"
    printf '  "finished_at": "%s",\n' "$_h_finished_json"
    printf '  "published_at": "%s"\n' "$_h_published_json"
    printf '}\n'
  } >"$_h_temporary" || { rm -f "$_h_temporary"; return 1; }
  harness_utf8_validate_file latest.pointer "$_h_temporary" || { rm -f "$_h_temporary"; return 1; }
  chmod 0444 "$_h_temporary" 2>/dev/null || true
  if ! mv -f "$_h_temporary" "$_h_destination"; then
    rm -f "$_h_temporary"
    return 1
  fi
}

harness_report_cleanup_temporaries() {
  [[ -n "$HARNESS_REPORT_RECORDS" ]] && rm -f "$HARNESS_REPORT_RECORDS"
  [[ -n "$HARNESS_REPORT_ARTIFACT_RECORDS" ]] && rm -f "$HARNESS_REPORT_ARTIFACT_RECORDS"
  [[ -n "$HARNESS_REPORT_TOOLCHAIN_JSON" ]] && rm -f "$HARNESS_REPORT_TOOLCHAIN_JSON"
  HARNESS_REPORT_RECORDS=""
  HARNESS_REPORT_ARTIFACT_RECORDS=""
  HARNESS_REPORT_TOOLCHAIN_JSON=""
}

harness_report_finalize() {
  local _h_exit_code="$1"
  [[ "$HARNESS_REPORT_ACTIVE" == "true" ]] || return 0
  [[ "$HARNESS_REPORT_FINALIZED" == "false" ]] || return 0
  HARNESS_REPORT_FINALIZED="true"

  local _h_result _h_finished _h_temporary _h_digest _h_size
  if [[ "$_h_exit_code" -eq 0 ]]; then _h_result="passed"; else _h_result="failed"; fi
  _h_finished="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  if [[ -e "$HARNESS_REPORT_OUTPUT" || -L "$HARNESS_REPORT_OUTPUT" ]]; then
    printf 'error: canonical report path already exists\n' >&2
    return 1
  fi
  _h_temporary="$(mktemp "$HARNESS_RUN_DIR/.report.XXXXXX")" || return 1
  chmod 0600 "$_h_temporary" 2>/dev/null || true
  if ! harness_report_render "$_h_exit_code" "$_h_finished" "$_h_result" >"$_h_temporary"; then
    rm -f "$_h_temporary"
    return 1
  fi
  if ! harness_utf8_validate_file report.document "$_h_temporary"; then
    rm -f "$_h_temporary"
    return 1
  fi
  if ! harness_path_has_no_symlink_components "$HARNESS_RUN_DIR" "$HARNESS_EVIDENCE_ROOT" \
    || ! mv "$_h_temporary" "$HARNESS_REPORT_OUTPUT"; then
    rm -f "$_h_temporary"
    return 1
  fi
  chmod 0444 "$HARNESS_REPORT_OUTPUT" 2>/dev/null || true
  harness_sha256_file_into _h_digest "$HARNESS_REPORT_OUTPUT" || return 1
  harness_file_size_into _h_size "$HARNESS_REPORT_OUTPUT" || return 1
  harness_assert_structural_value latest.report_digest "$_h_digest" || return 1
  harness_complete_run_marker || return 1
  # The completion marker guards every published view; latest remains the final
  # authoritative commit point after the non-authoritative summary copy.
  harness_atomic_copy "$HARNESS_REPORT_OUTPUT" "$HARNESS_REPORT_SUMMARY_OUTPUT" || return 1
  harness_publish_latest_pointer "$_h_digest" "$_h_size" "$_h_result" "$_h_finished" || return 1

  HARNESS_REPORT_ACTIVE="false"
  HARNESS_RUN_CONTEXT_ACTIVE="false"
  harness_report_cleanup_temporaries
  printf 'evidence: %s\n' "$HARNESS_REPORT_OUTPUT"
}

harness_run_context_complete_without_report() {
  [[ "$HARNESS_RUN_CONTEXT_ACTIVE" == "true" ]] || return 0
  if [[ "$HARNESS_RUN_CONTEXT_HAS_VALIDATED_EVIDENCE" == "true" ]]; then
    harness_complete_run_marker || return 1
  fi
  HARNESS_RUN_CONTEXT_ACTIVE="false"
}

harness_report_on_exit() {
  local _h_exit_code=$?
  local _h_evidence_status=0
  trap - EXIT

  if [[ "$HARNESS_REPORT_ACTIVE" == "true" ]]; then
    if [[ "$HARNESS_REPORT_EVIDENCE_INVALID" == "true" ]]; then
      _h_evidence_status=1
    elif [[ -n "$HARNESS_REPORT_CURRENT_STAGE" ]]; then
      if [[ "$_h_exit_code" -eq 3 ]]; then
        harness_report_stage_finish blocked "$_h_exit_code" "command reported blocked/unavailable" || _h_evidence_status=$?
      else
        harness_report_stage_finish failed "$_h_exit_code" "command terminated before completing" || _h_evidence_status=$?
      fi
    fi
    if [[ "$_h_evidence_status" -eq 0 ]]; then
      harness_report_finalize "$_h_exit_code" || _h_evidence_status=$?
    fi
  elif [[ "$HARNESS_RUN_CONTEXT_ACTIVE" == "true" ]]; then
    if [[ "$HARNESS_REPORT_EVIDENCE_INVALID" == "true" ]]; then
      _h_evidence_status=1
    else
      harness_run_context_complete_without_report || _h_evidence_status=$?
    fi
  fi
  harness_report_cleanup_temporaries
  harness_private_tmpdir_cleanup
  if [[ "$_h_evidence_status" -ne 0 && "$_h_exit_code" -eq 0 ]]; then
    _h_exit_code=1
  fi
  exit "$_h_exit_code"
}
