# Dell DDAE Metrics Exporter

[English](README.md) | [繁體中文](README.zh-TW.md)

Dell DDAE Metrics Exporter 是 Dell Data Domain Active Enterprise（DDAE）1.5.0
的唯讀監控服務。它會將系統狀態轉成 Prometheus 指標，也能將 serviceability alerts
和 Serviceability Logs 傳送到 Kafka。

| 項目 | 內容 |
|---|---|
| 目前的候選版本 | [`v1.0.0-rc3`](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/releases/tag/v1.0.0-rc3) |
| DDAE 目標版本 | 1.5.0 |
| 執行環境 | Go 1.26.6 |
| 授權 | Apache License 2.0 |
| 已完成的驗證 | 本機單元、元件、安全、建置及文件檢查 |

RC3 是候選版本。Authenticated DDAE、Kafka、OpenSearch、部署 E2E 及獨立審查仍為
後續 external release gates。

## 目錄

- [概述](#概述)
- [DDAE API 相容性](#ddae-api-相容性)
- [系統需求](#系統需求)
- [快速開始](#快速開始)
- [設定](#設定)
- [Prometheus 指標](#prometheus-指標)
- [Kafka 事件](#kafka-事件)
- [部署](#部署)
- [維運](#維運)
- [開發](#開發)
- [授權](#授權)

## 概述

### 功能

| 功能 | 行為 |
|---|---|
| 資源監控 | 收集 API 可用性、叢集狀態、Coordinator 和 Worker 設定、節點狀態與容量、系統鎖定狀態及 readiness。 |
| Prometheus 輸出 | 透過 `/metrics` 提供最近一次收集的資料。 |
| 告警收集 | 讀取 serviceability issue 清單，並在設定的限制內取得 detail records。 |
| 告警傳送 | 先將告警事件寫入本機 bbolt outbox，再傳送到 Kafka。 |
| Serviceability Logs | 讀取 Serviceability Event 清單和 detail records，將新增或變更的資料傳送到獨立 Kafka topic。 |
| 獨立 pipelines | `resources`、`alerts` 和 `serviceability_logs` 可分別開關及設定週期。 |
| 獨立狀態檔 | Alerts 使用 `state.db`；Serviceability Logs 使用 `serviceability-logs.db`。 |
| YAML 設定 | 使用具版本、型別及大小限制的 YAML。環境變數可覆蓋 YAML。 |
| TLS | 預設驗證 DDAE 和 Kafka 的憑證及主機名稱。支援自訂 CA 和 Kafka mTLS。 |
| Kafka 驗證 | 支援 `PLAIN`、`SCRAM-SHA-256` 和 `SCRAM-SHA-512`。 |
| 部署方式 | 提供 release archives、Docker、Kubernetes 及 Linux/systemd 說明。 |

### 資料流

```text
DDAE Management API
  |
  +-- resources ------------> current snapshot ------------> Prometheus /metrics
  |
  +-- serviceability issues -> alert outbox ---------------> Kafka alert topic
  |
  +-- serviceability events -> serviceability log outbox --> Kafka log topic
```

### 監控模式

設定中至少要啟用一個 pipeline。

| 模式 | `resources` | `alerts` | `serviceability_logs` | 是否需要 Kafka |
|---|---:|---:|---:|---:|
| 預設 | `true` | `true` | `false` | 是 |
| 僅監控資源 | `true` | `false` | `false` | 否 |
| 僅傳送告警 | `false` | `true` | `false` | 是 |
| 僅傳送 Serviceability Logs | `false` | `false` | `true` | 是 |
| 全部啟用 | `true` | `true` | `true` | 是 |

已啟用的 pipelines 都會參與 `/readyz` 判定。
`ddae_monitoring_enabled{pipeline}` 會固定回報三個 pipelines 的啟用狀態。

## DDAE API 相容性

### API operations

OAuth token request 使用固定路徑。所有監控 operations 使用固定且唯讀的 `GET` suffix。

| Operation | Prefix 設定 | 預設 request | 用途 |
|---|---|---|---|
| Authentication | 固定路徑 | `POST /auth/realms/ddae/protocol/openid-connect/token` | 取得 `dv-admin-rest` bearer token。 |
| Ping | `ping_prefix` | `GET /ping` | 確認 Management API 可用性。 |
| Clusters | `api_prefix` | `GET /v1/ddae-clusters` | 讀取 cluster 狀態及 Coordinator/Worker 設定。 |
| Nodes | `api_prefix` | `GET /v1/infrastructure-nodes` | 讀取 node 狀態、capacity、allocatable resources 及 conditions。 |
| System lock | `api_prefix` | `GET /v1/system-lock` | 讀取 appliance lock 狀態。 |
| System shutdown | `api_prefix` | `GET /v1/system-shutdown` | 讀取 control-plane 和 node readiness。 |
| Alert list | `api_prefix` | `GET /v1/serviceability-issues` | 列出 serviceability issue IDs。 |
| Alert detail | `api_prefix` | `GET /v1/serviceability-issues/{id}` | 讀取一筆已驗證的 alert detail。 |
| Log list | `api_prefix` | `GET /v1/serviceability-events` | 列出 Serviceability Event IDs。 |
| Log detail | `api_prefix` | `GET /v1/serviceability-events/{id}` | 讀取一筆已驗證的 Serviceability Log。 |

使用專用 DDAE 帳號，並只授予上述 operations 的唯讀權限。DDAE 連線最低使用
TLS 1.2。

### Path prefixes

Ping 和其他 Management API operations 分別使用不同的 prefix 設定：

| YAML key | 環境變數 | 預設值 | 適用範圍 |
|---|---|---|---|
| `ddae.paths.ping_prefix` | `DDAE_PING_PATH_PREFIX` | 空字串 | `/ping` |
| `ddae.paths.api_prefix` | `DDAE_API_PATH_PREFIX` | `/v1` | 其他所有 Management API suffixes |

常用設定：

| Route 格式 | `ping_prefix` | `api_prefix` | Ping 結果 | API 結果範例 |
|---|---|---|---|---|
| 目前預設值 | `""` | `/v1` | `/ping` | `/v1/ddae-clusters` |
| 保留 v1.0.0-rc2 routes | `/rest/v1` | `/rest/v1` | `/rest/v1/ping` | `/rest/v1/ddae-clusters` |
| Dell PDF 範例 | `/rest` | `/rest/v1` | `/rest/ping` | `/rest/v1/ddae-clusters` |

Prefix 可為空字串。非空值必須符合以下規則：

- 最大長度：128 bytes。
- 以一個 `/` 開頭，結尾不使用 `/`。
- 每個 segment 只包含 `A-Z`、`a-z`、`0-9`、`.`、`_`、`~` 或 `-`。
- 不包含空 segment、重複 slash、`.` segment 或 `..` segment。
- 不包含空白、Unicode、percent encoding、query string、fragment、scheme、authority、
  user information、反斜線或 control characters。

Exporter 會直接串接已驗證的 prefix 和程式內固定的 suffix。它不會執行 path
normalization、runtime discovery 或 alternate-path fallback。Route 失敗時會回傳錯誤，
但不會在 metrics 或錯誤文字中顯示設定的 prefix。

## 系統需求

### 執行需求

| 範圍 | 需求 |
|---|---|
| 所有模式 | DDAE 1.5.0 HTTPS origin，例如 `https://ddae.example.com`。 |
| 所有模式 | DDAE 專用唯讀 username、password 和 `dv-admin-rest` client secret。 |
| DDAE 使用 private CA | PEM CA bundle。 |
| Alerts | Kafka brokers、專用 alert topic、TLS 及 `acks=all`。 |
| Serviceability Logs | 獨立 Kafka topic 及 ACL。 |
| Kafka SASL | SASL username 及 password。 |
| Kafka mTLS | Client certificate 及 private key。 |
| Kafka pipelines | 支援 file lock 和 fsync 的可寫入 persistent directory。 |
| Prometheus | 可連線到 TCP port 9469 的網路路徑。 |
| 執行主機 | 已同步的系統時間。 |

僅監控資源時不需要 Kafka 或 persistent state。

### 從原始碼建置的需求

| 工具 | 版本或用途 |
|---|---|
| Git | 下載 repository。 |
| Go | Language version `1.26.0`、toolchain `go1.26.6`。 |
| Go module access | Local cache 沒有 dependencies 時使用。 |

## 快速開始

以下步驟以 Linux amd64 release archive 為例。macOS 或 arm64 請更換 archive 名稱。

### 1. 下載並驗證 release

從 [`v1.0.0-rc3` release](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/releases/tag/v1.0.0-rc3)
下載 archive。

```bash
ARCHIVE=dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64.tar.gz
shasum -a 256 "$ARCHIVE"
tar -xzf "$ARCHIVE"
cd dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64
```

將 SHA-256 輸出與 release notes 內的值比對。

每個 platform archive 包含：

| 檔案 | 用途 |
|---|---|
| `ddae-exporter` | Exporter 執行檔。 |
| `config.example.yaml` | 完整 YAML 範例。 |
| `README.md` | 安裝及設定參考。 |
| `RUNBOOK.md` | 維運、復原及 rollback 步驟。 |
| `LICENSE` | Apache License 2.0。 |

### 2. 建立 credential files

每個 `*_file` 設定都指向一個一般檔案，檔案內只放一個原始值。不要加入 key 名稱、
引號、YAML 或 JSON。

| YAML key | Runtime path 範例 | 內容範例 |
|---|---|---|
| `ddae.credentials.username_file` | `/secure/runtime/ddae-username` | `ddae-exporter-reader` |
| `ddae.credentials.password_file` | `/secure/runtime/ddae-password` | `REPLACE_WITH_DDAE_READ_ONLY_PASSWORD` |
| `ddae.credentials.client_secret_file` | `/secure/runtime/ddae-client-secret` | `REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET` |
| `kafka.sasl.password_file` | `/secure/runtime/kafka-password` | `REPLACE_WITH_KAFKA_SASL_PASSWORD` |

檔案內容範例：

`/secure/runtime/ddae-username`

```text
ddae-exporter-reader
```

`/secure/runtime/ddae-password`

```text
REPLACE_WITH_DDAE_READ_ONLY_PASSWORD
```

`/secure/runtime/ddae-client-secret`

```text
REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET
```

`/secure/runtime/kafka-password`

```text
REPLACE_WITH_KAFKA_SASL_PASSWORD
```

安裝 secret manager 提供的檔案：

```bash
sudo install -d -o ddae-exporter -g ddae-exporter -m 0700 /secure/runtime
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-username /secure/runtime/ddae-username
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-password /secure/runtime/ddae-password
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/ddae-client-secret /secure/runtime/ddae-client-secret
sudo install -o ddae-exporter -g ddae-exporter -m 0600 \
  /path/from/secret-manager/kafka-password /secure/runtime/kafka-password
```

本機開發可使用互動方式建立檔案：

```bash
install -d -m 0700 secrets
umask 077
printf 'DDAE username: '
IFS= read -r ddae_username_value
printf 'DDAE password: '
IFS= read -r -s ddae_password_value
printf '\nDDAE dv-admin-rest client secret: '
IFS= read -r -s ddae_client_secret_value
printf '\nKafka SASL password (press Enter when unused): '
IFS= read -r -s kafka_password_value
printf '\n'
printf '%s' "$ddae_username_value" > secrets/ddae-username
printf '%s' "$ddae_password_value" > secrets/ddae-password
printf '%s' "$ddae_client_secret_value" > secrets/ddae-client-secret
printf '%s' "$kafka_password_value" > secrets/kafka-password
chmod 0600 secrets/ddae-username secrets/ddae-password \
  secrets/ddae-client-secret secrets/kafka-password
unset ddae_username_value ddae_password_value ddae_client_secret_value kafka_password_value
```

Credential file 規則：

| 項目 | 要求 |
|---|---|
| 檔案類型 | 一般檔案 |
| 上層目錄權限 | `0700` |
| 檔案權限 | `0600` |
| Owner | Exporter 執行帳號 |
| Encoding | Valid UTF-8 |
| 內容 | 一個非空值 |
| 最大大小 | 64 KiB |
| YAML value | Exporter 可讀取的 absolute path |

Reader 會移除結尾的一個 `LF`、`CRLF` 或 `CR`。`secrets/` directory 已由
`.gitignore` 排除。

### 3. 建立設定檔

複製範例：

```bash
cp config.example.yaml config.yaml
chmod 0600 config.yaml
```

第一次執行可先使用僅監控資源的最小設定。將所有 path 和 endpoint 換成實際值：

```yaml
version: 1

monitoring:
  resources:
    enabled: true
  alerts:
    enabled: false
  serviceability_logs:
    enabled: false

ddae:
  base_url: https://ddae.example.com
  paths:
    ping_prefix: ""
    api_prefix: /v1
  credentials:
    username_file: /absolute/path/secrets/ddae-username
    password_file: /absolute/path/secrets/ddae-password
    client_secret_file: /absolute/path/secrets/ddae-client-secret
```

Alerts、Serviceability Logs、Kafka、自訂 CA 及 runtime limits 請參考
[設定章節](#設定)。

### 4. 啟動 Exporter

```bash
./ddae-exporter --config ./config.yaml
```

也可使用環境變數指定設定檔：

```bash
DDAE_EXPORTER_CONFIG_FILE=/absolute/path/config.yaml ./ddae-exporter
```

### 5. 驗證程序

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

| 檢查 | 預期結果 |
|---|---|
| `/healthz` | HTTP server 啟動後回傳 HTTP 200。 |
| `/readyz` | 所有已啟用 pipelines ready 後回傳 HTTP 200；其他狀態回傳 503。 |
| `/metrics` | 包含 `ddae_build_info`、三筆 `ddae_monitoring_enabled` 及已啟用 pipelines 的 metrics。 |

## 設定

### 設定檔選擇及優先順序

YAML 使用 `version: 1`、UTF-8 encoding、單一 document，最大大小為 1 MiB。程式會在
HTTP server 和 background workers 啟動前拒絕 unknown keys、錯誤型別及超出範圍的值。

設定優先順序由高至低：

1. 個別環境變數。
2. `--config` 指定的 YAML。
3. `DDAE_EXPORTER_CONFIG_FILE` 指定的 YAML。
4. 程式預設值。

Path-prefix 環境變數明確設定為空字串時，仍視為有效 override。

### 完整 YAML 及逐行說明

以下範例的每個設定都有 inline 說明。部署前將 `.invalid`、`replace-with-*` 及
`/secure/runtime/*` 換成實際值。

```yaml
version: 1 # 設定格式版本；目前固定為整數 1。

monitoring: # 三個監控 pipelines 的設定。
  resources: # Prometheus 資源指標。
    enabled: true # 啟用 ping、clusters、nodes、lock 及 power collectors。
    interval: 30s # 資源收集週期；使用 Go duration 格式。
    stale_after: 120s # Snapshot 過期時間；必須大於 resources.interval。
  alerts: # DDAE alert-to-Kafka pipeline。
    enabled: true # 啟用 alert list/detail、outbox 及 Kafka producer。
    interval: 30s # Alert list 收集週期。
    list_response_max_bytes: 8388608 # Alert list body 上限；單位 bytes，範圍 1–67108864。
    detail: # 每筆 alert detail 的限制。
      response_max_bytes: 1048576 # Detail body 上限；單位 bytes，範圍 1–67108864。
      refresh_interval: 10m # 既有 alert 的 refresh 間隔；不得小於 alerts.interval。
      max_per_cycle: 200 # 每輪最多處理的 detail 筆數；範圍 1–10000。
      concurrency: 4 # 同時執行的 detail requests；範圍 1–128，且不得大於 max_per_cycle。
  serviceability_logs: # Serviceability Logs-to-Kafka pipeline。
    enabled: false # 啟用獨立 worker、producer 及 serviceability-logs.db。
    interval: 30s # Serviceability Log list 收集週期。
    list_response_max_bytes: 8388608 # Log list body 上限；單位 bytes，範圍 1–67108864。
    detail: # 每筆 Log detail 的限制。
      response_max_bytes: 1048576 # Detail body 上限；單位 bytes，範圍 1–67108864。
      refresh_interval: 10m # 既有 Log 的 refresh 間隔；不得小於 serviceability_logs.interval。
      max_per_cycle: 200 # 每輪最多處理的 detail 筆數；範圍 1–10000。
      concurrency: 4 # 同時執行的 detail requests；範圍 1–128，且不得大於 max_per_cycle。

server: # Exporter HTTP server 設定。
  listen_address: 127.0.0.1:9469 # Listener；container/Kubernetes 使用 0.0.0.0:9469。
  shutdown_grace_period: 15s # 停止 workers、同步 state 及關閉 HTTP 的總時間。

security: # TLS diagnostic mode 的全域確認開關。
  allow_insecure_tls: false # Target 關閉 TLS verification 前，此值也必須為 true。

ddae: # DDAE Management API client 設定。
  base_url: https://ddae.example.invalid # HTTPS origin；可含 port，不含 path、query 或 user information。
  paths: # Ping 及其他 Management API 的獨立 prefixes。
    ping_prefix: "" # 預設空字串；組合後為 GET /ping。
    api_prefix: /v1 # 其他 APIs 的預設 prefix；例如 GET /v1/ddae-clusters。
  source_instance: replace-with-stable-appliance-name # 固定 Kafka source identity；啟用 Kafka pipeline 時必填。
  credentials: # DDAE OAuth password-grant credential files。
    username_file: /secure/runtime/ddae-username # 內容只放 DDAE 唯讀 username。
    password_file: /secure/runtime/ddae-password # 內容只放 DDAE 唯讀 password。
    client_secret_file: /secure/runtime/ddae-client-secret # 內容只放 dv-admin-rest client secret。
  tls: # DDAE HTTPS trust 設定。
    ca_file: /etc/ddae-exporter/trust/ddae-ca.pem # 額外 PEM CA bundle；使用 system roots 時移除此行。
    insecure_skip_verify: false # 只在 allow_insecure_tls 同為 true 時關閉 DDAE certificate/hostname 驗證。
  request_timeout: 5s # 每次 token 或 API request deadline；必須小於 cycle_timeout。
  cycle_timeout: 20s # 每輪收集 deadline；必須小於每個 enabled pipeline interval。
  response_max_bytes: 4194304 # 一般 DDAE response 上限；單位 bytes，範圍 1–67108864。
  retry_max: 2 # 初次 safe request 後的 retry 次數；範圍 0–10。

kafka: # Alerts 及 Serviceability Logs 共用的連線設定。
  brokers: # Bootstrap broker 清單；啟用 Kafka pipeline 時需要 1–64 筆。
    - kafka.example.invalid:9093 # Broker 格式為 host:port；多台 broker 時增加 list item。
  topic: ddae-serviceability-alerts # Alert 專用 topic；最大 249 bytes。
  serviceability_logs_topic: ddae-serviceability-logs # Log 專用 topic；不得與 alert topic 相同。
  client_id: ddae-exporter # Kafka client ID；1–128 bytes。
  tls: # Kafka TLS 及 optional mTLS。
    ca_file: /etc/ddae-exporter/trust/kafka-ca.pem # 額外 PEM CA bundle；使用 system roots 時移除此行。
    # client_cert_file: /secure/runtime/kafka-client.crt # Optional mTLS client certificate PEM。
    # client_key_file: /secure/runtime/kafka-client.key # Optional 且必須配對的 mTLS private key。
    insecure_skip_verify: false # 只在 allow_insecure_tls 同為 true 時關閉 Kafka certificate/hostname 驗證。
  sasl: # Optional Kafka SASL；此範例使用 SCRAM-SHA-512。
    mechanism: SCRAM-SHA-512 # 可用 PLAIN、SCRAM-SHA-256 或 SCRAM-SHA-512。
    username: replace-with-runtime-identity # Kafka SASL username。
    password_file: /secure/runtime/kafka-password # 內容只放 Kafka SASL password。
  publish_timeout: 10s # 每筆 record acknowledgement deadline；最小 1s。

state: # Alerts 及 Serviceability Logs 的本機 persistent state。
  dir: /var/lib/ddae-exporter # Kafka pipelines 使用的可寫入 absolute directory。
  outbox_max_bytes: 1073741824 # Alert outbox payload 上限；單位 bytes。
  outbox_max_events: 100000 # Alert outbox record 上限；範圍 1–10000000。
  checkpoint_retention: 720h # Alert 從完整清單消失後的 checkpoint 保留時間。
  checkpoint_max_alerts: 100000 # Alert checkpoint 上限；範圍 1–10000000。
  serviceability_logs_outbox_max_bytes: 1073741824 # Log outbox payload 上限；單位 bytes。
  serviceability_logs_outbox_max_events: 100000 # Log outbox record 上限；範圍 1–10000000。
  serviceability_logs_checkpoint_retention: 720h # Log 從完整清單消失後的 checkpoint 保留時間。
  serviceability_logs_checkpoint_max_records: 100000 # Log checkpoint 上限；範圍 1–10000000。

logging: # Application logging 設定。
  level: info # 可用 debug、info、warn 或 error。
  format: json # 可用 json 或 text。
```

### 設定參考

#### Monitoring、HTTP 及 DDAE

| YAML key | 預設值或條件 | 說明 |
|---|---|---|
| `monitoring.resources.enabled` | `true` | 啟用 resource collectors 及 resource metrics。 |
| `monitoring.resources.interval` | `30s` | Resource collection interval。 |
| `monitoring.resources.stale_after` | `120s` | Current resource snapshot 的最大年齡。 |
| `monitoring.alerts.enabled` | `true` | 啟用 alert list/detail、Kafka 及 alert state。 |
| `monitoring.alerts.interval` | `30s` | Alert list collection interval。 |
| `monitoring.serviceability_logs.enabled` | `false` | 啟用 Log collection、Kafka producer 及專用 state file。 |
| `monitoring.serviceability_logs.interval` | `30s` | Serviceability Log list collection interval。 |
| `server.listen_address` | `127.0.0.1:9469` | `host:port` 格式的 HTTP listener。 |
| `server.shutdown_grace_period` | `15s` | Graceful shutdown 的總時間。 |
| `ddae.base_url` | 必填 | 一個不含 path、query、fragment 或 user information 的 HTTPS origin。 |
| `ddae.paths.ping_prefix` | 空字串 | 與 `/ping` 串接的 prefix。 |
| `ddae.paths.api_prefix` | `/v1` | 與其他 Management API suffixes 串接的 prefix。 |
| `ddae.source_instance` | 使用 Kafka 時必填 | 固定 event source identity，1–128 UTF-8 bytes。 |
| `ddae.credentials.username_file` | 必填 | DDAE username file。 |
| `ddae.credentials.password_file` | 必填 | DDAE password file。 |
| `ddae.credentials.client_secret_file` | 必填 | `dv-admin-rest` client secret file。 |
| `ddae.tls.ca_file` | System roots | 額外 PEM CA bundle。 |
| `ddae.request_timeout` | `5s` | 每次 token 或 DDAE request deadline。 |
| `ddae.cycle_timeout` | `20s` | 每輪 collection cycle 的總 deadline。 |
| `ddae.response_max_bytes` | `4194304` | 一般 response body 上限；範圍 `1–67108864` bytes。 |
| `ddae.retry_max` | `2` | 初次 safe request 後的 retry 次數；範圍 `0–10`。 |

Duration 使用 Go 格式，例如 `500ms`、`5s`、`10m` 和 `720h`。

必要的時間關係：

```text
ddae.request_timeout < ddae.cycle_timeout < each enabled pipeline interval
monitoring.resources.interval < monitoring.resources.stale_after
alerts.detail.refresh_interval >= alerts.interval
serviceability_logs.detail.refresh_interval >= serviceability_logs.interval
```

#### Alert detail limits

| YAML key | 預設值 | 範圍或規則 |
|---|---:|---|
| `monitoring.alerts.list_response_max_bytes` | `8388608` | `1–67108864` bytes |
| `monitoring.alerts.detail.response_max_bytes` | `1048576` | `1–67108864` bytes |
| `monitoring.alerts.detail.refresh_interval` | `10m` | 不得小於 `alerts.interval` |
| `monitoring.alerts.detail.max_per_cycle` | `200` | `1–10000` records |
| `monitoring.alerts.detail.concurrency` | `4` | `1–128`，且不得大於 `max_per_cycle` |

超過 `max_per_cycle` 的 records 會依固定順序延後。New/changed records 和
refresh-due records 使用不同 queues；兩個 queues 同時有資料時，scheduler 會為兩者
保留處理容量。

#### Serviceability Log detail limits

| YAML key | 預設值 | 範圍或規則 |
|---|---:|---|
| `monitoring.serviceability_logs.list_response_max_bytes` | `8388608` | `1–67108864` bytes |
| `monitoring.serviceability_logs.detail.response_max_bytes` | `1048576` | `1–67108864` bytes |
| `monitoring.serviceability_logs.detail.refresh_interval` | `10m` | 不得小於 `serviceability_logs.interval` |
| `monitoring.serviceability_logs.detail.max_per_cycle` | `200` | `1–10000` records |
| `monitoring.serviceability_logs.detail.concurrency` | `4` | `1–128`，且不得大於 `max_per_cycle` |

不完整的清單仍可提供已驗證的 IDs 進行有限的 detail processing。只有完整清單可用於
absence retention。Record 從清單消失時，pipeline 不會產生 deletion event。

#### Kafka 及 persistent state

| YAML key | 預設值或條件 | 說明 |
|---|---|---|
| `kafka.brokers` | 使用 Kafka 時必填 | 1–64 個 TLS broker addresses。 |
| `kafka.topic` | 啟用 Alerts 時必填 | Alert topic，最大 249 bytes。 |
| `kafka.serviceability_logs_topic` | `ddae-serviceability-logs` | Log topic；兩個 pipelines 同時啟用時不得與 alert topic 相同。 |
| `kafka.client_id` | `ddae-exporter` | Kafka client ID，1–128 bytes。 |
| `kafka.tls.ca_file` | System roots | 額外 broker CA bundle。 |
| `kafka.tls.client_cert_file` | 選用 | mTLS client certificate。 |
| `kafka.tls.client_key_file` | 選用 | 與 client certificate 配對的 mTLS private key。 |
| `kafka.sasl.mechanism` | 選用 | `PLAIN`、`SCRAM-SHA-256` 或 `SCRAM-SHA-512`。 |
| `kafka.sasl.username` | 使用 SASL 時必填 | Kafka SASL username。 |
| `kafka.sasl.password_file` | 使用 SASL 時必填 | Kafka SASL password file。 |
| `kafka.publish_timeout` | `10s` | 等待 broker acknowledgement 的 hard deadline；最小 `1s`。 |
| `state.dir` | `/var/lib/ddae-exporter` | Kafka pipeline state 使用的可寫入 absolute directory。 |
| `state.outbox_max_bytes` | `1073741824` | Alert outbox byte 上限。 |
| `state.outbox_max_events` | `100000` | Alert outbox record 上限。 |
| `state.checkpoint_retention` | `720h` | Alert checkpoint retention。 |
| `state.checkpoint_max_alerts` | `100000` | Alert checkpoint 上限。 |
| `state.serviceability_logs_outbox_max_bytes` | `1073741824` | Log outbox byte 上限。 |
| `state.serviceability_logs_outbox_max_events` | `100000` | Log outbox record 上限。 |
| `state.serviceability_logs_checkpoint_retention` | `720h` | Log checkpoint retention。 |
| `state.serviceability_logs_checkpoint_max_records` | `100000` | Log checkpoint 上限。 |

Kafka delivery 使用 `acks=all`、idempotent producer、per-key ordering 及 durable
at-least-once outbox。Publish timeout 發生時，由於 broker 結果不確定，record 會保留在
outbox。Consumer 必須使用 Kafka record key 執行 idempotent upsert。

Outbox 達到 byte 或 record 上限時，Exporter 會保留現有 records、停止接收該 pipeline
的新 records，並將該 pipeline 回報為 not ready。

#### TLS verification

| YAML key | 預設值 | 效果 |
|---|---:|---|
| `security.allow_insecure_tls` | `false` | 任何 target 關閉驗證前都必須啟用的全域確認。 |
| `ddae.tls.insecure_skip_verify` | `false` | 只有全域確認同為 `true` 時，才關閉 DDAE certificate 及 hostname 驗證。 |
| `kafka.tls.insecure_skip_verify` | `false` | 只有全域確認同為 `true` 時，才關閉 Kafka certificate 及 hostname 驗證。 |

DDAE 和 Kafka 設定彼此獨立。一般部署將三個值維持為 `false`，並安裝正確的 CA
bundle。Target 的 effective insecure mode 不可與 `ca_file` 同時使用。TLS 最低版本
仍為 1.2。

#### 環境變數

環境變數保留 environment-only deployments 的相容性。Direct secret values 用於既有
部署；新部署使用 file-based secrets。

| 群組 | 環境變數 |
|---|---|
| Pipeline switches | `DDAE_RESOURCE_MONITORING_ENABLED`、`DDAE_ALERT_MONITORING_ENABLED`、`DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED` |
| Pipeline intervals | `DDAE_COLLECTION_INTERVAL`、`DDAE_RESOURCE_COLLECTION_INTERVAL`、`DDAE_ALERT_COLLECTION_INTERVAL`、`DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL`、`DDAE_STALE_AFTER` |
| Alert limits | `ALERT_LIST_RESPONSE_MAX_BYTES`、`ALERT_DETAIL_RESPONSE_MAX_BYTES`、`ALERT_DETAIL_REFRESH_INTERVAL`、`ALERT_DETAIL_MAX_PER_CYCLE`、`ALERT_DETAIL_CONCURRENCY` |
| Log limits | `SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES`、`SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES`、`SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL`、`SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE`、`SERVICEABILITY_LOG_DETAIL_CONCURRENCY` |
| DDAE | `DDAE_BASE_URL`、`DDAE_PING_PATH_PREFIX`、`DDAE_API_PATH_PREFIX`、`DDAE_SOURCE_INSTANCE`、`DDAE_CA_FILE`、`DDAE_TLS_INSECURE_SKIP_VERIFY`、`DDAE_REQUEST_TIMEOUT`、`DDAE_CYCLE_TIMEOUT`、`DDAE_RESPONSE_MAX_BYTES`、`DDAE_RETRY_MAX` |
| DDAE credentials | `DDAE_USERNAME`、`DDAE_PASSWORD`、`DDAE_CLIENT_SECRET` 及各自的 `*_FILE` variables |
| Kafka | `KAFKA_BROKERS`、`KAFKA_TOPIC`、`KAFKA_SERVICEABILITY_LOG_TOPIC`、`KAFKA_CLIENT_ID`、`KAFKA_PUBLISH_TIMEOUT` |
| Kafka TLS | `KAFKA_CA_FILE`、`KAFKA_CLIENT_CERT_FILE`、`KAFKA_CLIENT_KEY_FILE`、`KAFKA_TLS_INSECURE_SKIP_VERIFY` |
| Kafka SASL | `KAFKA_SASL_MECHANISM`、`KAFKA_SASL_USERNAME`、`KAFKA_SASL_PASSWORD`、`KAFKA_SASL_PASSWORD_FILE` |
| State | `STATE_DIR`、`KAFKA_OUTBOX_MAX_BYTES`、`KAFKA_OUTBOX_MAX_EVENTS`、`CHECKPOINT_RETENTION`、`CHECKPOINT_MAX_ALERTS` |
| Log state | `SERVICEABILITY_LOG_OUTBOX_MAX_BYTES`、`SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS`、`SERVICEABILITY_LOG_CHECKPOINT_RETENTION`、`SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS` |
| Server 及 logging | `EXPORTER_LISTEN_ADDRESS`、`SHUTDOWN_GRACE_PERIOD`、`LOG_LEVEL`、`LOG_FORMAT` |
| TLS acknowledgement | `ALLOW_INSECURE_TLS` |

## Prometheus 指標

預設 listener 為 `127.0.0.1:9469`。

| Endpoint | 行為 |
|---|---|
| `GET /metrics` | Prometheus exposition 及 collector diagnostics。最多五個 concurrent requests；handler timeout 為九秒。 |
| `GET /healthz` | Process liveness。 |
| `GET /readyz` | 所有已啟用 pipelines 的 readiness。Ready 時回傳 200，其他狀態回傳 503。 |

### 單位

| 單位 | 意義 |
|---|---|
| `boolean` | `0` 或 `1` |
| `one-hot boolean` | 固定 state set 中只有目前 state 的 series 為 `1`，其他為 `0` |
| `seconds` | 秒，可包含小數 |
| `bytes` | Bytes，不是 KiB 或 MiB |
| `CPU cores` | CPU cores，可包含小數 |
| `nodes`、`alerts`、`events`、`logs`、`records` | 非負整數數量 |
| `counter` | 單次 process lifetime 內只增加的非負值 |

### 共用 metrics

| Metric | Type | 單位 | 值域及意義 |
|---|---|---|---|
| `ddae_build_info{version,go_version}` | gauge | 無 | 固定為 `1`；labels 表示 Exporter 及 Go 版本。 |
| `ddae_monitoring_enabled{pipeline}` | gauge | boolean | `0` 或 `1`；固定回報 `resources`、`alerts` 及 `serviceability_logs`。 |
| `ddae_collector_success{collector}` | gauge | boolean | 最近一次 collector 成功時為 `1`。 |
| `ddae_collector_duration_seconds{collector}` | gauge | seconds | 最近一次 collection 的非負執行時間。 |

### Resource metrics

| Metric | Type | 單位 | 值域及意義 |
|---|---|---|---|
| `ddae_up` | gauge | boolean | 所有必要 resource collectors 成功且 snapshot 未過期時為 `1`。 |
| `ddae_snapshot_age_seconds` | gauge | seconds | 最近完整 resource snapshot 的非負年齡。 |
| `ddae_management_api_up` | gauge | boolean | Ping collector 完成有效 authenticated request 時為 `1`。 |
| `ddae_cluster_state_info{cluster,state}` | gauge | one-hot boolean | `state` 為 `available` 或 `unknown`。 |
| `ddae_cluster_coordinator_configured_cpu_cores{cluster}` | gauge | CPU cores | 非負有限 configured CPU。 |
| `ddae_cluster_coordinator_configured_memory_bytes{cluster}` | gauge | bytes | 非負 configured memory。 |
| `ddae_cluster_worker_configured_cpu_cores{cluster}` | gauge | CPU cores | 非負有限 worker CPU configuration。 |
| `ddae_cluster_worker_configured_memory_bytes{cluster}` | gauge | bytes | 非負 worker memory configuration。 |
| `ddae_node_state_info{node,state}` | gauge | one-hot boolean | 回報一個固定 normalized node state。 |
| `ddae_node_ready{node}` | gauge | boolean | Normalized node state 為 `ready` 時為 `1`。 |
| `ddae_node_capacity_cpu_cores{node}` | gauge | CPU cores | 非負有限 total capacity。 |
| `ddae_node_capacity_memory_bytes{node}` | gauge | bytes | 非負 total memory capacity。 |
| `ddae_node_capacity_ephemeral_storage_bytes{node}` | gauge | bytes | 非負 total ephemeral-storage capacity。 |
| `ddae_node_allocatable_cpu_cores{node}` | gauge | CPU cores | 非負有限 allocatable CPU。 |
| `ddae_node_allocatable_memory_bytes{node}` | gauge | bytes | 非負 allocatable memory。 |
| `ddae_node_allocatable_ephemeral_storage_bytes{node}` | gauge | bytes | 非負 allocatable ephemeral storage。 |
| `ddae_node_condition{node,condition}` | gauge | boolean | `condition` 為 `disk_pressure` 或 `memory_pressure`；有 pressure 時為 `1`。 |
| `ddae_system_locked` | gauge | boolean | Appliance locked 時為 `1`。 |
| `ddae_control_plane_ready` | gauge | boolean | Control-plane nodes ready 時為 `1`。 |
| `ddae_nodes_ready` | gauge | nodes | 從 `0` 到 `ddae_nodes_total` 的整數。 |
| `ddae_nodes_total` | gauge | nodes | 非負整數。 |

DDAE response 缺少 CPU、memory 或 storage 欄位時，對應 series 不會輸出。存在的 quantity
必須可解析為 Kubernetes quantity，並產生非負有限值。

Node state 值域：

| State | 意義 |
|---|---|
| `maintenance_mode` | Node 處於 maintenance mode。 |
| `scheduling_disabled` | Scheduling 已停用。 |
| `not_ready` | Node 尚未 ready。 |
| `ready` | Node ready。 |
| `restarting` | Node 正在 restarting。 |
| `shutting_down` | Node 正在 shutting down。 |
| `powered_off` | Node 已 powered off。 |
| `powering_on` | Node 正在 powering on。 |
| `unknown` | Source state 不符合任何固定已知值。 |

### Alert metrics

| Metric | Type | 單位 | 值域及意義 |
|---|---|---|---|
| `ddae_alert_list_complete` | gauge | boolean | 最近 list structure 及 count relationship 完整時為 `1`。 |
| `ddae_alert_detail_deferred` | gauge | alerts | 因 `max_per_cycle` 而延後的 alert details 數量。 |
| `ddae_alert_pipeline_ready` | gauge | boolean | List/detail collection、persistent state 及 outbox capacity 正常時為 `1`。 |
| `ddae_kafka_publish_success` | gauge | boolean | 最近 required Kafka batch 已收到 acknowledgement 時為 `1`。 |
| `ddae_kafka_publish_duration_seconds` | gauge | seconds | 非負 publish duration。 |
| `ddae_kafka_events_published_total` | counter | events | 已收到 acknowledgement 的 alert events 數量。 |
| `ddae_kafka_events_failed_total{reason}` | counter | events | 依固定 failure class 分組的失敗 alert events。 |
| `ddae_kafka_buffered_events` | gauge | events | `0` 到 `state.outbox_max_events`；byte 上限可能先到達。 |

### Serviceability Log metrics

| Metric | Type | 單位 | 值域及意義 |
|---|---|---|---|
| `ddae_serviceability_log_list_complete` | gauge | boolean | 最近 list 通過 structure 及 count completeness checks 時為 `1`。 |
| `ddae_serviceability_log_detail_deferred` | gauge | logs | 因 `max_per_cycle` 而延後的 Log details 數量。 |
| `ddae_serviceability_log_pipeline_ready` | gauge | boolean | List/detail collection、state 及 capacity 正常時為 `1`。 |
| `ddae_serviceability_log_kafka_publish_success` | gauge | boolean | 最近 required Log publish 已收到 acknowledgement 時為 `1`。 |
| `ddae_serviceability_log_kafka_publish_duration_seconds` | gauge | seconds | 非負 publish duration。 |
| `ddae_serviceability_log_records_published_total` | counter | records | 已收到 acknowledgement 的 Log records 數量。 |
| `ddae_serviceability_log_records_failed_total{reason}` | counter | records | 依固定 failure class 分組的失敗 Log records。 |
| `ddae_serviceability_log_buffered_records` | gauge | records | `0` 到 `state.serviceability_logs_outbox_max_events`；byte 上限可能先到達。 |

### 固定 label 值域

| Label | 值域 |
|---|---|
| `pipeline` | `resources`、`alerts`、`serviceability_logs` |
| `collector` | `ping`、`clusters`、`nodes`、`lock`、`power`、`alert_list`、`alert_detail`、`serviceability_log_list`、`serviceability_log_detail` |
| Cluster `state` | `available`、`unknown` |
| Node `state` | `maintenance_mode`、`scheduling_disabled`、`not_ready`、`ready`、`restarting`、`shutting_down`、`powered_off`、`powering_on`、`unknown` |
| `condition` | `disk_pressure`、`memory_pressure` |
| `reason` | `auth`、`tls`、`timeout`、`transport`、`http`、`decode`、`validation`、`kafka_auth`、`kafka_timeout`、`kafka_rejected`、`buffer_full`、`internal` |

Site、environment 和 region 等部署維度由 Prometheus service discovery 或 relabeling
設定加入。

## Kafka 事件

### Alert event

每筆 alert record 使用 schema version `1.0`。

| Envelope field | JSON type | 值或來源 |
|---|---|---|
| `schema_version` | string | 固定為 `1.0`。 |
| `event_type` | string | 固定為 `ddae.serviceability_alert.upsert`。 |
| `source_system` | string | 固定為 `dell_ddae`。 |
| `source_instance` | string | 設定的固定 appliance identity，1–128 UTF-8 bytes。 |
| `alert_id` | string | 來自已驗證 list ID 的 1–256 ASCII characters。 |
| `content_hash_sha256` | string | Canonical normalized `alert` JSON 的 64-character lowercase SHA-256。 |
| `observed_at` | string | UTC RFC 3339 observation time。 |
| `alert` | object | 允許的 typed alert fields。 |

Alert payload fields：

| Field | JSON type | 值域或上限 |
|---|---|---|
| `severity` | string | `critical`、`error`、`warning`、`info`、`normal` 或 `unknown` |
| `acknowledged` | boolean | `true` 或 `false` |
| `occurrence_count` | integer | 非負整數 |
| `created_at`、`updated_at` | string | UTC RFC 3339 timestamp |
| `clear_type` | string | 最大 64 UTF-8 bytes |
| `auto_clear_timeout_raw` | integer | 非負 raw value；Dell 1.5.0 未定義單位 |
| `app_name`、`component`、`namespace` | string | 各最大 256 UTF-8 bytes |
| `message` | string | 最大 8192 UTF-8 bytes |
| `reason` | string | 最大 4096 UTF-8 bytes |
| `remedies` | array of strings | 最多 32 筆，每筆最大 2048 UTF-8 bytes |
| `resource_id`、`related` | string | 最大 512 UTF-8 bytes |
| `symptom_id` | string | 最大 256 UTF-8 bytes |
| `related_events` | array of typed objects | 最多 100 筆 one-level objects |

Alert JSON 範本：

```json
{
  "schema_version": "1.0",
  "event_type": "ddae.serviceability_alert.upsert",
  "source_system": "dell_ddae",
  "source_instance": "fixture-site",
  "alert_id": "alert-1",
  "content_hash_sha256": "49ccb1aaa6bbe2aecdfddba289a1ac720e153a14b1e59fe14380086233d513ea",
  "observed_at": "2026-08-24T03:00:00Z",
  "alert": {
    "severity": "warning",
    "acknowledged": false,
    "occurrence_count": 2,
    "created_at": "2026-08-24T02:00:00Z",
    "message": "synthetic operator message",
    "remedies": [
      "inspect synthetic node"
    ]
  }
}
```

Kafka record contract：

| 項目 | 值 |
|---|---|
| Topic | `kafka.topic` |
| Key | `source_instance + NUL + alert_id` 的 lowercase SHA-256 |
| Headers | `content-type=application/json`、`ddae-schema-version=1.0` |
| Event 大小上限 | 256 KiB |
| Delivery | Durable outbox、`acks=all`、idempotent producer、at-least-once replay |

DDAE response 未提供的 fields 會省略。Typed allowlist 以外的 source fields 不會複製到
event。

### Serviceability Log event

Serviceability Logs 使用獨立 topic、record identity、producer 及 state file。

| Envelope field | JSON type | 值或來源 |
|---|---|---|
| `schema_version` | string | 固定為 `1.0`。 |
| `event_type` | string | 固定為 `ddae.serviceability_log.upsert`。 |
| `source_system` | string | 固定為 `dell_ddae`。 |
| `source_instance` | string | 設定的固定 appliance identity，1–128 UTF-8 bytes。 |
| `log_id` | string | 已驗證的 list/detail ID，1–256 UTF-8 bytes。 |
| `content_hash_sha256` | string | Canonical normalized `log` JSON 的 64-character lowercase SHA-256。 |
| `observed_at` | string | UTC RFC 3339 observation time。 |
| `log` | object | 允許的 typed Log fields。 |

Log payload fields：

| Field | JSON type | 值域或上限 |
|---|---|---|
| `severity` | string | `critical`、`error`、`warning`、`info`、`normal` 或 `unknown` |
| `acknowledged` | boolean | `true` 或 `false` |
| `occurrence_count` | integer | 非負整數 |
| `created_at`、`updated_at` | string | UTC RFC 3339 timestamp |
| `app_name`、`component`、`namespace` | string | 各最大 256 UTF-8 bytes |
| `message` | string | 最大 8192 UTF-8 bytes |
| `reason` | string | 最大 4096 UTF-8 bytes |
| `remedies` | array of strings | 最多 32 筆，每筆最大 2048 UTF-8 bytes |
| `resource_id`、`related` | string | 最大 512 UTF-8 bytes |
| `symptom_id` | string | 最大 256 UTF-8 bytes |

Serviceability Log JSON 範本：

```json
{
  "schema_version": "1.0",
  "event_type": "ddae.serviceability_log.upsert",
  "source_system": "dell_ddae",
  "source_instance": "fixture-site",
  "log_id": "log-1",
  "content_hash_sha256": "a51e004808c177b20fc11b89fdfc0823f10e9dccf5a4a11481158a802fd0d17a",
  "observed_at": "2026-08-24T03:00:00Z",
  "log": {
    "severity": "warning",
    "acknowledged": false,
    "occurrence_count": 2,
    "created_at": "2026-08-24T02:00:00Z",
    "message": "synthetic serviceability log",
    "remedies": [
      "inspect synthetic node"
    ]
  }
}
```

Kafka record contract：

| 項目 | 值 |
|---|---|
| Topic | `kafka.serviceability_logs_topic`；預設為 `ddae-serviceability-logs` |
| Key | `source_instance + NUL + serviceability_log + NUL + log_id` 的 lowercase SHA-256 |
| Headers | `content-type=application/json`、`ddae-schema-version=1.0`、`ddae-record-kind=serviceability_log` |
| Event 大小上限 | 256 KiB |
| Delivery | 獨立 durable outbox、`acks=all`、idempotent producer、per-key ordering、at-least-once replay |

Source 未提供的 optional fields 會省略。`labels`、`links` 和 unknown source fields 不會
包含在 event。存在但格式錯誤的值會使該 record 失敗，並保留原有 checkpoint。

## 部署

### 從原始碼建置

```bash
git clone https://github.com/crispkid/Dell-DDAE-Metrics-Exporter.git
cd Dell-DDAE-Metrics-Exporter
git checkout v1.0.0-rc3
go mod download
go test ./...
./scripts/build.sh
```

執行檔會建立在 `bin/ddae-exporter`。

寫入 release metadata：

```bash
VERSION=1.0.0-rc3 \
REVISION=REPLACE_WITH_GIT_SHA \
BUILD_DATE=REPLACE_WITH_UTC_RFC3339 \
./scripts/build.sh
```

### Docker

在 repository root 建置 image：

```bash
docker build \
  --build-arg VERSION=1.0.0-rc3 \
  --build-arg REVISION=REPLACE_WITH_GIT_SHA \
  --build-arg BUILD_DATE=REPLACE_WITH_UTC_RFC3339 \
  --tag ddae-exporter:v1.0.0-rc3 \
  .
```

最終 image 使用 `scratch`、UID/GID `65532`，entrypoint 為 `/ddae-exporter`。

建立 host directories：

```bash
sudo install -d -m 0700 -o 65532 -g 65532 /srv/ddae-exporter/state
sudo install -d -m 0700 -o 65532 -g 65532 /etc/ddae-exporter/secrets
sudo install -d -m 0755 -o root -g root /etc/ddae-exporter/trust
sudo install -o root -g root -m 0600 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
```

Container 內使用以下 paths：

| 資料 | Container path |
|---|---|
| YAML | `/etc/ddae-exporter/config.yaml` |
| Credentials | `/run/secrets/...` |
| CA bundles | `/run/trust/...` |
| Persistent state | `/var/lib/ddae-exporter` |

將 `server.listen_address` 設為 `0.0.0.0:9469`，更新 YAML 中所有 paths，再啟動
container：

```bash
docker run --rm \
  --name ddae-exporter \
  --read-only \
  --user 65532:65532 \
  --publish 127.0.0.1:9469:9469 \
  --mount type=bind,source=/etc/ddae-exporter/config.yaml,target=/etc/ddae-exporter/config.yaml,readonly \
  --mount type=bind,source=/etc/ddae-exporter/secrets,target=/run/secrets,readonly \
  --mount type=bind,source=/etc/ddae-exporter/trust,target=/run/trust,readonly \
  --mount type=bind,source=/srv/ddae-exporter/state,target=/var/lib/ddae-exporter \
  ddae-exporter:v1.0.0-rc3 --config /etc/ddae-exporter/config.yaml
```

### Kubernetes

部署檔案：

| 檔案 | 內容 |
|---|---|
| [`deploy/kubernetes/configmap.yaml`](deploy/kubernetes/configmap.yaml) | 非敏感 YAML 範例。 |
| [`deploy/kubernetes/deployment.yaml`](deploy/kubernetes/deployment.yaml) | ServiceAccount、3 GiB RWO PVC、單一 replica `Recreate` Deployment 及 ClusterIP Service。 |
| [`deploy/kubernetes/networkpolicy.yaml`](deploy/kubernetes/networkpolicy.yaml) | Default-deny ingress 及 egress policy。 |

部署前：

1. 將 image reference 換成 immutable image digest。
2. 更新 ConfigMap 內的 DDAE、Kafka、monitoring mode 及 runtime limits。
3. 選擇支援 RWO、fsync 和 file locking 的 storage class。
4. 在 NetworkPolicy 加入精確的 DNS、DDAE、Kafka 和 Prometheus rules。
5. 透過 service mesh 或 mTLS proxy 提供受控的 Prometheus access。

從受控檔案建立 Secrets：

```bash
kubectl -n monitoring create secret generic ddae-exporter-credentials \
  --from-file=username=/secure/runtime/ddae-username \
  --from-file=password=/secure/runtime/ddae-password \
  --from-file=client-secret=/secure/runtime/ddae-client-secret

kubectl -n monitoring create secret generic ddae-exporter-kafka \
  --from-file=password=/secure/runtime/kafka-password

kubectl -n monitoring create secret generic ddae-exporter-trust \
  --from-file=ddae-ca.pem=/secure/runtime/ddae-ca.pem \
  --from-file=kafka-ca.pem=/secure/runtime/kafka-ca.pem
```

套用 resources：

```bash
kubectl -n monitoring apply -f deploy/kubernetes/configmap.yaml
kubectl -n monitoring apply -f /secure/runtime/ddae-exporter-allow-policy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/networkpolicy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/deployment.yaml
```

檢查 deployment：

```bash
kubectl -n monitoring rollout status deployment/ddae-exporter
kubectl -n monitoring get pod,service -l app.kubernetes.io/name=ddae-exporter
kubectl -n monitoring get pvc ddae-exporter-state
kubectl -n monitoring logs deployment/ddae-exporter
```

Deployment 使用單一 replica 和 `Recreate`，確保只有一個 bbolt writer。

### Linux 及 systemd

Repository 提供
[`deploy/systemd/ddae-exporter.service`](deploy/systemd/ddae-exporter.service)。Unit 使用
`LoadCredential=` 傳遞 secrets。

建立 service account 及 directories：

```bash
sudo useradd --system \
  --home-dir /var/lib/ddae-exporter \
  --shell /usr/sbin/nologin \
  ddae-exporter

sudo install -o root -g root -m 0755 bin/ddae-exporter /usr/local/bin/ddae-exporter
sudo install -d -o root -g ddae-exporter -m 0750 /etc/ddae-exporter
sudo install -d -o root -g root -m 0700 /etc/ddae-exporter/secrets
sudo install -d -o root -g root -m 0755 /etc/ddae-exporter/trust
sudo install -d -o ddae-exporter -g ddae-exporter -m 0700 /var/lib/ddae-exporter
```

安裝設定及 unit：

```bash
sudo install -o root -g ddae-exporter -m 0640 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
sudo install -o root -g root -m 0644 \
  deploy/systemd/ddae-exporter.service /etc/systemd/system/ddae-exporter.service
```

安裝 systemd credential source files：

```bash
sudo install -o root -g root -m 0600 /secure/runtime/ddae-username \
  /etc/ddae-exporter/secrets/ddae-username
sudo install -o root -g root -m 0600 /secure/runtime/ddae-password \
  /etc/ddae-exporter/secrets/ddae-password
sudo install -o root -g root -m 0600 /secure/runtime/ddae-client-secret \
  /etc/ddae-exporter/secrets/ddae-client-secret
sudo install -o root -g root -m 0600 /secure/runtime/kafka-password \
  /etc/ddae-exporter/secrets/kafka-password
```

更新 `/etc/ddae-exporter/config.yaml`，將 CA bundles 安裝到
`/etc/ddae-exporter/trust/`，再啟動 service：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ddae-exporter
sudo systemctl status ddae-exporter
sudo journalctl -u ddae-exporter
```

### Prometheus

本機 scrape 設定：

```yaml
scrape_configs:
  - job_name: dell-ddae
    scrape_interval: 30s
    static_configs:
      - targets: [127.0.0.1:9469]
        labels:
          site: example-site
          environment: production
```

透過 mTLS proxy 進行 remote scrape：

```yaml
scrape_configs:
  - job_name: dell-ddae
    scheme: https
    tls_config:
      ca_file: /etc/prometheus/trust/exporter-ca.pem
      cert_file: /etc/prometheus/certs/prometheus.pem
      key_file: /etc/prometheus/certs/prometheus-key.pem
      min_version: TLS12
    static_configs:
      - targets: [ddae-exporter-proxy.example.invalid:9443]
        labels:
          site: example-site
          environment: production
```

## 維運

### 啟動檢查

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

| 範圍 | Metrics 或 evidence |
|---|---|
| Process | `/healthz`、`ddae_build_info` |
| 啟用模式 | `ddae_monitoring_enabled` |
| Resources | `ddae_up`、`ddae_collector_success`、`ddae_snapshot_age_seconds` |
| Alerts | `ddae_alert_list_complete`、`ddae_alert_pipeline_ready`、`ddae_alert_detail_deferred` |
| Alert delivery | `ddae_kafka_publish_success`、`ddae_kafka_buffered_events`、`ddae_kafka_events_failed_total` |
| Logs | `ddae_serviceability_log_list_complete`、`ddae_serviceability_log_pipeline_ready`、`ddae_serviceability_log_detail_deferred` |
| Log delivery | `ddae_serviceability_log_kafka_publish_success`、`ddae_serviceability_log_buffered_records`、`ddae_serviceability_log_records_failed_total` |

### 從 RC2 遷移 path

RC2 的 Ping 和其他 API operations 都使用 `/rest/v1`。RC3 升級期間可用下列設定保留
相同 routes：

```yaml
ddae:
  paths:
    ping_prefix: /rest/v1
    api_prefix: /rest/v1
```

確認目標 DDAE gateway 提供的 routes 後再變更 prefixes。設定變更後重新啟動
Exporter。

### State 及 recovery

`state.db` 和 `serviceability-logs.db` 都只能有一個 writer。複製、還原或移動 state
files 前先停止 Exporter。Kafka 無法使用期間保留兩個 databases，讓 pending records
可在服務恢復後 replay。

Incident response、outbox recovery 及 rollback 步驟位於
[`docs/runbook.md`](docs/runbook.md)。

## 開發

### 指令

| 用途 | 指令 |
|---|---|
| Lint 及 vet | `./scripts/stage-lint.sh` |
| Race-enabled tests | `./scripts/stage-test.sh` |
| Coverage gate | `./scripts/stage-coverage.sh` |
| Build | `./scripts/build.sh` |
| Security policy | `./scripts/security-policy.sh` |
| CI policy | `./scripts/stage-ci-policy.sh` |
| Supply-chain artifacts | `./scripts/supply-chain.sh` |

### RC3 本機驗證

| 檢查 | 結果 |
|---|---|
| Lint、vet、formatting | 通過 |
| Race-enabled unit/component tests | 通過 |
| Coverage | 81.5%；門檻 80% |
| Build | 通過 |
| Security policy | 通過 |
| `govulncheck` v1.7.0 | `No vulnerabilities found.` |
| Reproducible build 及 CycloneDX SBOM | 通過 |
| External authenticated integration 及 E2E | 保留為後續 release gates |

## 授權

本專案使用 [Apache License 2.0](LICENSE)。Dell 及相關產品名稱為各自權利人的商標。
專案名稱只用於表示目標系統，不代表任何形式的 endorsement。
