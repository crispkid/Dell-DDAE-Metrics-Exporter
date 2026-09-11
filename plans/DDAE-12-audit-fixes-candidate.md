# DDAE-12 Bug 修復與無效程式碼清理提案

## Purpose and Observable Outcome

修復 2026-09-11 程式碼審查確認的 8 項 Bug，清理 6 組無作用的程式碼，
並補上可重現問題的回歸測試。優先確保告警不被卡住、健康狀態誠實，
以及既有待送事件與累計資料受到保護。

- Status: **待需求、實作方案與測試方案核准；本提案尚未實作。**
- Proposed Change ID: `DDAE-12`。
- Proposed Specification Version: `2.8.0`；這是規格版本，不是 release 版本。
- Baseline: HEAD `3b057a41758e2aa6edd1301418ef2fd2953035c9`，
  加上目前工作目錄中已核准、
  尚未 commit 的 DDAE-11 query-only Kafka SASL 修正與其文件。
- Requirements / plan approver: 本次提出修正要求的 Repository 使用者；
  canonical records 沿用既有核准流程記錄真實 actor 與本次回覆。
- Request evidence: 使用者於 2026-09-11（Asia/Taipei）表示
  「請修復Bug並清除多餘的程式碼」。這是修正意圖，不記為尚未呈現的
  具體需求／方案／測試計畫已獲核准。

## Context and Constraints

審查報告位於本機 ignored 路徑 `output/_code-audit-20260911/REPORT.md`。
本提案下表保留問題、程式位置與預期結果，不以該暫存報告作為唯一契約。
先前合成故障注入屬調查證據，不冒充本次正式 acceptance tests。

PROJECT.md 啟用 required SDD。本次跨 pipeline 的修復與 DDAE-11 帳密載入
是不同成果，使用新的 Change ID；保留 DDAE-11 的修改、歷史 approval bytes、
驗證限制與未完成 trace，不藉由切換 active change 宣稱前項工作已 Verified。

此次範圍包括 B5 同根因的 S1：拒絕被污染的 query outbox payload。
S2「security-policy.sh 的兩個 TLS target 規則過時」是另一項安全流程變更，
本提案不修改該規則，也不關閉或弱化安全 gate。完整驗證仍須如實列出此阻擋。
其他不確定問題，例如重複 legacy node condition 的衝突值處理，另行釐清，
不混入已確認 Bug 的修復。

不新增 API、設定名稱、metric、label、Kafka schema、topic、認證機制或相依套件。
保留既有 read-only DDAE 存取、API/版本邊界、secret/TLS 規則、HTTP routes、
逾時／併行／回應大小上限、Kafka at-least-once delivery、shutdown 與部署契約。
不清空 state、不自動修復損毀 DB、不做 migration；不包含 commit、push、tag、
release、部署或重新建立 Docker 測試環境。

## Candidate Requirements and Acceptance Criteria

下表的 MUST 為待核准要求；核准前不是已生效規格。

