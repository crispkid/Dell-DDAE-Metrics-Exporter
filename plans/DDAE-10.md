# DDAE-10 Bounded history backfill

## Purpose and Observable Outcome

Implement the immutable approved candidate SHA256
90950a41c5efbdec938a90515de99427a4a68f3fa4e0f144c021ecf7e0e046d1.

## Context and Constraints

User replied OK to the explicit requirements/implementation-plan approval request.
Canonical transcription preserves the candidate contract; no personal name inferred.

## Approved Plan Basis

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->


### Requirement Coverage

REQ-DDAE-10-001 through REQ-DDAE-10-009 and their matching AC IDs are covered by the approved task/test matrix below.

### Technical Design

新增 internal/historyscan，僅實作 typed TimeWindow、bounded splitter、狀態機，
不懂認證或任意 URL。新增 internal/historystate 管理獨立進度 DB；
兩種來源 adapter 仍在 internal/ddae 與 internal/queryclient 使用受限參數。
回補 worker 與既有前景 manager 使用各自 deadline；query 回補使用獨立
client/session，避免登入互斥鎖拖住即時 overview。狀態／事件寫入仍共享
既有 durable store；CPU／DB 工作須有界且不能持有鎖等待網路。
同一次 scan 在固定時間窗口內由新到舊走訪，滿頁以 midpoint 切分，
秒精度向外取整，最小 1 秒仍滿時停止，不推測完整性。
每頁 IDs 先 durable 暫存，再按有限 detail budget 排程；完成頁處理後才
完成 leaf window。容量不夠时不 ack 原窗口，下輪安全重取。
前景與回補相同 source version 的寫入走同一 dedup 邊界。

### Task Breakdown

| Task | Requirements | Expected acceptance tests | Harness stage |
|---|---|---|---|
| TASK-DDAE-10-001 | REQ-DDAE-10-001 | TEST-DDAE-10-001: defaults、YAML/env 覆寫、主功能關閉、lookback/retention、所有界限 | test, security |
| TASK-DDAE-10-002 | REQ-DDAE-10-002 | TEST-DDAE-10-002: record HTTP methods/paths/encoding、注入／redirect/TLS 拒絕、無 raw body logging | test, security |
| TASK-DDAE-10-003 | REQ-DDAE-10-003 | TEST-DDAE-10-003: 合成 720 logs、1474 queries、重疊／毫秒、1000 ties、錯誤計數、範圍外資料 | test, coverage |
| TASK-DDAE-10-004 | REQ-DDAE-10-004, REQ-DDAE-10-008 | TEST-DDAE-10-004: 重啟、崩潰邊界、schema/source mismatch、佇列/bytes 上限、rollback | test, coverage |
| TASK-DDAE-10-005 | REQ-DDAE-10-005 | TEST-DDAE-10-005: 前景/背景相同 ID、舊 marker、一次終態 aggregate、filtered absence、missing marker | test, security |
| TASK-DDAE-10-006 | REQ-DDAE-10-006 | TEST-DDAE-10-006: 慢回補不擋 overview、request/detail budgets、無重疊、取消、scrape 無 network | test, coverage |
| TASK-DDAE-10-007 | REQ-DDAE-10-007 | TEST-DDAE-10-007: 指標精確名稱/labels、啟動/健康pending/blocked/stale、readyz 不偽造完整性 | test, coverage |
| TASK-DDAE-10-008 | REQ-DDAE-10-008 | TEST-DDAE-10-008: 斷線重試、query retention floor、lookback 改變、停用保留state | test, security |
| TASK-DDAE-10-009 | REQ-DDAE-10-009 | TEST-DDAE-10-009: 全部 config example 欄位、建置、文件／契約一致性、Demo 分段值比對 | test, build, integration |

### Verification Basis

先寫 requirement-linked 合成測試並记录新增能力在實作前不支援；
不修改舊 assertion 讓失敗消失。迭代使用限定 package 測試，最終跑
Harness verify 與 diff review，保留實際 failed/blocked gate。
Demo live comparison 固定截止、比對 ID 集合（僅在記憶體）、比對數量與事件
白名單，記錄來源持續變動及 TLS 例外；真 Kafka/E2E 仍需測試目的地。
既有 Win32／POSIX 路徑與權限失敗不歸此變更掩蓋。

### Compatibility and Recovery

