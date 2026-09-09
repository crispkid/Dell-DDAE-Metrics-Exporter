# DDAE-10：日誌及查詢歷史回補（需求、設計與驗證候選稿）

## Purpose and Observable Outcome

新增有界、可恢復的時間區間回補，讓 Exporter 能在既有單次清單上限外繼續
取得 Serviceability Logs 及 query history。保留即時 overview、資源與 alert
行為，不把回補進度或 HTTP 200 宣稱為永久稽核完整性。

候選 Change ID：DDAE-10；候選規格版本：2.6.0。這是 DDAE-9 以外的獨立
歷史收集能力，並不改寫 DDAE-9 的既有核准歷史。
使用者在調查及修改建議後回覆 OK，作為著手準備此工作的指示；本候選稿的
具體設定、狀態與 readiness 行為尚未取得核准。不切換 HARNESS/ACTIVE_CHANGE，
也不修改實作，直到使用者核准此需求與計畫。Requirements/Plan approval：Pending。

## Context and Constraints

2026-09-09 Demo 只讀調查：

- Management UI 使用 serviceability-events 的 filter；回應有 threshold、
  totalRecords、results。updatetime le/ge 分段取得 456+264，唯一聯集 720，
  與前後 totalRecords=720 相符。
- limit、threshold 未提高回傳上限；offset 語意未證明，不採用。
  createtime 切分未奏效，id filter HTTP 500，不採用這兩種探索方式。
- Insights history 支援 filter JSON 的 startDate/endDate 及
  sortBy=createDate、sortOrder=asc/desc；不是 limit/offset 分頁。
- 固定截止 2026-09-09T19:29:53Z：最新 1000、較早 476、重疊 2，
  唯一聯集 1474，與同條件 queriesByTime 報表加總相符。
- allQueries=true、persistenceEnabled=true；來源 retention 配置仍未知。
- 證據：output/validation/pagination-report.zh-TW.md、log-update-windows.json、
  query-history-boundaries.json、older-query-details.json。它們是探索證據，
  不是未實作 DDAE-10 的 acceptance pass。只把合成資料用作測試 fixture。

現有程式邊界：internal/ddae/client.go 的日誌清單無 filter；
internal/queryclient/client.go 僅准固定路徑、不接受任意 query string；
internal/serviceability/pipeline.go 的完整清單用於 ReconcileListed；
internal/queries/pipeline.go 將 overview 和 detail 放在同次輪詢；
internal/logstate 與 internal/querystate 已負責 outbox/checkpoint，但未保存掃描進度。

不包含：逐服務健康、額外節點效能指標、任意 SQL／JMX、Kubernetes 存取、
alert 歷史回補、Kafka schema 改版、恢復 Portable、部署或修復既有 Windows 測試。
此次服務狀態探索不構成新增服務健康 collector 的授權或規格。

## Requirement Index and Acceptance Criteria

