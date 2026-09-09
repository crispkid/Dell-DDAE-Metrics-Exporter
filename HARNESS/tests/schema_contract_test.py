#!/usr/bin/env python3
"""Baseline-development tests for the bundled Draft 2020-12 schemas."""

import copy
import hashlib
import json
import math
import os
import re
import sys
from datetime import datetime
from importlib.metadata import version
from pathlib import Path, PurePosixPath

try:
    from jsonschema import Draft202012Validator, FormatChecker
except ImportError as exc:  # pragma: no cover - dependency preflight
    raise SystemExit(
        "jsonschema 4.26.0 is required for schema contract tests; "
        "it is a baseline CI dependency, not a Harness runtime dependency"
    ) from exc


ROOT = Path(__file__).resolve().parents[2]
CASE_SCHEMA_PATH = ROOT / "HARNESS/schemas/agent-eval-case.schema.json"
RESULT_SCHEMA_PATH = ROOT / "HARNESS/schemas/agent-eval-result.schema.json"
REPORT_SCHEMA_PATH = ROOT / "HARNESS/schemas/harness-report.schema.json"
LATEST_SCHEMA_PATH = ROOT / "HARNESS/schemas/latest-pointer.schema.json"
CASE_EXAMPLE_PATH = ROOT / "HARNESS/templates/evals/case.json.example"
RESULT_EXAMPLE_PATH = ROOT / "HARNESS/templates/evals/result.json.example"
REPORT_EXAMPLE_PATH = ROOT / "HARNESS/templates/evidence/report.json.example"
LATEST_EXAMPLE_PATH = ROOT / "HARNESS/templates/evidence/latest-pointer.json.example"


def reject_constant(value: str):
    raise ValueError(f"non-finite JSON number is forbidden: {value}")


def reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key is forbidden: {key}")
        result[key] = value
    return result


def load(path: Path):
    with path.open(encoding="utf-8") as stream:
        return json.load(
            stream,
            parse_constant=reject_constant,
            object_pairs_hook=reject_duplicate_keys,
        )


def assert_invalid(validator, instance, label: str):
    if validator.is_valid(instance):
        raise AssertionError(f"negative schema fixture unexpectedly passed: {label}")


def assert_semantically_invalid(check, label: str):
    try:
        check()
    except AssertionError:
        return
    raise AssertionError(f"negative semantic fixture unexpectedly passed: {label}")


def assert_report_semantics(document):
    run_id = document["run_id"]
    run_type = document["run_type"]
    expected_prefix = f"-{run_type}."
    if expected_prefix not in run_id:
        raise AssertionError("report run_id does not agree with run_type")
    expected_path = f"runs/{run_id}/report.json"
    if document["storage"]["canonical_path"] != expected_path:
        raise AssertionError("report canonical_path does not agree with run_id")
    for artifact in document["artifacts"]:
        if not artifact["path"].startswith(f"runs/{run_id}/"):
            raise AssertionError("report artifact path does not agree with run_id")
    stage_records = document["stages"]
    stage_names = [stage["stage"] for stage in stage_records]
    if len(set(stage_names)) != len(stage_names):
        raise AssertionError("report stage names are not unique")
    if run_type == "verify":
        internal_prefix = ["doctor", "repo-doctor", "governance-doctor"]
        if stage_names[:3] != internal_prefix:
            raise AssertionError("verify report is missing its ordered internal preflight stages")
        scheduled = set(document["effective_policy"]["verify_stages"])
        product_names = {
            "lint", "format", "typecheck", "test", "coverage", "build",
            "integration", "e2e", "security", "deploy", "ci-policy", "supply-chain",
        }
        observed_product = [name for name in stage_names if name in product_names]
        if document["result"] == "passed" and set(observed_product) != scheduled:
            raise AssertionError("passed verify report does not cover every scheduled product stage")
        required = set(document["effective_policy"]["required_stages"])
        records_by_name = {stage["stage"]: stage for stage in stage_records}
        if document["result"] == "passed" and any(
            records_by_name.get(name, {}).get("status") != "passed" for name in required
        ):
            raise AssertionError("passed verify report has a non-passing required stage")
        sdd_required = document["effective_policy"]["sdd_mode"] == "required"
        if sdd_required:
            if "sdd-check" not in stage_names:
                raise AssertionError("strict-SDD verify report is missing final sdd-check")
            if observed_product and stage_names.index("sdd-check") <= max(
                stage_names.index(name) for name in observed_product
            ):
                raise AssertionError("strict-SDD closure ran before its product-stage evidence")
            if document["result"] == "passed" and records_by_name["sdd-check"]["status"] != "passed":
                raise AssertionError("passed strict-SDD report has a non-passing closure stage")
        elif "sdd-check" in stage_names:
            raise AssertionError("SDD-disabled verify report contains an sdd-check stage")


def assert_latest_semantics(document):
    run_id = document["run_id"]
    if f"-{document['run_type']}." not in run_id:
        raise AssertionError("latest run_id does not agree with run_type")
    if document["report_path"] != f"runs/{run_id}/report.json":
        raise AssertionError("latest report_path does not agree with run_id")


