# Dell DDAE Metrics Exporter

Dell DDAE Metrics Exporter 是用 Go 開發的唯讀監控程式，設計目標為 Dell Data Domain
Active Enterprise（DDAE）1.5.0。它會讀取 DDAE 管理 API，將叢集、節點和系統狀態
轉成 Prometheus 指標，也能把告警與 Serviceability Logs 傳送到 Kafka。

目前的候選版本是 `v1.0.0-rc3`。

RC3 已完成本機單元、元件、安全、建置和文件檢查。實際 DDAE、Kafka、OpenSearch 與
完整部署環境的整合驗證屬於後續 release gate，因此 RC3 是測試用候選版本，不是 GA
版本。

## 第一次使用請照這個順序

1. 先看[可用功能](#可用功能)，決定要開啟資源監控、告警或 Serviceability Logs。
2. 按照[安裝前準備](#安裝前準備)建立 DDAE 唯讀帳號，並準備 Kafka 和 CA 檔案。
3. 選擇安裝方式：[Release 套件](#使用-release-套件安裝)、[原始碼](#從原始碼安裝)、[Docker](#docker-安裝)、[Kubernetes](#kubernetes-安裝)或[systemd](#linux--systemd-安裝)。
4. 依照[建立帳號密碼檔](#4-建立帳號密碼檔)建立憑證檔案。
5. 參考[完整 YAML 設定](#完整-yaml-設定)修改設定檔。範例中的每一個設定都有說明。
6. 啟動程式後，按照[啟動後檢查](#啟動後檢查)確認健康狀態和監控資料。
7. 串接 Prometheus 或 Kafka 前，可先查閱[指標的單位與值域](#prometheus-指標)及[Kafka 告警格式](#kafka-告警格式)。

如果只想先確認 DDAE 連線，可以先啟用 `resources`，並關閉 `alerts` 和
`serviceability_logs`。這個模式不需要 Kafka。

## 可用功能

| 功能 | 說明 |
|---|---|
| DDAE 資源監控 | 定期讀取 API 連線狀態、叢集狀態、Coordinator/Worker 配置、節點資源、系統鎖定狀態和就緒狀態。 |
| Prometheus 指標 | 在背景更新監控結果，並由 `/metrics` 提供 Prometheus 格式的資料。 |
| DDAE 告警 | 先取得 serviceability issue 清單，再依設定的數量和同時連線數讀取詳細內容。 |
| Kafka 告警 | 先將告警存入本機 bbolt outbox，再傳送到 Kafka；傳送失敗時會保留資料供後續重送。 |
| Serviceability Logs | 讀取 Serviceability Event 清單和詳細內容，將新增或變更的資料傳送到獨立的 Kafka topic。 |
| 獨立的告警與 Log 儲存 | 告警使用 `state.db`，Serviceability Logs 使用 `serviceability-logs.db`，兩者有各自的 outbox、checkpoint 和容量限制。 |
| 分開開關 | `resources`、`alerts` 和 `serviceability_logs` 可以分別開啟、關閉和設定收集週期。 |
| YAML 設定 | 使用有版本和型別檢查的 YAML，也可以用命令列或環境變數指定設定檔。 |
| DDAE API 路徑 | Ping 和其他管理 API 可分別設定 prefix。預設使用 `/ping` 和 `/v1/*`，也可設定成 RC2 或 Dell PDF 使用的路徑。 |
| TLS | DDAE 和 Kafka 預設檢查憑證與主機名稱，可加入自有 CA；Kafka 也支援 mTLS。 |
| Kafka 登入 | 支援 `PLAIN`、`SCRAM-SHA-256` 和 `SCRAM-SHA-512` SASL。 |
| 執行限制 | 可設定回應大小、逾時、重試、同時讀取數量和 outbox 容量。 |
| 部署方式 | 提供原始碼建置、Docker、Kubernetes 和 Linux/systemd 所需檔案。 |

程式的資料流如下：

```text
Dell DDAE read-only Management API
        |
        +--> resource collectors --> current snapshot --> Prometheus /metrics
        |
        +--> alert list --> bounded detail GETs --> typed event --> durable outbox --> Kafka
        |
        +--> log list --> bounded detail GETs --> typed log event --> dedicated outbox --> dedicated Kafka topic
```

## 選擇監控模式

`resources`、`alerts` 和 `serviceability_logs` 至少要開啟一項。常用組合如下：

| 模式 | `resources` | `alerts` | `serviceability_logs` | 程式會做的事 |
|---|---:|---:|---:|---|
| 預設 | `true` | `true` | `false` | 提供 resource metrics 並發布 alerts。 |
| 只監控資源 | `true` | `false` | `false` | 提供 DDAE 資源指標，不連線到 Kafka。 |
| 只傳送告警 | `false` | `true` | `false` | 讀取告警、保存待送資料並傳送到 Kafka。 |
| 只傳送 Logs | `false` | `false` | `true` | 讀取 Serviceability Logs，使用獨立狀態檔並傳送到 Kafka。 |
| 全部啟用 | `true` | `true` | `true` | 三項功能分別排程、儲存和判斷是否就緒。 |

已開啟的功能都會納入 `/readyz` 判定。`ddae_monitoring_enabled` 指標會固定輸出
`resources`、`alerts` 和 `serviceability_logs` 三筆資料，值為 `1` 表示已開啟，`0`
表示已關閉。

## DDAE API 路徑

Exporter 只會呼叫下列 API：

| 類別 | 使用的 prefix | 預設 method 和 path | 用途 |
|---|---|---|---|
| 登入 | 固定路徑 | `POST /auth/realms/ddae/protocol/openid-connect/token` | 取得 `dv-admin-rest` bearer token。 |
| Ping | `ping_prefix` | `GET /ping` | 確認 Management API reachability。 |
| Clusters | `api_prefix` | `GET /v1/ddae-clusters` | 取得 cluster 狀態與 Coordinator/Worker 配置。 |
| Nodes | `api_prefix` | `GET /v1/infrastructure-nodes` | 取得 node 狀態、capacity、allocatable 與 conditions。 |
| System lock | `api_prefix` | `GET /v1/system-lock` | 取得 appliance lock 狀態。 |
| System shutdown | `api_prefix` | `GET /v1/system-shutdown` | 取得 control-plane 與 node readiness。 |
| Alert list | `api_prefix` | `GET /v1/serviceability-issues` | 取得告警清單與 lifecycle input。 |
| Alert detail | `api_prefix` | `GET /v1/serviceability-issues/{id}` | 取得經 allowlist 處理的告警內容。 |
| Serviceability Log list | `api_prefix` | `GET /v1/serviceability-events` | 發現 Serviceability Log IDs 並評估 completeness。 |
| Serviceability Log detail | `api_prefix` | `GET /v1/serviceability-events/{id}` | 取得經 typed allowlist 處理的 log detail。 |

請為 Exporter 建立專用的 DDAE 唯讀帳號，只授予讀取上述資料所需的權限。DDAE 連線
最低使用 TLS 1.2，預設會檢查憑證鏈和主機名稱。

### 分別設定 Ping 和其他 API 的 prefix

Exporter 提供兩個 prefix 設定。`ping_prefix` 只影響 Ping，`api_prefix` 用於其他管理
API。每項操作的 suffix、HTTP method 和 OAuth token path 由程式固定。實際路徑會把
prefix 和固定 suffix 直接接在一起，不使用 `path.Join`、URL resolution、runtime discovery
或 alternate-path fallback。

| 設定 | 預設值 | 使用範圍 |
|---|---|---|
| `ddae.paths.ping_prefix` | 空字串 | 只套用到 `/ping`。 |
| `ddae.paths.api_prefix` | `/v1` | 套用到 clusters、nodes、lock、shutdown、alert list/detail 與 Serviceability Log list/detail。 |

請依 DDAE 環境選擇一組設定：

| 環境 | `ping_prefix` | `api_prefix` | Ping 路徑 | API 路徑範例 |
|---|---|---|---|---|
| 新預設／實機回報的 service shape | `""` | `/v1` | `/ping` | `/v1/ddae-clusters` |
| 保留 v1.0.0-rc2 routes | `/rest/v1` | `/rest/v1` | `/rest/v1/ping` | `/rest/v1/ddae-clusters` |
| Dell PDF Ping operation + API examples | `/rest` | `/rest/v1` | `/rest/ping` | `/rest/v1/ddae-clusters` |

Prefix 可以是空字串。非空值需符合以下規則：

- 最大 128 bytes。
- 以一個 `/` 開頭，結尾不加 `/`。
- 每一段只使用 `A-Z`、`a-z`、`0-9`、`.`、`_`、`~`、`-`。
- 每一段使用實際名稱，不使用 `.`、`..`、空白、重複 `/` 或空的路徑段。
- 填入一般 ASCII 路徑，不加入 percent encoding、反斜線、Unicode、query、fragment、
  scheme、authority 或帳號資料。
- 程式會照原值使用，不做 trim、大小寫轉換、clean、decode 或 normalization。格式錯誤
  時會在 HTTP server 和背景工作啟動前停止並回報錯誤。

也可以用 `DDAE_PING_PATH_PREFIX` 和 `DDAE_API_PATH_PREFIX` 覆蓋 YAML。設定優先順序為
環境變數、YAML、程式預設值。環境變數設為空字串時，程式會把它當成有效設定。Prefix
和完整 endpoint 不會加入 Prometheus label；路徑回傳錯誤時，程式會回報原始錯誤，
不會自動改用另一組路徑。

## Exporter HTTP 端點

Exporter 預設只監聽本機的 `127.0.0.1:9469`。

| 端點 | 回應內容 |
|---|---|
| `GET /metrics` | Prometheus exposition 與 collector diagnostics；最多同時處理 5 個 requests，每個 handler 的 timeout 為 9 秒。 |
| `GET /healthz` | 程序存活狀態。 |
| `GET /readyz` | 所有已啟用 pipeline 的 readiness；ready 時回 `200`，其餘狀態回 `503`。 |

## Prometheus 指標

### 共用指標的單位與值域

| Metric | Type | 單位 | 值域與意義 |
|---|---|---|---|
| `ddae_build_info{version,go_version}` | gauge | 無單位 | 固定為 `1`；labels 表示 Exporter 與 Go build version。 |
| `ddae_monitoring_enabled{pipeline}` | gauge | boolean | `0` 或 `1`；固定輸出 `resources`、`alerts`、`serviceability_logs` 三筆 series。 |
| `ddae_collector_success{collector}` | gauge | boolean | `0` 或 `1`；最近一輪指定 collector 的結果，輸出已啟用 pipeline 的 collector labels。 |
| `ddae_collector_duration_seconds{collector}` | gauge | seconds | 大於或等於 `0` 的秒數，可包含小數；輸出已啟用 pipeline 的 collector labels。 |

### 資源監控指標的單位與值域

| Metric | Type | 單位 | 值域與意義 |
|---|---|---|---|
| `ddae_up` | gauge | boolean | `0` 或 `1`；所有必要 resource collectors 成功且 snapshot 仍在 freshness window 時為 `1`。 |
| `ddae_snapshot_age_seconds` | gauge | seconds | 大於或等於 `0` 的秒數；表示最近完整 resource snapshot 的年齡。 |
| `ddae_management_api_up` | gauge | boolean | `0` 或 `1`；ping collector 完成有效 authenticated request 時為 `1`。 |
| `ddae_cluster_state_info{cluster,state}` | gauge | one-hot boolean | 每筆為 `0` 或 `1`；`state` 固定為 `available`、`unknown`，每個 cluster 的目前狀態對應 series 為 `1`。 |
| `ddae_cluster_coordinator_configured_cpu_cores{cluster}` | gauge | CPU cores | 大於或等於 `0` 的有限數值，可為小數；表示 configured CPU。 |
| `ddae_cluster_coordinator_configured_memory_bytes{cluster}` | gauge | bytes | 大於或等於 `0` 的有限數值；由 Kubernetes quantity 轉換。 |
| `ddae_cluster_worker_configured_cpu_cores{cluster}` | gauge | CPU cores | 大於或等於 `0` 的有限數值，可為小數；表示 worker configuration object 的 CPU quantity。 |
| `ddae_cluster_worker_configured_memory_bytes{cluster}` | gauge | bytes | 大於或等於 `0` 的有限數值；由 worker configuration quantity 轉換。 |
| `ddae_node_state_info{node,state}` | gauge | one-hot boolean | 每筆為 `0` 或 `1`；每個 node 的目前 normalized state 對應 series 為 `1`。 |
| `ddae_node_ready{node}` | gauge | boolean | `0` 或 `1`；normalized node state 為 `ready` 時為 `1`。 |
| `ddae_node_capacity_cpu_cores{node}` | gauge | CPU cores | 大於或等於 `0` 的有限數值，可為小數；表示 total capacity。 |
| `ddae_node_capacity_memory_bytes{node}` | gauge | bytes | 大於或等於 `0` 的有限數值；表示 total memory capacity。 |
| `ddae_node_capacity_ephemeral_storage_bytes{node}` | gauge | bytes | 大於或等於 `0` 的有限數值；表示 total ephemeral-storage capacity。 |
| `ddae_node_allocatable_cpu_cores{node}` | gauge | CPU cores | 大於或等於 `0` 的有限數值，可為小數；表示 allocatable CPU。 |
| `ddae_node_allocatable_memory_bytes{node}` | gauge | bytes | 大於或等於 `0` 的有限數值；表示 allocatable memory。 |
| `ddae_node_allocatable_ephemeral_storage_bytes{node}` | gauge | bytes | 大於或等於 `0` 的有限數值；表示 allocatable ephemeral storage。 |
| `ddae_node_condition{node,condition}` | gauge | boolean | `0` 或 `1`；`condition` 固定為 `disk_pressure`、`memory_pressure`，`1` 表示 pressure 存在。 |
| `ddae_system_locked` | gauge | boolean | `0` 或 `1`；appliance locked 時為 `1`。 |
| `ddae_control_plane_ready` | gauge | boolean | `0` 或 `1`；control-plane nodes ready 時為 `1`。 |
| `ddae_nodes_ready` | gauge | nodes | `0` 到 `ddae_nodes_total` 的整數。 |
| `ddae_nodes_total` | gauge | nodes | 大於或等於 `0` 的整數。 |

CPU、memory 與 storage 欄位在 DDAE response 缺少值時省略對應 series；present value
會先通過 Kubernetes quantity parsing，並只輸出非負有限數值。

`ddae_node_state_info` 的 `state` 固定值域如下：

| State | 意義 |
|---|---|
| `maintenance_mode` | Node 處於 maintenance mode。 |
| `scheduling_disabled` | Node 已停用 scheduling。 |
| `not_ready` | Node 尚未 ready。 |
| `ready` | Node ready。 |
| `restarting` | Node 正在 restart。 |
| `shutting_down` | Node 正在 shutdown。 |
| `powered_off` | Node 已關機。 |
| `powering_on` | Node 正在開機。 |
| `unknown` | Source state 經 bounded normalization 後歸入 unknown。 |

### 告警指標的單位與值域

| Metric | Type | 單位 | 值域與意義 |
|---|---|---|---|
| `ddae_alert_list_complete` | gauge | boolean | `0` 或 `1`；list 結構有效且 `totalRecords` 至多為可用 result 數量時為 `1`。 |
| `ddae_alert_detail_deferred` | gauge | alerts | 大於或等於 `0` 的整數；表示最近一輪因 `max_per_cycle` 延後的 alert IDs。 |
| `ddae_alert_pipeline_ready` | gauge | boolean | `0` 或 `1`；list/detail、persistent state 與 outbox capacity 符合條件時為 `1`。 |
| `ddae_kafka_publish_success` | gauge | boolean | `0` 或 `1`；最近一次 required Kafka publish batch 已取得 acknowledgement 時為 `1`。 |
| `ddae_kafka_publish_duration_seconds` | gauge | seconds | 大於或等於 `0` 的秒數，可包含小數。 |
| `ddae_kafka_events_published_total` | counter | events | 大於或等於 `0` 的整數；程序運行期間單調增加，計算 Kafka 已確認的 events。 |
| `ddae_kafka_events_failed_total{reason}` | counter | events | 大於或等於 `0` 的整數；程序運行期間依固定 failure class 單調增加。 |
| `ddae_kafka_buffered_events` | gauge | events | `0` 到 `state.outbox_max_events` 的整數；byte hard limit 也可能先限制可保存數量。 |

### Serviceability Logs 指標的單位與值域

| Metric | Type | 單位 | 值域與意義 |
|---|---|---|---|
| `ddae_serviceability_log_list_complete` | gauge | boolean | `0` 或 `1`；最近 list 通過結構與 `totalRecords` completeness checks 時為 `1`。 |
| `ddae_serviceability_log_detail_deferred` | gauge | logs | 大於或等於 `0` 的整數；最近 bounded cycle 延後的 detail 數量。 |
| `ddae_serviceability_log_pipeline_ready` | gauge | boolean | `0` 或 `1`；list/detail、專用 state 與 capacity 健康時為 `1`。 |
| `ddae_serviceability_log_kafka_publish_success` | gauge | boolean | `0` 或 `1`；最近 required log publish 已取得 Kafka acknowledgement 時為 `1`。 |
| `ddae_serviceability_log_kafka_publish_duration_seconds` | gauge | seconds | 大於或等於 `0` 的秒數，可包含小數。 |
| `ddae_serviceability_log_records_published_total` | counter | records | 大於或等於 `0` 且程序期間單調增加；計算 Kafka 已確認的 log records。 |
| `ddae_serviceability_log_records_failed_total{reason}` | counter | records | 大於或等於 `0` 且依固定 failure class 單調增加。 |
| `ddae_serviceability_log_buffered_records` | gauge | records | `0` 到 `state.serviceability_logs_outbox_max_events`；byte hard limit 也可能先限制數量。 |

固定 label 值域如下：

| Label | 固定值域 |
|---|---|
| `pipeline` | `resources`、`alerts`、`serviceability_logs` |
| `collector` | `ping`、`clusters`、`nodes`、`lock`、`power`、`alert_list`、`alert_detail`、`serviceability_log_list`、`serviceability_log_detail`；僅輸出已啟用 pipeline 的 collectors。 |
| Cluster `state` | `available`、`unknown` |
| Node `state` | `maintenance_mode`、`scheduling_disabled`、`not_ready`、`ready`、`restarting`、`shutting_down`、`powered_off`、`powering_on`、`unknown` |
| `condition` | `disk_pressure`、`memory_pressure` |
| `reason` | `auth`、`tls`、`timeout`、`transport`、`http`、`decode`、`validation`、`kafka_auth`、`kafka_timeout`、`kafka_rejected`、`buffer_full`、`internal` |

Site、environment、region 等部署維度可由 Prometheus service discovery 或
relabeling 加入。

## Kafka 告警格式

每筆告警都是 JSON，欄位格式固定如下：

| Field | JSON type | 值域或來源 |
|---|---|---|
| `schema_version` | string | 固定為 `1.0`。 |
| `event_type` | string | 固定為 `ddae.serviceability_alert.upsert`。 |
| `source_system` | string | 固定為 `dell_ddae`。 |
| `source_instance` | string | YAML 中設定的穩定 appliance identity；1–128 UTF-8 bytes。 |
| `alert_id` | string | 1–256 個 ASCII 字元，使用英數字、`.`、`_`、`:`、`-`，並與 requested list ID 相同。 |
| `content_hash_sha256` | string | Canonical normalized `alert` object 的 64 字元小寫 SHA-256 hex。 |
| `observed_at` | string | UTC RFC 3339 observation time。 |
| `alert` | object | 僅含下列 typed allowlist fields。 |

`alert` 物件可包含 `severity`、`acknowledged`、`occurrence_count`、`created_at`、
`updated_at`、`clear_type`、`auto_clear_timeout_raw`、`app_name`、`component`、
`namespace`、`message`、`reason`、`remedies`、`resource_id`、`symptom_id`、`related`
與一層 `related_events`。

| Kafka record 項目 | 格式 |
|---|---|
| Key | `source_instance + NUL + alert_id` 的小寫 SHA-256 hex。 |
| Header | `content-type=application/json`。 |
| Header | `ddae-schema-version=1.0`。 |
| 傳送方式 | 先寫入可保存的 outbox，再以 at-least-once 方式發布。逾時且無法確認 broker 是否已收到時，record 會留在 outbox 等待重送。Consumer 應使用 record key 做可重複執行的 upsert。 |
| Event size | 最大 256 KiB。 |

### 告警 JSON 範本

以下是完整範例。`content_hash_sha256` 是 `alert` 物件的 SHA-256；時間使用 UTC。
DDAE 沒有提供的選填欄位不會出現在輸出中。

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

`alert` 內的欄位型別與值域如下：

| Field | JSON type | 值域／上限 |
|---|---|---|
| `severity` | string | `critical`、`error`、`warning`、`info`、`normal`、`unknown`。 |
| `acknowledged` | boolean | `true` 或 `false`。 |
| `occurrence_count` | integer | 大於或等於 `0`。 |
| `created_at`、`updated_at` | string | UTC RFC 3339 timestamp。 |
| `clear_type` | string | 最多 64 UTF-8 bytes。 |
| `auto_clear_timeout_raw` | integer | 大於或等於 `0`；Dell 1.5.0 未定義此 raw value 的單位。 |
| `app_name`、`component`、`namespace` | string | 每個欄位最多 256 UTF-8 bytes。 |
| `message` | string | 最多 8192 UTF-8 bytes。 |
| `reason` | string | 最多 4096 UTF-8 bytes。 |
| `remedies` | array of strings | 最多 32 筆，每筆最多 2048 UTF-8 bytes。 |
| `resource_id`、`related` | string | 最多 512 UTF-8 bytes。 |
| `symptom_id` | string | 最多 256 UTF-8 bytes。 |
| `related_events` | array of typed objects | 最多 100 筆；每筆使用相同 optional fields，並維持一層結構。 |

Kafka consumer 可以用下列固定資料辨識告警：

| Metadata | 值 |
|---|---|
| Record key | `SHA-256(source_instance + NUL + alert_id)` 的 64 字元小寫 hex。 |
| `content-type` header | `application/json` |
| `ddae-schema-version` header | `1.0` |

## Kafka Serviceability Log 格式

Serviceability Logs 使用獨立的格式、record kind、topic 和狀態檔。每筆資料都是下列
JSON 結構：

| Field | JSON type | 值域或來源 |
|---|---|---|
| `schema_version` | string | 固定為 `1.0`。 |
| `event_type` | string | 固定為 `ddae.serviceability_log.upsert`。 |
| `source_system` | string | 固定為 `dell_ddae`。 |
| `source_instance` | string | YAML 的穩定 appliance identity，1–128 UTF-8 bytes。 |
| `log_id` | string | Detail ID，1–256 UTF-8 bytes，並與 requested list ID 相同。 |
| `content_hash_sha256` | string | Canonical normalized `log` object 的 64 字元小寫 SHA-256。 |
| `observed_at` | string | UTC RFC 3339 observation time。 |
| `log` | object | 下表的 typed allowlist fields。 |

| Log field | JSON type | 值域／上限 |
|---|---|---|
| `severity` | string | `critical`、`error`、`warning`、`info`、`normal`、`unknown`。 |
| `acknowledged` | boolean | `true` 或 `false`。 |
| `occurrence_count` | integer | 大於或等於 `0`。 |
| `created_at`、`updated_at` | string | UTC RFC 3339 timestamp。 |
| `app_name`、`component`、`namespace` | string | 各最多 256 UTF-8 bytes。 |
| `message` | string | 最多 8192 UTF-8 bytes。 |
| `reason` | string | 最多 4096 UTF-8 bytes。 |
| `remedies` | array of strings | 最多 32 筆，每筆最多 2048 UTF-8 bytes。 |
| `resource_id`、`related` | string | 最多 512 UTF-8 bytes。 |
| `symptom_id` | string | 最多 256 UTF-8 bytes。 |

輸出只包含表格中的欄位。DDAE 沒有提供的選填欄位會省略；如果 DDAE 傳回的欄位值不符
格式，該筆資料會回報失敗，原有 checkpoint 會保留。

### Serviceability Log JSON 範本

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

| Kafka record 項目 | 格式 |
|---|---|
| Topic | `kafka.serviceability_logs_topic`，預設 `ddae-serviceability-logs`，與 alert topic 不同。 |
| Key | `SHA-256(source_instance + NUL + serviceability_log + NUL + log_id)` 的 64 字元小寫 hex。 |
| Headers | `content-type=application/json`、`ddae-schema-version=1.0`、`ddae-record-kind=serviceability_log`。 |
| 傳送方式 | 使用獨立 outbox、`acks=all`、idempotent producer、相同 key 依序傳送，並支援 at-least-once 重送。 |
| Event size | 最大 256 KiB。 |

## 安裝前準備

先確認要開啟哪些監控功能，再準備對應項目。只使用資源監控時，不需要 Kafka 和
persistent state directory。

| 使用的功能 | 需要準備的項目 |
|---|---|
| 全部 | DDAE 1.5.0 的 HTTPS 位址，例如 `https://ddae.example.com`。 |
| 全部 | Exporter 專用的 DDAE 唯讀 username、password 和 `dv-admin-rest` client secret。 |
| 全部 | DDAE 使用內部 CA 時，準備 PEM 格式的 CA bundle。 |
| 告警 | Kafka broker、告警專用 topic、TLS 和 `acks=all`。 |
| 告警 | 依 Kafka 環境準備 SASL、mTLS，或同時準備兩者。 |
| 告警 | 可使用 file lock 和 fsync 的持久化目錄；每一套 DDAE 使用自己的目錄。 |
| Serviceability Logs | 獨立的 Kafka topic 與 ACL、OpenSearch mapping，以及 `serviceability-logs.db` 所需容量。 |
| Prometheus | Prometheus 能連到 Exporter 的受控網路路徑；跨主機可使用 mTLS reverse proxy 或 service mesh。 |
| 執行主機 | 已完成時間同步，讓逾時、資料新鮮度和 `observed_at` 時間正確。 |

## 使用 Release 套件安裝

RC3 提供 macOS 和 Linux 的 amd64、arm64 套件。請到
[GitHub Releases](https://github.com/crispkid/Dell-DDAE-Metrics-Exporter/releases/tag/v1.0.0-rc3)
下載符合主機作業系統和 CPU 架構的檔案：

| 作業系統 | CPU | 檔名 |
|---|---|---|
| macOS | Intel | `dell-ddae-metrics-exporter_1.0.0-rc3_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `dell-ddae-metrics-exporter_1.0.0-rc3_darwin_arm64.tar.gz` |
| Linux | x86-64 | `dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64.tar.gz` |
| Linux | arm64 | `dell-ddae-metrics-exporter_1.0.0-rc3_linux_arm64.tar.gz` |

以下以 Linux x86-64 為例。先在 Release 頁面核對 SHA-256，再解壓縮：

```bash
shasum -a 256 dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64.tar.gz
tar -xzf dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64.tar.gz
cd dell-ddae-metrics-exporter_1.0.0-rc3_linux_amd64
```

套件內包含：

| 檔案 | 用途 |
|---|---|
| `ddae-exporter` | Exporter 執行檔。 |
| `config.example.yaml` | 完整 YAML 設定範例。 |
| `README.md` | 安裝、設定和資料格式說明。 |
| `RUNBOOK.md` | 維運、復原和 rollback 步驟。 |
| `LICENSE` | Apache License 2.0。 |

接著按照[建立帳號密碼檔](#4-建立帳號密碼檔)準備檔案，把
`config.example.yaml` 複製成自己的設定檔，再啟動：

```bash
cp config.example.yaml config.yaml
chmod 0600 config.yaml
./ddae-exporter --config ./config.yaml
```

啟動前請先把 `config.yaml` 內的範例位址、帳號密碼檔路徑、CA 路徑和 Kafka 設定換成
實際值。

## 從原始碼安裝

### 1. 安裝工具

| 工具 | 要求 |
|---|---|
| Git | 下載原始碼。 |
| Go | `go.mod` 宣告 language version `1.26.0` 與 toolchain `go1.26.6`。 |
| Go module source/cache | 第一次下載相依套件時使用。 |

確認 Go 版本：

```bash
go version
```

### 2. 下載 RC3 原始碼

```bash
git clone https://github.com/crispkid/Dell-DDAE-Metrics-Exporter.git
cd Dell-DDAE-Metrics-Exporter
git checkout v1.0.0-rc3
```

### 3. 建置程式

在專案根目錄執行：

```bash
go mod download
go test ./...
./scripts/build.sh
```

完成後會產生 `bin/ddae-exporter`。如果要自行寫入版本、commit 和建置時間，可以執行：

```bash
VERSION=1.0.0-rc3 \
REVISION=REPLACE_WITH_GIT_SHA \
BUILD_DATE=2026-09-01T00:00:00Z \
./scripts/build.sh
```

`REVISION` 請換成 `git rev-parse HEAD` 顯示的 commit SHA，`BUILD_DATE` 請填 UTC 時間。

### 4. 建立帳號密碼檔

先建立只允許目前帳號讀取的目錄：

```bash
mkdir -p secrets
chmod 0700 secrets
```

每個 `*_file` 都是一般文字檔，檔案內只放一個值。請依序執行下列命令；password 和
client secret 輸入時不會顯示在畫面上：

```bash
umask 077
printf 'DDAE username: '
IFS= read -r ddae_username_value
printf 'DDAE password: '
IFS= read -r -s ddae_password_value
printf '\nDDAE dv-admin-rest client secret: '
IFS= read -r -s ddae_client_secret_value
printf '\nKafka SASL password（未使用 Kafka SASL 可直接按 Enter）: '
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

四個檔案的用途如下：

| YAML key | 本機檔案 | 範例內容 | 內容來源 |
|---|---|---|---|
| `ddae.credentials.username_file` | `secrets/ddae-username` | `ddae-exporter-reader` | DDAE 唯讀帳號。 |
| `ddae.credentials.password_file` | `secrets/ddae-password` | `REPLACE_WITH_DDAE_READ_ONLY_PASSWORD` | DDAE 唯讀帳號的密碼。 |
| `ddae.credentials.client_secret_file` | `secrets/ddae-client-secret` | `REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET` | `dv-admin-rest` client secret。 |
| `kafka.sasl.password_file` | `secrets/kafka-password` | `REPLACE_WITH_KAFKA_SASL_PASSWORD` | Kafka SASL 密碼；只有啟用 SASL 時需要。 |

例如 `ddae-username` 的內容是：

```text
ddae-exporter-reader
```

檔案內不要加 `username=`、`password=`、YAML key、JSON 或引號。程式接受 UTF-8，單一
檔案最大 64 KiB，並會移除檔案結尾的一個換行字元。

### 5. 建立本機設定檔

專案內的完整 YAML 範例位於
[`deploy/systemd/config.example.yaml`](deploy/systemd/config.example.yaml)。建立本機副本：

```bash
cp deploy/systemd/config.example.yaml config.local.yaml
mkdir -p state trust
chmod 0700 state
chmod 0600 config.local.yaml
```

執行 `pwd` 取得專案的完整路徑，然後修改 `config.local.yaml`：

```bash
pwd
```

請把 YAML 內的 `/secure/runtime/...` 改成剛才建立的檔案完整路徑，例如：

```yaml
ddae:
  credentials:
    username_file: /absolute/path/Dell-DDAE-Metrics-Exporter/secrets/ddae-username
    password_file: /absolute/path/Dell-DDAE-Metrics-Exporter/secrets/ddae-password
    client_secret_file: /absolute/path/Dell-DDAE-Metrics-Exporter/secrets/ddae-client-secret
```

再依下表設定其他項目：

| 區段 | 設定內容 |
|---|---|
| `monitoring` | 選擇要使用的資源監控、告警和 Serviceability Logs，並設定各自的收集週期。 |
| `ddae` | 填入 DDAE HTTPS 位址、固定的來源名稱、帳號密碼檔路徑和 CA 路徑。 |
| `kafka` | 開啟告警或 Serviceability Logs 時，填入 broker、topic、TLS 和需要的 SASL 設定。 |
| `state` | 開啟 Kafka 功能時，填入持久化目錄的完整路徑和容量限制。 |
| `server` | 設定 Exporter 監聽位址和關機等待時間。 |
| `logging` | 選擇 `json` 或 `text`，以及 log level。 |

本機開啟告警或 Serviceability Logs 時，請將 `state.dir` 設成剛建立的 `state` 目錄
完整路徑。

### 帳號密碼檔的部署範例

正式部署時，可以把 secret manager 輸出的檔案安裝到 `/secure/runtime`。每個檔案
只放原始值，並設定為只有 Exporter 執行帳號可以讀取。下表的內容都是佔位值：

| YAML key | 範例 path | 單行範例內容 | 內容來源 |
|---|---|---|---|
| `ddae.credentials.username_file` | `/secure/runtime/ddae-username` | `ddae-exporter-reader` | DDAE dedicated read-only identity。 |
| `ddae.credentials.password_file` | `/secure/runtime/ddae-password` | `REPLACE_WITH_DDAE_READ_ONLY_PASSWORD` | 該 DDAE identity 的 password。 |
| `ddae.credentials.client_secret_file` | `/secure/runtime/ddae-client-secret` | `REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET` | DDAE 管理者提供的 `dv-admin-rest` client secret。 |
| `kafka.sasl.password_file` | `/secure/runtime/kafka-password` | `REPLACE_WITH_KAFKA_SASL_PASSWORD` | Kafka SASL identity 的 password；SASL 啟用時使用。 |

例如 `/secure/runtime/ddae-username` 的檔案內容是：

```text
ddae-exporter-reader
```

`/secure/runtime/ddae-password` 的格式是：

```text
REPLACE_WITH_DDAE_READ_ONLY_PASSWORD
```

`/secure/runtime/ddae-client-secret` 的格式是：

```text
REPLACE_WITH_DV_ADMIN_REST_CLIENT_SECRET
```

檔案內使用純值，不加 `username=`、`password=`、YAML key、JSON 或引號。如果組織的
secret manager 已經輸出這些檔案，可在 Linux 服務帳號建立後安裝到 Exporter 的執行
目錄：

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

只監控資源時使用前三個 DDAE 檔案。開啟告警或 Serviceability Logs，且 Kafka 使用
SASL 時，再加入 `kafka-password`。本機測試可以將 secret manager 輸出的檔案安裝到
專案內的 `secrets/` 目錄；這個目錄已由 `.gitignore` 排除：

```bash
install -d -m 0700 secrets
install -m 0600 /path/from/secret-manager/ddae-username secrets/ddae-username
install -m 0600 /path/from/secret-manager/ddae-password secrets/ddae-password
install -m 0600 /path/from/secret-manager/ddae-client-secret secrets/ddae-client-secret
install -m 0600 /path/from/secret-manager/kafka-password secrets/kafka-password
```

| 檢查項目 | 預期值 |
|---|---|
| 檔案類型 | 一般檔案。 |
| 上層目錄權限 | `0700`。 |
| 帳號密碼檔權限 | `0600`。 |
| 擁有者 | 執行 Exporter 的帳號。 |
| 內容 | 單一且非空的 UTF-8 值，最大 64 KiB。 |
| YAML 路徑 | Exporter 可以讀取的完整路徑。 |

### 6. 啟動

```bash
./bin/ddae-exporter --config ./config.local.yaml
```

也可以用環境變數指定設定檔：

```bash
DDAE_EXPORTER_CONFIG_FILE=/absolute/path/config.yaml ./bin/ddae-exporter
```

同一項設定出現在多個地方時，優先順序如下：

| 優先度 | 來源 |
|---:|---|
| 1 | 個別環境變數。 |
| 2 | `--config` 指定的 YAML；同時設定 selector 時由此路徑選檔。 |
| 3 | `DDAE_EXPORTER_CONFIG_FILE` 指定的 YAML。 |
| 4 | 程式預設值。 |

既有部署仍可全部使用環境變數，不一定要使用 YAML。

### 7. 檢查本機執行結果

另開一個終端機，依序執行：

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

`/healthz` 應回傳 HTTP 200。`/readyz` 會在所有已開啟的功能完成第一次收集後回傳
HTTP 200。`/metrics` 應包含 `ddae_build_info` 和 `ddae_monitoring_enabled`。

## 完整 YAML 設定

YAML 的 `version` 固定為 `1`，檔案使用 UTF-8，大小上限為 1 MiB。程式會檢查欄位
名稱、資料型別和數值範圍。密碼和 client secret 只在 YAML 中填入檔案路徑。

以下是完整結構，每一個設定行後面都有說明。`.invalid`、`replace-with-*` 和
`/secure/runtime/*` 都是範例，部署前要換成環境中的實際值或路徑：

```yaml
version: 1 # 設定格式版本；目前固定使用整數 1。

monitoring: # 三項監控功能的設定。
  resources: # Prometheus 資源指標。
    enabled: true # true 表示讀取 ping、clusters、nodes、lock 和 power。
    interval: 30s # 每隔多久收集一次資源資料；時間格式可用 s、m、h。
    stale_after: 120s # 資料超過此時間視為過期；必須大於 interval。
  alerts: # DDAE 告警傳送到 Kafka 的設定。
    enabled: true # true 表示讀取告警並啟用 outbox 和 Kafka producer。
    interval: 30s # 每隔多久讀取一次告警清單。
    list_response_max_bytes: 8388608 # 告警清單回應上限，單位 bytes；範圍 1–67108864。
    detail: # 每筆告警詳細內容的讀取限制。
      response_max_bytes: 1048576 # 單筆詳細內容上限，單位 bytes；範圍 1–67108864。
      refresh_interval: 10m # 同一筆告警重新讀取的間隔；不得小於 alerts.interval。
      max_per_cycle: 200 # 每輪最多讀取幾筆詳細內容；範圍 1–10000。
      concurrency: 4 # 同時讀取的筆數；範圍 1–128，且不得大於 max_per_cycle。
  serviceability_logs: # Serviceability Logs 傳送到獨立 Kafka topic 的設定。
    enabled: false # true 表示啟用獨立 worker、producer 和 serviceability-logs.db。
    interval: 30s # 每隔多久讀取一次 Log 清單。
    list_response_max_bytes: 8388608 # Log 清單回應上限，單位 bytes；範圍 1–67108864。
    detail: # 每筆 Log 詳細內容的讀取限制。
      response_max_bytes: 1048576 # 單筆詳細內容上限，單位 bytes；範圍 1–67108864。
      refresh_interval: 10m # 同一筆 Log 重新讀取的間隔；不得小於 serviceability_logs.interval。
      max_per_cycle: 200 # 每輪最多讀取幾筆詳細內容；範圍 1–10000。
      concurrency: 4 # 同時讀取的筆數；範圍 1–128，且不得大於 max_per_cycle。

server: # Exporter HTTP 服務設定。
  listen_address: 127.0.0.1:9469 # 監聽位址；Docker/Kubernetes 使用 0.0.0.0:9469。
  shutdown_grace_period: 15s # 停止背景工作、寫入狀態和關閉 HTTP 的總等待時間。

security: # TLS 診斷模式的總開關。
  allow_insecure_tls: false # 要停用個別連線的 TLS 檢查時，這一項也必須設為 true。

ddae: # DDAE 管理 API 連線設定。
  base_url: https://ddae.example.invalid # DDAE HTTPS 位址，可包含 port，不加其他 path。
  paths: # Ping 和其他管理 API 可使用不同 prefix。
    ping_prefix: "" # 預設空字串，送出的路徑為 GET /ping。
    api_prefix: /v1 # 其他 API 的預設 prefix，例如 GET /v1/ddae-clusters。
  source_instance: replace-with-stable-appliance-name # 固定的來源名稱；使用 Kafka 時必填。
  credentials: # DDAE 登入資料的檔案路徑。
    username_file: /secure/runtime/ddae-username # 檔案內容只放 DDAE 唯讀 username。
    password_file: /secure/runtime/ddae-password # 檔案內容只放 DDAE 唯讀帳號的 password。
    client_secret_file: /secure/runtime/ddae-client-secret # 檔案內容只放 dv-admin-rest client secret。
  tls: # DDAE HTTPS 憑證設定。
    ca_file: /etc/ddae-exporter/trust/ddae-ca.pem # 自有 CA 的 PEM 檔；使用系統 CA 時刪除這一行。
    insecure_skip_verify: false # 與 allow_insecure_tls 同為 true 時，停用 DDAE 憑證和主機名稱檢查。
  request_timeout: 5s # 單次 token 或 API request 的逾時；必須小於 cycle_timeout。
  cycle_timeout: 20s # 一輪收集工作的總逾時；必須小於已開啟功能的 interval。
  response_max_bytes: 4194304 # 一般 API 回應上限，單位 bytes；範圍 1–67108864。
  retry_max: 2 # 第一次 request 失敗後最多重試幾次；範圍 0–10。

kafka: # 告警和 Serviceability Logs 共用的 Kafka 連線設定。
  brokers: # Kafka broker 清單；開啟告警或 Logs 時需要 1–64 筆。
    - kafka.example.invalid:9093 # 格式為 host:port；有多台 broker 時增加清單項目。
  topic: ddae-serviceability-alerts # 告警專用 topic，最多 249 bytes。
  serviceability_logs_topic: ddae-serviceability-logs # Log 專用 topic，需與告警 topic 不同。
  client_id: ddae-exporter # Kafka client ID，長度 1–128 bytes。
  tls: # Kafka TLS 和 mTLS 設定。
    ca_file: /etc/ddae-exporter/trust/kafka-ca.pem # 自有 CA 的 PEM 檔；使用系統 CA 時刪除這一行。
    # client_cert_file: /secure/runtime/kafka-client.crt # 使用 mTLS 時填入 client certificate PEM。
    # client_key_file: /secure/runtime/kafka-client.key # 使用 mTLS 時填入配對的 private key。
    insecure_skip_verify: false # 與 allow_insecure_tls 同為 true 時，停用 Kafka 憑證和主機名稱檢查。
  sasl: # Kafka SASL 登入設定；以下範例使用 SCRAM-SHA-512。
    mechanism: SCRAM-SHA-512 # 可填 PLAIN、SCRAM-SHA-256 或 SCRAM-SHA-512。
    username: replace-with-runtime-identity # Kafka SASL username。
    password_file: /secure/runtime/kafka-password # 檔案內容只放 Kafka SASL password。
  publish_timeout: 10s # 等待 Kafka 確認每筆資料的最長時間；最小 1s。

state: # 告警和 Serviceability Logs 的本機持久化設定。
  dir: /var/lib/ddae-exporter # 可寫入的完整目錄路徑；使用 Kafka 時必填。
  outbox_max_bytes: 1073741824 # 告警 outbox 的資料上限，單位 bytes。
  outbox_max_events: 100000 # 告警 outbox 的筆數上限；範圍 1–10000000。
  checkpoint_retention: 720h # 告警從清單消失後，checkpoint 保留多久。
  checkpoint_max_alerts: 100000 # 最多保留幾筆告警 checkpoint；範圍 1–10000000。
  serviceability_logs_outbox_max_bytes: 1073741824 # Log outbox 的資料上限，單位 bytes。
  serviceability_logs_outbox_max_events: 100000 # Log outbox 的筆數上限；範圍 1–10000000。
  serviceability_logs_checkpoint_retention: 720h # Log 從完整清單消失後，checkpoint 保留多久。
  serviceability_logs_checkpoint_max_records: 100000 # 最多保留幾筆 Log checkpoint；範圍 1–10000000。

logging: # 程式日誌設定。
  level: info # 可用 debug、info、warn、error。
  format: json # 可用 json、text。
```

### Pipeline、DDAE 與 HTTP

| YAML key | Default／啟用條件 | 說明 |
|---|---|---|
| `monitoring.resources.enabled` | `true` | 控制 resource collectors 與 resource metrics。 |
| `monitoring.resources.interval` | `30s` | Resource pipeline 背景週期。 |
| `monitoring.resources.stale_after` | `120s` | Current resource snapshot 的最大年齡。 |
| `monitoring.alerts.enabled` | `true` | 控制 alert list/detail、Kafka 與 state。 |
| `monitoring.alerts.interval` | `30s` | Alert list 背景週期。 |
| `monitoring.serviceability_logs.enabled` | `false` | 控制 Serviceability Log list/detail、專用 Kafka producer 與專用 state。 |
| `monitoring.serviceability_logs.interval` | `30s` | Serviceability Log list 背景週期。 |
| `server.listen_address` | `127.0.0.1:9469` | HTTP listener。 |
| `server.shutdown_grace_period` | `15s` | Cancellation、state sync 與 HTTP shutdown budget。 |
| `ddae.base_url` | 所有模式 | 單一 DDAE HTTPS origin。 |
| `ddae.paths.ping_prefix` | 空字串 | Ping namespace；最多 128 bytes，與固定 `/ping` suffix 直接串接。 |
| `ddae.paths.api_prefix` | `/v1` | 其他 allowlisted Management API GET namespace；最多 128 bytes。 |
| `ddae.source_instance` | Alerts 或 Serviceability Logs | 1–128 bytes 的穩定 event identity。 |
| `ddae.credentials.username_file` | 所有模式 | DDAE username secret file。 |
| `ddae.credentials.password_file` | 所有模式 | DDAE password secret file。 |
| `ddae.credentials.client_secret_file` | 所有模式 | `dv-admin-rest` client secret file。 |
| `ddae.tls.ca_file` | System roots | 額外的 PEM CA bundle。 |
| `ddae.request_timeout` | `5s` | 單次 token 或 DDAE request deadline。 |
| `ddae.cycle_timeout` | `20s` | 一輪 aggregate collection deadline。 |
| `ddae.response_max_bytes` | `4194304` | 一般 DDAE response body hard limit，單位為 bytes；範圍 `1–67108864`。 |
| `ddae.retry_max` | `2` | 初次 safe request 後的 retry 次數，範圍 `0–10`。 |

Duration 使用 Go duration 語法，例如 `500ms`、`5s`、`10m`、`720h`。有效時間關係如下：

| 關係 | 用途 |
|---|---|
| `ddae.request_timeout < ddae.cycle_timeout` | 保留 cycle 聚合處理時間。 |
| `ddae.cycle_timeout < enabled pipeline interval` | 確保每輪工作落在 pipeline 週期內。 |
| `monitoring.resources.stale_after > monitoring.resources.interval` | 保留 resource snapshot freshness window。 |
| `monitoring.serviceability_logs.detail.refresh_interval >= monitoring.serviceability_logs.interval` | 保證 listed logs 依 bounded policy 定期 refresh。 |

### Alert detail

| YAML key | Default | 說明 |
|---|---:|---|
| `monitoring.alerts.list_response_max_bytes` | `8388608` | Alert list response hard limit，範圍 `1–67108864` bytes。 |
| `monitoring.alerts.detail.response_max_bytes` | `1048576` | 每筆 detail response hard limit，範圍 `1–67108864` bytes。 |
| `monitoring.alerts.detail.refresh_interval` | `10m` | 清單內 alert 的 detail refresh 間隔，設定值至少等於 alert interval。 |
| `monitoring.alerts.detail.max_per_cycle` | `200` | 每輪 detail requests 上限，範圍 `1–10000`。 |
| `monitoring.alerts.detail.concurrency` | `4` | 同時 detail requests，範圍 `1–128`，設定值至多等於 `max_per_cycle`。 |

超過單輪上限的項目會依 deterministic 順序延後，並由
`ddae_alert_detail_deferred` 呈現數量。New／changed 與 refresh-due 分成兩個佇列；
兩者都有 backlog 時，`max_per_cycle: 1` 會輪替處理，較大的上限至少保留一個且約
四分之一的名額給 refresh-due，再將未使用名額借給另一類。

### Serviceability Log detail

| YAML key | Default | 說明 |
|---|---:|---|
| `monitoring.serviceability_logs.list_response_max_bytes` | `8388608` | Log list response hard limit，範圍 `1–67108864` bytes。 |
| `monitoring.serviceability_logs.detail.response_max_bytes` | `1048576` | 每筆 log detail response hard limit，範圍 `1–67108864` bytes。 |
| `monitoring.serviceability_logs.detail.refresh_interval` | `10m` | Listed log detail refresh 間隔，至少等於 log interval。 |
| `monitoring.serviceability_logs.detail.max_per_cycle` | `200` | 每輪 detail requests 上限，範圍 `1–10000`。 |
| `monitoring.serviceability_logs.detail.concurrency` | `4` | 同時 detail requests，範圍 `1–128`，至多等於 `max_per_cycle`。 |

New／changed 與 refresh-due 使用獨立的 deterministic fairness 排程。Incomplete list
仍可處理安全 IDs，並以 `ddae_serviceability_log_list_complete 0` 與 readiness `0`
呈現；只有 complete list 能進行 absence retention，且不會建立 deletion event。

### Kafka 與 persistent state

| YAML key | Default／啟用條件 | 說明 |
|---|---|---|
| `kafka.brokers` | Alerts 或 Serviceability Logs | 1–64 個 Kafka broker addresses；所有連線使用 TLS。 |
| `kafka.topic` | Alert pipeline | Dedicated topic，最長 249 bytes。 |
| `kafka.serviceability_logs_topic` | `ddae-serviceability-logs` | Dedicated log topic，最長 249 bytes；兩條 Kafka pipelines 同時啟用時必須與 alert topic 不同。 |
| `kafka.client_id` | `ddae-exporter` | Kafka client identity。 |
| `kafka.tls.ca_file` | System roots | 額外的 broker CA PEM bundle。 |
| `kafka.tls.client_cert_file` | 選用 | Kafka mTLS certificate path，搭配 client key。 |
| `kafka.tls.client_key_file` | 選用 | Kafka mTLS private-key path，搭配 client certificate。 |
| `kafka.sasl.mechanism` | 選用 | `PLAIN`、`SCRAM-SHA-256` 或 `SCRAM-SHA-512`。 |
| `kafka.sasl.username` | SASL 啟用時 | Kafka SASL username。 |
| `kafka.sasl.password_file` | SASL 啟用時 | Kafka SASL password secret file。 |
| `kafka.publish_timeout` | `10s` | 每筆 publish 的 hard deadline，包含已送出的 in-flight produce；最小 `1s`。逾時時保留 outbox record 供重播。 |
| `state.dir` | `/var/lib/ddae-exporter` | Enabled Kafka pipelines 的 absolute persistent directory。 |
| `state.outbox_max_bytes` | `1073741824` | Durable outbox byte hard limit。 |
| `state.outbox_max_events` | `100000` | Durable outbox record hard limit。 |
| `state.checkpoint_retention` | `720h` | Alert checkpoint retention。 |
| `state.checkpoint_max_alerts` | `100000` | Retained alert checkpoints hard limit。 |
| `state.serviceability_logs_outbox_max_bytes` | `1073741824` | `serviceability-logs.db` outbox byte hard limit。 |
| `state.serviceability_logs_outbox_max_events` | `100000` | 專用 log outbox record hard limit。 |
| `state.serviceability_logs_checkpoint_retention` | `720h` | Complete list 中消失且無 pending output 的 log checkpoint retention。 |
| `state.serviceability_logs_checkpoint_max_records` | `100000` | Retained log checkpoints hard limit。 |

兩個 outbox 分別保留待確認 records；Kafka 恢復後會依序重播。若 hard timeout 發生在 broker
可能已接受、但 client 尚未收到確認的期間，該 record 仍會保留，因此可能以相同 key
再次送出。Consumer 必須以 record key 做 idempotent upsert。達到任一 outbox hard
limit 時會保留現有 records、暫停該 pipeline 的新增內容，並只將對應的 alert 或
Serviceability Logs readiness 設為 `0`。

`state.db` 使用明確的 state schema version。啟動時會在單一 bbolt transaction
驗證 outbox records、checkpoints 與交叉關係；有效的舊版無版本資料庫會就地標記為
目前版本，並由已驗證 records 重新計算 event count 與 byte count。未支援的未來版本
或 primary state 損壞會讓啟動 fail closed，原始 records/checkpoints 不會被自動刪除。
升級前先停止唯一 writer 並保留受保護的 `state.db` 副本。

`serviceability-logs.db` 使用相同的 schema/integrity model，但具有獨立 buckets、
derived counters、limits 與 file lock。Log pipeline capacity 或 corruption 不會消耗、
acknowledge 或修改 `state.db`。Rollback 可先將 `monitoring.serviceability_logs.enabled`
設為 `false` 並保留專用資料庫供稍後 replay。

Token cache 以 generation 區分 renewal。併發的初次取 token 或 401 renewal 共享同一個
結果；延遲抵達的舊 generation 401 會直接採用已存在的新 token。Token transport、
HTTP `429` 與 `5xx` 使用 `ddae.retry_max` 的有限重試預算，credential rejection 不重試。

### Logging

| YAML key | Default | 可用值 |
|---|---|---|
| `logging.level` | `info` | `debug`、`info`、`warn`、`error`。 |
| `logging.format` | `json` | `json`、`text`。 |

Logs 使用 component 與固定 failure class，涵蓋 `auth`、`tls`、`timeout`、`transport`、
`http`、`decode`、`validation`、`kafka_auth`、`kafka_timeout`、`kafka_rejected`、
`buffer_full` 與 `internal`。敏感內容由 logging boundary 排除。

### TLS 驗證選項

| YAML key | Default | 生效方式 |
|---|---:|---|
| `security.allow_insecure_tls` | `false` | 提供明確的全域診斷授權。 |
| `ddae.tls.insecure_skip_verify` | `false` | 與全域授權同為 `true` 時，針對 DDAE 關閉 certificate 與 hostname verification。 |
| `kafka.tls.insecure_skip_verify` | `false` | 與全域授權同為 `true` 時，針對 Kafka 關閉 certificate 與 hostname verification。 |

Target-specific 設定彼此獨立，因此可只調整 DDAE 或只調整 Kafka。Effective insecure
target 會在 startup 輸出一次固定 warning；TLS minimum 仍為 1.2。一般部署使用正確的
CA 並維持三個欄位為 `false`。Global flag 為 `true`、target flag 為 `false` 時仍執行
完整驗證；target insecure mode 與同一 target 的 `ca_file` 採二選一設定。

### Secret files 與環境變數

| 項目 | 行為 |
|---|---|
| YAML secret | 使用 `*_file` path，由 runtime 讀取檔案內容。 |
| File size | 每個 secret file 最大 64 KiB。 |
| File content | 使用 valid UTF-8；讀取時移除一個尾端 `LF`、`CRLF` 或 `CR`。 |
| Environment direct value | `DDAE_USERNAME`、`DDAE_PASSWORD`、`DDAE_CLIENT_SECRET`、`KAFKA_SASL_PASSWORD` 提供相容介面。 |
| Environment file path | 對應的 `*_FILE` environment variable 可覆蓋 YAML path。 |
| 來源選擇 | 每個 secret 可選用 direct environment value 或 environment `*_FILE`；任一 environment 選擇都優先於 YAML path。 |

主要 YAML 與 environment 對照如下：

| YAML | Environment |
|---|---|
| `monitoring.resources.enabled` | `DDAE_RESOURCE_MONITORING_ENABLED` |
| `monitoring.resources.interval` | `DDAE_RESOURCE_COLLECTION_INTERVAL` |
| `monitoring.resources.stale_after` | `DDAE_STALE_AFTER` |
| `monitoring.alerts.enabled` | `DDAE_ALERT_MONITORING_ENABLED` |
| `monitoring.alerts.interval` | `DDAE_ALERT_COLLECTION_INTERVAL` |
| `monitoring.serviceability_logs.enabled` | `DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED` |
| `monitoring.serviceability_logs.interval` | `DDAE_SERVICEABILITY_LOG_COLLECTION_INTERVAL` |
| `monitoring.serviceability_logs.list_response_max_bytes` | `SERVICEABILITY_LOG_LIST_RESPONSE_MAX_BYTES` |
| `monitoring.serviceability_logs.detail.response_max_bytes` | `SERVICEABILITY_LOG_DETAIL_RESPONSE_MAX_BYTES` |
| `monitoring.serviceability_logs.detail.refresh_interval` | `SERVICEABILITY_LOG_DETAIL_REFRESH_INTERVAL` |
| `monitoring.serviceability_logs.detail.max_per_cycle` | `SERVICEABILITY_LOG_DETAIL_MAX_PER_CYCLE` |
| `monitoring.serviceability_logs.detail.concurrency` | `SERVICEABILITY_LOG_DETAIL_CONCURRENCY` |
| `ddae.base_url` | `DDAE_BASE_URL` |
| `ddae.paths.ping_prefix` | `DDAE_PING_PATH_PREFIX` |
| `ddae.paths.api_prefix` | `DDAE_API_PATH_PREFIX` |
| `ddae.source_instance` | `DDAE_SOURCE_INSTANCE` |
| `server.listen_address` | `EXPORTER_LISTEN_ADDRESS` |
| `ddae.tls.ca_file` | `DDAE_CA_FILE` |
| `ddae.tls.insecure_skip_verify` | `DDAE_TLS_INSECURE_SKIP_VERIFY` |
| `security.allow_insecure_tls` | `ALLOW_INSECURE_TLS` |
| `kafka.brokers` | `KAFKA_BROKERS`，以逗號分隔。 |
| `kafka.topic` | `KAFKA_TOPIC` |
| `kafka.serviceability_logs_topic` | `KAFKA_SERVICEABILITY_LOG_TOPIC` |
| `kafka.tls.ca_file` | `KAFKA_CA_FILE` |
| `kafka.tls.insecure_skip_verify` | `KAFKA_TLS_INSECURE_SKIP_VERIFY` |
| `state.dir` | `STATE_DIR` |
| `state.serviceability_logs_outbox_max_bytes` | `SERVICEABILITY_LOG_OUTBOX_MAX_BYTES` |
| `state.serviceability_logs_outbox_max_events` | `SERVICEABILITY_LOG_OUTBOX_MAX_EVENTS` |
| `state.serviceability_logs_checkpoint_retention` | `SERVICEABILITY_LOG_CHECKPOINT_RETENTION` |
| `state.serviceability_logs_checkpoint_max_records` | `SERVICEABILITY_LOG_CHECKPOINT_MAX_RECORDS` |
| `logging.level` | `LOG_LEVEL` |
| `logging.format` | `LOG_FORMAT` |

完整 environment 相容介面可由
[`internal/config/config.go`](internal/config/config.go) 與
[`internal/config/yaml.go`](internal/config/yaml.go) 對照。

## Docker 安裝

### 1. 建置 image

```bash
docker build \
  --build-arg VERSION=1.0.0-rc3 \
  --build-arg REVISION=REPLACE_WITH_GIT_SHA \
  --build-arg BUILD_DATE=2026-09-01T00:00:00Z \
  --tag ddae-exporter:v1.0.0-rc3 \
  .
```

Image 使用 `scratch` base、UID/GID `65532`，啟動程式是 `/ddae-exporter`。

### 2. 準備執行目錄

```bash
sudo install -d -m 0700 -o 65532 -g 65532 /srv/ddae-exporter/state
sudo install -d -m 0700 -o 65532 -g 65532 /etc/ddae-exporter/secrets
sudo install -d -m 0755 -o root -g root /etc/ddae-exporter/trust
```

將 YAML、帳號密碼檔和 CA bundle 放入主機上的對應目錄。YAML 內請使用下列 container
路徑，並將 `server.listen_address` 設為 `0.0.0.0:9469`：

| 類別 | Container 內的路徑 |
|---|---|
| YAML | `/etc/ddae-exporter/config.yaml` |
| Secrets | `/run/secrets/...` |
| CA bundles | `/run/trust/...` |
| Persistent state | `/var/lib/ddae-exporter` |

將設定檔安裝到 `/etc/ddae-exporter/config.yaml`：

```bash
sudo install -o root -g root -m 0600 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
```

再依[帳號密碼檔的部署範例](#帳號密碼檔的部署範例)安裝 secret，並依環境修改
`/etc/ddae-exporter/config.yaml`。

### 3. 啟動 container

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

只監控資源時需要 DDAE 設定、帳號密碼檔和 CA mount。開啟告警或 Serviceability Logs
時，再加入 Kafka secret 和持久化 state mount。

## Kubernetes 安裝

Repository 提供的 manifest：

| File | 內容 |
|---|---|
| [`deploy/kubernetes/configmap.yaml`](deploy/kubernetes/configmap.yaml) | 非敏感 YAML 設定範例。 |
| [`deploy/kubernetes/deployment.yaml`](deploy/kubernetes/deployment.yaml) | ServiceAccount、3 GiB RWO PVC、單 replica `Recreate` Deployment 與 ClusterIP Service。 |
| [`deploy/kubernetes/networkpolicy.yaml`](deploy/kubernetes/networkpolicy.yaml) | Default-deny ingress/egress policy。 |

### 1. 準備部署值

| 項目 | 設定方式 |
|---|---|
| Image | 使用已建置的 immutable image digest。 |
| ConfigMap | 填入 DDAE/Kafka endpoints、monitoring mode 與 runtime limits。 |
| PVC | 選擇支援 RWO、fsync 與 file lock 的 storage class。 |
| NetworkPolicy | 加入 DNS、DDAE、Kafka 與 Prometheus 的精確 allow policy。 |
| Prometheus ingress | 使用 mTLS service mesh 或 proxy 提供受控的 TCP 9469 路徑。 |

### 2. 建立 Secrets

以下範例使用 `monitoring` namespace 與受控檔案：

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

### 3. 套用

```bash
kubectl -n monitoring apply -f deploy/kubernetes/configmap.yaml
kubectl -n monitoring apply -f /secure/runtime/ddae-exporter-allow-policy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/networkpolicy.yaml
kubectl -n monitoring apply -f deploy/kubernetes/deployment.yaml
```

### 4. 觀察

```bash
kubectl -n monitoring rollout status deployment/ddae-exporter
kubectl -n monitoring get pod,service -l app.kubernetes.io/name=ddae-exporter
kubectl -n monitoring get pvc ddae-exporter-state
kubectl -n monitoring logs deployment/ddae-exporter
```

Deployment 以 `replicas: 1` 與 `strategy: Recreate` 維持單一 bbolt writer。

## Linux / systemd 安裝

Repository 提供 [`deploy/systemd/ddae-exporter.service`](deploy/systemd/ddae-exporter.service)，
並以 `LoadCredential=` 將 secret files 交給 Exporter。

### 1. 安裝帳號、binary 與 directories

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

### 2. 安裝設定與 unit

```bash
sudo install -o root -g ddae-exporter -m 0640 \
  deploy/systemd/config.example.yaml /etc/ddae-exporter/config.yaml
sudo install -o root -g root -m 0644 \
  deploy/systemd/ddae-exporter.service /etc/systemd/system/ddae-exporter.service
```

依環境更新 `/etc/ddae-exporter/config.yaml`，並將 CA bundles 安裝於
`/etc/ddae-exporter/trust/`。

### 3. 安裝 systemd credentials

Unit 使用下列 credential 名稱：

| Credential | 來源內容 |
|---|---|
| `ddae-username` | DDAE username。 |
| `ddae-password` | DDAE password。 |
| `ddae-client-secret` | `dv-admin-rest` client secret。 |
| `kafka-password` | Kafka SASL password。 |

將受控檔案安裝至 unit 內 `LoadCredential=` 對應的 paths：

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

重新載入並啟動：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ddae-exporter
sudo systemctl status ddae-exporter
sudo journalctl -u ddae-exporter
```

## Prometheus 設定

同一台主機可抓取 loopback listener：

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

跨主機可由 mTLS reverse proxy 或 service mesh 保護 Exporter endpoint：

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

建議觀察項目：

| Pipeline | 指標 |
|---|---|
| 共用 | `ddae_monitoring_enabled` 與 `ddae_build_info`。 |
| Resource | `ddae_up`、`ddae_collector_success`、`ddae_snapshot_age_seconds`。 |
| Alert | `ddae_alert_list_complete`、`ddae_alert_pipeline_ready`、`ddae_alert_detail_deferred`。 |
| Kafka | `ddae_kafka_publish_success`、`ddae_kafka_buffered_events`、`ddae_kafka_events_failed_total`。 |
| Serviceability Logs | `ddae_serviceability_log_list_complete`、`ddae_serviceability_log_pipeline_ready`、`ddae_serviceability_log_detail_deferred`。 |
| Logs Kafka | `ddae_serviceability_log_kafka_publish_success`、`ddae_serviceability_log_buffered_records`、`ddae_serviceability_log_records_failed_total`。 |

## 啟動後檢查

```bash
curl --fail --silent --show-error http://127.0.0.1:9469/healthz
curl --include http://127.0.0.1:9469/readyz
curl --silent http://127.0.0.1:9469/metrics
```

| 檢查 | 預期結果 |
|---|---|
| `/healthz` | HTTP server 啟動後回 `200`。 |
| `/readyz` | 已啟用 pipeline 完成首次準備後回 `200`。 |
| `/metrics` | 包含 `ddae_build_info`、三筆固定 `ddae_monitoring_enabled`，以及已啟用 pipeline 的 metrics。 |
| Alert Kafka | 產生測試 alert 後，alert topic 收到 schema `1.0` event，consumer 可依 key upsert。 |
| Logs Kafka | 啟用 pipeline 並產生新或變更 log 後，專用 topic 收到 `ddae.serviceability_log.upsert`。 |

詳細維運、incident、outbox recovery 與 rollback 步驟見
[`docs/runbook.md`](docs/runbook.md)。

## 開發指令

| 目的 | 指令 |
|---|---|
| Lint | `./scripts/stage-lint.sh` |
| Race-enabled tests | `./scripts/stage-test.sh` |
| Coverage gate | `./scripts/stage-coverage.sh` |
| Build | `./scripts/build.sh` |
| Security policy | `./scripts/security-policy.sh` |
| CI policy | `./scripts/stage-ci-policy.sh` |
| Supply-chain artifacts | `./scripts/supply-chain.sh` |

目前 repository 的本機基準結果為 lint、race-enabled tests、build、security 與 CI
policy 通過，coverage 為 81.5%，高於 80% gate；source-reachable vulnerability scan
結果為 `No vulnerabilities found`。

## 專案文件

| 文件 | 內容 |
|---|---|
| [`docs/runbook.md`](docs/runbook.md) | Container、Kubernetes、systemd 維運與 recovery。 |

## License and trademark notice

程式碼依本 repository 的 [Apache License 2.0](LICENSE) 授權。Dell 與相關產品名稱為
其權利人所有；本專案名稱用於描述相容目標。
