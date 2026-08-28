# Dell DDAE Metrics Exporter

Dell DDAE Metrics Exporter 是以 Go 開發的唯讀監控服務，目標相容版本為 Dell Data
Domain Active Enterprise（DDAE）1.5.0。服務將叢集、節點與 appliance operability
資料轉換為 Prometheus metrics，並將 serviceability alerts 與選用的 Serviceability
Logs / Events 整理為彼此獨立、具版本的 Kafka 事件。

## 專案能力

| 能力 | 已實作行為 |
|---|---|
| DDAE 資源監控 | 定期收集 API reachability、cluster 狀態、Coordinator/Worker 配置資源、node 狀態與容量、system lock、control-plane 與 node readiness。 |
| Prometheus 輸出 | 由背景收集程序維護 current snapshot，並透過 `/metrics` 提供穩定的 metric contract。 |
| DDAE 告警收集 | 從 serviceability issue 清單取得 ID，再以有限的並行量讀取 detail。 |
| Kafka 告警發布 | 將通過型別與欄位 allowlist 的告警寫入 durable bbolt outbox，再以 at-least-once、idempotent producer 發布。 |
| Serviceability Logs 收集 | 從 Serviceability Event 清單取得安全 ID，以 bounded list/detail 流程建立 typed log event，並將新或變更內容發布至專用 Kafka topic。 |
| 隔離的 Logs delivery | 使用 `serviceability-logs.db`、專用 outbox/checkpoints、Kafka producer、topic、capacity 與 readiness，保護既有 alerts。 |
| 獨立 pipeline | Resources、alerts 與 `serviceability_logs` 可分別啟用及設定週期；Serviceability Logs 預設關閉。 |
| YAML 設定 | 支援具版本、具型別、大小受限的 YAML；命令列與環境變數可選擇設定檔。 |
| TLS | DDAE 與 Kafka 預設驗證 certificate 與 hostname，支援自訂 CA；Kafka 另支援 mTLS。 |
| Kafka 驗證 | 支援 `PLAIN`、`SCRAM-SHA-256` 與 `SCRAM-SHA-512` SASL。 |
| 運行保護 | 對 response 大小、timeout、retry、detail concurrency、outbox 容量與 metric labels 設定明確界線。 |
| 部署 | Repository 提供 source build、OCI container、Kubernetes 與 Linux/systemd 部署材料。 |

資料流如下：

```text
Dell DDAE read-only Management API
        |
        +--> resource collectors --> current snapshot --> Prometheus /metrics
        |
        +--> alert list --> bounded detail GETs --> typed event --> durable outbox --> Kafka
        |
        +--> log list --> bounded detail GETs --> typed log event --> dedicated outbox --> dedicated Kafka topic
```

## Monitoring 模式

有效設定至少啟用一條 pipeline：

| 模式範例 | `resources` | `alerts` | `serviceability_logs` | 運行內容 |
|---|---:|---:|---:|---|
| 預設 | `true` | `true` | `false` | 提供 resource metrics 並發布 alerts。 |
| Resource-only | `true` | `false` | `false` | 提供 DDAE resource metrics。 |
| Alert-only | `false` | `true` | `false` | 發布 alerts 並使用 alert persistent state。 |
| Logs-only | `false` | `false` | `true` | 發布 Serviceability Logs 並使用專用 log state。 |
| 全部啟用 | `true` | `true` | `true` | 三條 pipeline 獨立排程、儲存與判定 readiness。 |

任意至少一條 pipeline 啟用的組合皆有效。啟用的 pipeline 會參與 `/readyz` 判定；
`ddae_monitoring_enabled{pipeline="resources"}` 與
`pipeline="alerts"`、`pipeline="serviceability_logs"` 三筆固定 series 可直接呈現模式。

## DDAE 唯讀介面

Exporter 使用下列 API：