def assert_report_pointer_pair(report, latest, report_path: Path):
    expected_digest = "sha256:" + hashlib.sha256(report_path.read_bytes()).hexdigest()
    expected_size = report_path.stat().st_size
    shared_fields = ("run_id", "run_type", "result", "finished_at")
    for field in shared_fields:
        if latest[field] != report[field]:
            raise AssertionError(f"latest {field} does not agree with report")
    if latest["report_path"] != report["storage"]["canonical_path"]:
        raise AssertionError("latest report_path does not agree with report storage")
    if latest["report_digest"] != expected_digest:
        raise AssertionError("latest report_digest does not identify report bytes")
    if latest["report_size_bytes"] != expected_size:
        raise AssertionError("latest report_size_bytes does not identify report bytes")


def metric_identity(metric):
    return metric["name"], metric.get("k")


def assert_case_result_pair(case, result):
    if case["schema_version"] != result["schema_version"]:
        raise AssertionError("case and result schema versions differ")
    if result["suite"] != case["suite"]:
        raise AssertionError("result suite does not agree with case")
    if len(result["cases"]) != 1:
        raise AssertionError("bundled result must contain exactly its bundled case")

    case_result = result["cases"][0]
    if case_result["case_id"] != case["id"]:
        raise AssertionError("result case_id does not agree with case")
    if case_result["case_version"] != case["version"]:
        raise AssertionError("result case_version does not agree with case")

    expected_inputs = {
        "fixture_revision": case["fixture"]["revision"],
        "fixture_digest": case["fixture"]["digest"],
        "agent_config_digest": case["agent"]["config_digest"],
        "environment_identity": case["environment"]["identity"],
        "reference_solution_digest": case["reference"]["solution_digest"],
    }
    for field, expected in expected_inputs.items():
        if case_result["inputs"][field] != expected:
            raise AssertionError(f"result input {field} does not agree with case")

    declared_graders = {
        grader["id"]: grader for grader in case["verifier"]["graders"]
    }
    observed_input_graders = {
        grader["grader_id"]: grader for grader in case_result["inputs"]["graders"]
    }
    if len(observed_input_graders) != len(case_result["inputs"]["graders"]):
        raise AssertionError("result input grader IDs are not unique")
    if set(observed_input_graders) != set(declared_graders):
        raise AssertionError("result input grader IDs do not agree with case")
    for grader_id, grader in declared_graders.items():
        if observed_input_graders[grader_id]["config_digest"] != grader["config_digest"]:
            raise AssertionError("result grader config digest does not agree with case")

    trials = case_result["trials"]
    if len(trials) != case["trials"]["count"]:
        raise AssertionError("result trial count does not agree with case")
    indexes = [trial["trial_index"] for trial in trials]
    if len(set(indexes)) != len(indexes) or set(indexes) != set(range(1, len(trials) + 1)):
        raise AssertionError("result trial indexes are not unique and contiguous")

    required_graders = {
        grader_id for grader_id, grader in declared_graders.items() if grader["required"]
    }
    declared_artifacts = set(case["artifacts"]["paths"])
    for trial in trials:
        trial_graders = [grader["grader_id"] for grader in trial["graders"]]
        observed_graders = {grader["grader_id"]: grader for grader in trial["graders"]}
        if len(set(trial_graders)) != len(trial_graders):
            raise AssertionError("trial grader IDs are not unique")
        if not required_graders.issubset(trial_graders):
            raise AssertionError("trial is missing a required grader")
        if trial["status"] == "passed" and any(
            observed_graders[grader_id]["status"] != "passed"
            for grader_id in required_graders
        ):
            raise AssertionError("passed trial has a non-passing required grader")
        if trial["status"] in ("passed", "failed"):
            weighted_total = sum(
                observed_graders[grader_id]["score"] * declared_graders[grader_id]["weight"]
                for grader_id in declared_graders
                if grader_id in observed_graders
                and observed_graders[grader_id]["status"] in ("passed", "failed")
            )
            weight_total = sum(
                declared_graders[grader_id]["weight"]
                for grader_id in declared_graders
                if grader_id in observed_graders
                and observed_graders[grader_id]["status"] in ("passed", "failed")
            )
            if weight_total <= 0:
                raise AssertionError("completed trial has no weighted grader result")
            threshold_met = weighted_total / weight_total >= case["verifier"]["threshold"]
            if (trial["status"] == "passed") != threshold_met:
                raise AssertionError("trial status does not agree with weighted grader threshold")
        artifacts = trial.get("artifacts", {})
        if trial["status"] in ("passed", "failed") and set(artifacts) != declared_artifacts:
            raise AssertionError("completed trial artifacts do not agree with case")
        prefix = f"artifacts/{case['id']}/trial-{trial['trial_index']}/"
        for name, artifact in artifacts.items():
            path = PurePosixPath(artifact["path"])
            if path.is_absolute() or ".." in path.parts or artifact["path"] != prefix + name:
                raise AssertionError("artifact path escapes or disagrees with its artifact key")
        for grader in trial["graders"]:
            artifact_ref = grader.get("artifact")
            if artifact_ref is not None and artifact_ref not in artifacts:
                raise AssertionError("grader artifact reference does not resolve")

    summary = case_result["summary"]
    status_counts = {
        "passed": sum(trial["status"] == "passed" for trial in trials),
        "failed": sum(trial["status"] == "failed" for trial in trials),
        "blocked": sum(trial["status"] == "blocked" for trial in trials),
        "errors": sum(trial["status"] == "error" for trial in trials),
    }
    if summary["total"] != len(trials):
        raise AssertionError("summary total does not agree with trials")
    for field, expected in status_counts.items():
        if summary[field] != expected:
            raise AssertionError(f"summary {field} does not agree with trials")
    expected_rate = status_counts["passed"] / len(trials) if trials else 0
    if not math.isclose(summary["pass_rate"], expected_rate):
        raise AssertionError("summary pass_rate does not agree with trials")
    duration = sum(trial["duration_seconds"] for trial in trials)
    if "duration_seconds" in summary and not math.isclose(summary["duration_seconds"], duration):
        raise AssertionError("summary duration does not agree with trials")

    declared_metrics = [metric_identity(metric) for metric in case["metrics"]]
    observed_metrics = [metric_identity(metric) for metric in case_result["metrics"]]
    if len(set(declared_metrics)) != len(declared_metrics):
        raise AssertionError("case metric identities are not unique")
    if len(set(observed_metrics)) != len(observed_metrics):
        raise AssertionError("result metric identities are not unique")
    if set(observed_metrics) != set(declared_metrics):
        raise AssertionError("result metrics do not agree with case")
    metric_values = {metric_identity(metric): metric["value"] for metric in case_result["metrics"]}
    for name, k in declared_metrics:
        if name == "duration_seconds":
            expected = duration
        elif name in ("tool_calls", "tokens", "cost"):
            expected = sum(trial.get("metrics", {}).get(name, 0) for trial in trials)
        elif name == "pass_at":
            expected = int(any(trial["status"] == "passed" for trial in trials[:k]))
        elif name == "pass_k":
            expected = int(len(trials[:k]) == k and all(
                trial["status"] == "passed" for trial in trials[:k]
            ))
        else:  # pragma: no cover - schema constrains portable metric names
            raise AssertionError(f"unsupported portable metric: {name}")
        if not math.isclose(metric_values[(name, k)], expected):
            raise AssertionError(f"aggregate metric {name} does not agree with trials")


