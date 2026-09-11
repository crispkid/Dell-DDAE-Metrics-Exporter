# DDAE-11 Query-only Kafka SASL credential loading

## Purpose and Observable Outcome

Correct query-only Kafka SASL credential loading under Specification 2.7.0.

## Context and Constraints

The user replied "Ok" on 2026-09-11 to the explicit approval request for
[the candidate](DDAE-11-query-kafka-sasl-candidate.md), SHA256
7f213471d37ef90860b890f7ea76cb0dcd313554aab427122ffe2330cde884f8.
This approves the candidate requirements, implementation and test plan, including
new Change ID DDAE-11, local scope and preserved external gates.
Candidate status text is an immutable pre-approval snapshot; this record carries
the subsequent approval. No human name or independently typed digest is inferred.
Canonical heading normalization and expanded requirement IDs are administrative
transcription only. Historical DDAE-10 and earlier records remain unchanged.

## Approved Plan Basis

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

### Requirement Coverage

REQ-DDAE-11-001, REQ-DDAE-11-002 and REQ-DDAE-11-003 map to their matching AC and TEST IDs below.

### Technical Design

只在 `internal/config/config.go` 的 SASL credential 載入條件加入
`|| cfg.Query.Events`。沿用 `requiredText`、`loadSecret`、`layeredLookup`；
不新增 loader、public key、依賴或 producer 的認證實作。

主要測試放在 `internal/config/kafka_sasl_test.go`，使用既有 config test
helpers 與合成資料。YAML 案例必須經過 `decodeYAML` 和 `layeredLookup`；
隔離案例以記錄讀取次數或會失敗的 reader 證明未讀取 Kafka 密碼檔。
必要的真實暫存檔只用 t.TempDir 與合成 secret，不讀取操作者環境中的帳密。

### Task Breakdown

| Task | Requirement / acceptance | Planned executable evidence | Harness stage |
|---|---|---|---|
| TASK-DDAE-11-001 | REQ-DDAE-11-001 | TEST-DDAE-11-001：新增 `TestQueryEventsKafkaSASLCredentials`、`TestQueryEventsKafkaSASLYAMLPrecedence`；先記錄 RC4 缺陷造成的失敗，再加入條件修正 | test |
| TASK-DDAE-11-002 | REQ-DDAE-11-002 | TEST-DDAE-11-002：新增 `TestQueryEventsKafkaSASLRejectsInvalidCredentials`，含 missing/empty/NUL/UTF-8、file boundary、reader failure、conflict 與錯誤 redaction | test |
| TASK-DDAE-11-003 | REQ-DDAE-11-003 | TEST-DDAE-11-003：新增 `TestKafkaSASLPipelineSelection`，驗證各 pipeline 組合與不讀檔隔離；執行現有 config、Kafka、app 與 TLS 契約測試 | test |
| TASK-DDAE-11-004 | REQ-DDAE-11-001, REQ-DDAE-11-002, REQ-DDAE-11-003 | 更新 canonical SDD records、雙語 README 的 SASL 適用條件、runbook 與 README Review Notes 的修正狀態；驗證文件契約，保留完整 gate 的實際結果與 diff review | test, security, build |

### Verification Basis

核准後，先轉錄本提案至 SPECIFICATION.md、SPEC_CHANGELOG.md、
DEVELOPMENT_PLAN.md、TEST_PLAN.md、TRACEABILITY.md、plans/DDAE-11.md，
更新 PROJECT.md 的目前變更與 HARNESS/ACTIVE_CHANGE，計算 canonical digests，
保留本提案及實際核准證據。純轉錄不宣稱使用者逐字輸入過 digest。
若轉錄新增行為或改變本方案，必須重新核准。
新增 trace rows 起始為 Planned，產品修改前通過 governance:approved。

先執行新增回歸測試，保存修正前失敗；再執行最小修正，確認相同測試通過。
迭代執行 `go test ./internal/config ./internal/kafka ./internal/app ./internal/contract`。
最終執行 `./HARNESS/harness.sh verify`、`./HARNESS/harness.sh security`、
`./HARNESS/harness.sh sdd:check` 與 `git diff --check`，如實記錄各項結果。
不能以 config 單元測試宣稱真實 Kafka SASL 登入成功，也不能把 blocked gate
或尚無獨立 reviewer 的狀態記為完整交付／Verified。

