# DDAE-11 Query-only Kafka SASL 修正提案

## Purpose and Observable Outcome

只開啟查詢事件輸出時，Exporter 正確讀取既有 Kafka SASL 帳號與密碼。
缺少必要帳密時，在設定載入階段回報錯誤；不再以空帳密建立 producer。

- Status: **待核准；尚未修改產品程式或新增正式測試。**
- Proposed Change ID: `DDAE-11`。
- Proposed Specification Version: `2.7.0`，依 PROJECT.md 的向後相容行為修正规則；
  這是規格版本，不是軟體 release 版本。
- Baseline: `3b057a41758e2aa6edd1301418ef2fd2953035c9` / `v1.0.0-rc4`。
- Requirements / plan approver: 本次提出修正要求的 Repository 使用者。
- Request evidence: 使用者於 2026-09-11（Asia/Taipei）表示「請修正這個問題」。
  這是修正意圖；不記為尚未呈現之本提案的核准。

## Context and Constraints

`internal/config/config.go` 的 broker 必填條件已包含 `cfg.Query.Events`，
但 SASL credential 載入條件只有 alerts / serviceability logs。
`internal/kafka/producer.go` 的 query producer 共用 Kafka transport，
直接取用 `cfg.KafkaSASLUsername` 與 `cfg.KafkaSASLPassword`。
因此 alerts、logs 都關閉，query events 與 SASL 開啟時會漏讀帳密。
resources 開啟與否不影響這個觸發條件。

現有 query 設定測試未檢查此組合的 Kafka credential 值；producer 建立測試
只證明 client 可建立，不代表 broker 已接受認證。以上為原始碼查核，
不是實際 SASL broker 的登入證據。

PROJECT.md 啟用 required SDD，認證／secret 修改須先核准需求、實作方案
與測試方案。RC4 已發布，因此提出獨立 DDAE-11，不重用 DDAE-10。
保留歷史規格、核准 digest、未完成驗證與既有 release；不擴充認證種類，
不變更 TLS、安全檢查門檻、Kafka schema、topics、state 或 deployment。

## Candidate Requirements and Acceptance Criteria

| Requirement | Requirement text | Acceptance |
|---|---|---|
| REQ-DDAE-11-001 | 任一 Kafka 輸出（alerts、serviceability logs、query events）啟用且選用 SASL 時，MUST 透過既有 loader 載入 username 與 password。適用 PLAIN、SCRAM-SHA-256、SCRAM-SHA-512；MUST 保留 YAML/env precedence 與 direct/file secret 規則。 | AC-DDAE-11-001 |
| REQ-DDAE-11-002 | Query-only events + SASL MUST 在設定載入時拒絕缺少或不合法的必要帳密、不可讀取／空值／過大／內容不合法的密碼檔與 direct/file 衝突；錯誤不得包含帳密、檔案內容或底層錯誤中的敏感值。 | AC-DDAE-11-002 |
| REQ-DDAE-11-003 | MUST 保留 query metrics-only 與 resources-only 的 Kafka secret 隔離，保留 alerts/logs/mixed profiles 的既有認證行為。未選用 SASL 時不新增 SASL 帳密要求；既有 TLS/mTLS 規則、broker/topic 驗證與 query pipeline 關閉行為不變。 | AC-DDAE-11-003 |

AC-DDAE-11-001：使用三種 SASL mechanism，測試 query-only events 的 direct
password、password file、YAML 設定，以及 env 對 YAML username/mechanism/
password file 的覆寫。逐一驗證實際 Config secret 值，不能只檢查無錯誤。

AC-DDAE-11-002：測試缺少 username/password、空值、不合法值、密碼檔讀取
失敗、大小界限與 direct/file 衝突；驗證錯誤名稱及合成 canary 不外洩。
密碼沿用 loadSecret 的既有定義，不另加 trimming 或新的字元限制。

AC-DDAE-11-003：涵蓋三種 Kafka 輸出的所有開關組合；均關閉時另以
query metrics-only 與 resources-only 驗證不讀取 Kafka 密碼檔。
另測 query 關閉但 events 原始設定為 true 的既有隔離行為，以及 events
啟用但 SASL 為空時不要求帳密。既有格式、衝突與 TLS 檢查繼續生效。

## Candidate Plan Basis

<!-- SDD_APPROVED_PLAN_BASIS_BEGIN -->

### Requirement Coverage and Technical Design

只在 `internal/config/config.go` 的 SASL credential 載入條件加入
`|| cfg.Query.Events`。沿用 `requiredText`、`loadSecret`、`layeredLookup`；
不新增 loader、public key、依賴或 producer 的認證實作。

主要測試放在 `internal/config/kafka_sasl_test.go`，使用既有 config test
helpers 與合成資料。YAML 案例必須經過 `decodeYAML` 和 `layeredLookup`；
隔離案例以記錄讀取次數或會失敗的 reader 證明未讀取 Kafka 密碼檔。
必要的真實暫存檔只用 t.TempDir 與合成 secret，不讀取操作者環境中的帳密。

### Requirement-linked Tasks and Tests

| Task | Requirement / acceptance | Planned executable evidence | Harness stage |
|---|---|---|---|
| TASK-DDAE-11-001 | REQ/AC-DDAE-11-001 | TEST-DDAE-11-001：新增 `TestQueryEventsKafkaSASLCredentials`、`TestQueryEventsKafkaSASLYAMLPrecedence`；先記錄 RC4 缺陷造成的失敗，再加入條件修正 | test |
| TASK-DDAE-11-002 | REQ/AC-DDAE-11-002 | TEST-DDAE-11-002：新增 `TestQueryEventsKafkaSASLRejectsInvalidCredentials`，含 missing/empty/NUL/UTF-8、file boundary、reader failure、conflict 與錯誤 redaction | test |
| TASK-DDAE-11-003 | REQ/AC-DDAE-11-003 | TEST-DDAE-11-003：新增 `TestKafkaSASLPipelineSelection`，驗證各 pipeline 組合與不讀檔隔離；執行現有 config、Kafka、app 與 TLS 契約測試 | test |
| TASK-DDAE-11-004 | REQ-DDAE-11-001 至 003 | 更新 canonical SDD records、雙語 README 的 SASL 適用條件、runbook 與 README Review Notes 的修正狀態；驗證文件契約，保留完整 gate 的實際結果與 diff review | test, security, build |

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

- 2026-09-11：確認根因、建立提案；`instructions:doctor` 與 `doctor` 通過。
- 尚未新增正式 acceptance tests、修改產品程式或執行新測試；等待本提案核准。

## Surprises and Discoveries

失敗條件是「沒有 alerts/logs publisher」，不限定 resources 是否關閉。
既有 producer 建立測試未攜帶 SASL 帳密，不能取代本次 credential 值回歸測試。

## Decision Log

- 選項 A（建議）：修正共用 loader 的一個判斷，加入上述回歸測試。
  範圍最小，沿用既有安全與設定規則，直接處理根因。
- 選項 B：各 producer 個別重讀帳密。會重複 loader 與 precedence 邏輯，
  增加維護及 secret handling 風險，因此不建議。
- 決策權：本次需求核准者；目前尚未對此具體提案記錄核准。

## Recovery and Idempotence

提案尚不改變 runtime；核准前可直接修訂。核准後依 marker-bounded basis
保留 digest，進度寫在 block 外。產品修正不寫入或遷移使用者 state。

## Outcomes and Retrospective

目前成果是可核准的需求、修正方案與測試計畫；RC4 缺陷尚未修正。
