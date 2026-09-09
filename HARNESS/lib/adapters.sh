#!/usr/bin/env bash

# Conservative Make, Node.js, Python, Go, and Rust command adapters. This module
# expects harness.sh to provide ROOT_DIR, run helpers, and project settings.

select_node_script() {
  local manifest="$1"
  shift
  require_command node
  local script status
  for script in "$@"; do
    if node -e '
      const fs = require("fs");
      const path = process.argv[1];
      const name = process.argv[2];
      const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
      process.exit(pkg.scripts && pkg.scripts[name] ? 0 : 4);
    ' "$manifest" "$script"; then
      printf '%s\n' "$script"
      return 0
    else
      status=$?
      [[ "$status" -eq 4 ]] || return "$status"
    fi
  done
  return 3
}

run_node_package_script() {
  local manifest="$1"
  local script="$2"
  local directory manager
  directory="$(dirname "$manifest")"
  manager="$(node -e '
    const fs = require("fs");
    const pkg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    const value = typeof pkg.packageManager === "string" ? pkg.packageManager : "";
    process.stdout.write(value ? value.split("@")[0] : "");
  ' "$manifest")"

  if [[ -z "$manager" ]]; then
    if [[ -f "$directory/pnpm-lock.yaml" ]]; then
      manager="pnpm"
    elif [[ -f "$directory/yarn.lock" ]]; then
      manager="yarn"
    elif [[ -f "$directory/bun.lock" || -f "$directory/bun.lockb" ]]; then
      manager="bun"
    else
      manager="npm"
    fi
  fi

  case "$manager" in
    pnpm)
      if command -v pnpm >/dev/null 2>&1; then
        run_in "$directory" pnpm run "$script"
      elif command -v corepack >/dev/null 2>&1; then
        run_in "$directory" corepack pnpm run "$script"
      else
        die "packageManager declares pnpm but pnpm/corepack is unavailable: $directory"
      fi
      ;;
    yarn)
      if command -v yarn >/dev/null 2>&1; then
        run_in "$directory" yarn run "$script"
      elif command -v corepack >/dev/null 2>&1; then
        run_in "$directory" corepack yarn run "$script"
      else
        die "packageManager declares Yarn but yarn/corepack is unavailable: $directory"
      fi
      ;;
    bun)
      require_command bun
      run_in "$directory" bun run "$script"
      ;;
    npm)
      require_command npm
      run_in "$directory" npm run "$script"
      ;;
    *)
      die "unsupported packageManager '$manager' in $manifest"
      ;;
  esac
}

RUNNER_FOUND=0

run_node_stage() {
  RUNNER_FOUND=0
  local candidates=(
    "$ROOT_DIR/package.json"
    "$ROOT_DIR"/*/package.json
    "$ROOT_DIR"/*/*/package.json
  )
  local manifest script status

  if [[ -f "$ROOT_DIR/package.json" ]]; then
    if script="$(select_node_script "$ROOT_DIR/package.json" "$@")"; then
      RUNNER_FOUND=1
      run_node_package_script "$ROOT_DIR/package.json" "$script"
      return 0
    else
      status=$?
      [[ "$status" -eq 3 ]] || return "$status"
    fi
  fi

  for manifest in "${candidates[@]}"; do
    [[ -f "$manifest" ]] || continue
    [[ "$manifest" == "$ROOT_DIR/package.json" ]] && continue
    is_ignored_path "$manifest" && continue
    if script="$(select_node_script "$manifest" "$@")"; then
      RUNNER_FOUND=1
      run_node_package_script "$manifest" "$script"
    else
      status=$?
      [[ "$status" -eq 3 ]] || return "$status"
    fi
  done

  return 0
}

run_make_stage() {
  RUNNER_FOUND=0
  local makefile=""
  [[ -f "$ROOT_DIR/Makefile" ]] && makefile="$ROOT_DIR/Makefile"
  [[ -z "$makefile" && -f "$ROOT_DIR/makefile" ]] && makefile="$ROOT_DIR/makefile"
  [[ -n "$makefile" ]] || return 0

  local target
  for target in "$@"; do
    if grep -Eq "^${target}([[:space:]]*):" "$makefile"; then
      require_command make
      RUNNER_FOUND=1
      run_in "$ROOT_DIR" make "$target"
      return 0
    fi
  done
  return 0
}

python_bin_for() {
  local directory="$1"
  if [[ -x "$directory/.venv/bin/python" ]]; then
    printf '%s\n' "$directory/.venv/bin/python"
  elif [[ -x "$directory/venv/bin/python" ]]; then
    printf '%s\n' "$directory/venv/bin/python"
  elif command -v python3 >/dev/null 2>&1; then
    command -v python3
  elif command -v python >/dev/null 2>&1; then
    command -v python
  else
    return 127
  fi
}

run_python_tool() {
  local directory="$1"
  local tool="$2"
  shift 2
  local python_bin

  if [[ -x "$directory/.venv/bin/$tool" ]]; then
    run_in "$directory" "$directory/.venv/bin/$tool" "$@"
  elif [[ -x "$directory/venv/bin/$tool" ]]; then
    run_in "$directory" "$directory/venv/bin/$tool" "$@"
  elif command -v "$tool" >/dev/null 2>&1; then
    run_in "$directory" "$tool" "$@"
  elif python_bin="$(python_bin_for "$directory")" && "$python_bin" -c "import $tool" >/dev/null 2>&1; then
    run_in "$directory" "$python_bin" -m "$tool" "$@"
  else
    die "$tool is configured for $directory but is not installed"
  fi
}

