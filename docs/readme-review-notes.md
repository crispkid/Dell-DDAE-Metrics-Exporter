# README Review Notes

本文件記錄此次 README 重寫的查核範圍、實際驗證結果與需由維護者處理的事項。
這些內容刻意獨立於專案 Landing Page；README 描述目前可由 Repository 核對的操作介面，
本文件不代表 release 核准。

## 本次變更

- 重寫 `README.md` 與 `README.zh-TW.md`，採一致的章節、設定、指令與範例。
- 以 resources-only 作為完整的首次安裝流程，另外說明 Kafka、查詢、歷史回補、
  Docker、Kubernetes 與 systemd。
- 保留現有 `docs/architecture.svg`，並用文字補充查詢與回補資料流。
  既有文件契約測試要求使用此靜態資產。
- 完整設定表以 `internal/config/{config,yaml,query,backfill}.go`、
  部署範例與測試交叉核對；81 個直接 YAML-to-environment 映射均有文件說明，
  另列共用回補 prefix/suffix、相容 interval 別名與直接 credential 變數。
- 未修改產品程式、部署 manifests、規格、approval digest 或 traceability status。
- 此次僅更新文件，未建立 commit、tag、GitHub Release 或部署。

## 實際驗證

以下結果來自本次工作期間的本機執行（2026-09-10）：

| 檢查 | 結果 | 範圍 |
|---|---|---|
| `go test ./internal/config ./internal/contract` | 通過 | 嚴格 YAML、README 範例、雙語程式碼區塊與部署契約 |
| `./scripts/build.sh` | 通過 | 以有效 Go 1.26.6 toolchain 建置執行檔 |
| README 靜態檢查 | 通過 | 兩份文件的標題層級一致、技術區塊一致、shell 語法正確、YAML 可解析、本機文件連結存在 |
| README resources-only 啟動 | 通過 | 從 README 擷取 YAML，替換本機測試 origin、port、CA 與路徑，啟動實際 exporter |
| HTTPS fixture 與 HTTP 驗證 | 通過 | 私有 CA 驗證、password-grant 參數、5 個 allowlisted GET、`/healthz`、`/readyz` 與 `/metrics` |
| `./HARNESS/harness.sh verify` | **Blocked / overall failed** | Doctor、governance、lint、race tests、coverage、build 通過；integration 缺少授權環境設定 |
| Coverage | **81.7%** | 本次 Harness 計算結果，高於設定的 80% 門檻 |
| `./HARNESS/harness.sh sdd:check` | **未通過** | DDAE-10 trace row 尚非 handoff 所需的 `Verified` |
| `./HARNESS/harness.sh security` | **未通過** | 相關測試通過後，TLS guarded-target 靜態規則因目標數量不一致而失敗 |

最初的 sandbox 執行受到 Go cache 與本機 listener 權限限制；取得執行權限後，
已重新執行相關測試與建置。上表依重跑結果記錄。

本機保留的 verify 報告為：
`test-results/harness/runs/20260910T154515Z-verify.KjSpVM/report.json`。
此目錄由 Git 忽略；交付／release 所需的證據留存仍依 Harness 契約辦理。
README 檢查與 smoke helper 位於 Git 忽略的 `output/_readme-validation/main.go`，
並使用 build-ignore 標記避免加入一般 Go package 測試。

本次未執行真實 DDAE/Kafka 整合、Kubernetes 或 systemd 部署。
Docker、Kafka CLI、Kubernetes 與 systemd 指令已做來源核對及 shell 語法檢查；
平台上的實際部署結果需在相應環境驗證。完整 verify 在 integration 停止後，
尚未執行其後的 E2E、security、CI-policy 與 supply-chain stages；
security 另行執行的結果如上。

## 需確認的程式與驗證事項

### 1. Query-only Kafka SASL credential 載入條件

**證據：** [config.go](../internal/config/config.go) 已在 broker 必填條件納入
`cfg.Query.Events`，但 SASL username/password 的載入條件仍只包含
`cfg.AlertMonitoringEnabled || cfg.ServiceabilityLogMonitoringEnabled`。
因此，query-only 加上 events 與 SASL 的組合，可能將空白 SASL credentials
傳入 producer。此次只做靜態確認，未對真實 SASL broker 重現。

