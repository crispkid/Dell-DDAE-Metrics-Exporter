# 本機 Docker Kafka 認證實測（2026-09-11）

## 結果

**19 個案例全部符合預期，實際從 Kafka 讀回 10 筆查詢事件。**
三種 SASL mechanism 與 mTLS 均完成真實 broker 連線、傳送及消費驗證。
舊 RC4 在 query-only SASL 設定下無法傳送；本機 DDAE-11 修正版以相同
Kafka 帳密與 CA 成功傳送。這是本機補充驗證紀錄，不是 release 核准。

使用者於本次對話明確要求：「請利用本機的Docker安裝Kafka完成真實測試」。
本次依此建立隔離環境，沒有新增產品行為、變更核准的 plan basis、
調整安全門檻、建立 commit 或更新 GitHub Release。

## 實測邊界與環境

- 真實元件：本機 exporter 執行檔、Docker Kafka broker、獨立 Kafka consumer。
- 合成元件：本機 HTTPS Insights / OIDC fixture、查詢資料、帳密與短效憑證。
- 啟用 query monitoring 與 query events；resources、alerts、Serviceability Logs 關閉。
- 測試時間：2026-09-11 01:10:22–01:11:22 UTC（台灣時間 09:10:22–09:11:22）。
- 主機為 macOS arm64；Docker Engine 29.7.2 / Docker Desktop 4.90.0。
- Kafka 4.3.1，broker 回報 commit `26b251a451ce941d`，使用 linux/arm64 image。
- 單節點 KRaft、每個案例獨立 topic、1 partition、replication factor 1。
- SASL_SSL：`127.0.0.1:50014` → Container `19094`。
- mTLS：`127.0.0.1:50015` → Container `19095`，broker 要求 client certificate。
- 上述為此次動態分配的測試 port，已隨清理釋放，不是產品預設 port。
- 客戶端驗證 CA 與 hostname；未啟用 insecure TLS。TLS 由 Kafka 原生處理。
- broker/controller 的內部 PLAINTEXT listener 僅綁定 Container 內的 loopback，未發布。

使用本機已存在的官方 image，以 immutable digest 啟動：

```text
apache/kafka@sha256:77e3df9054047a88b520d0cc46e16696d3b22022e1d580aeccd2632df6532837
```

