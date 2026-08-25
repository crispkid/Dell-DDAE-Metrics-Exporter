# Dell DDAE Metrics Exporter

Dell DDAE Metrics Exporter 是以 Go 開發的唯讀監控服務，目標相容版本為 Dell Data
Domain Active Enterprise（DDAE）1.5.0。服務將叢集、節點與 appliance operability
資料轉換為 Prometheus metrics，並將 serviceability alerts 整理為具版本的 Kafka
事件。

## 專案能力

| 能力 | 已實作行為 |
|---|---|
| DDAE 資源監控 | 定期收集 API reachability、cluster 狀態、Coordinator/Worker 配置資源、node 狀態與容量、system lock、control-plane 與 node readiness。 |
| Prometheus 輸出 | 由背景收集程序維護 current snapshot，並透過 `/metrics` 提供穩定的 metric contract。 |
| DDAE 告警收集 | 從 serviceability issue 清單取得 ID，再以有限的並行量讀取 detail。 |
| Kafka 告警發布 | 將通過型別與欄位 allowlist 的告警寫入 durable bbolt outbox，再以 at-least-once、idempotent producer 發布。 |
| 獨立 pipeline | Resource monitoring 與 alert monitoring 可分別啟用並設定各自週期，支援 dual、resource-only 與 alert-only 三種模式。 |
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
```

## Monitoring 模式

有效設定至少啟用一條 pipeline：

| 模式 | `monitoring.resources.enabled` | `monitoring.alerts.enabled` | 運行內容 |
|---|---:|---:|---|
| Dual | `true` | `true` | 提供 resource metrics，並將 alerts 發布至 Kafka。 |
| Resource-only | `true` | `false` | 提供 DDAE resource metrics，執行時只需要 DDAE 連線與 credentials。 |
| Alert-only | `false` | `true` | 收集 alerts 並發布至 Kafka，使用 persistent state 保存 outbox 與 checkpoints。 |

兩條 pipeline 預設皆為 `true`。啟用的 pipeline 會參與 `/readyz` 判定；
`ddae_monitoring_enabled{pipeline="resources"}` 與
`ddae_monitoring_enabled{pipeline="alerts"}` 可直接呈現實際模式。

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

部署時適合使用 dedicated、least-privilege、read-only DDAE identity。TLS 最低版本為
TLS 1.2，certificate chain 與 hostname verification 預設啟用。

## HTTP endpoints

預設 listener 為 `127.0.0.1:9469`。

| Endpoint | 回應內容 |
|---|---|
| `GET /metrics` | Prometheus exposition 與 collector diagnostics。 |
| `GET /healthz` | 程序存活狀態。 |
| `GET /readyz` | 所有已啟用 pipeline 的 readiness；ready 時回 `200`，其餘狀態回 `503`。 |

## Prometheus metrics

### 共用 metrics

| Metric | 說明 |
|---|---|
| `ddae_build_info{version,go_version}` | Exporter build 與 Go version。 |
| `ddae_monitoring_enabled{pipeline}` | `resources` 與 `alerts` pipeline 的啟用狀態。 |

### Resource pipeline metrics

| 類別 | Metrics |
|---|---|
| Collector | `ddae_up`、`ddae_collector_success{collector}`、`ddae_collector_duration_seconds{collector}`、`ddae_snapshot_age_seconds` |
| DDAE API | `ddae_management_api_up` |
| Cluster | `ddae_cluster_state_info{cluster,state}`、`ddae_cluster_coordinator_configured_cpu_cores`、`ddae_cluster_coordinator_configured_memory_bytes`、`ddae_cluster_worker_configured_cpu_cores`、`ddae_cluster_worker_configured_memory_bytes` |
| Node state | `ddae_node_state_info{node,state}`、`ddae_node_ready`、`ddae_node_condition{node,condition}` |
| Node capacity | `ddae_node_capacity_cpu_cores`、`ddae_node_capacity_memory_bytes`、`ddae_node_capacity_ephemeral_storage_bytes` |
| Node allocatable | `ddae_node_allocatable_cpu_cores`、`ddae_node_allocatable_memory_bytes`、`ddae_node_allocatable_ephemeral_storage_bytes` |
| Appliance | `ddae_system_locked`、`ddae_control_plane_ready`、`ddae_nodes_ready`、`ddae_nodes_total` |

### Alert pipeline metrics

| Metric | 說明 |
|---|---|
| `ddae_alert_list_complete` | 本輪 alert list 完整狀態。 |
| `ddae_alert_detail_deferred` | 依 per-cycle 上限延後處理的 detail 數量。 |
| `ddae_alert_pipeline_ready` | Alert pipeline readiness。 |
| `ddae_kafka_publish_success` | 最近一次 Kafka publish 結果。 |
| `ddae_kafka_publish_duration_seconds` | Kafka publish 耗時。 |
| `ddae_kafka_events_published_total` | 成功發布事件總數。 |
| `ddae_kafka_events_failed_total{reason}` | 依固定 failure class 分類的失敗事件總數。 |
| `ddae_kafka_buffered_events` | Durable outbox 目前保存的事件數。 |

Metric type、help、label 與語意使用固定的 v1 contract。Site、environment、region 等
部署維度可由 Prometheus service discovery 或 relabeling 加入。

## Kafka alert event

每筆 event 使用 typed JSON 與固定的 schema contract：

| Field | 值或來源 |
|---|---|
| `schema_version` | 固定為 `1.0`。 |
| `event_type` | 固定為 `ddae.serviceability_alert.upsert`。 |
| `source_system` | 固定為 `dell_ddae`。 |
| `source_instance` | YAML 中設定的穩定 appliance identity。 |
| `alert_id` | Alert list 與 detail response 對應的 ID。 |
| `content_hash_sha256` | Canonical normalized `alert` object 的 SHA-256。 |
| `observed_at` | UTC RFC 3339 observation time。 |
| `alert` | 僅含核准 typed fields 的 object。 |

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

## 安裝前準備

| 啟用範圍 | 準備項目 |
|---|---|
| 所有模式 | DDAE 1.5.0 HTTPS origin。 |
| 所有模式 | Dedicated read-only username、password 與 `dv-admin-rest` client secret。 |
| 所有模式 | DDAE 使用 private CA 時，準備 PEM CA bundle。 |
| Alert pipeline | Kafka brokers、dedicated topic 與支援 TLS、`acks=all` 的 broker 設定。 |
| Alert pipeline | 依環境選用 Kafka SASL、mTLS，或兩者組合。 |
| Alert pipeline | 支援 file lock 與 fsync 的 persistent state directory；每個 DDAE target 使用一份獨立 state。 |
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
| `monitoring` | 選擇 dual、resource-only 或 alert-only，並設定收集週期。 |
| `ddae` | 填入 DDAE HTTPS origin、stable source instance、credentials file paths 與 CA path。 |
| `kafka` | Alert pipeline 啟用時填入 brokers、topic、TLS 與選用的 SASL 設定。 |
| `state` | Alert pipeline 啟用時填入 absolute persistent directory。 |
| `server` | 選擇 Exporter listener 與 shutdown grace period。 |
| `logging` | 選擇 `json` 或 `text` 格式，以及 log level。 |

本機啟用 alert pipeline 時，請將 `state.dir` 設為剛建立之 `state` directory 的
absolute path。

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

以下範例展示 dual mode 的完整結構；`.invalid` 與 `replace-with-*` 為部署時要替換的
範例值：

```yaml
version: 1