| 類別 | Method 與 path | 用途 |
|---|---|---|
| Authentication | `POST /auth/realms/ddae/protocol/openid-connect/token` | 取得 `dv-admin-rest` bearer token。 |
| Ping | `GET /rest/v1/ping` | 確認 Management API reachability。 |
| Clusters | `GET /rest/v1/ddae-clusters` | 取得 cluster 狀態與 Coordinator/Worker 配置。 |
| Nodes | `GET /rest/v1/infrastructure-nodes` | 取得 node 狀態、capacity、allocatable 與 conditions。 |
| System lock | `GET /rest/v1/system-lock` | 取得 appliance lock 狀態。 |
| System shutdown | `GET /rest/v1/system-shutdown` | 取得 control-plane 與 node readiness。 |
| Alert list | `GET /rest/v1/serviceability-issues` | 取得告警清單與 lifecycle input。 |
| Alert detail | `GET /rest/v1/serviceability-issues/{id}` | 取得經 allowlist 處理的告警內容。 |
| Serviceability Log list | `GET /rest/v1/serviceability-events` | 發現 Serviceability Log IDs 並評估 completeness。 |
| Serviceability Log detail | `GET /rest/v1/serviceability-events/{id}` | 取得經 typed allowlist 處理的 log detail。 |

部署時適合使用 dedicated、least-privilege、read-only DDAE identity。TLS 最低版本為
TLS 1.2，certificate chain 與 hostname verification 預設啟用。

## HTTP endpoints

預設 listener 為 `127.0.0.1:9469`。

| Endpoint | 回應內容 |
|---|---|
| `GET /metrics` | Prometheus exposition 與 collector diagnostics；最多同時處理 5 個 requests，每個 handler 的 timeout 為 9 秒。 |
| `GET /healthz` | 程序存活狀態。 |
| `GET /readyz` | 所有已啟用 pipeline 的 readiness；ready 時回 `200`，其餘狀態回 `503`。 |

## Prometheus metrics

### 共用 metrics 的單位與值域

| Metric | Type | 單位 | 值域與意義 |
|---|---|---|---|
| `ddae_build_info{version,go_version}` | gauge | 無單位 | 固定為 `1`；labels 表示 Exporter 與 Go build version。 |
| `ddae_monitoring_enabled{pipeline}` | gauge | boolean | `0` 或 `1`；固定輸出 `resources`、`alerts`、`serviceability_logs` 三筆 series。 |
| `ddae_collector_success{collector}` | gauge | boolean | `0` 或 `1`；最近一輪指定 collector 的結果，輸出已啟用 pipeline 的 collector labels。 |
| `ddae_collector_duration_seconds{collector}` | gauge | seconds | 大於或等於 `0` 的秒數，可包含小數；輸出已啟用 pipeline 的 collector labels。 |

### Resource pipeline metrics 的單位與值域

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

### Alert pipeline metrics 的單位與值域

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

### Serviceability Logs pipeline metrics 的單位與值域

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

## Kafka alert event

每筆 event 使用 typed JSON 與固定的 schema contract：

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

`alert` object 可包含 `severity`、`acknowledged`、`occurrence_count`、`created_at`、
`updated_at`、`clear_type`、`auto_clear_timeout_raw`、`app_name`、`component`、
`namespace`、`message`、`reason`、`remedies`、`resource_id`、`symptom_id`、`related`
與一層 `related_events`。

| Kafka record 項目 | 契約 |
|---|---|
| Key | `source_instance + NUL + alert_id` 的小寫 SHA-256 hex。 |
| Header | `content-type=application/json`。 |
| Header | `ddae-schema-version=1.0`。 |
| Delivery | Durable outbox 搭配 at-least-once 發布；hard timeout 後不確認或刪除不確定的 record，重播可能產生相同 key 的 duplicate，consumer 必須依 record key 執行 idempotent upsert。 |
| Event size | 最大 256 KiB。 |

### 告警 JSON 範本

以下是 repository sanitized fixture 對應的實際 event shape。範例中的
`content_hash_sha256` 是 `alert` object canonical JSON 的實際 SHA-256，時間已轉為 UTC；
source 未提供的 optional fields 會從輸出省略。

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

Alert 欄位型別與值域：

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

Kafka consumer 可用下列固定 metadata 辨識資料：