python_directory_matches() {
  local directory="$1"
  local pattern="$2"
  local config_file
  local config_files=(
    "$directory/pyproject.toml"
    "$directory/setup.cfg"
    "$directory/setup.py"
    "$directory/requirements.txt"
    "$directory/pytest.ini"
    "$directory/tox.ini"
    "$directory/mypy.ini"
    "$directory/.mypy.ini"
  )

  for config_file in "${config_files[@]}"; do
    [[ -f "$config_file" ]] || continue
    if grep -Eq "$pattern" "$config_file"; then
      return 0
    fi
  done
  return 1
}

run_python_stage() {
  RUNNER_FOUND=0
  local stage="$1"
  local candidates=(
    "$ROOT_DIR/pyproject.toml"
    "$ROOT_DIR/setup.cfg"
    "$ROOT_DIR/setup.py"
    "$ROOT_DIR/requirements.txt"
    "$ROOT_DIR"/*/pyproject.toml
    "$ROOT_DIR"/*/setup.cfg
    "$ROOT_DIR"/*/setup.py
    "$ROOT_DIR"/*/requirements.txt
    "$ROOT_DIR"/*/*/pyproject.toml
    "$ROOT_DIR"/*/*/setup.cfg
    "$ROOT_DIR"/*/*/setup.py
    "$ROOT_DIR"/*/*/requirements.txt
  )
  local manifest directory python_bin seen="|" ran=0

  for manifest in "${candidates[@]}"; do
    [[ -f "$manifest" ]] || continue
    is_ignored_path "$manifest" && continue
    directory="$(dirname "$manifest")"
    case "$seen" in
      *"|$directory|"*) continue ;;
    esac
    seen="${seen}${directory}|"

    case "$stage" in
      lint)
        if [[ -f "$directory/ruff.toml" || -f "$directory/.ruff.toml" ]] \
          || python_directory_matches "$directory" '\[tool\.ruff'; then
          run_python_tool "$directory" ruff check .
          ran=1
        fi
        ;;
      format)
        if [[ -f "$directory/ruff.toml" || -f "$directory/.ruff.toml" ]] \
          || python_directory_matches "$directory" '\[tool\.ruff'; then
          run_python_tool "$directory" ruff format --check .
          ran=1
        elif python_directory_matches "$directory" '\[tool\.black\]|\[black\]'; then
          run_python_tool "$directory" black --check .
          ran=1
        fi
        ;;
      typecheck)
        if python_directory_matches "$directory" '\[tool\.mypy\]|\[mypy\]'; then
          run_python_tool "$directory" mypy .
          ran=1
        fi
        ;;
      test)
        if [[ -d "$directory/tests" || -f "$directory/pytest.ini" ]]; then
          python_bin="$(python_bin_for "$directory")" || die "Python is required for tests in $directory"
          run_in "$directory" "$python_bin" -m pytest
          ran=1
        fi
        ;;
      coverage)
        if [[ -f "$directory/.coveragerc" ]] \
          || python_directory_matches "$directory" '\[tool\.coverage|\[coverage:'; then
          python_bin="$(python_bin_for "$directory")" || die "Python is required for coverage in $directory"
          if "$python_bin" -c 'import pytest_cov' >/dev/null 2>&1; then
            run_in "$directory" "$python_bin" -m pytest --cov=. --cov-fail-under="$HARNESS_COVERAGE_THRESHOLD"
            ran=1
          else
            die "coverage is configured for $directory but pytest-cov is not installed"
          fi
        fi
        ;;
      integration)
        if [[ -d "$directory/tests/integration" ]]; then
          python_bin="$(python_bin_for "$directory")" || die "Python is required for integration tests in $directory"
          run_in "$directory" "$python_bin" -m pytest tests/integration
          ran=1
        fi
        ;;
      e2e)
        if [[ -d "$directory/tests/e2e" ]]; then
          python_bin="$(python_bin_for "$directory")" || die "Python is required for E2E tests in $directory"
          run_in "$directory" "$python_bin" -m pytest tests/e2e
          ran=1
        fi
        ;;
    esac
  done

  [[ "$ran" -eq 1 ]] && RUNNER_FOUND=1
  return 0
}

run_compiled_stage() {
  RUNNER_FOUND=0
  local stage="$1"
  local ran=0

  if [[ -f "$ROOT_DIR/go.mod" ]]; then
    case "$stage" in
      lint) require_command go; run_in "$ROOT_DIR" go vet ./...; ran=1 ;;
      test) require_command go; run_in "$ROOT_DIR" go test ./...; ran=1 ;;
      build) require_command go; run_in "$ROOT_DIR" go build ./...; ran=1 ;;
    esac
  fi

  if [[ -f "$ROOT_DIR/Cargo.toml" ]]; then
    case "$stage" in
      lint) require_command cargo; run_in "$ROOT_DIR" cargo clippy --all-targets -- -D warnings; ran=1 ;;
      format) require_command cargo; run_in "$ROOT_DIR" cargo fmt --all -- --check; ran=1 ;;
      typecheck) require_command cargo; run_in "$ROOT_DIR" cargo check --all-targets; ran=1 ;;
      test) require_command cargo; run_in "$ROOT_DIR" cargo test --all-targets; ran=1 ;;
      build) require_command cargo; run_in "$ROOT_DIR" cargo build; ran=1 ;;
    esac
  fi

  [[ "$ran" -eq 1 ]] && RUNNER_FOUND=1
  return 0
}

STAGE_RAN=0