monitoring:
  resources:
    enabled: true
    interval: 30s
    stale_after: 120s
  alerts:
    enabled: true
    interval: 30s
    list_response_max_bytes: 8388608
    detail:
      response_max_bytes: 1048576
      refresh_interval: 10m
      max_per_cycle: 200
      concurrency: 4

server:
  listen_address: 127.0.0.1:9469
  shutdown_grace_period: 15s

security:
  allow_insecure_tls: false

ddae:
  base_url: https://ddae.example.invalid
  source_instance: replace-with-stable-appliance-name
  credentials:
    username_file: /secure/runtime/ddae-username
    password_file: /secure/runtime/ddae-password
    client_secret_file: /secure/runtime/ddae-client-secret
  tls:
    ca_file: /etc/ddae-exporter/trust/ddae-ca.pem
    insecure_skip_verify: false
  request_timeout: 5s
  cycle_timeout: 20s
  response_max_bytes: 4194304
  retry_max: 2

kafka:
  brokers:
    - kafka.example.invalid:9093
  topic: ddae-serviceability-alerts
  client_id: ddae-exporter
  tls:
    ca_file: /etc/ddae-exporter/trust/kafka-ca.pem
    insecure_skip_verify: false
  sasl:
    mechanism: SCRAM-SHA-512
    username: replace-with-runtime-identity
    password_file: /secure/runtime/kafka-password
  publish_timeout: 10s