**選項與影響：** 可在獨立、依 SDD 授權的修正中納入 query events 條件與測試；
或先採 README 已描述的 query metrics-only／整合 alert-or-Log profile。
後者保持現有流程，前者才能針對 query-only SASL 組合完成驗證。

**建議：** 由 Repository Maintainers 與需求核准者安排修正及 regression tests，
驗證 direct/file secret、缺少 credential、各 SASL mechanism 與 TLS 組合。
此次 README 不宣稱 query-only SASL 已經驗證成功。

### 2. Security policy 尚採兩個 TLS target 的靜態契約

**證據：** [security-policy.sh](../scripts/security-policy.sh) 要求 production source
恰有兩處 `InsecureSkipVerify:`，且只列 Management API 與 Kafka。
[queryclient/client.go](../internal/queryclient/client.go) 也有受設定保護的 TLS client。
本次 security stage 實際回報：
`TLS verification bypass differs from the two approved guarded targets`。

**選項與影響：** 維持原規則會持續阻擋 security gate；依已核准 query 契約更新
精確目標與對應測試，可以保留既有 fail-closed 安全要求。

**建議：** 維護者應在受治理的修正中同步 client/config 測試與 policy，
確認全域及目標雙重 opt-in、CA 衝突、查詢與驗證端點的作用範圍。
此次未降低門檻或修改 policy。

### 3. SDD handoff 尚未完成

**證據：** [TRACEABILITY.md](../TRACEABILITY.md) 中
`REQ-DDAE-10-001` 只有一列，狀態為 `Implemented`。
[Harness SDD 檢查](../HARNESS/lib/sdd.sh) 的 handoff 階段要求 `Verified`。
因此「needs exactly one traceability row with a valid handoff status」不等於發現重複列。

**建議：** 由核准者／維護者完成所需同次驗證與留存證據，再更新 trace status。
不應為使文件或 release 流程通過而直接改成 `Verified`。
已核准的規格／plan basis 與完成交付是不同狀態。

## 部署與文件後續確認

| 項目 | Repository 證據與待確認內容 | 建議／負責角色 |
|---|---|---|
| RC4 與 image 發布 | 本機可見 tag 為 rc1、rc2、rc3；Kubernetes 範例仍引用 `:1.0.0`。本次未查核遠端 registry 的 image 是否存在，也未發布 RC4。 | Release owner 完成 release gates 後確認 tag、產物與 digest；README 以自行建置並指定實際 digest 的流程避免假設已發布 image。 |
| 本機 Docker dependencies | Repository 未提供受版本控制的 Docker Compose 或 Helm chart。先前本機 Kafka/Prometheus 診斷使用 ignored output 中的測試環境，非正式部署套件。 | 若要提供一鍵本機整合環境，另行核准、加入可維護且可驗證的 deployment/test assets；此次不建立虛構指令。 |
| Kubernetes 平台條件 | Manifests 提供 default-deny policy、3Gi PVC、Secret mounts；站點 allow policies、CNI、storage class 與 registry pull 權限是部署環境輸入。 | Platform owner 提供已審核的網路規則、可用儲存與 pull credentials，再進行實際 rollout/metrics 驗證。 |
| 平台版本矩陣 | Go/build toolchain 有明確版本；Repository 尚未宣告最低 Docker、Kubernetes、kubectl 或 systemd 版本矩陣。 | Maintainers 在授權部署環境測試後補上實測版本；README 以需要的功能／API 說明條件。 |
| State 容量 | 各 outbox 預算之外，checkpoint、回補進度與 bbolt overhead 也占用磁碟。3Gi 是 manifest 範例配置。 | Platform owner 依啟用流程、流量及保留時間評估容量；勿把範例容量視為所有工作負載的保證。 |
| 舊文件與架構圖 | `PROJECT.md` 前段仍含 Portable 歷史描述，後段已有 retirement 指示；架構 SVG 主要呈現 Management API 流程。 | Maintainers 可另行整理描述性文件、補充 query/backfill 靜態架構圖，保留既有核准歷史與 digest。 |

以上事項的處理權責屬於 Repository Maintainers、需求核准者、Release owner
或部署環境的 Platform owner；本次文件重寫不推定這些事項已獲核准或完成。
