# DDAE-9 Query monitoring

## Purpose and Observable Outcome

Implement approved candidate SHA256 f0dbc5ac07d9e379c25dea6cd6b344bfc8aadec87f98ddacc189a06065fb067f.

## Context and Constraints

Candidate remains immutable; user replied OK after requirement/plan approval
request and clarification of queue, Prometheus and Kafka fields. No name inferred.

## Approved Plan Basis

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

### Requirement Coverage

REQ-DDAE-9-001 through REQ-DDAE-9-009 and matching AC IDs implement the exact
approved candidate. Tables below administratively split tests by owning requirement.

### Technical Design


新增 internal/queryclient（schema、固定 GET routes、OIDC session）與
internal/queries（scheduler、normalization）、internal/querystate（獨立 bbolt
狀態/outbox）、query publisher；沿用現有 Kafka transport/TLS 實作，避免改變
alert/log event schema。整合 app lifecycle、snapshot、metrics、server health、
config/YAML/env，query-only 模式不得要求 Management API 認證。

固定業務 GET routes：/ui/api/insights/cluster/info、
/ui/api/insights/overview/queries、/ui/api/insights/history/queries、
/ui/api/insights/history/queries/{validated-id}。不跟隨回應中的 self URL。
近期 /ui/api/query 已探索，但本次不作為完整並行數來源，也不新增第二套 duration
schema：並行數使用 overview、逐筆 detail 使用 Insights history。

ID 僅接受 1–256 bytes 的 ASCII 字母、數字、底線、連字號；URL escape path
segment；任何不相符 ID 回報 invalid，不帶入任意 URL。JSON duration 嚴格接受
非負整數 milliseconds，轉為 seconds；缺值與 0 分開。UTC RFC3339 timestamp
嚴格驗證；拒絕不合理未來樣本（超過 5 秒），elapsed 不能以 CPU time 取代。
同一 history list 內重複 ID 視為完整性錯誤。

預設 interval=30s、request_timeout=5s、cycle_timeout=20s、stale_after=90s、
response_max_bytes=16MiB（上限64MiB）、detail.max_per_cycle=100、concurrency=4
（1–32）、max_history_records=1000（1–10000）、retry_max=2，重试仅限 GET
的暫時性錯誤且受 cycle deadline 約束。overview 與 details 狀態獨立，detail
失敗不得抹去有效的 overview；但 enabled pipeline readiness 必須反映失敗。

Checkpoint 預設上限100000、retention=720h；過期 checkpoint 再出現的終態 record
不得重新計入 aggregate：以持久化 terminal watermark/retention floor 限制
可接納時間，晚到且超界的 record 回報 gap，不假裝已完整處理。達到列表上限、
超過本地預算或 history continuity 未證明時，coverage 保持 unknown/limited。
不使用未驗證的 pagination 參數來宣稱掃描完整。

Outbox 預設256MiB/10000事件，单事件上限64KiB；user/source 各上限1024 UTF-8
bytes，不截斷身份造成碰撞。Kafka event export 關閉時仍需 query state 以保留
observed counters；不建立 Kafka producer。安全例外只涵蓋選定 query detail
欄位，禁止原始 payload logging/capture。

### Metric mapping

所有 metric HELP 必須說明 upstream sample 或 observed coverage；下列皆無 user、
query_id、source、SQL 等 unbounded labels。固定 state label 不超過
finished/failed/canceled/unknown；Histogram state 僅前三個 terminal 狀態。