state:
  dir: /var/lib/ddae-exporter
  outbox_max_bytes: 1073741824
  outbox_max_events: 100000
  checkpoint_retention: 720h
  checkpoint_max_alerts: 100000

logging:
  level: info
  format: json
```

### Pipeline、DDAE 與 HTTP

| YAML key | Default／啟用條件 | 說明 |
|---|---|---|
| `monitoring.resources.enabled` | `true` | 控制 resource collectors 與 resource metrics。 |
| `monitoring.resources.interval` | `30s` | Resource pipeline 背景週期。 |
| `monitoring.resources.stale_after` | `120s` | Current resource snapshot 的最大年齡。 |
| `monitoring.alerts.enabled` | `true` | 控制 alert list/detail、Kafka 與 state。 |
| `monitoring.alerts.interval` | `30s` | Alert list 背景週期。 |
| `server.listen_address` | `127.0.0.1:9469` | HTTP listener。 |
| `server.shutdown_grace_period` | `15s` | Cancellation、state sync 與 HTTP shutdown budget。 |
| `ddae.base_url` | 所有模式 | 單一 DDAE HTTPS origin。 |
| `ddae.source_instance` | Alert pipeline | 1–128 bytes 的穩定 event identity。 |
| `ddae.credentials.username_file` | 所有模式 | DDAE username secret file。 |
| `ddae.credentials.password_file` | 所有模式 | DDAE password secret file。 |
| `ddae.credentials.client_secret_file` | 所有模式 | `dv-admin-rest` client secret file。 |
| `ddae.tls.ca_file` | System roots | 額外的 PEM CA bundle。 |
| `ddae.request_timeout` | `5s` | 單次 token 或 DDAE request deadline。 |
| `ddae.cycle_timeout` | `20s` | 一輪 aggregate collection deadline。 |
| `ddae.response_max_bytes` | `4194304` | 一般 DDAE response body hard limit，單位為 bytes。 |
| `ddae.retry_max` | `2` | 初次 safe request 後的 retry 次數，範圍 `0–10`。 |

Duration 使用 Go duration 語法，例如 `500ms`、`5s`、`10m`、`720h`。有效時間關係如下：

| 關係 | 用途 |
|---|---|
| `ddae.request_timeout < ddae.cycle_timeout` | 保留 cycle 聚合處理時間。 |
| `ddae.cycle_timeout < enabled pipeline interval` | 確保每輪工作落在 pipeline 週期內。 |
| `monitoring.resources.stale_after > monitoring.resources.interval` | 保留 resource snapshot freshness window。 |

### Alert detail

| YAML key | Default | 說明 |
|---|---:|---|
| `monitoring.alerts.list_response_max_bytes` | `8388608` | Alert list response hard limit。 |
| `monitoring.alerts.detail.response_max_bytes` | `1048576` | 每筆 detail response hard limit。 |
| `monitoring.alerts.detail.refresh_interval` | `10m` | 清單內 alert 的 detail refresh 間隔，設定值至少等於 alert interval。 |
| `monitoring.alerts.detail.max_per_cycle` | `200` | 每輪 detail requests 上限，範圍 `1–10000`。 |
| `monitoring.alerts.detail.concurrency` | `4` | 同時 detail requests，範圍 `1–128`，設定值至多等於 `max_per_cycle`。 |

超過單輪上限的項目會依 deterministic 順序延後，並由
`ddae_alert_detail_deferred` 呈現數量。

### Kafka 與 persistent state

| YAML key | Default／啟用條件 | 說明 |
|---|---|---|
| `kafka.brokers` | Alert pipeline | 1–64 個 Kafka broker addresses；所有連線使用 TLS。 |
| `kafka.topic` | Alert pipeline | Dedicated topic，最長 249 bytes。 |
| `kafka.client_id` | `ddae-exporter` | Kafka client identity。 |
| `kafka.tls.ca_file` | System roots | 額外的 broker CA PEM bundle。 |
| `kafka.tls.client_cert_file` | 選用 | Kafka mTLS certificate path，搭配 client key。 |
| `kafka.tls.client_key_file` | 選用 | Kafka mTLS private-key path，搭配 client certificate。 |
| `kafka.sasl.mechanism` | 選用 | `PLAIN`、`SCRAM-SHA-256` 或 `SCRAM-SHA-512`。 |
| `kafka.sasl.username` | SASL 啟用時 | Kafka SASL username。 |
| `kafka.sasl.password_file` | SASL 啟用時 | Kafka SASL password secret file。 |
| `kafka.publish_timeout` | `10s` | 每筆 publish 的 hard deadline，包含已送出的 in-flight produce；最小 `1s`。逾時時保留 outbox record 供重播。 |
| `state.dir` | `/var/lib/ddae-exporter` | Alert pipeline 的 absolute persistent directory。 |
| `state.outbox_max_bytes` | `1073741824` | Durable outbox byte hard limit。 |
| `state.outbox_max_events` | `100000` | Durable outbox record hard limit。 |
| `state.checkpoint_retention` | `720h` | Alert checkpoint retention。 |
| `state.checkpoint_max_alerts` | `100000` | Retained alert checkpoints hard limit。 |

Outbox 保留待確認 records；Kafka 恢復後會依序重播。若 hard timeout 發生在 broker
可能已接受、但 client 尚未收到確認的期間，該 record 仍會保留，因此可能以相同 key
再次送出。Consumer 必須以 record key 做 idempotent upsert。達到任一 outbox hard
limit 時會保留現有 records、暫停新增內容，並將 alert readiness 設為 `0`。

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
CA 並維持三個欄位為 `false`。

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
| `ddae.base_url` | `DDAE_BASE_URL` |
| `ddae.source_instance` | `DDAE_SOURCE_INSTANCE` |
| `server.listen_address` | `EXPORTER_LISTEN_ADDRESS` |
| `ddae.tls.ca_file` | `DDAE_CA_FILE` |
| `ddae.tls.insecure_skip_verify` | `DDAE_TLS_INSECURE_SKIP_VERIFY` |
| `security.allow_insecure_tls` | `ALLOW_INSECURE_TLS` |
| `kafka.brokers` | `KAFKA_BROKERS`，以逗號分隔。 |
| `kafka.topic` | `KAFKA_TOPIC` |
| `kafka.tls.ca_file` | `KAFKA_CA_FILE` |
| `kafka.tls.insecure_skip_verify` | `KAFKA_TLS_INSECURE_SKIP_VERIFY` |
| `state.dir` | `STATE_DIR` |
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

Resource-only 模式使用 DDAE config、credentials 與 trust mounts；dual 與 alert-only
模式再使用 Kafka secrets 與 persistent state mount。

## Kubernetes 安裝

Repository 提供的 manifest：

| File | 內容 |
|---|---|
| [`deploy/kubernetes/configmap.yaml`](deploy/kubernetes/configmap.yaml) | 非敏感 YAML 設定範例。 |
| [`deploy/kubernetes/deployment.yaml`](deploy/kubernetes/deployment.yaml) | ServiceAccount、2 GiB RWO PVC、單 replica `Recreate` Deployment 與 ClusterIP Service。 |
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
| `/metrics` | 包含 `ddae_build_info`、兩筆 `ddae_monitoring_enabled`，以及已啟用 pipeline 的 metrics。 |
| Kafka | 產生測試 alert 後，topic 收到 schema `1.0` event，consumer 可依 key upsert。 |

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

目前 repository 的本機基準結果為 lint、race-enabled tests、build 與 CI policy 通過，
coverage 為 81.1%，高於 80% gate。

## 專案文件

| 文件 | 內容 |
|---|---|
| [`docs/runbook.md`](docs/runbook.md) | Container、Kubernetes、systemd 維運與 recovery。 |

## License and trademark notice

程式碼依本 repository 的 [Apache License 2.0](LICENSE) 授權。Dell 與相關產品名稱為
其權利人所有；本專案名稱用於描述相容目標。