新開關預設 false，保留現有事件 schema、topics、metrics 及原 state files。
新增 DB 不影響舊版本；先停用 backfill 再回滾可保留已排隊事件及 dedup。
未完成進度可重播。獨立 schema／checksum 錯誤停止該分支，不自動刪除 DB。
無新外部依賴，不需要資料庫帳密、Kafka schema 變更或新增外部服務。
規格核准後轉錄至 canonical SPECIFICATION/TEST_PLAN/DEVELOPMENT_PLAN/TRACEABILITY，
刷新相應 digests，再開始產品實作；保留 DDAE-9 的歷史核准紀錄。


<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

2026-09-09: approval recorded; canonical records transcribed before implementation.

## Surprises and Discoveries

See candidate and output/validation/pagination-report.zh-TW.md for live exploration.

## Decision Log

Current repository user approved the linked candidate by OK. Default-off, 24h
bounded recovery, separate progress DB and explicit readiness changes are approved.

## Recovery and Idempotence

Preserve existing databases; record events before progress acknowledgement.

## Outcomes and Retrospective

Local implementation completed; active trace rows are Implemented, not Verified.
No production/pre-production release is claimed. Independent review is outstanding.

### Implementation and review outcome (2026-09-09)

- Added opt-in YAML/environment configuration, strict bounds and complete examples.
- Added typed GET time bounds and independent background clients/sessions.
- Added fixed-window splitting, conservative boundary overlap, late-arrival rescans,
  durable pending IDs, source/window fingerprints and ack-after-event commit.
- Preserved existing Kafka schemas/topics and query terminal dedup; old source
  versions cannot roll back newer observations. Filtered logs never reconcile absence.
- Added bounded metrics and readiness integration; default-off creates no history DB.
- Updated README, runbook and docs/history-backfill.md with limits and recovery.
- Self-review followed CODE_REVIEW.md for routes, privacy, bounded work, persistence,
  metrics and lifecycle. This is not the required independent review.

### Actual current-run verification

- Complete non-race `go test -json ./...`: 17 packages pass, 4 fail;
  385 passed test/subtest entries and 21 failed entries. The same 21 inherited
  Windows STATE_DIR/POSIX-permission tests fail as before this change; one failing
  app test also leaks its database handle after Fatal. Full handoff remains failed.
- All eight directly exercised source/history/service/query/server packages pass
  their full local test runs. Targeted config/app/logstate and complete-example tests pass.
- historyscan coverage 83.0%; historystate coverage 81.6%. This is package evidence,
  not a claim that the full Harness coverage gate ran or passed.
- Final build and `go vet ./...` pass; `git diff --check` passes.
- `go test -race ./...` fails before execution: cgo is unavailable in this environment.
- Full Harness verify confirms exact SDD approval basis and stops at existing
  Go-format violations in other files. Subsequent required stages were not executed.
  Retained Harness report: test-results/harness/runs/20260909T200912Z-verify.3fC1nh/report.json.

### Authorized Demo functional evidence

- Both Demo hosts and diagnostic TLS bypass were explicitly authorized. Initial
  automatic approval rejection was resolved by a fresh explicit user authorization.
- Real production adapters/worker with 7-day test lookback and 3-detail budgets:
  two log cycles persisted 6 sanitized events; query state persisted 3 terminal
  observations; both branches healthy with pending work retained.
- A separate fixed recent-24-hour sweep with the allowed 1000-detail budget completed:
  21 log events, 1442 finished query observations, no pending windows or IDs.
  These are this sweep's counts, not the earlier 1474 historical exploration count.
  Query events were disabled; log events entered only the local durable outbox.
- Full exporter with resource + query monitoring and query backfill, two cycles:
  healthz/readyz 200, recovery success 1, blocked 0, pending records decreasing;
  running/queued overview and privacy-label checks remain independently valid.
  Query history_complete stays 0. The final binary was rechecked with fresh state.
- Raw responses/credentials are not included in reports. Real per-record runtime
  state remains only under ignored output/ddae10; do not commit generated state.

### Evidence and unresolved gates

Ignored output/ddae10 contains coverage-tests.txt, coverage.out,
targeted-tests-final.txt, all-tests-final.jsonl, test-summary.json,
final-test-exits.json, race.txt, vet.txt, verify.txt, live-probe.jsonl,
live-full.jsonl and live-exporter.json. These actual results do not replace
Harness stages. No Kafka broker/OpenSearch delivery was exercised. Source retention
configuration, trusted Demo TLS, full Harness pass and an independent reviewer are
still required for release. Synthetic tests and non-production Demo results must
not be described as production observations.