def main() -> int:
    if version("jsonschema") != "4.26.0":
        raise AssertionError("schema conformance requires exactly jsonschema==4.26.0")
    for invalid_json in ('{"id":1,"id":2}', '{"value":NaN}', '{"value":Infinity}'):
        try:
            json.loads(
                invalid_json,
                parse_constant=reject_constant,
                object_pairs_hook=reject_duplicate_keys,
            )
        except ValueError:
            pass
        else:
            raise AssertionError("strict JSON parser accepted a forbidden document")
    case_schema = load(CASE_SCHEMA_PATH)
    result_schema = load(RESULT_SCHEMA_PATH)
    report_schema = load(REPORT_SCHEMA_PATH)
    latest_schema = load(LATEST_SCHEMA_PATH)
    case = load(CASE_EXAMPLE_PATH)
    result = load(RESULT_EXAMPLE_PATH)
    report = load(REPORT_EXAMPLE_PATH)
    latest = load(LATEST_EXAMPLE_PATH)

    Draft202012Validator.check_schema(case_schema)
    Draft202012Validator.check_schema(result_schema)
    Draft202012Validator.check_schema(report_schema)
    Draft202012Validator.check_schema(latest_schema)
    format_checker = FormatChecker()

    @format_checker.checks("date-time")
    def strict_rfc3339(value):
        if not isinstance(value, str):
            return True
        if not re.fullmatch(
            r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})",
            value,
        ):
            return False
        try:
            datetime.fromisoformat(value[:-1] + "+00:00" if value.endswith("Z") else value)
        except ValueError:
            return False
        return True
    case_validator = Draft202012Validator(case_schema, format_checker=format_checker)
    result_validator = Draft202012Validator(result_schema, format_checker=format_checker)
    report_validator = Draft202012Validator(report_schema, format_checker=format_checker)
    latest_validator = Draft202012Validator(latest_schema, format_checker=format_checker)
    case_validator.validate(case)
    result_validator.validate(result)
    report_validator.validate(report)
    latest_validator.validate(latest)
    assert_report_semantics(report)
    assert_latest_semantics(latest)
    assert_report_pointer_pair(report, latest, REPORT_EXAMPLE_PATH)
    assert_case_result_pair(case, result)

    generated_report_path = os.environ.get("HARNESS_GENERATED_REPORT")
    generated_latest_path = os.environ.get("HARNESS_GENERATED_LATEST")
    generated_agent_result_path = os.environ.get("HARNESS_GENERATED_AGENT_RESULT")
    if generated_report_path:
        generated_report = load(Path(generated_report_path))
        report_validator.validate(generated_report)
        assert_report_semantics(generated_report)
    if generated_latest_path:
        generated_latest = load(Path(generated_latest_path))
        latest_validator.validate(generated_latest)
        assert_latest_semantics(generated_latest)
    if generated_agent_result_path:
        result_validator.validate(load(Path(generated_agent_result_path)))

    for document, validator, label in (
        (case, case_validator, "agent case schema"),
        (result, result_validator, "agent result schema"),
        (report, report_validator, "harness report schema"),
    ):
        unsupported_schema = copy.deepcopy(document)
        unsupported_schema["schema_version"] = "unsupported"
        assert_invalid(validator, unsupported_schema, label)

    wrong_result_run_type = copy.deepcopy(result)
    wrong_result_run_type["run_id"] = "20260813T010000Z-verify.Abc123"
    assert_invalid(result_validator, wrong_result_run_type, "agent result run type mismatch")

    missing_version = copy.deepcopy(case)
    del missing_version["version"]
    assert_invalid(case_validator, missing_version, "case missing version")

    bad_network = copy.deepcopy(case)
    bad_network["environment"]["network"] = "sometimes"
    assert_invalid(case_validator, bad_network, "case invalid network enum")

    bad_deterministic_grader = copy.deepcopy(case)
    del bad_deterministic_grader["verifier"]["graders"][0]["command"]
    assert_invalid(case_validator, bad_deterministic_grader, "deterministic grader missing command")

    all_zero_weights = copy.deepcopy(case)
    all_zero_weights["verifier"]["graders"][0]["weight"] = 0
    assert_invalid(case_validator, all_zero_weights, "all grader weights are zero")

    missing_fixture_digest = copy.deepcopy(case)
    del missing_fixture_digest["fixture"]["digest"]
    assert_invalid(case_validator, missing_fixture_digest, "fixture missing digest")

    missing_agent_digest = copy.deepcopy(case)
    del missing_agent_digest["agent"]["config_digest"]
    assert_invalid(case_validator, missing_agent_digest, "agent config missing digest")

    missing_grader_ref = copy.deepcopy(case)
    del missing_grader_ref["verifier"]["graders"][0]["config_ref"]
    assert_invalid(case_validator, missing_grader_ref, "grader missing config reference")

    missing_grader_digest = copy.deepcopy(case)
    del missing_grader_digest["verifier"]["graders"][0]["config_digest"]
    assert_invalid(case_validator, missing_grader_digest, "grader config missing digest")

    missing_reference_digest = copy.deepcopy(case)
    del missing_reference_digest["reference"]["solution_digest"]
    assert_invalid(case_validator, missing_reference_digest, "reference missing digest")

    uppercase_digest = copy.deepcopy(case)
    uppercase_digest["agent"]["config_digest"] = "sha256:" + "A" * 64
    assert_invalid(case_validator, uppercase_digest, "uppercase digest")

    short_digest = copy.deepcopy(case)
    short_digest["reference"]["solution_digest"] = "sha256:abcd"
    assert_invalid(case_validator, short_digest, "short digest")

    unknown_digest_algorithm = copy.deepcopy(case)
    unknown_digest_algorithm["fixture"]["digest"] = "sha512:" + "a" * 64
    assert_invalid(case_validator, unknown_digest_algorithm, "unknown digest algorithm")

    extra_k = copy.deepcopy(case)
    extra_k["metrics"][-1]["k"] = 1
    assert_invalid(case_validator, extra_k, "non-pass metric with k")

    excessive_trials = copy.deepcopy(case)
    excessive_trials["trials"]["count"] = 1001
    assert_invalid(case_validator, excessive_trials, "case trial count above portable maximum")

    unsupported_retention = copy.deepcopy(case)
    unsupported_retention["artifacts"]["retention_days"] = 30
    assert_invalid(case_validator, unsupported_retention, "unsupported case retention field")

    invalid_retention_floor = copy.deepcopy(case)
    invalid_retention_floor["artifacts"]["minimum_retention_days"] = 0
    assert_invalid(case_validator, invalid_retention_floor, "invalid retention floor")

    missing_case_version = copy.deepcopy(result)
    del missing_case_version["cases"][0]["case_version"]
    assert_invalid(result_validator, missing_case_version, "result missing case version")

    missing_threshold = copy.deepcopy(result)
    missing_threshold["cases"][0]["status"] = "failed"
    del missing_threshold["cases"][0]["threshold_met"]
    assert_invalid(result_validator, missing_threshold, "failed result missing threshold")

    missing_completed_grader_score = copy.deepcopy(result)
    del missing_completed_grader_score["cases"][0]["trials"][0]["graders"][0]["score"]
    assert_invalid(
        result_validator,
        missing_completed_grader_score,
        "completed grader missing score",
    )

    missing_metrics = copy.deepcopy(result)
    del missing_metrics["cases"][0]["metrics"]
    assert_invalid(result_validator, missing_metrics, "result missing declared metrics")

    bad_metric = copy.deepcopy(result)
    bad_metric["cases"][0]["metrics"][0]["value"] = 2
    assert_invalid(result_validator, bad_metric, "pass metric outside 0..1")

    old_artifact = copy.deepcopy(result)
    old_artifact["cases"][0]["trials"][0]["artifacts"]["diff.patch"] = "diff.patch"
    assert_invalid(result_validator, old_artifact, "string-only artifact identity")

    artifact_without_digest = copy.deepcopy(result)
    del artifact_without_digest["cases"][0]["trials"][0]["artifacts"]["diff.patch"]["digest"]
    assert_invalid(result_validator, artifact_without_digest, "artifact missing digest")

    artifact_without_size = copy.deepcopy(result)
    del artifact_without_size["cases"][0]["trials"][0]["artifacts"]["diff.patch"]["size_bytes"]
    assert_invalid(result_validator, artifact_without_size, "artifact missing byte size")

    missing_inputs = copy.deepcopy(result)
    del missing_inputs["cases"][0]["inputs"]
    assert_invalid(result_validator, missing_inputs, "result missing observed inputs")

    missing_input_digest = copy.deepcopy(result)
    del missing_input_digest["cases"][0]["inputs"]["agent_config_digest"]
    assert_invalid(result_validator, missing_input_digest, "result input missing digest")

    missing_environment_identity = copy.deepcopy(result)
    del missing_environment_identity["cases"][0]["inputs"]["environment_identity"]
    assert_invalid(result_validator, missing_environment_identity, "result missing environment identity")

    missing_provenance = copy.deepcopy(result)
    del missing_provenance["provenance"]
    assert_invalid(result_validator, missing_provenance, "result missing provenance")

    missing_dataset_digest = copy.deepcopy(result)
    del missing_dataset_digest["provenance"]["dataset_digest"]
    assert_invalid(result_validator, missing_dataset_digest, "result missing dataset digest")

    clean_with_diff = copy.deepcopy(result)
    clean_with_diff["provenance"]["repository_dirty"] = False
    assert_invalid(result_validator, clean_with_diff, "clean result with diff digest")

    malformed_time = copy.deepcopy(result)
    malformed_time["started_at"] = "not-a-time"
    assert_invalid(result_validator, malformed_time, "invalid result timestamp")

    result_extra_k = copy.deepcopy(result)
    result_extra_k["cases"][0]["metrics"][-1]["k"] = 1
    assert_invalid(result_validator, result_extra_k, "non-pass result metric with k")

    completed_without_artifacts = copy.deepcopy(result)
    del completed_without_artifacts["cases"][0]["trials"][0]["artifacts"]
    assert_invalid(result_validator, completed_without_artifacts, "completed trial missing artifacts")

    passed_without_cases = copy.deepcopy(result)
    passed_without_cases["cases"] = []
    assert_invalid(result_validator, passed_without_cases, "passed suite without cases")

    blocked_suite = copy.deepcopy(result)
    blocked_suite["status"] = "blocked"
    blocked_suite["cases"] = []
    blocked_suite["provenance"]["dataset_digest"] = None
    blocked_suite["failure"] = {
        "phase": "suite-setup",
        "code": "fixture-unavailable",
        "detail": "The isolated fixture could not be prepared.",
    }
    result_validator.validate(blocked_suite)

    passed_without_dataset_identity = copy.deepcopy(result)
    passed_without_dataset_identity["provenance"]["dataset_digest"] = None
    assert_invalid(
        result_validator,
        passed_without_dataset_identity,
        "passed suite without dataset identity",
    )

    blocked_suite_without_failure = copy.deepcopy(blocked_suite)
    del blocked_suite_without_failure["failure"]
    assert_invalid(result_validator, blocked_suite_without_failure, "blocked suite without failure")

    passed_suite_with_failure = copy.deepcopy(result)
    passed_suite_with_failure["failure"] = copy.deepcopy(blocked_suite["failure"])
    assert_invalid(result_validator, passed_suite_with_failure, "passed suite with failure")

    blocked_case_result = copy.deepcopy(result)
    blocked_case_result["status"] = "blocked"
    blocked_case_result["failure"] = {
        "phase": "case-execution",
        "code": "case-blocked",
        "detail": "At least one case was blocked.",
    }
    blocked_case = blocked_case_result["cases"][0]
    blocked_case["status"] = "blocked"
    blocked_case["threshold_met"] = None
    blocked_case["trials"] = []
    blocked_case["summary"] = {
        "total": 0,
        "passed": 0,
        "failed": 0,
        "blocked": 0,
        "errors": 0,
        "pass_rate": 0,
    }
    blocked_case["metrics"] = []
    blocked_case["failure"] = {
        "phase": "case-setup",
        "code": "environment-unavailable",
        "detail": "The case environment could not be created.",
    }
    result_validator.validate(blocked_case_result)

    blocked_case_without_failure = copy.deepcopy(blocked_case_result)
    del blocked_case_without_failure["cases"][0]["failure"]
    assert_invalid(result_validator, blocked_case_without_failure, "blocked case without failure")

    blocked_case_with_boolean_threshold = copy.deepcopy(blocked_case_result)
    blocked_case_with_boolean_threshold["cases"][0]["threshold_met"] = False
    assert_invalid(
        result_validator,
        blocked_case_with_boolean_threshold,
        "blocked case with invented threshold",
    )

    failed_case_without_trials = copy.deepcopy(result)
    failed_case_without_trials["status"] = "failed"
    failed_case_without_trials["cases"][0]["status"] = "failed"
    failed_case_without_trials["cases"][0]["threshold_met"] = False
    failed_case_without_trials["cases"][0]["trials"] = []
    assert_invalid(result_validator, failed_case_without_trials, "failed case without trials")

    failed_case_without_metrics = copy.deepcopy(result)
    failed_case_without_metrics["status"] = "failed"
    failed_case_without_metrics["cases"][0]["status"] = "failed"
    failed_case_without_metrics["cases"][0]["threshold_met"] = False
    failed_case_without_metrics["cases"][0]["metrics"] = []
    assert_invalid(result_validator, failed_case_without_metrics, "failed case without metrics")

    blocked_trial_result = copy.deepcopy(blocked_case_result)
    blocked_case = blocked_trial_result["cases"][0]
    blocked_case["trials"] = [{
        "trial_index": 1,
        "status": "blocked",
        "started_at": "2026-08-13T01:00:00Z",
        "finished_at": "2026-08-13T01:00:00Z",
        "duration_seconds": 0,
        "graders": [],
        "failure": {
            "phase": "trial-setup",
            "code": "sandbox-unavailable",
            "detail": "The isolated trial sandbox could not be created.",
        },
    }]
    blocked_case["summary"]["total"] = 1
    blocked_case["summary"]["blocked"] = 1
    result_validator.validate(blocked_trial_result)

    blocked_trial_without_failure = copy.deepcopy(blocked_trial_result)
    del blocked_trial_without_failure["cases"][0]["trials"][0]["failure"]
    assert_invalid(result_validator, blocked_trial_without_failure, "blocked trial without failure")

    failed_trial_without_graders = copy.deepcopy(result)
    failed_trial_without_graders["status"] = "failed"
    failed_trial_without_graders["cases"][0]["status"] = "failed"
    failed_trial_without_graders["cases"][0]["threshold_met"] = False
    failed_trial_without_graders["cases"][0]["trials"][0]["status"] = "failed"
    failed_trial_without_graders["cases"][0]["trials"][0]["graders"] = []
    assert_invalid(result_validator, failed_trial_without_graders, "failed trial without graders")

    blocked_grader_result = copy.deepcopy(blocked_trial_result)
    blocked_trial = blocked_grader_result["cases"][0]["trials"][0]
    blocked_trial["graders"] = [{
        "grader_id": "outcome-and-scope",
        "status": "blocked",
        "failure": {
            "phase": "grader-setup",
            "code": "grader-unavailable",
            "detail": "The isolated grader could not be started.",
        },
    }]
    result_validator.validate(blocked_grader_result)

    blocked_grader_without_failure = copy.deepcopy(blocked_grader_result)
    del blocked_grader_without_failure["cases"][0]["trials"][0]["graders"][0]["failure"]
    assert_invalid(result_validator, blocked_grader_without_failure, "blocked grader without failure")

    report_with_command = copy.deepcopy(report)
    report_with_command["stages"][0]["command"] = "secret command"
    assert_invalid(report_validator, report_with_command, "raw command in report")

    report_missing_effective_policy_field = copy.deepcopy(report)
    del report_missing_effective_policy_field["effective_policy"]["governance_mode"]
    assert_invalid(
        report_validator,
        report_missing_effective_policy_field,
        "report missing effective policy field",
    )

    report_missing_sdd = copy.deepcopy(report)
    del report_missing_sdd["sdd"]
    assert_invalid(report_validator, report_missing_sdd, "report missing SDD evidence")

    report_missing_sdd_policy = copy.deepcopy(report)
    del report_missing_sdd_policy["effective_policy"]["sdd_mode"]
    assert_invalid(report_validator, report_missing_sdd_policy, "report missing SDD policy")

    required_verify_not_verified = copy.deepcopy(report)
    required_verify_not_verified["sdd"]["contract_status"] = "not-run"
    assert_invalid(
        report_validator,
        required_verify_not_verified,
        "passed strict-SDD verify without verified contract",
    )

    verified_without_plan_digest = copy.deepcopy(report)
    verified_without_plan_digest["sdd"]["plan_basis_digest"] = None
    assert_invalid(
        report_validator,
        verified_without_plan_digest,
        "verified SDD report missing approved plan-basis identity",
    )

    approved_sdd = copy.deepcopy(report)
    approved_sdd["run_type"] = "agent-eval"
    approved_sdd["run_id"] = "20260813T010000Z-agent-eval.Abc123"
    approved_sdd["storage"]["canonical_path"] = (
        "runs/20260813T010000Z-agent-eval.Abc123/report.json"
    )
    approved_sdd["sdd"]["contract_status"] = "approved"
    report_validator.validate(approved_sdd)

    approved_without_test_digest = copy.deepcopy(approved_sdd)
    approved_without_test_digest["sdd"]["test_plan_digest"] = None
    assert_invalid(
        report_validator,
        approved_without_test_digest,
        "approved SDD report missing test-plan identity",
    )

    off_report = copy.deepcopy(report)
    off_report["effective_policy"]["sdd_mode"] = "off"
    off_report["sdd"] = {
        "mode": "off",
        "contract_status": "disabled",
        "change_id": None,
        "specification_version": None,
        "specification_digest": None,
        "plan_basis_digest": None,
        "test_plan_digest": None,
        "traceability_digest": None,
    }
    report_validator.validate(off_report)

    off_report_with_identity = copy.deepcopy(off_report)
    off_report_with_identity["sdd"]["change_id"] = "CHG-101"
    assert_invalid(
        report_validator,
        off_report_with_identity,
        "disabled SDD report containing an active identity",
    )

    mismatched_sdd_policy = copy.deepcopy(report)
    mismatched_sdd_policy["effective_policy"]["sdd_mode"] = "off"
    assert_invalid(
        report_validator,
        mismatched_sdd_policy,
        "report with mismatched effective and observed SDD modes",
    )

    agent_report_with_unrun_sdd = copy.deepcopy(report)
    agent_report_with_unrun_sdd["run_type"] = "agent-eval"
    agent_report_with_unrun_sdd["run_id"] = "20260813T010000Z-agent-eval.Abc123"
    agent_report_with_unrun_sdd["storage"]["canonical_path"] = (
        "runs/20260813T010000Z-agent-eval.Abc123/report.json"
    )
    agent_report_with_unrun_sdd["sdd"] = {
        "mode": "required",
        "contract_status": "not-run",
        "change_id": None,
        "specification_version": None,
        "specification_digest": None,
        "plan_basis_digest": None,
        "test_plan_digest": None,
        "traceability_digest": None,
    }
    report_validator.validate(agent_report_with_unrun_sdd)

    required_disabled_sdd = copy.deepcopy(report)
    required_disabled_sdd["sdd"]["contract_status"] = "disabled"
    assert_invalid(
        report_validator,
        required_disabled_sdd,
        "required SDD report claiming disabled contract",
    )

    unrun_sdd_with_identity = copy.deepcopy(agent_report_with_unrun_sdd)
    unrun_sdd_with_identity["sdd"]["change_id"] = "CHG-101"
    assert_invalid(
        report_validator,
        unrun_sdd_with_identity,
        "unrun SDD report containing observed identity",
    )

    report_duplicate_verify_stage = copy.deepcopy(report)
    report_duplicate_verify_stage["effective_policy"]["verify_stages"] = ["test", "test"]
    assert_invalid(
        report_validator,
        report_duplicate_verify_stage,
        "report duplicate effective verify stage",
    )

    report_missing_scheduled_stage = copy.deepcopy(report)
    report_missing_scheduled_stage["stages"] = [
        stage for stage in report_missing_scheduled_stage["stages"]
        if stage["stage"] != "test"
    ]
    assert_semantically_invalid(
        lambda: assert_report_semantics(report_missing_scheduled_stage),
        "passed report missing scheduled product stage",
    )

    report_duplicate_observed_stage = copy.deepcopy(report)
    report_duplicate_observed_stage["stages"].append(
        copy.deepcopy(report_duplicate_observed_stage["stages"][3])
    )
    assert_semantically_invalid(
        lambda: assert_report_semantics(report_duplicate_observed_stage),
        "report duplicate observed stage",
    )

    report_sdd_before_product = copy.deepcopy(report)
    report_sdd_before_product["stages"][3], report_sdd_before_product["stages"][4] = (
        report_sdd_before_product["stages"][4],
        report_sdd_before_product["stages"][3],
    )
    assert_semantically_invalid(
        lambda: assert_report_semantics(report_sdd_before_product),
        "strict-SDD report closes before product stage",
    )

    report_invalid_effective_coverage = copy.deepcopy(report)
    report_invalid_effective_coverage["effective_policy"]["coverage_threshold"] = 100.1
    assert_invalid(
        report_validator,
        report_invalid_effective_coverage,
        "report invalid effective coverage threshold",
    )

    report_run_type_mismatch = copy.deepcopy(report)
    report_run_type_mismatch["run_id"] = "20260813T010000Z-agent-eval.Abc123"
    assert_invalid(report_validator, report_run_type_mismatch, "report run type mismatch")

    report_without_artifact_digest = copy.deepcopy(report)
    report_without_artifact_digest["artifacts"] = [
        {
            "name": "example",
            "path": "runs/20260813T010000Z-verify.Abc123/example.txt",
            "size_bytes": 1,
        }
    ]
    assert_invalid(report_validator, report_without_artifact_digest, "report artifact missing digest")

    passed_nonzero = copy.deepcopy(report)
    passed_nonzero["exit_code"] = 7
    assert_invalid(report_validator, passed_nonzero, "passed report with nonzero exit")

    passed_stage_nonzero = copy.deepcopy(report)
    passed_stage_nonzero["stages"][0]["exit_code"] = 7
    assert_invalid(report_validator, passed_stage_nonzero, "passed stage with nonzero exit")

    failed_stage_zero = copy.deepcopy(report)
    failed_stage_zero["stages"][0]["status"] = "failed"
    assert_invalid(report_validator, failed_stage_zero, "failed stage with zero exit")

    blocked_stage_wrong_exit = copy.deepcopy(report)
    blocked_stage_wrong_exit["stages"][0]["status"] = "blocked"
    blocked_stage_wrong_exit["stages"][0]["exit_code"] = 1
    assert_invalid(report_validator, blocked_stage_wrong_exit, "blocked stage with non-three exit")

    mismatched_network_alias = copy.deepcopy(report)
    mismatched_network_alias["stages"][0]["network_allowed"] = not (
        mismatched_network_alias["stages"][0]["network_contract_allows"]
    )
    assert_invalid(report_validator, mismatched_network_alias, "mismatched network aliases")

    escaping_artifact = copy.deepcopy(report)
    escaping_artifact["artifacts"] = [{
        "name": "escape",
        "path": "runs/../outside",
        "digest": "sha256:" + "a" * 64,
        "size_bytes": 1,
    }]
    assert_invalid(report_validator, escaping_artifact, "escaping report artifact path")

    bad_latest_digest = copy.deepcopy(latest)
    bad_latest_digest["report_digest"] = "sha256:abcd"
    assert_invalid(latest_validator, bad_latest_digest, "invalid latest report digest")

    mismatched_latest = copy.deepcopy(latest)
    mismatched_latest["report_path"] = "runs/20260813T010001Z-verify.Other/report.json"
    latest_validator.validate(mismatched_latest)
    try:
        assert_latest_semantics(mismatched_latest)
    except AssertionError:
        pass
    else:
        raise AssertionError("negative semantic fixture unexpectedly passed: mismatched latest run")

    malformed_latest_time = copy.deepcopy(latest)
    malformed_latest_time["published_at"] = "yesterday"
    assert_invalid(latest_validator, malformed_latest_time, "invalid latest timestamp")

    pointer_wrong_digest = copy.deepcopy(latest)
    pointer_wrong_digest["report_digest"] = "sha256:" + "0" * 64
    latest_validator.validate(pointer_wrong_digest)
    assert_semantically_invalid(
        lambda: assert_report_pointer_pair(report, pointer_wrong_digest, REPORT_EXAMPLE_PATH),
        "pointer digest does not identify report bytes",
    )

    pointer_wrong_size = copy.deepcopy(latest)
    pointer_wrong_size["report_size_bytes"] += 1
    latest_validator.validate(pointer_wrong_size)
    assert_semantically_invalid(
        lambda: assert_report_pointer_pair(report, pointer_wrong_size, REPORT_EXAMPLE_PATH),
        "pointer size does not identify report bytes",
    )

    mismatched_case_id = copy.deepcopy(result)
    mismatched_case_id["cases"][0]["case_id"] = "another-case"
    result_validator.validate(mismatched_case_id)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, mismatched_case_id),
        "result case ID differs from descriptor",
    )

    mismatched_trial_count = copy.deepcopy(result)
    mismatched_trial_count["cases"][0]["trials"].pop()
    result_validator.validate(mismatched_trial_count)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, mismatched_trial_count),
        "result trial count differs from descriptor",
    )

    mismatched_grader = copy.deepcopy(result)
    mismatched_grader["cases"][0]["trials"][0]["graders"][0]["grader_id"] = "other-grader"
    result_validator.validate(mismatched_grader)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, mismatched_grader),
        "trial grader differs from descriptor",
    )

    required_grader_failure_in_passed_trial = copy.deepcopy(result)
    grader = required_grader_failure_in_passed_trial["cases"][0]["trials"][0]["graders"][0]
    grader["status"] = "failed"
    grader["score"] = 0
    result_validator.validate(required_grader_failure_in_passed_trial)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, required_grader_failure_in_passed_trial),
        "passed trial contains failed required grader",
    )

    weighted_threshold_mismatch = copy.deepcopy(result)
    weighted_threshold_mismatch["cases"][0]["trials"][0]["graders"][0]["score"] = 0
    result_validator.validate(weighted_threshold_mismatch)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, weighted_threshold_mismatch),
        "trial status differs from weighted threshold",
    )

    mismatched_summary = copy.deepcopy(result)
    mismatched_summary["cases"][0]["summary"]["passed"] = 2
    result_validator.validate(mismatched_summary)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, mismatched_summary),
        "summary differs from trials",
    )

    unresolved_artifact_ref = copy.deepcopy(result)
    unresolved_artifact_ref["cases"][0]["trials"][0]["graders"][0]["artifact"] = "missing.log"
    result_validator.validate(unresolved_artifact_ref)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, unresolved_artifact_ref),
        "grader artifact reference does not resolve",
    )

    escaped_agent_artifact = copy.deepcopy(result)
    escaped_agent_artifact["cases"][0]["trials"][0]["artifacts"]["diff.patch"]["path"] = (
        "../diff.patch"
    )
    result_validator.validate(escaped_agent_artifact)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, escaped_agent_artifact),
        "agent artifact path escapes result directory",
    )

    mismatched_aggregate_metric = copy.deepcopy(result)
    mismatched_aggregate_metric["cases"][0]["metrics"][-1]["value"] += 1
    result_validator.validate(mismatched_aggregate_metric)
    assert_semantically_invalid(
        lambda: assert_case_result_pair(case, mismatched_aggregate_metric),
        "aggregate metric differs from trials",
    )

    print("ok: evidence schemas and examples conform")
    return 0


if __name__ == "__main__":
    sys.exit(main())