原生 SASL/TLS 設定參考 Apache Kafka 的
[SASL 文件](https://kafka.apache.org/43/security/authentication-using-sasl/)與
[TLS 文件](https://kafka.apache.org/43/security/encryption-and-authentication-using-ssl/)，
並以此 image 的 CLI 與實際 broker 行為核對。

## 測試矩陣

帳號由 YAML 提供；密碼來源逐一切換。三種 SASL 均使用 SASL_SSL。

| 案例 | 數量 | 實際結果 |
|---|---:|---|
| PLAIN：YAML `password_file`、`KAFKA_SASL_PASSWORD`、`KAFKA_SASL_PASSWORD_FILE` | 3 | 每案成功讀回 1 筆事件 |
| SCRAM-SHA-256：相同三種密碼來源 | 3 | 每案成功讀回 1 筆事件 |
| SCRAM-SHA-512：相同三種密碼來源 | 3 | 每案成功讀回 1 筆事件 |
| 三種 SASL 使用錯誤密碼 | 3 | broker 明確拒絕認證；消費端讀取窗口內為 0 筆，outbox 保留 1 筆 |
| 舊 RC4：三種 SASL、正確帳密、YAML 密碼檔 | 3 | 重現缺陷：讀取窗口內為 0 筆，outbox 保留 1 筆 |
| 不受信任的 broker CA | 1 | TLS 失敗；讀取窗口內為 0 筆，outbox 保留 1 筆 |
| 缺少 SASL 密碼 | 1 | 設定驗證於啟動時拒絕，錯誤指出 `KAFKA_SASL_PASSWORD` |
| mTLS：正確 client certificate / key | 1 | 成功讀回 1 筆事件 |
| mTLS：缺少 client certificate | 1 | TLS 失敗；讀取窗口內為 0 筆，outbox 保留 1 筆 |

RC4 案例的 `passed` 表示「成功重現舊缺陷」，不是 RC4 成功傳送。
合計為 10 個成功傳送、8 個預期未傳送、1 個啟動拒絕案例。

## 如何確認真的送達

每個成功案例啟動實際 exporter，經過 HTTPS 登入、查詢收集、outbox 與
production Kafka producer，再由另一個 Kafka client 消費專用 topic。
測試同時確認：

- `/readyz` 回傳 `200`。
- `ddae_query_detail_collection_success 1`。
- `ddae_query_event_publish_success 1`、`ddae_query_events_pending 0`。
- 消費到 1 筆事件，record key 等於來源與 query ID 組成的 SHA-256 bytes。
- 三個 Kafka headers 分別包含 JSON content type、schema `1.0`、record kind `query`。
- JSON 符合欄位 allowlist，含正確的 query ID、來源、finished 狀態與秒數轉換。
- 事件未包含 SQL canary 或 Kafka 密碼；完成傳送流程的案例亦檢查 exporter log 未出現測試帳密。

預期未傳送案例確認收集成功，但 `/readyz` 為 `503`、publish success 為 `0`，
且 pending 為 `1`；消費端以 2 秒的有界讀取窗口確認未收到事件。
這是短時間功能驗證，不代表長時間重送、負載、故障切換或去重保證。

錯誤密碼另以 production publisher 與 broker log 交叉核對。
三種 SASL 均出現 broker 的 invalid-credentials 拒絕紀錄。
此輪 PLAIN、SCRAM-SHA-256 的 publisher error class 為 `kafka_timeout`，
SCRAM-SHA-512 為 `kafka_auth`：重試期間可能先遇到設定的 2 秒 publish deadline。
因此不能僅靠 `kafka_timeout` 判定根因；應同時核對 broker 認證紀錄。
本次未更動產品的 timeout、retry 或錯誤分類行為。

## 執行與證據留存

本機診斷 helper 位於 Git 忽略的 `output/_kafka-real-test/`，使用
`//go:build ignore`。它不是受版本控制的部署套件，也未加入正式 Harness stage。
以下是此工作目錄實際執行的主要指令；helper 與 RC4 本機產物須保留，才可重跑。
每次重跑先執行 `setup`，並將新輸出的 manifest 路徑用於 `test` 和 `cleanup`：

```bash
./scripts/build.sh
go run output/_kafka-real-test/setup.go output/_kafka-real-test/verify.go setup
go run output/_kafka-real-test/setup.go output/_kafka-real-test/verify.go test 'output/_kafka-sasl-2821538818/manifest.json'
go run output/_kafka-real-test/setup.go output/_kafka-real-test/verify.go cleanup 'output/_kafka-sasl-2821538818/manifest.json'
```

本輪摘要：[results.json](../output/_kafka-sasl-2821538818/results.json)。
該檔案與 helper 僅保留於本機；乾淨 checkout 不包含這些 ignored artifacts。
此文件保留不含 secret 的結果及其 SHA-256 身分：

| 證據 | SHA-256 |
|---|---|
| 測試時的 `bin/ddae-exporter` | `06af77c8b971dd12ab68dc2a4cef03539919f44d2829cb37b918dca6e6c4a3f1` |
| RC4 darwin/arm64 執行檔 | `413bdbf78af674b4b02aa809b8b81472294854956aa49b447702ed09f0549b7e` |
| `setup.go` | `79bd46e318b63499141911a465ab511e477e92f0d8226a25ebb31e288d419f44` |
| `verify.go` | `c17e37fbfb7958bd42e22e6107ddbfaa2556cc4d8a31fb3c8e51ff6998104dd5` |
| `results.json` | `b54ea76759bc49b9631f8006b343eae817d38ec1918670f1b92abe9527bb2f83` |

測試設定、憑證與合成 secrets 留在權限受限的 ignored output 目錄；
不應將整個目錄加入 Git 或當作公開 release 附件。

## 環境排查與清理

前兩次環境探索未通過，不計入最後的 19 個成功案例：

1. `output/_kafka-sasl-1905348221`：Docker Desktop 上的 internal network
   未建立可供主機連線的 port mapping；改用此測試專用 bridge network，
   仍僅發布 host loopback port。
2. `output/_kafka-sasl-1484411641`：已能登入 Kafka，但 `/tmp` 的 noexec
   設定阻止 Snappy JNI shared library 載入，訊息無法寫入。
   最後一輪使用 `/tmp:rw,exec,size=768m,mode=1777` 的有界 tmpfs，
   保留 Container read-only root filesystem，重新建立 broker 並跑完整矩陣。
   此輪亦修正診斷 helper 對錯誤密碼必定回傳 `kafka_auth` 的過度限定；
   最終仍要求實際 publisher 失敗與 broker 明確拒絕認證，並保存真實 error class。

三輪測試的 Kafka Container 與專用 Network 均已清理。
Container 內的測試 topic / broker data 是可丟棄資料，已隨 Container 移除；
消費結果摘要仍保留。Kafka image 與本機診斷證據保留，未操作其他服務。

## 後續交付邊界

本次證明真實 Kafka 端的功能，不是正式 DDAE → Kafka 全鏈路驗證。
未連線真實 DDAE、未啟動 Prometheus server、未測 OpenSearch、Kubernetes、
systemd、Windows、長時間負載、多 broker 或 SASL 與 mTLS 同時啟用的組合。
本次只直接讀取 exporter 的 `/metrics`。

正式 integration / E2E 環境、獨立安全審查與既有 release gates 仍需完成。
既有 security policy 的 TLS target 計數問題保持原狀；未將 trace rows 改為 Verified。
後續 Harness 結果與目前交付狀態見 [DDAE-11 執行紀錄](../plans/DDAE-11.md)。
已發布的 RC4 產物未更新，需另行授權 commit / release 才會交付修正版產物。
