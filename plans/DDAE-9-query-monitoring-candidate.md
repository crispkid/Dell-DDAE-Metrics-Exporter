# DDAE-9：查詢監控規格與實作計畫（待核准）

## Purpose and Observable Outcome

新增預設關閉的 Query monitoring pipeline：Prometheus 顯示叢集執行中／排隊
查詢數及已觀察查詢的耗時分布；Kafka 提供逐筆查詢的提交者、來源、狀態及耗時。
保留既有 resource、alert、serviceability log 功能及已完成的 parser 修正。
不恢復 Portable，不提交、取消或修改任何業務查詢。

這是新的 DDAE-9 功能範圍，建議規格版本 2.5.0。使用者已授權先測試、成功後
新增功能；本文件是供該功能進一步核准的具體需求及計畫，尚未取得本文件的
需求／計畫核准，亦未修改實作或切換 HARNESS/ACTIVE_CHANGE。

## Context and Constraints

2026-09-09 已授權 Demo 唯讀探索（僅保留去識別化結構證據）：

- Engine 回報版本 479-e.4，Insights 工作階段登入成功。
- 預設 public 角色 allQueries=false；帳號原已具備 sysadmin。以
  `X-Trino-Role: system=ROLE{sysadmin}` 發出 GET 後 allQueries=true。
- GET /ui/api/insights/overview/queries：HTTP 200，300 個時間樣本；最新樣本
  runningQueries=0、queuedQueries=0、failedQueries=0。
- GET /ui/api/insights/history/queries：HTTP 200，1000 筆；包含 queryId、state、
  user、createTime、cpuTime、elapsedTime。1000 是本次回傳筆數，尚未證明 API
  的固定上限、分頁或完整保留期間。
- GET /ui/api/insights/history/queries/{id}：HTTP 200；確認 user、source、
  submissionTime、completionTime、elapsedTime、queuedTime、executionTime、
  cpuTime 等欄位。單筆範例完成，總耗時 8 ms、排隊 1 ms、執行 7 ms。
- GET /ui/api/query：HTTP 200，100 筆 recent query；sessionUser、sessionSource
  與 queryStats 可讀，duration 以 59.25ms、441.18us 等字串回傳。
- 相同 Management API client 的 Bearer token 與 HTTP Basic 存取 Insights
  均 HTTP 401。已驗證可用的是經身分提供者登入後的 cookie session。
- TLS CA 尚未修復，本次顯式略過驗證；不是正式環境 TLS 或整合驗收證據。
- 未產生測試 SQL；非零並行查詢、session 過期、持續大量查詢與 Kafka/OpenSearch
  的實際整合尚未驗證。

證據：output/query-investigation/findings.json。前端 API 路徑來自此 appliance
提供的前端 bundle。這是版本限定的 Insights UI API 相容性，不能宣稱為 Dell
Management API 的官方穩定契約，也不能假設任意 DDAE/Starburst 版本都相容。

## Approved Plan Basis

以下為待核准的 basis；標記本身不代表已核准。

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

### Requirements and acceptance criteria

| Requirement | Acceptance | 規範 |
|---|---|---|
| REQ-DDAE-9-001 | AC-DDAE-9-001 | Query pipeline MUST 預設關閉；啟用後獨立設定 Insights HTTPS origin、明確 auth HTTPS origin/realm、角色與獨立密碼檔。MUST NOT 將 Management API client secret 自動送到 Insights。關閉時不驗證、不建立或使用此 pipeline 的認證、Kafka 或 state 資源。 |
| REQ-DDAE-9-002 | AC-DDAE-9-002 | MUST 使用觀察到的 OIDC 網頁登入與記憶體 cookie jar；登入 redirect 最多 8 次，只准設定的兩個 origin 及限定 auth/callback 路徑。密碼 POST 只能送到指定 auth origin 的 realm login-actions/authenticate；拒絕其他 host、HTTP、未知表單與 MFA/額外流程，不自動降級。一般業務請求只能 GET allowlist。401 最多重新登入一次；403 不以其他角色繞過。全程 TLS 預設驗證，沿用明確雙重 diagnostic opt-in，且與 Kafka/Management transport 隔離。 |
| REQ-DDAE-9-003 | AC-DDAE-9-003 | 每次 collection MUST 以設定角色確認 cluster/info.allQueries=true，否則不可將局部資料標示為叢集統計。角色只在 request header 選擇，不修改授權。受支援 schema 以 479-e.4 驗證；缺欄位、錯誤型別、HTML、截斷 JSON、重複 identity 或不合法時間 MUST 回報可辨識失敗。 |
| REQ-DDAE-9-004 | AC-DDAE-9-004 | running/queued MUST 取 overview 最新有效 upstream time 樣本，使用 gauge；樣本過期或沒有樣本不可輸出零或延長其新鮮度。MUST NOT 把 failedQueries 樣本或歷史清單筆數當作累積計數。背景輪詢不重疊；scrape 不觸發 API。 |
| REQ-DDAE-9-005 | AC-DDAE-9-005 | MUST 用 bounded history list + bounded detail GET 蒐集已觀察到的逐筆查詢；以 source_instance/query_id 識別。API 的 list 完整性與保留期限未經證明時，history coverage MUST 標為 unknown，不承諾完整稽核，也不依缺席推論查詢已完成或刪除。detail 未取得前不可捏造 user 或 duration。 |
| REQ-DDAE-9-006 | AC-DDAE-9-006 | Kafka query schema v1 MUST 只包含 source_instance、query_id、user、source、固定狀態、提交/完成/觀察時間及 elapsed/queued/execution/cpu 秒數；缺少 optional 欄位省略。MUST NOT 包含 SQL、prepared SQL、IP、principal、session properties、catalog/table metadata、任意錯誤文字、憑證或 cookie。user 與 source 是 detail data，不能作為 Prometheus labels。 |
| REQ-DDAE-9-007 | AC-DDAE-9-007 | Query detail Kafka MUST 使用獨立 topic、query-events.db、publisher 與 bounded durable outbox；預設 event 輸出關閉。topic 不可與 alerts/logs 重複。穩定 key、at-least-once、下游 idempotent upsert；重複 poll/重啟不得重複計入已觀察完成數/耗時。入列、dedup checkpoint 與 aggregate 更新需同一 transaction。滿額、寫入/送出失敗不可靜默丟失，MUST 顯示 unhealthy/backpressure。 |
| REQ-DDAE-9-008 | AC-DDAE-9-008 | MUST 提供有界 metrics、freshness、scope、history coverage、detail failure、outbox 與 publish 狀態。持久化已觀察 terminal query 的計數與 histogram aggregate，明確命名 observed，不能宣称代表所有查詢。未知 state 映射 unknown，不當成功或失敗終態。整體 health/readiness 需反映已啟用 Query pipeline，保留其他 pipeline 的 partial failure 語意。 |
| REQ-DDAE-9-009 | AC-DDAE-9-009 | MUST 更新 README、runbook、YAML/env precedence、deployment examples、NetworkPolicy 對 Insights/IdP 的設定需求與 SDD traceability；正常建置不含 Portable。測試 fixture 僅用合成資料。實測結果須區分功能、TLS、非零負載、Kafka 與完整 Harness gate，不聲稱未執行的項目通過。 |