此修改涉及 secret handling，依 CODE_REVIEW.md 仍需非實作者的獨立審查。
若尚無 reviewer，回報待審，不虛構通過，也不推定授權建立其他 agent/task。
已知 security policy 的兩個 TLS target 限制，以及外部 integration/E2E
等 inherited gates，不屬本次修正，不修改規則來繞過失敗。

### Compatibility and Recovery

正確的既有 query-only SASL 設定在重新建置、替換執行檔並重新啟動後生效。
缺少／無效帳密由較晚的 broker 認證失敗改為啟動設定錯誤；操作者補齊既有
設定即可，沒有新增設定名稱或資料庫 migration。
rollback 使用既有執行檔，保留設定與 state；會重新帶回 query-only SASL 缺陷。
不刪除 outbox、不透過關閉 TLS 或 SASL 作為 workaround。
本提案只授權本機程式／測試／文件修正；不包含 commit、push、tag 或 release。

<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- 2026-09-11: User approval recorded; canonical records prepared before code changes.
- 2026-09-11: `governance:approved` passed with the canonical specification,
  plan-basis and test-plan digests before adding tests or changing product code.
- Added the four candidate regression functions. Before the fix, all four top-level
  tests failed: 87 failing subtests and 48 passing subtests. Query-only credentials,
  required validation and query-only publisher selection failed as predicted;
  existing direct/file conflicts and other publisher combinations still passed.
  Retained synthetic results: `output/_ddae11-evidence/red.json` (ignored).
- Added only `cfg.Query.Events` to the existing SASL credential loading condition.
  The same targeted regression command then passed. Full verification is in progress.
- Initial JSON/broader test attempts were blocked by sandbox access to Go build
  cache. The red evidence was rerun with the required access; sandbox failures
  are not counted as behavioral failures or successful verification.

## Surprises and Discoveries

The defect occurs with query events and no alerts/logs; resources may be on or off.
Producer construction alone does not prove broker authentication.

## Decision Log

The user approved candidate option A: one predicate correction, shared loader,
requirement-linked regression tests and documentation. Option B (reading secrets
again in each producer) was rejected due to duplicated secret/precedence logic.
Independent review is required but no reviewer has yet been recorded.

## Recovery and Idempotence

No state writes or migration. Keep existing settings/state when replacing the
binary. Rolling back returns the defect; never remove pending outbox data.

## Outcomes and Retrospective

The local one-line fix, all candidate regression cases and adjacent documentation
are implemented. Active trace rows are Implemented, not Verified. At the initial
handoff, no commit, push, tag, release, real Kafka authentication or external
deployment was performed. Subsequent user-authorized Docker Kafka validation
is recorded separately below; release and full handoff gates remain unchanged.

### Verification and Review (2026-09-11)

- The same regression suite changed from 87 failing / 48 passing subtests to
  all 135 subtests and four top-level tests passing. Synthetic result summaries:
  `output/_ddae11-evidence/red.json` and `output/_ddae11-evidence/green.json`.
- `go test ./internal/config ./internal/kafka ./internal/app ./internal/contract`
  passed after granting Go cache and local-listener access.
- Initial full verify passed doctor, governance, lint, repository-wide race tests,
  coverage (81.8%, threshold 80%) and build. Integration returned blocked because
  authorized runner inputs were not enabled; overall verify failed (exit 3).
  Report: `test-results/harness/runs/20260911T005436Z-verify.7CHVi3/report.json`.
  E2E, security, CI-policy and supply-chain did not run inside that stopped verify.
- Separately executed security: its vet/selected tests passed, then the inherited
  exact-two-guarded-TLS-target rule failed (exit 1). No policy was weakened and
  the vulnerability-scanner step after that rule did not run.
- Separately executed sdd:check: current approval digests passed, then handoff
  validation rejected non-Verified trace rows (exit 1). This is not duplicate rows.
- Self-review followed CODE_REVIEW.md: verified the one-line predicate, existing
  loader/producer data flow, exact secret-value assertions, no-read profiles,
  failure redaction and bilingual configuration/metric names. No remaining
  scoped correctness findings. This is not the required independent review.
- Final verify after documentation review and trace updates again passed doctor,
  governance, lint, all repository race tests, 81.8% coverage and build; integration
  remained blocked (exit 3). Final report:
  `test-results/harness/runs/20260911T005633Z-verify.sDKRN1/report.json`.