| Metadata | 值 |
|---|---|
| Record key | `SHA-256(source_instance + NUL + alert_id)` 的 64 字元小寫 hex。 |
| `content-type` header | `application/json` |
| `ddae-schema-version` header | `1.0` |

## Kafka Serviceability Log event

Serviceability Logs 使用獨立 schema identity、record kind、topic 與 state。每筆 typed
JSON event 的 envelope：

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

`labels`、`links` 與 unknown source fields 不會進入 event。Absent optional fields 會
省略；invalid present values 會使該 event 失敗並保留既有 checkpoint。

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

| Kafka record 項目 | 契約 |
|---|---|
| Topic | `kafka.serviceability_logs_topic`，預設 `ddae-serviceability-logs`，與 alert topic 不同。 |
| Key | `SHA-256(source_instance + NUL + serviceability_log + NUL + log_id)` 的 64 字元小寫 hex。 |
| Headers | `content-type=application/json`、`ddae-schema-version=1.0`、`ddae-record-kind=serviceability_log`。 |
| Delivery | 專用 durable outbox、`acks=all`、idempotent producer、per-key ordering 與 at-least-once replay。 |
| Event size | 最大 256 KiB。 |

## 安裝前準備

| 啟用範圍 | 準備項目 |
|---|---|
| 所有模式 | DDAE 1.5.0 HTTPS origin。 |
| 所有模式 | Dedicated read-only username、password 與 `dv-admin-rest` client secret。 |
| 所有模式 | DDAE 使用 private CA 時，準備 PEM CA bundle。 |
| Alert pipeline | Kafka brokers、dedicated topic 與支援 TLS、`acks=all` 的 broker 設定。 |
| Alert pipeline | 依環境選用 Kafka SASL、mTLS，或兩者組合。 |
| Alert pipeline | 支援 file lock 與 fsync 的 persistent state directory；每個 DDAE target 使用一份獨立 state。 |
| Serviceability Logs pipeline | 專用 Kafka topic/ACL、OpenSearch mapping 與 `serviceability-logs.db` 容量；首次啟用前完成配置。 |
| Prometheus | Exporter listener 的受控連線路徑；跨主機連線可搭配 mTLS reverse proxy 或 service mesh。 |
| Runtime | 已同步的系統時間，供 timeout、staleness 與 `observed_at` 使用。 |

## 從原始碼安裝

### 工具版本

| 工具 | 版本或用途 |
|---|---|
| Git | 取得 repository。 |
| Go | `go.mod` 宣告 language version `1.26.0` 與 toolchain `go1.26.6`。 |
| Go module source/cache | 第一次下載相依套件時使用。 |

### 建置

從 repository root 執行：

```bash
go mod download
go test ./...
./scripts/build.sh
```

建置產物位於 `bin/ddae-exporter`。需要嵌入版本資訊時可執行：

```bash
VERSION=1.0.0 \
REVISION=REPLACE_WITH_GIT_SHA \
BUILD_DATE=2026-08-25T00:00:00Z \
./scripts/build.sh
```

### 建立本機設定

Repository 內的完整 YAML 範例位於
[`deploy/systemd/config.example.yaml`](deploy/systemd/config.example.yaml)。建立本機副本：

```bash
cp deploy/systemd/config.example.yaml config.local.yaml
mkdir -p state secrets trust
chmod 0700 state secrets
chmod 0600 config.local.yaml
```

接著依下表調整 `config.local.yaml`：

| 區段 | 設定內容 |
|---|---|
| `monitoring` | 選擇所需的 resources、alerts、Serviceability Logs 組合，並設定各自收集週期。 |
| `ddae` | 填入 DDAE HTTPS origin、stable source instance、credentials file paths 與 CA path。 |
| `kafka` | Alerts 或 Serviceability Logs 啟用時填入 brokers、各自 topic、TLS 與選用的 SASL 設定。 |
| `state` | 任一 Kafka pipeline 啟用時填入 absolute persistent directory 與各自 limits。 |
| `server` | 選擇 Exporter listener 與 shutdown grace period。 |
| `logging` | 選擇 `json` 或 `text` 格式，以及 log level。 |