| Requirement / acceptance | 問題與位置 | 預期可觀察結果 |
|---|---|---|
| REQ-DDAE-12-001 / AC-DDAE-12-001 | B1：`internal/outbox/store.go` 以 hash 判斷是否清除 pending | 同一 alert 的 A → B → A 三筆同時待送時，MUST 可逐筆 acknowledge 並於任一階段重啟；只有確認最新 pending sequence 才清除其 pending 標記，保留其他事件與順序。 |
| REQ-DDAE-12-002 / AC-DDAE-12-002 | B2：`internal/alerts/pipeline.go`、`internal/serviceability/pipeline.go` 依最後成功時間排序 | 持續失敗的 detail MUST 不阻止同類別其他持續合格 ID 輪到執行；兩條 pipeline 均保留既有類別配額、總量與併行限制，不把失敗寫成成功 checkpoint。 |
| REQ-DDAE-12-003 / AC-DDAE-12-003 | B3：`internal/snapshot/store.go` 只用整輪完成時間判斷健康 | resources 所需任一 family 缺少、過期或其必要採集失敗時，MUST 正確反映整體 readiness 與 `ddae_up`；保留單項原始採集時間與其他 pipeline 的隔離規則。 |
| REQ-DDAE-12-004 / AC-DDAE-12-004 | B4：`internal/ddae/types.go` 省略物件格式的 represented null pressure | 已出現但為 null 的 pressure 值 MUST 與 legacy array 的無效值一致，令該 collector 不完整；真正省略的 optional 欄位保持原契約，其他可用欄位仍可呈現。 |
| REQ-DDAE-12-005 / AC-DDAE-12-005 | B5 / S1：`internal/querystate/store.go` 缺少既有檔案與持久化內容完整性檢查 | 既有零長度、格式損毀、schema/source 不符或可核對的不一致 MUST 明確失敗並保留資料，不初始化成新 DB；污染事件 MUST 在 replay 前被拒絕，合法 retention、重啟與累計值保持相容。 |
| REQ-DDAE-12-006 / AC-DDAE-12-006 | B6：`internal/queries/pipeline.go`、`internal/historyscan/adapters.go`、`internal/queryclient/types.go` 共用請求前時間做未來時間檢查 | 即時採集與 backfill MUST 以回應取得時的時間驗證未來時間上限；請求開始的 observation ordering MUST 保留，慢但未逾時的合法回應可接收，舊觀察不能覆蓋新狀態。 |
| REQ-DDAE-12-007 / AC-DDAE-12-007 | B7：`internal/outbox/store.go` 未將 checkpoint 滿額納入 full | checkpoint 數量恰好等於上限時 MUST 回報容量已滿，並反映既有 pipeline health/readiness；只按既有規則清理可過期、非 pending 的 checkpoint。 |
| REQ-DDAE-12-008 / AC-DDAE-12-008 | B8：`internal/config/config.go`、`internal/config/query.go` Kafka topic 名稱驗證不完整 | alerts、logs、queries 的 topic MUST 共用合法名稱檢查：1–249 個 ASCII 英數字、`.`、`_`、`-`，且排除 `.` 與 `..`；保留各模式既有必填／關閉／topic 隔離語意與設定優先順序。 |
| REQ-DDAE-12-009 / AC-DDAE-12-009 | C1–C6：下節列出的無作用欄位、引用與重複讀取 | MUST 移除已確認無效的部分，保留真正使用中的 refresh 時間、handler closure、測試／契約 helper 與 build/provenance 介面，不藉清理新增 runtime 行為。 |
| REQ-DDAE-12-010 / AC-DDAE-12-010 | 相容性、文件與證據 | MUST 補足前述回歸測試、保留有效歷史測試及 DDAE-11 修正，更新受影響的雙語操作文件，分別記錄實際通過、失敗、blocked 與未執行的驗證，不將合成測試稱為真實外部整合。 |

## Candidate Plan Basis

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

### Requirement Coverage and Technical Design

**B1：acknowledge 依 sequence 清除 pending。**

沿用 `internal/logstate/store.go` 已有模式，在同一個 bbolt transaction
掃描時保留最新 pending hash 與 sequence。hash 一致性檢查仍存在；
以被確認的 sequence 是否為最新 pending 決定清除，而非只比較內容 hash。
保留已不存在 record 的 acknowledge 冪等性、per-alert 狀態及 transaction 原子性。
不更動資料格式；已損毀的 DB 繼續明確報錯，不自行刪掉阻塞事件。
新規格只修訂 DDAE-3 對 pending 清除條件的 hash-only 描述，歷史 basis 不改字。

**B2：為 detail 採集加入有界輪轉次序。**

兩條 pipeline 各自維護記憶體中的 eligible-ID 排程次序。首次建立次序時
採既有最後成功時間與 ID 決定順序；仍在等待的 ID 保持次序，新加入的 ID
排在已等待者之後。每次被選入本輪配額，即消耗一個排程輪次並移到隊尾，
不論該次請求最後成功、失敗或取消；採集與成功 checkpoint 則照實際結果處理。
每輪移除已不合格／已不在清單的排程記錄，記憶體量受目前 bounded list 限制。
重啟後從有效 checkpoint 重新建立排序，不新增持久化格式。

保留 new/changed 與 refresh 兩類配額：quota=1 時兩類交替，較大 quota
維持 refresh 至少 1、通常為 floor(limit/4)，並保留不足時的配額借用。
同類別內不再讓最後「成功」時間永久決定失敗者優先權；持續合格者前方
只有有限等待項目，新加入者不能不斷插隊。此為 DDAE-3／DDAE-4 排序說明的
明確修訂，保留其不得無限期飢餓與所有資源上限要求。

**B3、B4、B7：健康狀態與資料狀態一致。**