| Prometheus name | Type / unit | Source and behavior |
|---|---|---|
| ddae_queries_running | gauge / queries | latest overview.metric.runningQueries；非負整數；stale 時省略 |
| ddae_queries_queued | gauge / queries | latest overview.metric.queuedQueries；非負整數；stale 時省略 |
| ddae_query_sample_timestamp_seconds | gauge / Unix seconds | overview.time；不使用 HTTP fetch time 取代 |
| ddae_query_collection_success | gauge / boolean | 本次 overview/schema/scope 檢查成功 |
| ddae_query_detail_collection_success | gauge / boolean | 本次 bounded detail collection 無失敗 |
| ddae_query_scope_all | gauge / boolean | allQueries=true 且本次 scope 檢查有效；失敗為0 |
| ddae_query_history_complete | gauge / boolean | 本次 list 的完整性有證據才為1；本版 UI API 未證明時為0 |
| ddae_queries_observed_completed_total{state} | persistent counter / queries | deduplicated terminal records，非叢集完整總數 |
| ddae_query_observed_elapsed_seconds{state} | persistent histogram / seconds | 終態 elapsedTime/1000，只記一次；buckets .01,.05,.1,.5,1,5,10,30,60,300,+Inf |
| ddae_query_observed_execution_seconds{state} | persistent histogram / seconds | 終態 executionTime/1000，缺值省略，不填0 |
| ddae_query_observed_queued_seconds{state} | persistent histogram / seconds | 終態 queuedTime/1000，缺值省略，不填0 |
| ddae_query_events_pending | gauge / events | durable outbox pending count |
| ddae_query_event_publish_success | gauge / boolean | query publisher 狀態；未啟用 event export 時省略 |

新增 query mode 的 series budget 固定 <200；不得把 sampled gauges 單純積分成
準確 throughput，也不得用 truncated history 推算平均延遲/P95 的完整母體。


### Task Breakdown

| Task | Requirements | Work |
|---|---|---|
| TASK-DDAE-9-001 | REQ-DDAE-9-001 | Implement requirement and TEST-DDAE-9-001; preserve approved candidate boundaries. |
| TASK-DDAE-9-002 | REQ-DDAE-9-002 | Implement requirement and TEST-DDAE-9-002; preserve approved candidate boundaries. |
| TASK-DDAE-9-003 | REQ-DDAE-9-003 | Implement requirement and TEST-DDAE-9-003; preserve approved candidate boundaries. |
| TASK-DDAE-9-004 | REQ-DDAE-9-004 | Implement requirement and TEST-DDAE-9-004; preserve approved candidate boundaries. |
| TASK-DDAE-9-005 | REQ-DDAE-9-005 | Implement requirement and TEST-DDAE-9-005; preserve approved candidate boundaries. |
| TASK-DDAE-9-006 | REQ-DDAE-9-006 | Implement requirement and TEST-DDAE-9-006; preserve approved candidate boundaries. |
| TASK-DDAE-9-007 | REQ-DDAE-9-007 | Implement requirement and TEST-DDAE-9-007; preserve approved candidate boundaries. |
| TASK-DDAE-9-008 | REQ-DDAE-9-008 | Implement requirement and TEST-DDAE-9-008; preserve approved candidate boundaries. |
| TASK-DDAE-9-009 | REQ-DDAE-9-009 | Implement requirement and TEST-DDAE-9-009; preserve approved candidate boundaries. |

### Verification Basis

Use the candidate's full failure matrix: redirects, TLS, scope, stale samples,
strict types, bounds, durations, privacy, history limits, transactional dedup,
restart, outbox backpressure, fixed labels, readiness and pipeline isolation.
Write acceptance tests before implementation; run test/security/build and full
Harness verify. Two authorized live diagnostic cycles cannot certify trusted TLS,
nonzero load, Kafka/OpenSearch or unsupported versions. Independent review remains
required by CODE_REVIEW and cannot be asserted by this implementing agent.

### Compatibility and Recovery

Default off, separate state/topic, rollback by disabling query pipeline; preserve
unknown future schema and old files. No SQL or external Kafka test without an
authorized isolated broker. Preserve all previous local work.

<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

2026-09-09: user approval recorded; canonical transcription and tests begin.

## Surprises and Discoveries

See approved candidate for live API schema, role and authentication findings.

## Decision Log

User approved candidate and clarified inclusion of queue and destination fields.
No behavioral departure from that candidate is authorized by this transcription.

## Recovery and Idempotence

Keep existing db untouched. New query db uses explicit schema version and isolation.

## Outcomes and Retrospective

Implementation and release verification pending.

## Implementation outcome — 2026-09-09

Implemented query configuration/YAML/env, bounded session client, strict parsers,
separate transactional query state/outbox, background collection, Prometheus
observed aggregates, optional Kafka publisher, readiness and lifecycle integration.
Updated English/Traditional Chinese README, query guide, deployment example and
security-stage query tests. No dependency or lockfile changes, Portable restoration,
commit, external Kafka publishing or deployment.

