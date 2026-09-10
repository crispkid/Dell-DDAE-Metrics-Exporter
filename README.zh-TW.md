# Dell DDAE Metrics Exporter

[English](README.md) | 繁體中文

收集 Dell Data Analytics Engine 運作資料，提供 Prometheus metrics，並將結構化的 Serviceability 與查詢事件傳送至 Kafka。

[開始使用](#開始使用) · [設定參考](#設定參考) · [部署](#部署) · [疑難排解](#疑難排解)

## 專案概述

Dell DDAE Metrics Exporter 是提供給平台工程師、SRE 與 DDAE 維運人員的唯讀 Go 服務。每個 process 監控一個來源設備。資源收集採用 DDAE 1.5.0 Management API 契約；查詢監控使用獨立的 Insights session，以及在 engine `479-e.4` 觀察到的回應結構。

先啟動資源監控，再依環境需求啟用告警、Serviceability Log、查詢與有界限的歷史回補流程。Prometheus 讀取已收集的快照；Kafka consumer 負責後續索引與通知。

## 核心功能

- **資源可視性** — 設備可用性、叢集設定、節點就緒狀態、CPU／記憶體／儲存容量及系統鎖定狀態。
- **Prometheus 監控** — 資源指標、執行中與排隊查詢數、已觀測查詢的耗時分布，以及收集與傳送狀態。
- **結構化 Kafka 事件** — Serviceability 告警、Serviceability Log 與選用的查詢明細各自使用獨立 topic。
- **持久化處理** — 使用本機 bbolt checkpoint 與有容量上限的 outbox，支援重新啟動後復原及依 acknowledgement 確認傳送。
- **歷史回補** — 以獨立排程、有界限且可續跑的掃描收集 Serviceability Log 與查詢歷史。
- **明確設定** — 嚴格驗證的版本化 YAML、各流程獨立開關、Environment Variable 覆寫與 credential files。
- **安全連線** — 驗證 HTTPS 憑證、載入自訂 CA bundle，以及 Kafka mTLS 與 PLAIN／SCRAM 驗證。
- **部署範例** — 靜態執行檔、非 root Docker image、Kubernetes manifests 與強化安全設定的 systemd service。

## 系統架構

![Management API 流程架構](docs/architecture.svg)

上圖呈現 Management API 流程。資源 collector 更新記憶體中的快照，供 HTTP server 提供 metrics。告警與 Serviceability Log 分別經過獨立的本機 outbox，再傳送至 Kafka。

查詢流程獨立向 Insights 與 identity provider 驗證身分，儲存查詢 checkpoint 並更新 Prometheus metrics。啟用查詢事件後，會使用專用 Kafka topic。歷史掃描器將資料送入既有的 Log／查詢處理流程，並另外保存掃描進度。Kafka consumer 負責後續處理。

詳細設計請參閱[架構決策](docs/decisions/0001-ddae-v1-architecture.md)、[查詢監控指南](docs/query-monitoring.md)與[歷史回補指南](docs/history-backfill.md)。

## 技術組成

| 層級 | 技術 | 用途 |
|---|---|---|
| Runtime 與建置 | Go 1.26.6 | 靜態 exporter 執行檔與驗證工具 |
| HTTP | Go `net/http` | 收集端 client、metrics、liveness 與 readiness |
| Metrics | Prometheus Go client | Prometheus／OpenMetrics 輸出 |
| 訊息傳送 | Kafka、`franz-go` | 事件傳送 |
| 本機持久化 | bbolt | Outbox、checkpoint 與歷史掃描進度 |
| 設定 | YAML v3、Environment Variables | 嚴格驗證的啟動設定 |
| 資源解析 | Kubernetes apimachinery | CPU、記憶體與儲存容量單位解析 |
| 封裝 | Multi-stage Docker build、scratch runtime | 精簡的非 root image |
| 維運 | Kubernetes、systemd、GitHub Actions | 部署範例與驗證流程 |

## 目錄結構

```text
.
├── cmd/ddae-exporter/      # Process entry point
├── internal/              # Clients, collectors, metrics, state, and publishers
├── deploy/                # YAML examples, Kubernetes, and systemd
├── docs/                  # Operations, query, backfill, and architecture guides
├── integration/           # Authorized integration and deployment tests
├── testdata/              # Sanitized API fixtures and event examples
├── scripts/               # Build, test, security, and supply-chain stages
├── HARNESS/               # Verification runner and committed stage policy
├── .github/workflows/     # CI workflow
├── Dockerfile             # Static, non-root container image
└── go.mod                 # Go toolchain and dependency versions
```

## 開始使用

本流程從原始碼建置，連線至你的 DDAE endpoint，並且**只啟用資源監控**。需要 DDAE 帳號與 HTTPS 連線；啟用事件流程時再準備 Kafka。除非程式碼區塊明確啟動 Bash，以下指令均在 Repository 根目錄以 Bash 或 zsh 執行。執行前請替換所有 `<...>` placeholder。

### 1. 準備工具與存取權限

| 需求 | 版本／適用範圍 | 用途 | 檢查方式 |
|---|---|---|---|
| macOS 或 Linux | 原始碼／Harness 流程；Bash 3.2+ | 執行文件中的 shell 流程 | `uname -s; uname -m; bash --version` |
| Git | 取得原始碼 | Clone Repository | `git --version` |
| Go | 建置腳本要求實際使用的 toolchain 為 **1.26.6**；module 宣告 Go 1.26.0 | 編譯與下載 modules | `go version; go env GOVERSION GOOS GOARCH` |
| curl | 本機驗證 | 檢查 HTTP endpoint | `curl --version` |
| DDAE Management API | DDAE 1.5.0 契約 | 資源、告警與 Log 資料來源 | 已授權的 HTTPS origin 與帳號 |
| Docker | 選用：image 流程 | 建置／執行 Dockerfile | `docker version` |
| kubectl 與 cluster 存取權 | 選用：Kubernetes 流程 | 套用 Repository 中的 manifests | `kubectl version --client; kubectl cluster-info` |
| 支援 `LoadCredential` 的 systemd | 選用：Linux service 流程 | 執行 Repository 中的 service unit | `systemctl --version` |

透過作業系統的套件管理工具安裝 Git、Bash 與 curl。依 OS／CPU，參考 [Go 下載頁](https://go.dev/dl/)與[安裝指南](https://go.dev/doc/install/)安裝 Go **1.26.6**，接著確認 `go version`。執行檔會依實際的 `GOOS`／`GOARCH` 建置。Windows 開發者可在 WSL 使用 Linux 流程，詳見[跨機器開發指南](docs/development-portability.md)。使用 Docker 或 Kubernetes 時，先依平台的安裝程序備妥對應 CLI／runtime。

向 DDAE 管理者取得：

- HTTPS **origin**，例如 `https://ddae.example.invalid`；若使用 `443` 以外的 port，請一併提供。
- 具備所需 GET endpoint 權限、採最小權限原則的唯讀帳號與密碼。
- `dv-admin-rest` OAuth client 的 client secret。
- 使用私有 CA 時所需的 PEM CA bundle，以及正確的 Ping／API prefix。

Exporter 主機必須能解析該 hostname、連線至 HTTPS port，並信任名稱相符的伺服器憑證。請保留本機 TCP port `9469`，或指定其他明確的監聽位址。本機 process 需要讀取設定、credential files 與 CA files 的權限。

### 2. 取得原始碼並建置

Clone Repository，並記錄使用中的 revision：

```bash
git clone https://github.com/crispkid/Dell-DDAE-Metrics-Exporter.git
cd Dell-DDAE-Metrics-Exporter
git rev-parse HEAD
go version
go mod download
./scripts/build.sh
test -x bin/ddae-exporter
```

產物為 `bin/ddae-exporter`。下載 modules 需要連線至你設定的 Go module 來源；安裝 Go modules 與安裝編譯器／runtime 是不同步驟。

### 3. 建立本機目錄與 credential files

以下目錄均由 Git 忽略。資源監控使用記憶體保存快照；state 目錄則預先提供給後續事件／查詢流程使用。

```bash
umask 077
mkdir -p secrets trust-local state
chmod 700 secrets trust-local state
pwd
```

輸入管理者提供的三個值。以下 Bash 區塊以隱藏輸入方式讀取，避免值出現在畫面或 shell history：

```bash
bash <<'BASH'
set -eu
umask 077
for name in ddae-username ddae-password ddae-client-secret; do
  IFS= read -r -s -p "$name: " value </dev/tty
  printf '\n' >/dev/tty
  test -n "$value"
  printf '%s' "$value" > "secrets/$name"
  chmod 600 "secrets/$name"
  unset value
done
BASH
```

使用私有 CA 時，複製管理者提供的**公開 CA 憑證 bundle**；私鑰請分開保管：

```bash
cp "<path-to-ddae-ca-bundle.pem>" trust-local/ddae-ca.pem
chmod 600 trust-local/ddae-ca.pem
```

### 4. 建立 runtime 設定

將以下內容存成 Repository 根目錄的 `config.local.yaml`。修改 `ddae.base_url`，並以 `pwd` 輸出取代 `<absolute-repository-path>`。將 `ca_file` 設為剛複製的 CA file 絕對路徑；若使用系統信任的 CA，保留 `""`。首次啟動請保留範例中只啟用資源監控的開關。

<!-- quick-start-config:start -->
```yaml
version: 1
server:
  listen_address: 127.0.0.1:9469
  shutdown_grace_period: 15s
monitoring:
  resources:
    enabled: true
    interval: 30s
    stale_after: 120s
  alerts:
    enabled: false
  serviceability_logs:
    enabled: false
  queries:
    enabled: false
ddae:
  base_url: https://ddae.example.invalid
  paths:
    ping_prefix: ""
    api_prefix: /v1
  credentials:
    username_file: <absolute-repository-path>/secrets/ddae-username
    password_file: <absolute-repository-path>/secrets/ddae-password
    client_secret_file: <absolute-repository-path>/secrets/ddae-client-secret
  tls:
    ca_file: ""
    insecure_skip_verify: false
  request_timeout: 5s
  cycle_timeout: 20s
  response_max_bytes: 4194304
  retry_max: 2
state:
  dir: <absolute-repository-path>/state
security:
  allow_insecure_tls: false
logging:
  level: info
  format: json
```
<!-- quick-start-config:end -->

```bash
chmod 600 config.local.yaml
./bin/ddae-exporter --config "$PWD/config.local.yaml"
```

### 5. 驗證服務

讓 exporter 在前景執行。在另一個 terminal 檢查 liveness，並在第一輪收集完成後檢查 readiness：

```bash
curl --fail --show-error http://127.0.0.1:9469/healthz
curl --fail --show-error http://127.0.0.1:9469/readyz
curl --fail --show-error http://127.0.0.1:9469/metrics
```

預期結果：`/healthz` 回傳 HTTP `200` 與 `alive`；`/readyz` 回傳 HTTP `200` 與 `ready`。成功收集後，資源 metrics 包含 `ddae_up 1` 與 `ddae_management_api_up 1`。首次收集期間，readiness 可能回傳 `503`；若持續發生，請參考疑難排解章節。

安裝完成後確認：

- 執行檔持續運作，liveness 回傳 `200`。
- 收集完成後，readiness 回傳 `200`。
- `/metrics` 包含 DDAE 資源 metrics 與收集成功狀態。
- DDAE HTTPS 連線與 credential-file 權限正常。

在 exporter terminal 按 `Ctrl+C` 可正常關閉服務。完成此基礎驗證後，再啟用選用流程或採用部署範例。

## 設定參考

### 設定檔、優先順序與值格式

依執行方式選用設定範例：

| 執行方式 | 設定檔 | Credentials／state |
|---|---|---|
| 本機 process | `config.local.yaml`，以 `--config` 指定 | 本機 credential files；絕對路徑的 `state.dir` |
| 所有流程的完整設定 | [deploy/systemd/config.example.yaml](deploy/systemd/config.example.yaml) | 依環境修改範例路徑 |
| 查詢專用起始設定 | [deploy/query-monitoring.example.yaml](deploy/query-monitoring.example.yaml) | 獨立的查詢 credentials 與持久化 state |
| Docker | 將本機 YAML 掛載至 `/etc/ddae-exporter/config.yaml` | 唯讀 Secret 掛載；可寫入的 state 掛載 |
| Kubernetes | [deploy/kubernetes/configmap.yaml](deploy/kubernetes/configmap.yaml) | [deployment.yaml](deploy/kubernetes/deployment.yaml) 中的 Secret 與 PVC |
| systemd | `/etc/ddae-exporter/config.yaml` | `LoadCredential` 與 `/var/lib/ddae-exporter` |

**選擇設定檔：** `--config <path>` 優先於 `DDAE_EXPORTER_CONFIG_FILE`。兩者皆未指定時，使用 Environment Variables 與預設值。請明確指定要載入的 YAML。

**個別設定優先順序：** Environment Variable → 指定 YAML 中的欄位 → 內建預設值。`DDAE_COLLECTION_INTERVAL` 是資源與告警 interval 的相容別名：流程專用的環境設定 → 別名 → 該流程的 YAML 值 → 預設值。

**重新載入：** 設定、credentials 與 CA files 於啟動時讀取。修改後請重新啟動 process；Kubernetes 的 ConfigMap／Secret 更新後，也要重新 rollout pods。

**值格式：** YAML 使用 `version: 1`、單一文件與明確欄位名稱，並採嚴格驗證。開關使用 `true`／`false`，大小使用整數 bytes，時間使用 `30s`、`10m` 或 `720h` 等 Go duration。Origin URL 採 `https://hostname[:port]`，路徑 prefix 另外設定。`state.dir` 必須是絕對路徑。至少啟用一個監控流程。

下列表格採用點號分隔的 YAML 路徑。「必填」條件在對應流程啟用時適用。空字串預設值以 `""` 表示。

### Server、Logging 與傳輸安全

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `server.listen_address` | `EXPORTER_LISTEN_ADDRESS` | `127.0.0.1:9469` | 明確的 host:port；port 為 1–65535。 |
| `server.shutdown_grace_period` | `SHUTDOWN_GRACE_PERIOD` | `15s` | 正常關閉服務的時間上限，須為正值。 |
| `logging.level` | `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error`。 |
| `logging.format` | `LOG_FORMAT` | `json` | `json` 或 `text`。 |
| `security.allow_insecure_tls` | `ALLOW_INSECURE_TLS` | `false` | 允許各目標 TLS 驗證略過設定生效的診斷確認開關。 |

部署時請使用 CA 驗證。個別目標的 `insecure_skip_verify: true` 需搭配 `security.allow_insecure_tls: true` 才會生效。同一目標的自訂 CA 與略過驗證設定互斥。受控的診斷流程請參閱 [TLS 維運指南](docs/runbook.md)。

### DDAE 連線與身分驗證

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `ddae.base_url` | `DDAE_BASE_URL` | `""` | 資源、告警或 Log 啟用時必填；HTTPS origin。 |
| `ddae.source_instance` | `DDAE_SOURCE_INSTANCE` | `""` | 告警、Log 或查詢啟用時必填；固定識別名稱，1–128 UTF-8 bytes，使用名稱而非 URL。 |
| `ddae.credentials.username_file` | `DDAE_USERNAME_FILE` | `""` | Management API 所需的帳號檔案。 |
| `ddae.credentials.password_file` | `DDAE_PASSWORD_FILE` | `""` | Management API 所需的密碼檔案。 |
| `ddae.credentials.client_secret_file` | `DDAE_CLIENT_SECRET_FILE` | `""` | 必要的 `dv-admin-rest` client-secret 檔案。 |
| `ddae.tls.ca_file` | `DDAE_CA_FILE` | `""` | 加入系統信任根憑證的 PEM CA bundle。 |
| `ddae.tls.insecure_skip_verify` | `DDAE_TLS_INSECURE_SKIP_VERIFY` | `false` | 受全域開關保護的 DDAE TLS 診斷設定。 |
| `ddae.paths.ping_prefix` | `DDAE_PING_PATH_PREFIX` | `""` | `/ping` 前方的 prefix。 |
| `ddae.paths.api_prefix` | `DDAE_API_PATH_PREFIX` | `/v1` | Management API GET 路徑的 prefix。 |
| `ddae.request_timeout` | `DDAE_REQUEST_TIMEOUT` | `5s` | 每次 request 的 timeout，須為正值。 |
| `ddae.cycle_timeout` | `DDAE_CYCLE_TIMEOUT` | `20s` | 每輪收集的 timeout，須為正值。 |
| `ddae.response_max_bytes` | `DDAE_RESPONSE_MAX_BYTES` | `4194304` | 資源／驗證回應大小上限；1–67108864 bytes。 |
| `ddae.retry_max` | `DDAE_RETRY_MAX` | `2` | 重試次數上限，0–10。 |

**Credentials 處理。** YAML 透過檔案路徑引用 credentials。也可使用 `DDAE_USERNAME`、`DDAE_PASSWORD`、`DDAE_CLIENT_SECRET`、`QUERY_USERNAME`、`QUERY_PASSWORD` 與 `KAFKA_SASL_PASSWORD` 直接提供環境設定。每個 credential 的直接 Environment Variable 與 `_FILE` Environment Variable 擇一設定；同時提供會被驗證拒絕。直接的環境設定值會覆寫 YAML 中對應的檔案引用。

Credential files 必須是非空白的一般 UTF-8 檔案，上限為 `64 KiB`；檔案最後的換行會被移除。只開放預期的執行帳號讀取，並透過 Secret manager、container 掛載或 systemd credentials 注入。真實 endpoint、設定與 Secret 請放在安裝指南使用的 Git 忽略路徑中。

**API 路徑組合。** 預設會呼叫 `GET /ping` 與 `GET /v1/ddae-clusters`。明確的 prefix 決定 runtime 路徑，取代 runtime discovery 與 alternate-path fallback。

| 組合 | `ddae.paths.ping_prefix` | `ddae.paths.api_prefix` |
|---|---|---|
| Default | `""` | `/v1` |
| RC2 compatibility | `/rest/v1` | `/rest/v1` |
| Dell PDF layout | `/rest` | `/rest/v1` |

Prefix 最大長度是 128 bytes。可使用空字串，或由斜線分隔、各段採 `A–Z a–z 0–9 . _ ~ -` 字元的標準化絕對路徑；各段使用一般名稱，結尾保留完整路徑段。請與 DDAE 管理者確認適用的組合。

### 資源、告警與 Log 收集

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `monitoring.resources.enabled` | `DDAE_RESOURCE_MONITORING_ENABLED` | `true` | 啟用資源快照與 metrics。 |
| `monitoring.resources.interval` | `DDAE_RESOURCE_COLLECTION_INTERVAL` | `30s` | 資源收集間隔。 |
| `monitoring.resources.stale_after` | `DDAE_STALE_AFTER` | `120s` | Management API 流程的資料新鮮度上限。 |
| `monitoring.alerts.enabled` | `DDAE_ALERT_MONITORING_ENABLED` | `true` | 啟用告警收集與 Kafka 傳送。 |
| `monitoring.alerts.interval` | `DDAE_ALERT_COLLECTION_INTERVAL` | `30s` | 告警收集間隔。 |
| `monitoring.alerts.list_response_max_bytes` | `ALERT_LIST_RESPONSE_MAX_BYTES` | `8388608` | 清單回應上限，1–67108864 bytes。 |
| `monitoring.alerts.detail.response_max_bytes` | `ALERT_DETAIL_RESPONSE_MAX_BYTES` | `1048576` | 明細回應上限，1–67108864 bytes。 |
| `monitoring.alerts.detail.refresh_interval` | `ALERT_DETAIL_REFRESH_INTERVAL` | `10m` | 明細重新讀取間隔；至少等於收集間隔。 |
| `monitoring.alerts.detail.max_per_cycle` | `ALERT_DETAIL_MAX_PER_CYCLE` | `200` | 每輪明細處理上限，1–10000。 |
| `monitoring.alerts.detail.concurrency` | `ALERT_DETAIL_CONCURRENCY` | `4` | Worker 數，1–128；最多等於每輪明細上限。 |
| `monitoring.serviceability_logs.enabled` | `DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED` | `false` | 啟用 Serviceability Log 收集與傳送。 |
| `monitoring.serviceability_logs.interval` | `DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL` | `30s` | Log 收集間隔。 |
| `monitoring.serviceability_logs.list_response_max_bytes` | `SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES` | `8388608` | 清單回應上限，1–67108864 bytes。 |
| `monitoring.serviceability_logs.detail.response_max_bytes` | `SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES` | `1048576` | 明細回應上限，1–67108864 bytes。 |
| `monitoring.serviceability_logs.detail.refresh_interval` | `SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL` | `10m` | 明細重新讀取間隔；至少等於收集間隔。 |
| `monitoring.serviceability_logs.detail.max_per_cycle` | `SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE` | `200` | 每輪明細處理上限，1–10000。 |
| `monitoring.serviceability_logs.detail.concurrency` | `SERVICEABILITY_LOG_DETAIL_CONCURRENCY` | `4` | Worker 數，1–128；最多等於每輪明細上限。 |

對每個啟用的 Management API 流程，維持 `request_timeout < cycle_timeout < interval`。啟用資源監控時，另須符合 `interval < stale_after`。

### Kafka 傳輸與 Topic

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `kafka.brokers` | `KAFKA_BROKERS` | `[]` | 事件傳送必填；1–64 個 broker 位址。YAML 使用 list，環境值以逗號分隔。 |
| `kafka.topic` | `KAFKA_TOPIC` | `""` | 啟用告警時必填的告警 topic。 |
| `kafka.serviceability_logs_topic` | `KAFKA_SERVICEABILITY_LOG_TOPIC` | `ddae-serviceability-logs` | 專用 Log topic。 |
| `kafka.client_id` | `KAFKA_CLIENT_ID` | `ddae-exporter` | Kafka client 識別名稱，1–128 bytes。 |
| `kafka.tls.ca_file` | `KAFKA_CA_FILE` | `""` | 加入系統信任根憑證的 PEM CA bundle。 |
| `kafka.tls.client_cert_file` | `KAFKA_CLIENT_CERT_FILE` | `""` | mTLS 憑證；與 client key 一起提供。 |
| `kafka.tls.client_key_file` | `KAFKA_CLIENT_KEY_FILE` | `""` | mTLS 私鑰檔案。 |
| `kafka.tls.insecure_skip_verify` | `KAFKA_TLS_INSECURE_SKIP_VERIFY` | `false` | 受全域開關保護的 Kafka TLS 診斷設定。 |
| `kafka.sasl.mechanism` | `KAFKA_SASL_MECHANISM` | `""` | 空字串、`PLAIN`、`SCRAM-SHA-256` 或 `SCRAM-SHA-512`。 |
| `kafka.sasl.username` | `KAFKA_SASL_USERNAME` | `""` | 使用 SASL 時必填。 |
| `kafka.sasl.password_file` | `KAFKA_SASL_PASSWORD_FILE` | `""` | 使用 SASL 時必填的密碼檔案；直接值替代設定為 `KAFKA_SASL_PASSWORD`。 |
| `kafka.publish_timeout` | `KAFKA_PUBLISH_TIMEOUT` | `10s` | 傳送 timeout，至少 `1s`。 |

Kafka 連線使用 TLS。請設定 broker 的 TLS listener，並確保 advertised broker 位址均可連線；部署範例中的 `9093` 是 listener port 範例。私有 CA 提供 CA bundle，mTLS 提供憑證／私鑰組，broker 要求 SASL 時再提供對應值。啟用的事件類型使用不同 topic；topic 名稱為 1–249 bytes，並須通過設定中的名稱驗證。

### 持久化 State

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `state.dir` | `STATE_DIR` | `/var/lib/ddae-exporter` | 絕對路徑；有持久化需求的流程須可由 runtime 帳號寫入。 |
| `state.outbox_max_bytes` | `KAFKA_OUTBOX_MAX_BYTES` | `1073741824` | 告警 outbox 容量上限，須為正值 bytes。 |
| `state.outbox_max_events` | `KAFKA_OUTBOX_MAX_EVENTS` | `100000` | 告警事件筆數上限，1–10000000。 |
| `state.checkpoint_retention` | `CHECKPOINT_RETENTION` | `720h` | 告警 checkpoint 保留時間，須為正值。 |
| `state.checkpoint_max_alerts` | `CHECKPOINT_MAX_ALERTS` | `100000` | 告警 checkpoint 筆數上限，1–10000000。 |
| `state.serviceability_logs_outbox_max_bytes` | `SERVICEABILITY_LOG_OUTBOX_MAX_BYTES` | `1073741824` | Log outbox 容量上限，須為正值 bytes。 |
| `state.serviceability_logs_outbox_max_events` | `SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS` | `100000` | Log 事件筆數上限，1–10000000。 |
| `state.serviceability_logs_checkpoint_retention` | `SERVICEABILITY_LOG_CHECKPOINT_RETENTION` | `720h` | Log checkpoint 保留時間，須為正值。 |
| `state.serviceability_logs_checkpoint_max_records` | `SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS` | `100000` | Log checkpoint 筆數上限，1–10000000。 |

啟動時，各個啟用的流程會初始化自己的 bbolt 檔案與內部 bucket：

| 流程 | `state.dir` 內的檔案 |
|---|---|
| 告警 | `state.db` |
| Serviceability Log | `serviceability-logs.db` |
| 查詢，包含只輸出 metrics 的模式 | `query-events.db` |
| 已啟用的歷史回補 | `history-backfill.db` |

應用程式負責初始化與驗證既有 state。每個 state 目錄只供一個 writer 使用，filesystem 須支援 file lock 與 sync；目錄權限採 `0700`，資料庫檔案採 `0600`。重新啟動時維持相同的 `source_instance`。磁碟空間需涵蓋設定的 outbox 容量，**加上** checkpoint、歷史進度與資料庫額外空間。備份所有 state files 前先停止 exporter，升級時保留原目錄。詳見[維運指南](docs/runbook.md)與[歷史回補復原指南](docs/history-backfill.md)。

### 查詢監控

以下 YAML key 均位於 `monitoring.queries` 之下。查詢 credentials 與 Management API credentials 獨立。啟用查詢流程需設定 Insights origin、identity-provider origin、realm、具備檢視所有查詢權限的 role、來源識別名稱與持久化 state。

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `enabled` | `QUERY_ENABLED` | `false` | 啟用查詢收集與 metrics。 |
| `events_enabled` | `QUERY_EVENTS_ENABLED` | `false` | 另將選定的查詢明細傳送至 Kafka。 |
| `base_url` | `QUERY_BASE_URL` | `""` | 必填的 Insights HTTPS origin。 |
| `auth_url` | `QUERY_AUTH_URL` | `""` | 必填的 identity-provider HTTPS origin。 |
| `realm` | `QUERY_REALM` | `""` | 必填 realm：1–128 個英數字、底線或連字號。 |
| `role` | `QUERY_ROLE` | `""` | 必填的全查詢檢視 role；字元規則與 realm 相同。 |
| `username_file` | `QUERY_USERNAME_FILE` | `""` | 必填的查詢帳號檔案。 |
| `password_file` | `QUERY_PASSWORD_FILE` | `""` | 必填的查詢密碼檔案。 |
| `ca_file` | `QUERY_CA_FILE` | `""` | Insights CA bundle。 |
| `auth_ca_file` | `QUERY_AUTH_CA_FILE` | `""` | Identity-provider CA bundle。 |
| `insecure_skip_verify` | `QUERY_TLS_INSECURE_SKIP_VERIFY` | `false` | 受全域開關保護的查詢／驗證 TLS 診斷設定。 |
| `kafka_topic` | `QUERY_KAFKA_TOPIC` | `""` | 啟用查詢事件時必填；使用獨立的查詢 topic。 |
| `interval` | `QUERY_INTERVAL` | `30s` | 收集間隔。 |
| `request_timeout` | `QUERY_REQUEST_TIMEOUT` | `5s` | 每次 request 的 timeout。 |
| `cycle_timeout` | `QUERY_CYCLE_TIMEOUT` | `20s` | 每輪收集的 timeout。 |
| `stale_after` | `QUERY_STALE_AFTER` | `90s` | 查詢資料新鮮度上限。 |
| `response_max_bytes` | `QUERY_RESPONSE_MAX_BYTES` | `16777216` | 回應大小上限，1–67108864 bytes。 |
| `max_history_records` | `QUERY_MAX_HISTORY_RECORDS` | `1000` | 可接受的歷史清單上限，1–10000 筆。 |
| `detail_max_per_cycle` | `QUERY_DETAIL_MAX_PER_CYCLE` | `100` | 每輪明細處理上限，1–10000。 |
| `detail_concurrency` | `QUERY_DETAIL_CONCURRENCY` | `4` | Worker 數，1–32；最多等於每輪明細上限。 |
| `retry_max` | `QUERY_RETRY_MAX` | `2` | 重試次數上限，0–10。 |
| `checkpoint_retention` | `QUERY_CHECKPOINT_RETENTION` | `720h` | 查詢 checkpoint 保留時間，須為正值。 |
| `checkpoint_max_records` | `QUERY_CHECKPOINT_MAX_RECORDS` | `100000` | Checkpoint 筆數上限，1–1000000。 |
| `outbox_max_bytes` | `QUERY_OUTBOX_MAX_BYTES` | `268435456` | 查詢 outbox 容量上限，1–1073741824 bytes。 |
| `outbox_max_events` | `QUERY_OUTBOX_MAX_EVENTS` | `10000` | 查詢 outbox 筆數上限，1–1000000。 |

維持 `request_timeout < cycle_timeout < interval < stale_after`。查詢 session 會先確認全查詢檢視範圍，再接受資料。耗時 metrics 描述此 exporter 觀測到的終止狀態查詢，並透過持久化紀錄去重；請搭配下方的收集、權限範圍與涵蓋狀態 metrics 判讀。

### 歷史回補

`monitoring.serviceability_logs.backfill` 與 `monitoring.queries.backfill` 共用以下欄位。Environment Variable 的 prefix 分別為 `SERVICEABILITY_LOG_BACKFILL_` 與 `QUERY_BACKFILL_`，再接上表格中的 suffix。

| YAML key | Environment Variable | 預設值 | 必填條件／用途 |
|---|---|---|---|
| `enabled` | `ENABLED` | `false` | 啟用所屬流程的歷史掃描器。 |
| `lookback` | `LOOKBACK` | `24h` | 1h–720h；最多等於所屬流程的 checkpoint 保留時間。 |
| `overlap` | `OVERLAP` | `2m` | 1s–1h；最多等於 lookback。 |
| `interval` | `INTERVAL` | `30s` | 掃描間隔，5s–1h。 |
| `cycle_timeout` | `CYCLE_TIMEOUT` | `20s` | 每輪掃描時間上限，1s–1h。 |
| `rescan_interval` | `RESCAN_INTERVAL` | `1h` | 5s–720h；介於 interval 與 lookback 之間。 |
| `max_pages_per_cycle` | `MAX_PAGES_PER_CYCLE` | `4` | 每輪頁數上限，1–32。 |
| `detail_max_per_cycle` | `DETAIL_MAX_PER_CYCLE` | `25` | 每輪明細處理上限，1–1000。 |
| `detail_concurrency` | `DETAIL_CONCURRENCY` | `2` | Worker 數，1–8；最多等於每輪明細上限。 |
| `max_pending_records` | `MAX_PENDING_RECORDS` | `10000` | 待處理筆數上限，1000–100000。 |

先啟用所屬流程，並維持 `所屬流程 request_timeout < backfill.cycle_timeout < backfill.interval`。查詢回補另要求 `max_history_records >= 1000`。即時收集與回補的明細／並行上限會加總，請依上游服務承載量一起調整。掃描器會持久化時間窗與待處理 ID、限制時間窗切分數量，並在重新啟動後繼續處理。修改來源識別、路徑設定、overlap 或 state files 前，請參閱[歷史回補指南](docs/history-backfill.md)。

## 啟用其他流程

### 透過 Kafka 傳送告警與 Serviceability Log

先完成資源監控驗證，再停止 exporter。

1. 請 Kafka 管理者建立告警 topic；需要 Log 時，另建獨立的 Log topic。取得 TLS bootstrap／advertised 位址、CA 資料，以及可傳送至這些 topic 的 producer 身分與 ACL。Producer 採 idempotent delivery 與 all-replica acknowledgement，請配置相應權限。
2. 使用 SASL 時，以隱藏輸入方式建立 `secrets/kafka-password`：

```bash
bash <<'BASH'
set -eu
umask 077
IFS= read -r -s -p "kafka-password: " value </dev/tty
printf '\n' >/dev/tty
test -n "$value"
printf '%s' "$value" > secrets/kafka-password
chmod 600 secrets/kafka-password
unset value
BASH
```

在 YAML 設定 Kafka 帳號與 mechanism。使用私有 CA 時，複製管理者提供的 bundle：

```bash
cp "<path-to-kafka-ca-bundle.pem>" trust-local/kafka-ca.pem
chmod 600 trust-local/kafka-ca.pem
```

使用 mTLS 時，同時提供 client certificate 與 private-key file。

3. 複製完整設定，保留資源監控設定供對照：

```bash
cp deploy/systemd/config.example.yaml config.events.local.yaml
chmod 600 config.events.local.yaml
```

4. 編輯 `config.events.local.yaml`：填入已驗證的 DDAE origin、prefix、credential／CA 路徑；設定固定的 `ddae.source_instance`；將 `state.dir` 改成本機 state 的絕對路徑；設定 `kafka.brokers`、topic、TLS 與 SASL。範例啟用資源與告警；要加入 Log，啟用 `monitoring.serviceability_logs.enabled`。使用系統信任的 CA 時，將對應的自訂 CA 路徑改成 `""`。
5. 啟動並驗證：

```bash
./bin/ddae-exporter --config "$PWD/config.events.local.yaml"
```

在另一個 terminal 使用「開始使用」的指令檢查 `/readyz` 與 `/metrics`。來源存在可收集的紀錄時，確認對應的 published counter 增加，且 outbox 待傳送數量逐步減少。安裝 Kafka client tools 後，管理者可透過妥善保護的 client-properties file 檢查 topic 並讀取事件：

```bash
export KAFKA_BOOTSTRAP="<kafka-tls-host>:<tls-port>"
export KAFKA_CLIENT_CONFIG="<absolute-path-to-kafka-client.properties>"
export ALERT_TOPIC="ddae-serviceability-alerts"
kafka-topics.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
  --command-config "$KAFKA_CLIENT_CONFIG" --describe --topic "$ALERT_TOPIC"
kafka-console-consumer.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
  --consumer.config "$KAFKA_CLIENT_CONFIG" --topic "$ALERT_TOPIC" \
  --from-beginning --max-messages 1 \
  --property print.key=true --property print.headers=true
```

請使用 exporter 實際設定的 topic。Kafka client-properties file 屬於 Kafka CLI，需包含站點的 TLS／SASL 設定與 consumer 權限。來源尚無紀錄時，consumer 會等待；來源為空且 published counter 為零，也可能是正常狀態。事件內容屬於維運資料，只在已授權環境中檢視。

### 查詢 Metrics 與查詢事件

準備可透過指定 realm 與 role 檢視所有查詢的 Insights 帳號。Exporter 使用獨立的記憶體 cookie session 完成 Insights OIDC 登入流程。請允許連線至 Insights 與 identity provider 的 HTTPS endpoint。

建立獨立的查詢 credential files：

```bash
bash <<'BASH'
set -eu
umask 077
for name in query-username query-password; do
  IFS= read -r -s -p "$name: " value </dev/tty
  printf '\n' >/dev/tty
  test -n "$value"
  printf '%s' "$value" > "secrets/$name"
  chmod 600 "secrets/$name"
  unset value
done
BASH
```

使用私有 CA 時，依前述 CA 複製步驟，將提供的 bundle 複製至 `trust-local/insights-ca.pem` 與 `trust-local/identity-ca.pem`。接著複製查詢專用範例：

```bash
cp deploy/query-monitoring.example.yaml config.query.local.yaml
chmod 600 config.query.local.yaml
```

在 `monitoring.queries` 填入實際 origin、realm、role、credentials 絕對路徑與 CA 路徑。設定絕對路徑的 `state.dir` 與固定的 `ddae.source_instance`。系統已信任的 CA 可將對應路徑設為 `""`。只收集查詢 metrics 時，保留 `events_enabled: false`。使用相同 port／state 位置前，先停止原本的本機 exporter：

```bash
./bin/ddae-exporter --config "$PWD/config.query.local.yaml"
```

收集完成後，在 `/metrics` 確認 `ddae_query_collection_success 1`、`ddae_query_detail_collection_success 1`、`ddae_query_scope_all 1`，並檢查 `/readyz`。執行中／排隊中的 gauge 反映目前收集到的查詢狀態。

要在整合的告警／Log 設定中加入查詢事件，將 `monitoring.queries` 設定複製至該設定檔，啟用 `enabled: true` 與 `events_enabled: true`，並建立專用的 `kafka_topic`，例如 `ddae-queries`。查詢事件共用 Kafka 傳輸設定，並使用獨立 outbox。事件欄位與已觀測歷史資料的語意，請參閱[查詢監控指南](docs/query-monitoring.md)。

### Serviceability Log 與查詢歷史回補

所屬流程正常運作後，在完整範例中啟用 `monitoring.serviceability_logs.backfill.enabled` 或 `monitoring.queries.backfill.enabled`。先使用範例的 `24h` lookback、`2m` overlap 與 `30s` 掃描間隔，再依設定範圍調整。

重新啟動 exporter。檢查已啟用流程的 `ddae_history_backfill_success`、`ddae_history_backfill_pending_windows`、`ddae_history_backfill_pending_records` 與 `ddae_history_backfill_last_completed_timestamp_seconds`，並搭配 `ddae_history_backfill_incomplete` 和 `ddae_history_backfill_blocked` 判讀進度。完成狀態指的是有界限的掃描時間窗與上游可取得的歷史資料。詳見[回補操作與復原](docs/history-backfill.md)。

## 使用方式與介面

### Exporter HTTP Endpoint

預設 Base URL 為 `http://127.0.0.1:9469`。以下 endpoint 使用固定路徑，與上游 DDAE API prefix 分開設定。

| Method 與路徑 | 成功回應 | 用途 |
|---|---|---|
| `GET /healthz` | `200`、`alive` | Process liveness |
| `GET /readyz` | `200`、`ready` | 已啟用流程的 readiness；尚未就緒時為 `503` |
| `GET /metrics` | `200`、Prometheus／OpenMetrics | 已收集的 metrics 與運作狀態 |

完整的本機 request 請使用「開始使用」中的 curl 範例。本機存取使用 loopback 綁定位址。遠端抓取時，請在受信任的 proxy／service-mesh 邊界提供身分驗證與 TLS，並將存取範圍限制在 metrics 網路。

### 上游 API 存取

Management API credentials 透過 `POST /auth/realms/ddae/protocol/openid-connect/token` 與 `dv-admin-rest` password-grant client 完成驗證。業務資料透過以下固定 GET allowlist 讀取：

| 項目 | 預設路徑 |
|---|---|
| Ping | `/ping` |
| 叢集 | `/v1/ddae-clusters` |
| 基礎設施節點 | `/v1/infrastructure-nodes` |
| 系統鎖定 | `/v1/system-lock` |
| 關機／就緒狀態 | `/v1/system-shutdown` |
| Serviceability issue | `/v1/serviceability-issues`、`/v1/serviceability-issues/{id}` |
| Serviceability event | `/v1/serviceability-events`、`/v1/serviceability-events/{id}` |

查詢透過 `/ui/insights/login` 與設定的 identity-provider realm 進行身分驗證。讀取路徑為 `/ui/api/insights/cluster/info`、`/ui/api/insights/overview/queries`、`/ui/api/insights/history/queries` 與 `/ui/api/insights/history/queries/{id}`。Client 透過 `X-Trino-Role` 選擇 role 範圍並進行驗證。

### Kafka 事件契約

所有事件類型均包含 `content-type: application/json` 與 `ddae-schema-version: 1.0` header。

| 流程 | Record kind／payload | Key |
|---|---|---|
| 告警 | `ddae.serviceability_alert.upsert`；版本化 envelope 與結構化 `alert` | source、NUL、alert ID 的 SHA-256 小寫十六進位字串 |
| Serviceability Log | `ddae.serviceability_log.upsert`；版本化 envelope 與結構化 `log`；`ddae-record-kind: serviceability_log` | source、NUL、`serviceability_log`、NUL、Log ID 的 SHA-256 小寫十六進位字串 |
| 查詢 | 扁平的選定查詢欄位；`ddae-record-kind: query` | source、NUL、query ID 的原始 32-byte SHA-256 |

告警／Log envelope 包含來源識別、紀錄 ID、觀測時間與標準化內容雜湊。查詢紀錄包含來源／查詢識別、使用者、狀態、時間戳記與可取得的耗時欄位。Producer 會先選取事件契約定義的欄位，再進行序列化。

傳送語意為 **at least once**：持久化 outbox 保留待傳送紀錄直到收到 acknowledgement；傳送結果不確定時，同一紀錄可能再次傳送。Consumer 應以穩定的 key 執行 idempotent upsert。詳見[去識別化告警範例](testdata/ddae-1.5.0/alert-event.golden.json)、[規格](SPECIFICATION.md)與[查詢事件指南](docs/query-monitoring.md)。

## 部署

### Docker

Dockerfile 使用 Go `1.26.6` 建置靜態 Linux 執行檔，並封裝為 `scratch` image。預設 runtime UID／GID 為 `65532:65532`。

在本機以 Docker 執行資源監控前，先完成上述 credentials／設定步驟，停止主機上的 exporter，再建置 image：

```bash
docker build -t ddae-exporter:local .
export DDAE_CONTAINER_CA_FILE=""
test "$(id -u)" -ne 0
```

若使用先前複製的私有 CA，設定 `DDAE_CONTAINER_CA_FILE=/run/trust/ddae-ca.pem`。此本機 bind-mount 範例使用目前非 root 使用者的 UID／GID，讓 container 能讀取你的 `0600` 檔案。透過環境設定覆寫，將 `config.local.yaml` 中的主機路徑對應至 container 路徑：

```bash
docker run -d --name ddae-exporter \
  --user "$(id -u):$(id -g)" \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --publish 127.0.0.1:9469:9469 \
  --mount "type=bind,src=$PWD/config.local.yaml,dst=/etc/ddae-exporter/config.yaml,readonly" \
  --mount "type=bind,src=$PWD/secrets,dst=/run/secrets,readonly" \
  --mount "type=bind,src=$PWD/trust-local,dst=/run/trust,readonly" \
  --mount "type=bind,src=$PWD/state,dst=/var/lib/ddae-exporter" \
  --env EXPORTER_LISTEN_ADDRESS=0.0.0.0:9469 \
  --env DDAE_USERNAME_FILE=/run/secrets/ddae-username \
  --env DDAE_PASSWORD_FILE=/run/secrets/ddae-password \
  --env DDAE_CLIENT_SECRET_FILE=/run/secrets/ddae-client-secret \
  --env DDAE_CA_FILE="$DDAE_CONTAINER_CA_FILE" \
  --env STATE_DIR=/var/lib/ddae-exporter \
  ddae-exporter:local --config /etc/ddae-exporter/config.yaml
docker ps --filter name=ddae-exporter
docker logs --tail 100 ddae-exporter
curl --fail --show-error http://127.0.0.1:9469/readyz
```

使用事件／查詢設定時，改掛載該 YAML，並將所有啟用的 Kafka／查詢 credential、憑證與 CA 路徑改為 container 掛載路徑。State 掛載需可由選定 UID 寫入。使用 image 預設 UID `65532` 時，請依該身分配置掛載檔案與目錄權限。

在主機執行 curl 進行驗證。Prometheus container 需要可連線至 exporter 的網路路徑，例如共用的私有 container network 與 exporter service name；它自己的 loopback 位址代表自身。遠端抓取請透過受信任的存取邊界。

正常停止並保留主機 state：

```bash
docker stop --time 20 ddae-exporter
```

### Kubernetes

驗證 DDAE 與 Kafka 連線設定後，使用 [deploy/kubernetes/](deploy/kubernetes/)。Repository 中的範例啟用資源與告警、採 SCRAM-SHA-512，以單一 replica 和 `Recreate` 策略執行，並掛載 `3Gi` 的 `ReadWriteOnce` PVC。

先準備以下平台條件：

- Cluster 提供 manifests 使用的 `apps/v1`、`v1` 與 `networking.k8s.io/v1` resources，且操作帳號可建立 namespace 與其中的 resources。
- 可綁定 PVC、提供持久化儲存空間，並讓 UID／GID `65532` 寫入的 storage class／provisioner。需要指定 class 時，在本機副本設定 `storageClassName`；容量需涵蓋所有啟用流程。
- 依節點 Linux CPU 架構建置的 image，以及 cluster 可 pull 的 registry。
- 可正常使用的 DDAE／Kafka credentials、trust bundles 與已建立的 topic。
- 站點專用 NetworkPolicy，允許 DNS、設定的 DDAE／Kafka 目標、已啟用的 Insights／identity-provider 目標，以及受信任且受 mTLS 保護的 metrics 路徑。Repository 中的 policy 建立 default-deny 隔離；請搭配站點的 allow policies。

**建置並推送自己的 image。** 替換 registry、tag 與節點架構，使用已授權的 registry 帳號：

```bash
export IMAGE="<registry>/ddae-exporter:<tag>"
docker login "<registry>"
docker build --platform "<linux/architecture>" -t "$IMAGE" .
docker push "$IMAGE"
mkdir -p deploy/kubernetes/overlays/local
cp deploy/kubernetes/configmap.yaml deploy/kubernetes/overlays/local/configmap.yaml
cp deploy/kubernetes/deployment.yaml deploy/kubernetes/overlays/local/deployment.yaml
```

將本機 `deployment.yaml` 的 image 改成推送產物的不可變 digest（`<registry>/ddae-exporter@sha256:<digest>`）。在本機 ConfigMap 替換 origin、來源識別名稱、路徑 prefix、Kafka 位址與 SASL 帳號，保留其中列出的 container 內 credential 路徑。使用私有 registry 時，建立 pull Secret，並在本機 Deployment 的 `spec.template.spec.imagePullSecrets` 引用。

**建立 Namespace 與 Secret resources。** 以下使用安裝步驟建立的本機 Secret／CA files，另包含 `secrets/kafka-password` 與 `trust-local/kafka-ca.pem`。CA bundle 請使用管理者提供的檔案：

```bash
export NAMESPACE="ddae-monitoring"
kubectl create namespace "$NAMESPACE"
kubectl -n "$NAMESPACE" create secret generic ddae-exporter-credentials \
  --from-file=username=secrets/ddae-username \
  --from-file=password=secrets/ddae-password \
  --from-file=client-secret=secrets/ddae-client-secret
kubectl -n "$NAMESPACE" create secret generic ddae-exporter-kafka \
  --from-file=password=secrets/kafka-password
kubectl -n "$NAMESPACE" create secret generic ddae-exporter-trust \
  --from-file=ddae-ca.pem=trust-local/ddae-ca.pem \
  --from-file=kafka-ca.pem=trust-local/kafka-ca.pem
```

已有 namespace 時，直接選用該 namespace。既有 Secret 請透過平台的 Secret 管理程序更新／輪替。使用 mTLS 時，在 Kafka Secret 加入 client certificate／key，並設定對應掛載路徑。使用查詢功能時，建立包含 `username` 與 `password` key 的 `ddae-exporter-query`，將 `insights-ca.pem` 與 `identity-ca.pem` 加入 trust Secret，並在本機 ConfigMap 啟用及設定查詢。啟用查詢流程前，其 Secret 掛載為選用。

**套用網路與 workload 設定。** 將 policy-file placeholder 替換成已審核的站點 policy：

```bash
kubectl -n "$NAMESPACE" apply -f deploy/kubernetes/networkpolicy.yaml
kubectl -n "$NAMESPACE" apply -f "<approved-site-networkpolicy.yaml>"
kubectl -n "$NAMESPACE" apply -f deploy/kubernetes/overlays/local/configmap.yaml
kubectl -n "$NAMESPACE" apply -f deploy/kubernetes/overlays/local/deployment.yaml
kubectl -n "$NAMESPACE" get pvc,pods
kubectl -n "$NAMESPACE" rollout status deployment/ddae-exporter --timeout=120s
kubectl -n "$NAMESPACE" logs deployment/ddae-exporter --tail=100
kubectl -n "$NAMESPACE" port-forward service/ddae-exporter 9469:9469
```

預期結果：PVC 為 `Bound`、pod 進入 `Ready`，且 rollout 成功。Port-forward 執行期間，在另一個 terminal 使用「開始使用」中的 curl 檢查。操作前先停止其他占用本機 `9469` 的 process。更新時先套用修改後的本機檔案，再執行 `kubectl -n "$NAMESPACE" rollout restart deployment/ddae-exporter`，接著重做 rollout／readiness 驗證。

### Linux 與 systemd

提供的 unit 以 `ddae-exporter` 帳號執行，管理權限為 `0700` 的 `/var/lib/ddae-exporter`，並載入四項 credentials：DDAE 帳號／密碼／client secret，以及 Kafka 密碼。以下流程採用資源與告警的完整 SASL 設定範例。

在具備所需 systemd 功能與管理權限的 Linux 主機上，為該主機建置執行檔。建立一次 service account（以下使用 `useradd` 與 `/usr/sbin/nologin`；請依發行版調整帳號管理工具）：

```bash
sudo useradd --system --user-group --home-dir /var/lib/ddae-exporter \
  --shell /usr/sbin/nologin ddae-exporter
sudo install -m 0755 bin/ddae-exporter /usr/local/bin/ddae-exporter
sudo install -d -o root -g ddae-exporter -m 0750 /etc/ddae-exporter
sudo install -d -o root -g root -m 0700 /etc/ddae-exporter/secrets
sudo install -d -o root -g ddae-exporter -m 0750 /etc/ddae-exporter/trust
sudo install -o root -g root -m 0400 \
  secrets/ddae-username secrets/ddae-password secrets/ddae-client-secret \
  secrets/kafka-password /etc/ddae-exporter/secrets/
sudo install -o root -g ddae-exporter -m 0644 \
  trust-local/ddae-ca.pem trust-local/kafka-ca.pem /etc/ddae-exporter/trust/
sudo install -o root -g ddae-exporter -m 0640 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
sudo install -m 0644 deploy/systemd/ddae-exporter.service \
  /etc/systemd/system/ddae-exporter.service
sudoedit /etc/ddae-exporter/config.yaml
```

設定 DDAE origin、prefix、來源識別名稱、Kafka 位址／topic 與 SASL 帳號。Unit 中的 `DDAE_*_FILE` 與 `KAFKA_SASL_PASSWORD_FILE` 會覆寫範例的 credential 路徑。CA 使用複製後的路徑，或設為 `""` 採用系統信任。Unit 在服務啟動時建立 state 目錄。

使用查詢監控時，另安裝兩個查詢 credential files、取消 unit 中四行查詢 credential 設定的註解，並設定 query origin、role、CA files 與開關。接著驗證並啟動：

```bash
sudo systemd-analyze verify /etc/systemd/system/ddae-exporter.service
sudo systemctl daemon-reload
sudo systemctl enable --now ddae-exporter
sudo systemctl status ddae-exporter --no-pager
sudo journalctl -u ddae-exporter -n 100 --no-pager
curl --fail --show-error http://127.0.0.1:9469/readyz
```

預期結果：服務為 active，成功收集後 readiness 回傳 `200`。修改設定或 credentials 後，執行 `sudo systemctl restart ddae-exporter` 並重新檢查；若修改 unit 本身，先重新載入 systemd。

## 可觀測性

### Prometheus 抓取設定

當 Prometheus 可透過**同一主機的 loopback** 存取 exporter 時，將以下 job 加入既有 Prometheus 設定：

```yaml
scrape_configs:
  - job_name: ddae-exporter
    scrape_interval: 30s
    scrape_timeout: 10s
    static_configs:
      - targets: ["127.0.0.1:9469"]
```

將 job 合併至既有 `scrape_configs` list，再依 Prometheus 部署程序重新載入／啟動。遠端抓取請使用受信任的 endpoint 及對應 TLS／身分驗證設定。Scrape target 的 port 應與選定監聽 port 一致。

在 Prometheus 檢查 target 健康狀態，並查詢 `up{job="ddae-exporter"}`。若 Prometheus server 在本機的範例 port `9090` 提供服務：

```bash
export PROMETHEUS_URL="http://127.0.0.1:9090"
curl --fail --show-error --get "$PROMETHEUS_URL/api/v1/query" \
  --data-urlencode 'query=up{job="ddae-exporter"}'
```

成功的 scrape 會回傳 sample value `1`。Prometheus 的 `up` 表示抓取成功；上游資料健康狀態則需搭配 exporter readiness 與收集 metrics 判讀。

### Metrics 判讀

| 項目 | 常用 Metrics | 語意 |
|---|---|---|
| 建置與流程選擇 | `ddae_build_info`、`ddae_monitoring_enabled{pipeline}` | 建置識別；後者呈現資源／告警／Log 開關 |
| 資源收集 | `ddae_up`、`ddae_management_api_up`、`ddae_collector_success`、`ddae_snapshot_age_seconds` | 收集成功狀態與新鮮度 |
| 叢集設定 | `ddae_cluster_coordinator_configured_cpu_cores`、`ddae_cluster_worker_configured_memory_bytes` | 設定量：CPU 單位為 cores，記憶體為 bytes |
| 節點 | `ddae_node_ready`、`ddae_node_capacity_cpu_cores`、`ddae_node_allocatable_memory_bytes`、`ddae_node_condition` | 就緒狀態、容量、可分配量與固定的壓力條件 |
| 設備 | `ddae_system_locked`、`ddae_control_plane_ready`、`ddae_nodes_ready`、`ddae_nodes_total` | 鎖定與設備就緒狀態 |
| 告警 | `ddae_alert_pipeline_ready`、`ddae_kafka_events_published_total`、`ddae_kafka_events_failed_total`、`ddae_kafka_buffered_events` | 收集就緒、已確認傳送、失敗與待傳送數量 |
| Serviceability Log | `ddae_serviceability_log_pipeline_ready`、`ddae_serviceability_log_records_published_total`、`ddae_serviceability_log_buffered_records` | 獨立的 Log 收集與傳送狀態 |
| 查詢 | `ddae_queries_running`、`ddae_queries_queued`、`ddae_query_collection_success`、`ddae_query_scope_all` | 目前收集到的數量、收集健康狀態與全查詢權限範圍 |
| 已觀測查詢歷史 | `ddae_queries_observed_completed_total`、`ddae_query_observed_elapsed_seconds`、`ddae_query_observed_execution_seconds`、`ddae_query_observed_queued_seconds` | 去重後的已觀測終止查詢數與耗時 histogram |
| 查詢事件 | `ddae_query_events_pending`、`ddae_query_event_publish_success` | 待傳送事件與最近一次傳送狀態 |
| 回補 | `ddae_history_backfill_*{pipeline}` | 已啟用掃描器的狀態、待處理數量、完成時間與有界限掃描狀態 |

Configured、capacity 與 allocatable 各自保留原本的資源語意，適合用於資源規劃。壓力條件值為 `1`，表示該壓力條件成立。查詢耗時 histogram 使用 seconds，bucket 為 `0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 300, +Inf`。`ddae_query_history_complete` 以 `0` 表示整體涵蓋範圍未知；已觀測歷史 metrics 描述 exporter 收集到的時間窗。選用流程的 metrics 依啟用狀態提供。

### 健康檢查與 Logs

`/healthz` 代表 process liveness。`/readyz` 評估已啟用流程，包含資料新鮮度與必要 state 的健康狀態。Metrics handler 提供已收集的狀態，handler timeout 為 `9s`，最多同時處理 `5` 個 request。

Logs 預設採 `info` 等級的結構化 JSON；可依需求切換為 `text` 或其他設定等級。依部署方式查看前景輸出、`docker logs`、`kubectl logs` 或 systemd journal。分享診斷輸出時，請保留敏感資訊遮蔽措施。

## 安全性

- DDAE 使用採最小權限原則的唯讀帳號，Insights 使用獨立授權的帳號／role。
- 驗證上游 TLS 憑證，為各目標提供對應 CA bundle；client private key 放在受保護的 runtime files。
- Credentials 透過檔案或平台的 Secret 機制提供。本機設定、state 與事件 payload 均依維運資料管理。
- 本機 HTTP 綁定 loopback；遠端驗證、mTLS 與授權交由部署環境受信任的 proxy／service-mesh 邊界處理，並限制 exporter 周邊網路。
- 保留非 root 執行、唯讀 container root、移除 capabilities，以及 Kubernetes／systemd 的安全強化設定。
- 每個來源／state 目錄維持一個 active writer，升級前備份持久化 state。
- 授予 Kafka consumer 存取權前，審核所選事件 schema 與下游存取控制。

## 開發與測試

### 本機開發

使用「開始使用」中的 Repository 與 Go toolchain。Harness 另需要 `rg`（ripgrep）；race-enabled 測試需要可正常使用的 C compiler。以 `rg --version` 與 `cc --version` 檢查，需要時透過主機的開發工具套件管理程序安裝。

先執行 Repository 檢查：

```bash
./HARNESS/harness.sh doctor
./HARNESS/harness.sh instructions:doctor
./HARNESS/harness.sh sdd:check
```

進行實作時，依循 [AGENTS.md](AGENTS.md)、[PROJECT.md](PROJECT.md) 與目前變更已核准的規格／計畫。產物放在 Git 忽略的 `output/`、`bin/`、`coverage/` 或 `test-results/`。

### 測試與格式化

| 檢查 | 指令 | 證據 |
|---|---|---|
| 含 race detector 的 unit／component tests | `./HARNESS/harness.sh test` | Go package tests |
| README／schema 契約檢查 | `go test ./internal/config ./internal/contract` | YAML、語言一致性與部署契約 |
| Coverage | `./HARNESS/harness.sh coverage` | `coverage/coverage.out`；設定門檻為 `80%` |
| 格式與靜態分析 | `./HARNESS/harness.sh lint` | `gofmt` 檢查與 `go vet` |
| 格式化修改的 Go 檔案 | `gofmt -w "<changed-go-file>"` | 更新後的原始碼格式 |
| 建置 | `./HARNESS/harness.sh build` | `bin/ddae-exporter` |
| 完整交付檢查 | `./HARNESS/harness.sh verify` | `test-results/harness/` 中的報告 |

**已授權整合測試：** `integration` stage 使用非正式環境 DDAE、隔離的 Kafka 測試資料、明確提供的 credentials，以及 `DDAE_TEST_SOFTWARE_VERSION=1.5.0`。只在已授權 runner 上設定 `DDAE_INTEGRATION_ENABLED=1` 啟用。`e2e` stage 使用[維運指南](docs/runbook.md)列出的部署 endpoint 設定，並透過 `DDAE_E2E_ENABLED=1` 啟用。這些 stage 以真實系統邊界檢查補充隔離測試。

### 建置識別與 CI

`scripts/build.sh` 接受 `VERSION`、`REVISION` 與 `BUILD_DATE`。預設分別為 `dev`、目前 Git 短 revision 與 `1970-01-01T00:00:00Z`。Docker build arguments 使用相同名稱，其預設 revision 為 `unknown`。Release 產物請提供實際的 release metadata。

[GitHub Actions](.github/workflows/ci.yml) 於 pull request 與 push 至 `main` 時執行本機檢查，並安裝 `govulncheck v1.7.0` 與 `cyclonedx-gomod v1.10.0` 供安全／供應鏈檢查使用。已授權整合測試透過 `main` 上的手動 `workflow_dispatch`，使用 `ddae-nonproduction` runner／environment 執行。

供應鏈驗證使用乾淨且已 commit 的 revision、可重現建置、CycloneDX SBOM、checksum 與 provenance inputs。必要 stages 由 `HARNESS/config.env` 定義；`verify` 是交付檢查入口。證據與網路規則詳見[驗證政策](HARNESS/HARNESS.md)。

## 疑難排解

### 啟動或設定錯誤

閱讀啟動錯誤並確認 `--config` 指定的路徑。檢查 YAML 縮排、`version: 1`、欄位名稱、state／credentials 絕對路徑、duration 單位與時間大小關係。在本機檢查 Environment Variable 覆寫值；對外只分享名稱與遮蔽後的錯誤，不分享 credential 值。每個 credential 的直接／檔案 Environment Variable 擇一設定，修正後重新啟動。

### HTTP Port 已被占用

主機已安裝 `lsof` 時，可執行：

```bash
lsof -nP -iTCP:9469 -sTCP:LISTEN
```

透過正常關閉流程停止先前啟動的 exporter，或修改 `server.listen_address`，並同步更新 curl／scrape／port mapping 設定。

### DDAE TLS、身分驗證或路徑錯誤

- TLS 錯誤：確認 endpoint hostname、系統時間、CA bundle 與 runtime 讀取權限。使用相符 CA，維持原本的信任政策。
- `401`／`403`：向管理者確認唯讀帳號、密碼、`dv-admin-rest` client secret 與 endpoint 權限。
- Management API `404`：依設定章節選擇適用的 Ping／API prefix 組合。身分驗證維持固定的 token 路徑。
- Timeout：檢查 DNS、路由、firewall 規則，以及 request timeout、cycle timeout 與收集間隔的大小關係。

### Readiness 持續回傳 503

檢查各啟用流程的 metrics。確認上游收集資料仍在新鮮度範圍內、必要 state 可寫入、事件 topic 可存取，且 outbox 尚有容量。查詢流程另需確認獨立的 identity-provider 連線與全查詢 role。歷史回補則檢查 incomplete／blocked metrics 與[復原程序](docs/history-backfill.md)。成功抓取 `/metrics`、process liveness 與 readiness 各自代表不同狀態。

### Kafka 待傳送數量增加

檢查 broker TLS／advertised 位址、憑證、SASL mechanism、topic 是否存在，以及 producer ACL。比對 publish-success／failure metrics 與對應 buffered-event 數量。復原 broker 連線期間保留 state 目錄；收到 acknowledgement 後，durable outbox 中的紀錄即可逐步清空。

### Container 或 Kubernetes 啟動失敗

```bash
docker ps -a --filter name=ddae-exporter
docker logs --tail 100 ddae-exporter
kubectl -n "$NAMESPACE" get pods,pvc
kubectl -n "$NAMESPACE" describe deployment ddae-exporter
kubectl -n "$NAMESPACE" describe pods -l app.kubernetes.io/name=ddae-exporter
kubectl -n "$NAMESPACE" logs deployment/ddae-exporter --tail=100
```

依部署方式執行對應指令。檢查 image 架構與 pull 權限、Secret 名稱／key、掛載路徑與擁有者、PVC 綁定，以及站點的 allow policies。Kubernetes 範例在啟動時會用到 DDAE、Kafka 與 trust Secret；啟用查詢後也需要 query Secret。

### State Lock、識別或權限錯誤

確認前一個 process 已停止，同一目錄只有一個 writer。檢查實際 UID 的擁有者設定與掛載寫入權限。保留既有 state，修改識別或復原設定前先閱讀[維運指南](docs/runbook.md)。

## 文件索引

- [維運指南](docs/runbook.md) — 部署、metrics、整合測試設定與復原。
- [查詢監控](docs/query-monitoring.md) — Insights 設定、metrics、事件與涵蓋範圍語意。
- [歷史回補](docs/history-backfill.md) — 有界限的掃描、處理上限、進度與復原。
- [架構決策](docs/decisions/0001-ddae-v1-architecture.md) — 核心設計與契約。
- [跨機器開發](docs/development-portability.md) — 共用輸入與本機專用檔案。
- [產品規格](SPECIFICATION.md)、[測試計畫](TEST_PLAN.md)與[可追溯性紀錄](TRACEABILITY.md) — 受治理的產品行為與證據。
- [Harness 指南](HARNESS/HARNESS.md) — 指令與驗證契約。

## 參與開發與支援

本專案由 Repository Maintainers 負責。提出變更前，請閱讀 [AGENTS.md](AGENTS.md)、[PROJECT.md](PROJECT.md) 與 [CODE_REVIEW.md](CODE_REVIEW.md)。產品行為變更依循 Repository 中已核准的規格與計畫流程。

可透過 [GitHub Issues](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/issues) 提交可重現的問題或建議。請附上 Git revision、實際 Go 版本、部署方式、相關非機密設定與遮蔽後的錯誤。Endpoint、credentials、原始查詢文字與維運 payload 請透過已授權的支援管道提供。更新 README 時同步維護兩種語言，並執行文件契約測試。

## 授權

[Apache License 2.0](LICENSE)。