本機啟用 alert pipeline 時，請將 `state.dir` 設為剛建立之 `state` directory 的
absolute path。

### 建立 credential files

每個 `*_file` 都是一個 regular file，內容只放該欄位的原始值。檔案內容採 valid
UTF-8、最大 64 KiB；Exporter 會移除一個尾端 `LF`、`CRLF` 或 `CR`。下表使用的是
安全佔位值：

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

檔案內使用純值，不加 `username=`、`password=`、YAML key、JSON object 或引號。實際
值也可在本機互動輸入；以下命令從 repository root 建立 mode `0600` 的四個 files，
password 與 client secret 以 silent input 讀取，因此值本身不會成為命令列參數：

```bash
install -d -m 0700 secrets
umask 077
printf 'DDAE username: '
IFS= read -r ddae_username_value
printf 'DDAE password: '
IFS= read -r -s ddae_password_value
printf '\nDDAE dv-admin-rest client secret: '
IFS= read -r -s ddae_client_secret_value
printf '\nKafka SASL password: '
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

若組織的 secret manager 已輸出受控來源檔，可在 Linux service account 建立後安裝至
Exporter runtime directory：

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

Resource-only 模式使用前三個 DDAE files。Alerts 或 Serviceability Logs 啟用時依 Kafka SASL 設定
加入 `kafka-password`。以目前登入帳號進行本機測試時，可將受控來源檔安裝到 repository
內已由 `.gitignore` 排除的 `secrets/` directory，並在 YAML 填入 absolute paths：

```bash
install -d -m 0700 secrets
install -m 0600 /path/from/secret-manager/ddae-username secrets/ddae-username
install -m 0600 /path/from/secret-manager/ddae-password secrets/ddae-password
install -m 0600 /path/from/secret-manager/ddae-client-secret secrets/ddae-client-secret
install -m 0600 /path/from/secret-manager/kafka-password secrets/kafka-password
```

| 檢查項目 | 預期值 |
|---|---|
| File type | Regular file。 |
| Parent directory mode | `0700`。 |
| Credential file mode | `0600`。 |
| Owner | 執行 Exporter 的 account。 |
| Content | 單一非空 UTF-8 value，最大 64 KiB。 |
| YAML path | Exporter runtime 可讀取的 absolute path。 |

### 啟動

```bash
./bin/ddae-exporter --config ./config.local.yaml
```

設定檔也可透過環境變數選擇：

```bash
DDAE_EXPORTER_CONFIG_FILE=/absolute/path/config.yaml ./bin/ddae-exporter
```

設定來源優先順序為：

| 優先度 | 來源 |
|---:|---|
| 1 | 個別 environment variables。 |
| 2 | `--config` 指定的 YAML；同時設定 selector 時由此路徑選檔。 |
| 3 | `DDAE_EXPORTER_CONFIG_FILE` 指定的 YAML。 |
| 4 | 程式預設值。 |

Environment-only 設定介面保留相容性，因此現有部署也可直接使用個別 environment
variables 啟動。

## YAML 設定

YAML 使用 `version: 1`、單一 UTF-8 document、最大 1 MiB，並以 documented keys
進行 strict typed validation。Secret 欄位使用檔案路徑。

以下範例展示 dual mode 的完整結構；每個非空設定行都有 inline 說明，inline comments
本身也是有效 YAML。`.invalid`、`replace-with-*` 與 `/secure/runtime/*` 是部署時要換成
實際值或實際 path 的範例：

```yaml
version: 1 # 設定格式版本；目前固定使用整數 1。

monitoring: # 三條 monitoring pipelines 的父區段。
  resources: # Prometheus resource metrics pipeline。
    enabled: true # true 啟用 ping/clusters/nodes/lock/power collectors。
    interval: 30s # Resource background collection interval，使用 Go duration。
    stale_after: 120s # Snapshot freshness window，設定值需大於 resource interval。
  alerts: # DDAE alert detail 到 Kafka 的 pipeline。
    enabled: true # true 啟用 alert list/detail、outbox 與 Kafka producer。
    interval: 30s # Alert-list background collection interval，使用 Go duration。
    list_response_max_bytes: 8388608 # 單次 alert-list response 上限，單位 bytes；允許範圍 1–67108864。
    detail: # Per-alert detail retrieval limits。
      response_max_bytes: 1048576 # 每筆 alert-detail response 上限，單位 bytes；允許範圍 1–67108864。
      refresh_interval: 10m # 已存在 alert 的重新讀取間隔，至少等於 alert interval。
      max_per_cycle: 200 # 每輪最多 detail requests，允許範圍 1–10000。
      concurrency: 4 # 同時 detail requests，範圍 1–128 且至多等於 max_per_cycle。
  serviceability_logs: # DDAE Serviceability Logs / Events 到專用 Kafka topic 的 pipeline。
    enabled: false # false 為預設；true 建立獨立 worker、producer 與 serviceability-logs.db。
    interval: 30s # Serviceability Log list background interval，使用 Go duration。
    list_response_max_bytes: 8388608 # 單次 log-list response 上限，單位 bytes；範圍 1–67108864。
    detail: # Per-log detail retrieval limits。
      response_max_bytes: 1048576 # 每筆 log-detail response 上限，單位 bytes；範圍 1–67108864。
      refresh_interval: 10m # Listed log 的重新讀取間隔，至少等於 serviceability_logs interval。
      max_per_cycle: 200 # 每輪最多 detail requests，允許範圍 1–10000。
      concurrency: 4 # 同時 detail requests，範圍 1–128 且至多等於 max_per_cycle。

server: # Exporter HTTP server 設定。
  listen_address: 127.0.0.1:9469 # 明確 host:port；container/Kubernetes 使用 0.0.0.0:9469。
  shutdown_grace_period: 15s # Cancellation、state sync 與 HTTP shutdown 的總時間。

security: # 跨 target 的安全確認設定。
  allow_insecure_tls: false # true 表示明確允許個別 target 啟用 insecure diagnostic mode。

ddae: # Dell DDAE Management API client 設定。
  base_url: https://ddae.example.invalid # 單一 HTTPS origin；填入實際 scheme、host 與 optional port。
  source_instance: replace-with-stable-appliance-name # Kafka source identity；alerts 或 serviceability_logs 啟用時必填。
  credentials: # DDAE OAuth password-grant credential file paths。
    username_file: /secure/runtime/ddae-username # 內容只放 dedicated read-only username。
    password_file: /secure/runtime/ddae-password # 內容只放該 DDAE identity password。
    client_secret_file: /secure/runtime/ddae-client-secret # 內容只放 dv-admin-rest client secret。
  tls: # DDAE HTTPS trust 設定。
    ca_file: /etc/ddae-exporter/trust/ddae-ca.pem # 額外 PEM CA bundle；使用 system roots 時可省略此行。
    insecure_skip_verify: false # 與 allow_insecure_tls 同為 true 時，關閉 DDAE certificate/hostname verification。
  request_timeout: 5s # 單次 token 或 DDAE request deadline，需小於 cycle_timeout。
  cycle_timeout: 20s # 單輪 aggregate deadline，需小於每條 enabled pipeline interval。
  response_max_bytes: 4194304 # 一般 DDAE response body 上限，單位 bytes；允許範圍 1–67108864。
  retry_max: 2 # 初次 safe request 後的 retry 次數，允許範圍 0–10。

kafka: # Alert 與 Serviceability Logs producers 共用的連線安全設定。
  brokers: # Kafka bootstrap broker sequence；任一 Kafka pipeline 啟用時需要 1–64 筆。
    - kafka.example.invalid:9093 # 單一 broker host:port；增加 broker 時重複此 list item。
  topic: ddae-serviceability-alerts # Dedicated alert topic，最多 249 bytes。
  serviceability_logs_topic: ddae-serviceability-logs # Dedicated log topic；必須與 alert topic 不同。
  client_id: ddae-exporter # Kafka client ID，1–128 bytes。
  tls: # Kafka broker TLS 與 optional mTLS 設定。
    ca_file: /etc/ddae-exporter/trust/kafka-ca.pem # 額外 broker PEM CA bundle；system roots 可省略。
    # client_cert_file: /secure/runtime/kafka-client.crt # Optional mTLS client certificate PEM。
    # client_key_file: /secure/runtime/kafka-client.key # Optional mTLS private key；與 certificate 成對設定。
    insecure_skip_verify: false # 與 allow_insecure_tls 同為 true 時，關閉 Kafka certificate/hostname verification。
  sasl: # Optional Kafka SASL authentication；本例使用 SCRAM-SHA-512。
    mechanism: SCRAM-SHA-512 # 可用 PLAIN、SCRAM-SHA-256、SCRAM-SHA-512。
    username: replace-with-runtime-identity # SASL mechanism 啟用時使用的 Kafka username。
    password_file: /secure/runtime/kafka-password # 內容只放 Kafka SASL password。
  publish_timeout: 10s # 每筆 produce acknowledgement hard deadline，最小 1s。

state: # Alert 與 Serviceability Logs 各自獨立的 persistent bbolt state。
  dir: /var/lib/ddae-exporter # Enabled Kafka pipelines 使用的 absolute writable directory。
  outbox_max_bytes: 1073741824 # Outbox payload byte hard limit，單位 bytes。
  outbox_max_events: 100000 # Outbox record hard limit，允許範圍 1–10000000。
  checkpoint_retention: 720h # Alert 從 list 消失後的 checkpoint retention。
  checkpoint_max_alerts: 100000 # Retained alert checkpoints 上限，範圍 1–10000000。
  serviceability_logs_outbox_max_bytes: 1073741824 # 專用 log outbox byte hard limit。
  serviceability_logs_outbox_max_events: 100000 # 專用 log outbox record hard limit，範圍 1–10000000。
  serviceability_logs_checkpoint_retention: 720h # Log 從 complete list 消失後的 checkpoint retention。
  serviceability_logs_checkpoint_max_records: 100000 # Retained log checkpoints 上限，範圍 1–10000000。

logging: # Structured application logging 設定。
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

## OCI / Docker 安裝

### 建置 image

```bash
docker build \
  --build-arg VERSION=dev \
  --build-arg REVISION=REPLACE_WITH_GIT_SHA \
  --build-arg BUILD_DATE=2026-08-25T00:00:00Z \
  --tag ddae-exporter:local \
  .
```

最終 image 使用 `scratch` base、UID/GID `65532` 與 `/ddae-exporter` entrypoint。

### 準備 runtime directories

```bash
sudo install -d -m 0700 -o 65532 -g 65532 /srv/ddae-exporter/state
sudo install -d -m 0700 -o 65532 -g 65532 /etc/ddae-exporter/secrets
sudo install -d -m 0755 -o root -g root /etc/ddae-exporter/trust
```

將 YAML、secret files 與 CA bundles 放入對應 host directory，並將 YAML 內的 paths
設定成下列 container paths；同時將 `server.listen_address` 設為 `0.0.0.0:9469`：

| 類別 | Container path |
|---|---|
| YAML | `/etc/ddae-exporter/config.yaml` |
| Secrets | `/run/secrets/...` |
| CA bundles | `/run/trust/...` |
| Persistent state | `/var/lib/ddae-exporter` |

### 啟動 container

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
  ddae-exporter:local --config /etc/ddae-exporter/config.yaml
```

Resource-only 模式使用 DDAE config、credentials 與 trust mounts；alerts 或
Serviceability Logs 啟用時再使用 Kafka secrets 與 persistent state mount。

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

## 啟動後確認

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
policy 通過，coverage 為 81.2%，高於 80% gate；source-reachable vulnerability scan
結果為 `No vulnerabilities found`。

## 專案文件

| 文件 | 內容 |
|---|---|
| [`docs/runbook.md`](docs/runbook.md) | Container、Kubernetes、systemd 維運與 recovery。 |

## License and trademark notice

程式碼依本 repository 的 [Apache License 2.0](LICENSE) 授權。Dell 與相關產品名稱為
其權利人所有；本專案名稱用於描述相容目標。