- Final uncached `go test ./internal/config ./internal/contract -count=1` passed,
  including bilingual README code-block/configuration contracts. Final sdd:check
  still correctly rejected Implemented rather than Verified trace status.
- `git diff --check` passed. The approved candidate hash and canonical DDAE-11
  approval digests are unchanged. DDAE-10 specification/test-plan section bytes
  still match their recorded historical SHA256 identities; earlier sections,
  approval records and requirement statuses were preserved.
- No production secrets, new dependencies, generated tracked artifacts or
  unrelated implementation changes were introduced. These final outcome notes
  are the only source edit after the final verify; the approved basis is unchanged.

### Remaining Handoff Requirements

An independent reviewer and authorized real integration/E2E evidence are still
required. The inherited security-policy mismatch and subsequent release gates
remain outside this approved correction. Neither the fix nor local test success
certifies the published RC4 or a new release. Rebuild/replace/restart the exporter
to use the local correction; the existing RC4 release assets are unchanged.

### Supplemental Docker Kafka Validation (2026-09-11)

The user explicitly requested local Docker Kafka installation and real testing
in the current conversation. This authorizes isolated runtime validation, not
new product behavior, a changed approval basis, formal full-chain integration
acceptance, independent review, commit or release.

- Built the actual native exporter and ran it against native Kafka 4.3.1 in a
  dedicated loopback-published Docker environment. DDAE Insights/OIDC remained
  a synthetic HTTPS fixture; all credentials and certificates were test-only.
- All 19 cases met their expected outcomes. PLAIN, SCRAM-SHA-256 and
  SCRAM-SHA-512 each delivered through YAML password files, direct password
  environment variables and password-file environment variables. mTLS also
  delivered. An independent consumer read and validated 10 query records.
- The RC4 binary reproduced query-only SASL non-delivery for all three
  mechanisms with correct credentials. The fixed binary delivered with those
  credentials. Wrong passwords, an untrusted CA and missing mTLS credentials
  prevented delivery while preserving a pending event; a missing SASL password
  was rejected at startup.
- Broker-side authentication failures confirmed the wrong-password cause.
  The publisher sometimes reported kafka_timeout before kafka_auth because
  its publish deadline expired during retries. This observation was retained;
  no product error classification or retry behavior was changed.
- Early Docker networking and Snappy tmpfs permission failures were diagnosed
  and recorded separately, not counted as successful tests. The corrected
  environment then passed the entire 19-case matrix in one run.
- All test containers/networks and their disposable broker data were removed;
  the image and private local evidence were retained. Other Docker services
  were left untouched. No new product source or formal test-plan changes.
- Detailed matrix, environment, binary/helper/result SHA-256 identities and
  boundaries: [Docker Kafka validation report](../docs/kafka-auth-validation-2026-09-11.md).
  Machine-readable local results: `output/_kafka-sasl-2821538818/results.json`.

This supplemental evidence does not replace the authorized non-production DDAE
integration/E2E inputs or the required independent review. Approved plan-basis,
specification/test-plan digests and Implemented trace status remain unchanged.

### Post-validation Repository Checks (2026-09-11)

- `./HARNESS/harness.sh verify` passed doctor, instruction checks, approval
  digests, lint, the repository-wide race-test command, coverage (81.8%) and
  build. Existing test cache entries were used. Integration remained blocked
  because the authorized non-production DDAE runner was not enabled; overall
  exit was 3. Report:
  `test-results/harness/runs/20260911T011323Z-verify.LQcGxz/report.json`.
- The stopped verify did not run E2E, security, CI-policy or supply-chain.
  Separately run security passed its vet/selected tests, then failed the
  unchanged exact-two-TLS-target rule (exit 1); the subsequent vulnerability
  scan did not run.
- Separately run `sdd:check` confirmed approval digests and then failed the
  handoff status requirement (exit 1), because trace rows remain Implemented.
- Uncached `go test ./internal/config ./internal/kafka ./internal/app
  ./internal/contract -count=1` passed after the validation documentation edits.
- Final review confirmed no additional product-code changes, all test helpers
  and private artifacts ignored, the candidate hash unchanged, and no labeled
  DDAE-11 Docker containers or networks remaining. `git diff --check` passed.
  These outcome notes are outside the approved plan basis; independent review
  and the remaining release gates are still outstanding.