| Requirement | 規範性行為 | Acceptance |
|---|---|---|
| REQ-DDAE-10-001 | MUST 提供獨立、預設關閉的 logs/query backfill 設定；關閉時不建立回補 worker/狀態。啟用須有對應主 pipeline。YAML/env precedence 沿用現有規則。 | AC-DDAE-10-001 |
| REQ-DDAE-10-002 | MUST 僅由 typed UTC bounds 產生固定清單路徑及必要參數；禁止任意 filter、URL、offset、user/SQL 篩選；沿用各自認證與 TLS 約束。 | AC-DDAE-10-002 |
| REQ-DDAE-10-003 | MUST 固定每輪掃描的上下界；logs 用 updatetime，queries 用 startDate/endDate。達上限即切分，邊界保守重疊再去重；最小 1 秒仍達上限時停止推進並標 incomplete。 | AC-DDAE-10-003 |
| REQ-DDAE-10-004 | MUST 持久化未完成區間、待處理 ID/marker、已確認進度，綁定來源與設定指紋。只有資料已交給既有 durable store 後才可確認進度；崩潰可重做、不能跳過。 | AC-DDAE-10-004 |
| REQ-DDAE-10-005 | MUST 沿用既有事件白名單、獨立 topic、at-least-once 及 observed aggregates；跨前景/回補/重啟重複不能重計終態，不能以舊 marker 覆寫較新狀態。filtered list MUST NOT 用於 absence reconciliation。 | AC-DDAE-10-005 |
| REQ-DDAE-10-006 | MUST 限制頁數、detail 數、併發、response bytes、工作佇列與每輪期限；獨立背景回補不占用即時 overview 的執行鎖／請求配額；scrape 不呼叫 DDAE。 | AC-DDAE-10-006 |
| REQ-DDAE-10-007 | MUST 回報有界進度、失敗、過期與容量狀態；完成僅表示指定窗口走訪結束。query_history_complete 維持 0，不能把有限窗口當全域完整清單。 | AC-DDAE-10-007 |
| REQ-DDAE-10-008 | MUST 定義回滾與 retention gap；不因持久化進度損毀而自動丟棄資料，不因 interval 改變重設 checkpoint。停用回補保留既有事件及進度。 | AC-DDAE-10-008 |
| REQ-DDAE-10-009 | MUST 更新完整設定範例、README、runbook、traceability，並區分合成測試、Demo 功能、trusted TLS、Kafka 及完整 Harness 結果。 | AC-DDAE-10-009 |

## Candidate Configuration Contract

以下兩處具有相同 leaf keys：
monitoring.serviceability_logs.backfill、monitoring.queries.backfill。
環境前綴分別為 SERVICEABILITY_LOG_BACKFILL_ 與 QUERY_BACKFILL_。
主 pipeline 預設開關不改動；本功能不會自動啟用 queries 或 logs。

| YAML leaf / ENV suffix | 預設 | 限制與意義 |
|---|---|---|
| enabled / ENABLED | false | 對應主 pipeline 關閉而本設定為 true 時報錯 |
| lookback / LOOKBACK | 24h | 1h–720h，且不得大於對應 checkpoint retention |
| overlap / OVERLAP | 2m | 1s–1h、不得大於 lookback；增量掃描向前重疊 |
| interval / INTERVAL | 30s | 5s–1h；相同 worker 不重疊執行 |
| cycle_timeout / CYCLE_TIMEOUT | 20s | 主 client request_timeout < 此值 < interval |
| rescan_interval / RESCAN_INTERVAL | 1h | >= interval、<= lookback；重新走訪滾動 lookback 以捕捉晚到資料 |
| max_pages_per_cycle / MAX_PAGES_PER_CYCLE | 4 | 1–32，邏輯 list 呼叫數，retry 另受既有 retry_max 與 cycle deadline 限制 |
| detail_max_per_cycle / DETAIL_MAX_PER_CYCLE | 25 | 1–1000，與前景配額分開計算，文件列明合併上界 |
| detail_concurrency / DETAIL_CONCURRENCY | 2 | 1–8，<= detail_max_per_cycle |
| max_pending_records / MAX_PENDING_RECORDS | 10000 | 1000–100000，超限 backpressure，不丟棄 ID |

固定限制：最多 4096 待掃區間、32 MiB 進度邏輯資料、單次進度交易有界。
回補頁面沿用相應 list response byte limit。Query 頁面判斷使用觀察到的
1000 截斷界限；QUERY_MAX_HISTORY_RECORDS 必須 >=1000 才能啟用回補，
它不是可控制伺服器回傳上限的參數。Logs 採有效 threshold 及 totalRecords。

初次回補下界為 now-lookback，上界為啟動時固定的 now（秒精度向外取整）。
後續增量由 last completed end-overlap 起至新固定 end；每 rescan_interval
重掃當時的 rolling lookback，捕捉較晚才出現且仍在窗口內的紀錄。
不自動回補窗口之外的歷史；改 lookback 明確建立新的 bounded sweep，
已入列事件與 dedup checkpoint 不清除。

## API, State and Failure Contract