### Technical design

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

### Requirement-linked tasks and verification basis

| Task | Requirements | Tests / expected evidence | Harness stage |
|---|---|---|---|
| TASK-DDAE-9-001 | 001,002 | TEST-DDAE-9-001：disabled/query-only、secret precedence、TLS兩端隔離、evil redirect/form、expired session、403與MFA fail-closed | test,security |
| TASK-DDAE-9-002 | 003,004 | TEST-DDAE-9-002：scope false、empty/stale/future samples、bad JSON、負數/型別錯誤、nonzero synthetic sample、no scrape fan-out、cancel/deadline | test |
| TASK-DDAE-9-003 | 005,006 | TEST-DDAE-9-003：ID/path validation、list cap、重複ID、missing/null/user/duration、milliseconds precision、禁止SQL/secret欄位 | test,security |
| TASK-DDAE-9-004 | 007,008 | TEST-DDAE-9-004：same poll/restart dedup、state transitions、retention floor、crash atomicity、outbox滿額、Kafka failure、stable upsert key、fixed labels、health/partial failure | test,coverage |
| TASK-DDAE-9-005 | 009 | TEST-DDAE-9-005：README/schema/examples、deployment consistency、正常build、既有各pipeline回歸 | lint,test,build |
| TASK-DDAE-9-006 | 001–009 | TEST-DDAE-9-006：授權Demo上新 exporter 至少兩個週期，比對 upstream sample、query user/duration 的記憶體值，不保留實際SQL；Kafka需獨立授權測試broker/topic才執行 | integration,e2e |

正式 transcription 時每個 acceptance test 指定單一 owning requirement 並拆分
必要 test IDs；不得憑本次探索將實作測試寫成通過。先寫 synthetic failing tests，
再實作、targeted test、完整 verify；安全/認證改動依 CODE_REVIEW 取得所需的
獨立審查，不捏造 reviewer。

### Compatibility and recovery

query.enabled 預設false；既有行為與 metric 名稱不變。只增加獨立 db/topic。
關閉功能即可停止 poll/event producer，保留 bounded state 以便復原；回退既有
binary 不讀寫 query db。未知未來 DB schema 必須拒絕開啟，不降版覆寫資料。
部署範例使用 placeholder hosts/secret files，不記錄真實主機、帳號或密碼。

<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- 唯讀探索完成，確定可取得並行數、query user 與 duration；實作尚未開始。
- 全部新的 requirement/acceptance 狀態為 Planned；無 DDAE-9 release claim。

## Surprises and Discoveries

public role 只回傳自身範圍的空清單，而 sysadmin 提供全域資料；使用帳號名稱
推論權限會造成假零。Management API Bearer token 不可直接沿用。

## Decision Log

建議使用已驗證的版本限定 session adapter（可立即實作，但 SSO/前端升級可能
需更新）。替代方案是等待 Dell 支援的 machine-to-machine query API/事件來源
（契約較穩定，但需額外設定與權限，目前未驗證）。不採用人工固定 cookie，
以免 session 過期後 Exporter 永久停止。大量查詢完整稽核需另驗證 pagination
或 query event listener，不能以這次1000筆回應推論已具備。

決策者：本次對話的 repository user；推薦以上 session adapter、bounded
observed query 功能，完整稽核另案，原因是滿足已驗證資訊且誠實呈現完整性限制。
待使用者明確核准本文件後，才 transcription 到正式 SDD 文件並實作。

## Recovery and Idempotence

探索只執行登入與 GET；沒有送出SQL、kill query、改角色或部署。沒有持久化
credentials、cookies、query IDs、users或SQL。現有工作樹修改屬此前 parser與
Portable removal，不應因本案重置。

## Outcomes and Retrospective

API 能力驗證成功；本候選需求/實作計畫待核准，尚未加入Exporter。