- B3：集中使用一致的 freshness 判斷，確認 Ping、Clusters、Nodes、Lock、
  Power 必要 snapshot 的 Present 與各自 CollectedAt；同時保留整輪與
  collector success 條件。沿用既有 stale 邊界，不把早取得資料的時間改晚。
- B4：區分 object pressure key 不存在與值為 null；後者保留為可辨識的
  無效 condition，走既有 normalization／partial failure 路徑。與 legacy
  array 做對照，不丟棄其餘合法 node 欄位，也不擴充新的 pressure metric。
- B7：checkpoint 計數納入 alert `Stats.Full`／Health 的 `>=` 邊界，
  reconciliation 採既有安全 pruning 後再次判斷 full；參照 logstate 行為，
  保留 pending checkpoint，不以淘汰事件換取健康狀態。

**B5 / S1：在可證明的資料邊界拒絕損毀與污染。**

1. 開啟前區分新檔與既有檔。只有確定不存在的新檔可初始化 schema 1；
   既有零長度／非 regular／symlink／無法確認狀態的路徑明確失敗。
   既有檔缺失 meta、checkpoints、events 或版本／source 不合時，不建立替代 bucket。
2. 啟動時先驗證既有 logical state，再執行既有 retention pruning 或啟動
   publisher。檢查 bucket、sequence key／bucket sequence、checkpoint key/value、
   aggregate 結構，以及 outbox 的 pending count／payload byte sum 是否一致。
   檢查不新增每次 Prometheus scrape 全表掃描；持久化讀寫入口共用必要驗證。
3. 對持久化事件使用明確欄位 allowlist 與型別：拒絕未知／重複欄位、trailing
   JSON、無效 UTF-8、缺少必要值、source mismatch、無效 ID／state／timestamp
   結構、超過 64 KiB payload 及超出既有界限的 duration。沿用目前 Event 的
   欄位與正規化 state，保留 optional 欄位語意。輸入 Record 亦須符合這些
   可持久化條件，Records 在回傳可送往 Kafka 的 bytes 前再驗證該 record。
   不靠刪除未知欄位後繼續送出來掩蓋污染，錯誤不包含 payload 或 SQL canary。
4. 只使用資料模型確實保證的跨筆不變量。尚未早於「已儲存 retention Floor」
   的 pending 終態事件，必須有對應且一致的 final checkpoint；驗證發生在
   推進 Floor 之前，不能用本次 prune 掩蓋遺失。已早於 Floor 的 pending
   事件可能合法失去 checkpoint，仍保留並 replay，不因缺 checkpoint 判錯。
   非終態可以有多筆歷史變更，events 關閉期間也可更新 checkpoint；不把所有
   pending hash 強制等同目前 checkpoint。對非終態使用能確立的結構／來源檢查，
   不臆測不存在的交易歷史。
5. 累計 Completed／histograms 不能由已裁剪的 checkpoint 或待送事件重算。
   只驗證合法 state/key、histogram 維度、有限且非負的 sum、bucket/count
   一致性等可證明條件，保留真實累計值。不假裝能偵測所有一致性造假的離線
   修改，亦不以新增 metadata、schema migration 或重置計數器擴張本提案。
6. 合法 schema 1 DB 保持相容；負向測試在 reopen 失敗前後比較原始內容，
   確認沒有清空、刪除、重建或先 prune。保留 0700/0600 權限、單一 writer
   與有界讀取／既有容量限制。若實作發現必須改 schema 或修復既有資料才能
   達成要求，停止並提出修訂，不自行增加 migration。

持久化事件的 observed_at 仍是請求開始的排序時間；合法 completed_at 可以
晚於 observed_at。既有 DB 驗證不使用錯誤的「completed <= observed」條件，
也不以 reopen 當下的時鐘重新判定歷史事件的 freshness。

**B6：分開 observation ordering 與 response validation 時間。**

保留請求前的 observation time，用來建立 Event.Observed 及防止較舊請求覆蓋
新 checkpoint。取得 HTTP response 後再讀時間，用於 submission/completion
是否超前現在 5 秒的檢查。透過小型內部 helper 明確傳入兩個時間，保留現有
DecodeDetail 呼叫的相容入口；即時 query 與 backfill 均使用修正路徑。
不放寬 5 秒上限，不改變 HTTP deadline，也不把較舊請求因較晚完成而當作新觀察。

**B8：共用 Kafka topic validator。**