Logs：GET <configured api prefix>/serviceability-events，唯一新增參數 filter，
由程式產生 `(updatetime ge "<UTC>") and (updatetime le "<UTC>")`。
Queries：GET /ui/api/insights/history/queries，唯一新增參數為
sortBy=createDate、sortOrder=desc、filter={startDate,endDate}。
Scope 檢查仍在 query worker 使用既定 cluster/info 執行；報表 endpoint
僅屬探索驗證，不納入正式 runtime API。

驗證所有頁面 identity、時間型別與範圍；未符合邊界（容許 outward rounding
重疊）不可當作完成窗口。Logs totalRecords 與有效 unique count 不同時
切分；缺 threshold/total、malformed、duplicate ID 或無法切分均 fail closed。
Query 頁面 count>=1000 必須切分；小於上限只能作該窗口可走訪的操作性證據，
不是完整永久歷史。結果超出本地 response/max record 限制仍屬失敗。

使用新的獨立 STATE_DIR/history-backfill.db（schema v1）存兩種 pipeline 的
游標／待處理 IDs；不儲存 SQL、user、來源地址或原始回應。維持既有
query-events.db / serviceability-logs.db schema 與事件格式。
跨 DB 採保守順序：先 durable 記錄事件／既有去重結果，再 ack 進度。
任何中斷只能重播；禁止先 ack 造成遺漏。資料庫來源 origin/source identity
與窗口設定指紋不相符須報出可處理錯誤，不能自動套用到另一目標。

前景與回補可能讀到同 ID：共享既有 store 的交易序列化與事件版本檢查，
source updated marker 更舊時不覆寫较新 checkpoint／pending event；詳細資料
回應需重新驗證 ID。缺失 source marker 的事件仍按既有 content hash 去重，
不得虛構來源時間保證。Query terminal 計數僅使用既有原子 Record 入口一次。

超過既有 retention floor 的 query 不回補計數；記錄 gap、保持 incomplete。
進度未完成時不得因 pruning 悄悄移動下界；明確回報 expired 狀態，等待縮小
窗口／重新開始的操作指示。404、403、解析失敗、容量不足均保留待處理項目，
不由 disappearance 推論業務完成或刪除。取消必須在既有 shutdown grace 內結束。

## Observability and Readiness

新增固定 pipeline label：serviceability_logs、queries（僅啟用者輸出）。

| Metric | Type/unit | 行為 |
|---|---|---|
| ddae_history_backfill_enabled | gauge/bool | 已啟用回補 |
| ddae_history_backfill_success | gauge/bool | 最近 worker cycle 無 source/state/detail 錯誤；啟動前 0 |
| ddae_history_backfill_pending_windows | gauge/windows | durable 待走訪區間數 |
| ddae_history_backfill_pending_records | gauge/records | 尚未交付 store 的 IDs 數 |
| ddae_history_backfill_last_completed_timestamp_seconds | gauge/Unix seconds | 最近完成固定窗口的上界；從未完成時省略 |
| ddae_history_backfill_incomplete | gauge/bool | 當前窗口仍待掃／受阻／gap，無未完成項目時 0 |
| ddae_history_backfill_blocked | gauge/bool | 同秒超限、過期、容量或持久化錯誤阻止前進 |

每名稱 HELP 說明有限窗口，不代表所有歷史；最多 14 額外 series。
沒有 time/ID/user/source/error message labels；時間只能作 metric value。

回補關閉：既有 /readyz 行為保持。回補開啟：
- pending 本身不等於故障；最近成功、未過期且沒有 blocked 的 worker 可 ready。
- 連續來源／detail／state 失敗、同秒超限、retention gap 或 stale (>3*interval)
  使該回補分支不 ready；不移除仍有效的其他 metrics。
- 對 logs，在回補模式下，已驗證合法但被截斷的前景清單可繼續收集，
  readiness 由 healthy 前景及回補共同決定；不再單憑截斷就永久不 ready。
  原全域 list_complete 仍保持 0，時間窗口完成不得將它改為 1。
- 對 query，running/queued 的 freshness/scope 判斷不變；回補 worker 失敗
  影響 readiness 但不得覆寫即時 overview 狀態。healthz 仍是 process liveness。