The live login includes the precise engine /ui/insights/login/proceed hop, discovered
when the initial fail-closed allowlist rejected it. Added that authentication-only
hop and a synthetic regression without broadening business GET routes. No approved
product behavior was changed. The first unsuccessful live attempt is retained.

Checks actually run:
- Query acceptance tests across config, queryclient, querystate, queries, app,
  server and contract passed. Covered custom CA success, untrusted TLS rejection,
  role scope, coalesced login/session renewal, hostile forms/redirects, response
  bounds, schema/time errors, missing values, privacy, queue histogram mapping,
  duplicate/restart/retention behavior, capacity, publisher failure/recovery,
  query-only cancellation and no scrape-triggered network calls.
- New query core packages statement coverage: 83.2% combined (queryclient 88.0%,
  querystate 77.7%, queries 82.7%). Evidence:
  output/ddae9/final-query-coverage.out. This is not whole-project coverage.
- Exporter build, go vet ./... and git diff --check passed. Changed-source
  supplied-credential scan found zero matches.
- Final live query-only exporter: two snapshots, scope/overview/detail success=1,
  running=0, queued=0, history_complete=0, readyz=200. Observed finished count
  increased40 to48, continuing persisted state from the earlier successful run.
  Elapsed, execution and queued histograms present. A separate live in-memory
  comparison confirmed user identity and milliseconds-to-seconds mapping into
  the sanitized event; no raw values were retained in the report. Evidence:
  output/ddae9/final-live-cycle-1.txt, final-live-cycle-2.txt, live-mapping.txt.
- Full go test ./... failed in config, contract, app and logstate with previously
  observed Windows absolute-path, Unix permission and checkout-newline issues.
  Evidence: output/ddae9/final-all-tests.jsonl. New query tests passed.
- Race tests could not execute: cgo/C compiler unavailable. Evidence:
  output/ddae9/race.txt. No race-clean claim.
- Required Harness verify failed at lint on untouched files' checkout formatting;
  later required stages were not executed by that run. Evidence:
  test-results/harness/runs/20260909T184616Z-verify.OBMJq8/report.json.
  Explicit current-run PATH excluded WindowsApps aliases so real Go/Python
  metadata collection worked; no Harness policy was weakened.

Verification limits: demo used explicit diagnostic TLS bypass; no real nonzero
concurrent load or session-expiry test was performed on the appliance. No authorized
Kafka/OpenSearch test target was provided, so real delivery is unverified. The
implementing agent reviewed the diff; CODE_REVIEW requires an independent reviewer
for authentication and Kafka schema/delivery, which remains outstanding. Trace rows
are Implemented, not Verified. No release readiness or complete earlier repository
specification audit is claimed.


### Configuration completeness follow-up (2026-09-09)

The user requested configuration coverage for all features. Under TASK-DDAE-9-009,
the systemd example and Kubernetes ConfigMap now list every supported YAML leaf.
This adds all query settings and the previously omitted optional Kafka mTLS paths.
The query-only example now lists every query setting. Default activation is
unchanged. Kubernetes includes an optional separate query credential mount;
systemd documents the four credential directives to enable when queries are used.
The query deployment guide and NetworkPolicy comments explain required host,
secret, trust and egress customization without inserting real credentials.

`go test ./internal/config -run
'TestCompleteConfigurationExamples|TestQueryExampleKubernetesMount|TestQuery'
-count=1` passed. Coverage here means schema-field presence, strict YAML decoding,
default and all-feature configuration loading with synthetic secret contents and
a host-native temporary STATE_DIR, plus Kubernetes query secret/mount consistency.
It is not evidence of production credential, CA, systemd or Kubernetes execution.
`git diff --check` passed. The implementing agent reviewed the current changes.

The follow-up Harness verify ran and failed at the existing repository Go
formatting lint gate; subsequent required stages did not run. Doctor and
governance checks passed. Evidence:
`test-results/harness/runs/20260909T185621Z-verify.b0JCJy/report.json`
and `output/ddae9/config-verify.txt`. No release or full verification claim.