以一個小型 helper 檢查 Kafka 已有名稱語法，三種輸出共用；沿用各 caller
現有何時驗證、是否必填、topic 是否需互異的規則。合法 topic 無須修改。
非法名稱由啟動後重試失敗改為載入設定時明確失敗；錯誤只指出設定問題，
不帶 credential。規則依 Apache Kafka 4.3.1 的
[Topic validator](https://raw.githubusercontent.com/apache/kafka/4.3.1/clients/src/main/java/org/apache/kafka/common/internals/Topic.java)。

**C1–C6：清理範圍與保留項目。**

| ID | 修改 | 明確保留 |
|---|---|---|
| C1 | 移除 `internal/alerts/event.go` 的 `var _ = fmt.Sprintf` 及唯一對應 import | 既有 event 編碼、欄位與 redaction |
| C2 | 移除 `tokenRefreshResult.refreshAt` 欄位及其寫入 | 真正控制更新時機的 `tokenManager.refreshAt` |
| C3 | 移除 `Server.state`／`staleAfter` 欄位與重複初始化 | handler closure 使用的 state／staleAfter 參數與其他被讀取欄位 |
| C4 | 移除 `app.BuildInfo.Revision`／`BuildDate` 及 main → app 的無效傳遞 | `main.revision`／`main.buildDate` linker symbols、VERSION／REVISION／BUILD_DATE build inputs、ldflags、Docker／reproducible build／provenance 產物契約 |
| C5 | 移除 historyscan 測試的未用 `testSource.fail` | 既有測試與本次真正會觸發失敗的回歸測試 |
| C6 | 刪除 `ALLOW_INSECURE_TLS` 第二次相同載入／驗證區塊 | 第一次載入及所有三種 TLS target 的雙重 opt-in 檢查 |

C4 的 linker symbols 是刻意保留的打包介面，不新增假引用或 runtime
輸出來消除工具警告。若 unused 檢查列出它們，應逐項註明原因，而不是
關閉整體檢查。保留 `RequiredCollectors`、`config.Load`、`routeSet.operations`、
`ApprovedOperations`、`ApprovedOperationsForPrefixes` 等有測試／契約用途
的 helpers；不單憑 production entry 的 call graph 就刪除。

### Requirement-linked Tasks and Tests

以下 TEST ID 是核准後新增／擴充的測試群，不宣稱目前已存在或已通過。
每組先建立能呈現原缺陷的失敗 regression，再作修正；既有適用 regression
保留。固定時間、t.TempDir、合成 HTTPS 回應／state，避免真實帳密與不穩定等待。

| Task / test | Requirement / acceptance | 測試與成功條件 | Harness stage |
|---|---|---|---|
| TASK-DDAE-12-001 / TEST-DDAE-12-001 | REQ/AC-DDAE-12-001 | outbox：三筆皆 pending 的 A/B/A、更多重複狀態、跨 ID、每次 ack 前後 reopen、重複 ack；順序、hash、數量與剩餘 bytes 正確 | test |
| TASK-DDAE-12-002 / TEST-DDAE-12-002 | REQ/AC-DDAE-12-002 | alerts 與 serviceability：quota 1／較大 quota、持續失敗與正常 ID、new/refresh 競爭、持續新 ID、ID 移除／重入、取消；正常者在有限輪內被選到，狀態不假成功、配額／記憶體有界 | test |
| TASK-DDAE-12-003 / TEST-DDAE-12-003 | REQ/AC-DDAE-12-003 | snapshot／metrics／server 契約：各 family 錯開完成、stale 邊界與邊界外、缺 family、partial failure、resources 關閉；readiness、ddae_up 與單項 freshness 一致 | test |
| TASK-DDAE-12-004 / TEST-DDAE-12-004 | REQ/AC-DDAE-12-004 | ddae／collector 契約：pressure object/array 的 null、缺值與合法值對照；null 不判成功、optional omission 相容、其餘合法 metrics 保留 | test |
| TASK-DDAE-12-005 / TEST-DDAE-12-005 | REQ/AC-DDAE-12-005 | querystate／queries：新 DB、合法 v1 reopen、零 bytes／截斷／缺 bucket、來源／版本錯誤、未過期 terminal checkpoint 遺失、錯誤 count/bytes/sequence/aggregate；拒絕並保留原資料 | test |
| TASK-DDAE-12-005 / TEST-DDAE-12-005 | REQ/AC-DDAE-12-005 | payload 故障注入：未知 SQL canary、duplicate key、source mismatch、required/optional 欄位、type/state/time/duration/size 界限；不產出可送 Kafka 的污染 record，不洩漏內容；合法多筆歷史、metrics-only 更新、已 prune checkpoint 的舊 pending、非終態 optional completion、累計資料與 Final 冪等性均保留 | test, security |
| TASK-DDAE-12-006 / TEST-DDAE-12-006 | REQ/AC-DDAE-12-006 | queryclient／queries／historyscan：注入請求前後相差超過 5 秒的時間、合法等待中完成事件、真正超前 5 秒、timeout；兩條 caller 均驗證回應時間，Observed 保持請求開始，較舊觀察不能覆蓋較新資料 | test |
| TASK-DDAE-12-007 / TEST-DDAE-12-007 | REQ/AC-DDAE-12-007 | outbox／alerts：checkpoint 未滿、恰滿、已滿，pending 不可裁剪、合法 pruning 後恢復容量；Stats/Health/reconcile 及 readiness 正確 | test |
| TASK-DDAE-12-008 / TEST-DDAE-12-008 | REQ/AC-DDAE-12-008 | config：三條 pipeline、YAML/env precedence、1/249/250 長度、ASCII 合法字元、空值、`.`、`..`、slash、非 ASCII、空白/control；關閉與 topic 隔離規則不變 | test |
| TASK-DDAE-12-009 / TEST-DDAE-12-009 | REQ/AC-DDAE-12-009 | 編譯／既有測試／unused 分析：C1–C6 無效部分消失，token refresh、server routes、TLS opt-in、DDAE-11 SASL 135 regression subcases、build metadata 與 helper 契約保留 | lint, test, build |
| TASK-DDAE-12-010 / TEST-DDAE-12-010 | REQ/AC-DDAE-12-010，及 001–009 | 更新雙語 README、runbook 中 topic 規則、health/capacity、query corruption 排查與資料保護指引；執行完整 gate、diff review 與證據對照 | test, build, integration, e2e, security, ci-policy, supply-chain |

### Verification Basis

核准後先將本提案純轉錄到 canonical SPECIFICATION、SPEC_CHANGELOG、
TEST_PLAN、TRACEABILITY、DEVELOPMENT_PLAN 與 `plans/DDAE-12.md`，更新
PROJECT／HARNESS active change，計算 spec／plan-basis／test-plan digests，
保留原始 candidate 及真實回覆。明確標記原 DDAE-3／4 中受修訂的敘述，
歷史 approval bytes 不變。所有新／受影響 trace 起始為 Planned；通過
`governance:approved` 後才新增正式 acceptance tests 與實作。純轉錄不能
被描述為使用者逐字輸入過 digest；若增加行為或測試契約須重新核准。

迭代採最小 package 群（核准後由實際新增測試名稱選擇）：

```bash
go test ./internal/outbox ./internal/alerts ./internal/serviceability ./internal/logstate
go test ./internal/snapshot ./internal/ddae ./internal/metrics ./internal/server ./internal/contract
go test ./internal/queryclient ./internal/querystate ./internal/queries ./internal/historyscan
go test ./internal/config ./internal/app ./cmd/ddae-exporter
```

最終執行與記錄：

```bash
./HARNESS/harness.sh doctor
./HARNESS/harness.sh instructions:doctor
./HARNESS/harness.sh verify
./HARNESS/harness.sh security
./HARNESS/harness.sh sdd:check
git diff --check
```

`verify` 保持 committed config 的 lint、race test、coverage、build、integration、
e2e、security、ci-policy、supply-chain 必要 stages。若提早中止，明確列出後續
未執行項目，補跑安全、SDD 與相關局部檢查，不將後續階段誤記通過。

可重用審查時的版本化工具，不修改 go.mod/go.sum 或全域安裝：

```bash
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./cmd/... ./internal/... ./integration/...
go run honnef.co/go/tools/cmd/staticcheck@2026.1 -checks=U1000 ./cmd/... ./internal/...
go run golang.org/x/tools/cmd/deadcode@v0.49.0 -test -tags=integration,e2e ./cmd/... ./internal/... ./integration/...
```

工具需下載／cache／本機 listener 權限時依環境核准處理；不以不可用工具
冒充已執行。對保留 linker symbols 的 unused 結果逐項說明，不加假引用、
不弱化安全檢查。以上 deadcode 只分析 tagged tests，並未執行真實 integration。

外部 integration／E2E 需要既有腳本規定的、已授權的 non-production DDAE
與隔離 Kafka 設定；不設假的 enable 旗標或拿合成 API 冒充真實產品證據。
目前外部前提、S2 security 規則與 clean-commit supply-chain 前提仍可能阻擋
完整 handoff。只為執行 verify 不會擅自 commit 或發布；如實交付本機修復狀態
與未完成驗證，不把 blocked 或 skipped stage 記為成功。

此修改涉及 delivery、state 與資料保護，依 CODE_REVIEW.md 需要非實作者
獨立審查。尚無實際 reviewer／證據時列為待審，不自行推定授權建立 agent/task，
也不聲稱本人的 diff review 取代獨立審查。

### Compatibility and Recovery

合法設定與 schema 1 state 原地相容，不新增 migration。非法 topic 在啟動即
被拒絕；過期資料與 checkpoint 滿額會更早且正確反映健康狀態；已損毀的
query state 會停止啟動並保留資料，操作者需檢查損毀／還原來源，不能刪 DB
當成正常安裝步驟。其他 metric identity、Kafka payload/key/header 與
at-least-once 語意維持不變，不保證消除 Kafka 已送出但 ack 前中斷的合法重送。

本次只操作測試建立的隔離資料，不接觸使用者 runtime state。未來 rollout
前停止 exporter，保留完整 state 備份後替換 binary；rollback 使用原 binary
與保留的有效 state，會重新帶回舊缺陷。已損毀 state 不因 rollback 自動恢復。
任何資料修復、清空、遷移、部署與發布需另取得具體授權。

<!-- SDD_APPROVED_PLAN_BASIS_END -->

## Progress

- 2026-09-11：重讀專案 SDD／review／plan 約束、審查報告與受影響程式，
  整理 8 項 Bug、B5 同根因的 S1 及 C1–C6 清理方案。
- 2026-09-11：建立本 candidate；未切換 active change，未修改產品程式、
  正式測試或既有 DDAE-11 變更。需求與方案核准尚待使用者回覆。
- 2026-09-11：`git diff --check` 與 candidate whitespace 檢查通過；
  status 對照僅新增本提案。尚未執行本次產品測試或完整 verify。

## Surprises and Discoveries

- B1 的既有 A/B/A 測試先 ack 第一個 A，未涵蓋三筆皆 pending 的缺陷。
- B2 必須讓「嘗試排程」與「成功 checkpoint」分離，且不能讓新 ID 不斷插隊。
- B5 的舊 pending 可以比 retention Floor 更早並合法沒有 checkpoint；
  aggregate 是累計資料，不能從剩餘 outbox 重建。非終態時間欄位也不能
  被擅自套用終態鎖定的不變量。
- C4 的 app 欄位確實未被讀取，但 linker symbols 與 provenance/build inputs
  是另一層既有介面；移除無效傳遞不等於移除打包介面。
- S2 是 fail-closed 的安全流程阻擋，不是系統自動關閉 TLS。

## Decision Log

- 建議採本方案：逐一修復已重現問題並補 regression，保留資料與既有契約。
  相較只清理程式碼，可處理告警阻塞與錯誤健康判斷等實際運作影響。
- 已損毀 DB：建議拒絕啟動並保留證據。自動清空或重建雖可令程序啟動，
  卻可能遺失 pending、重複累計或掩蓋污染，因此不納入。
- Build 清理：建議只清除 app 的無效欄位／傳遞。連同 linker/build inputs
  一起刪除會擴及打包與 provenance 契約，需要獨立評估，不納入。
- Security 流程：建議把 S2 留待獨立明確範圍修正，不順手增加 target count
  或取消 gate；本次完整驗證的限制需保留。
- 決策權：本次 Repository 使用者／既有 Project Owner 核准流程。
  上述為建議，尚無本提案的實際核准紀錄。

## Recovery and Idempotence

candidate 可在核准前修訂，不改變 runtime。核准後固定本提案與 marker-bounded
basis，進度寫在 block 外；必要設計變更更新 digest 並重新核准。
新測試使用 t.TempDir／合成 state；不移動、刪除或覆寫既有工作目錄變更。

## Outcomes and Retrospective

目前成果是待核准的需求、技術方案與測試計畫。8 項 Bug 及 C1–C6 尚未因
本提案被修復／刪除；沒有新產品驗證通過或 release 的宣稱。