## Approved Plan Basis (candidate; not approved)

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

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

### Requirement-linked Tasks and Verification Basis

| Task | Requirements | Expected acceptance tests | Harness stage |
|---|---|---|---|
| TASK-DDAE-10-001 | 001 | TEST-DDAE-10-001: defaults、YAML/env 覆寫、主功能關閉、lookback/retention、所有界限 | test, security |
| TASK-DDAE-10-002 | 002 | TEST-DDAE-10-002: record HTTP methods/paths/encoding、注入／redirect/TLS 拒絕、無 raw body logging | test, security |
| TASK-DDAE-10-003 | 003 | TEST-DDAE-10-003: 合成 720 logs、1474 queries、重疊／毫秒、1000 ties、錯誤計數、範圍外資料 | test, coverage |
| TASK-DDAE-10-004 | 004,008 | TEST-DDAE-10-004: 重啟、崩潰邊界、schema/source mismatch、佇列/bytes 上限、rollback | test, coverage |
| TASK-DDAE-10-005 | 005 | TEST-DDAE-10-005: 前景/背景相同 ID、舊 marker、一次終態 aggregate、filtered absence、missing marker | test, security |
| TASK-DDAE-10-006 | 006 | TEST-DDAE-10-006: 慢回補不擋 overview、request/detail budgets、無重疊、取消、scrape 無 network | test, coverage |
| TASK-DDAE-10-007 | 007 | TEST-DDAE-10-007: 指標精確名稱/labels、啟動/健康pending/blocked/stale、readyz 不偽造完整性 | test, coverage |
| TASK-DDAE-10-008 | 008 | TEST-DDAE-10-008: 斷線重試、query retention floor、lookback 改變、停用保留state | test, security |
| TASK-DDAE-10-009 | 009 | TEST-DDAE-10-009: 全部 config example 欄位、建置、文件／契約一致性、Demo 分段值比對 | test, build, integration |

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

- 2026-09-09：讀取規格、前景 pipeline 與 state；完成本候選需求／設計／測試矩陣。
- 尚未實作 DDAE-10；尚未執行 DDAE-10 acceptance tests；未建立核准紀錄。

## Surprises and Discoveries

- 時間參數是已觀察的 UI API，不是官方穩定 cursor/pagination 契約。
- Query checkpoint retention floor 可能阻止歷史重計，必須限制 lookback 並揭露 gap。
- Logs readiness 與全域 list completeness 原本綁定，需要顯式規格調整而不是
  回補完成就偷偷把完整性指標設成 1。

## Decision Log and Open Questions

提案需由目前 repository user 核准，推薦上述預設與行為；尚無已作成的核准。

| 議題 | 選項與後果 | 推薦及理由 |
|---|---|---|
| 啟用方式 | 自動啟用會增加升級後負載；opt-in 需填設定 | 預設關閉，讓操作方控制歷史流量 |
| 回補範圍 | 無限歷史無法界定成本及 retention；有限窗口不承諾永久稽核 | 預設24h、最大720h且<=retention，成本與去重一致 |
| 進度儲存 | 改現有DB schema會增加回滾成本；獨立DB需保守跨DB順序 | 新DB，先寫事件再確認進度，崩潰最多重播 |
| 完整性 | 只因 count 相符宣告全域完整會誤導；有限窗口成功另行觀測 | history_complete保持0，新增窗口進度 |
| readiness | 一直等待完整歷史會永遠不ready；忽略錯誤會隱藏遺漏 | healthy pending可ready，blocked/錯誤/stale不可ready |

來源實際 retention、同秒超限的來源端替代 API、正式 Kafka/E2E 為已知外部限制，
不透過假設消除。若後續探索要求新增參數或改變本規格行為，更新候選與核准。

## Recovery and Idempotence

參照上方候選契約；禁止刪除現有狀態、重算既有 counter 或變更 topic。
停用只停止新增歷史讀取；publisher 按既有設定處理已入列事件。

## Outcomes and Retrospective

目前交付是可核准候選稿，並非已實作／Verified 的產品功能。
