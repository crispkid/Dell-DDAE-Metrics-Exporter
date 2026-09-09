# Windows 11 DDAE 可攜式診斷套件

將這個資料夾帶到測試環境，即可檢查 DDAE API、記錄 Parser 結果，並選擇加密保存業務 API 的 Request／Response，帶回分析主機重播。

現場不需要安裝 Go、Git、Docker、Python 或 WSL。自我檢查只使用內建資料與本機 loopback；現場診斷需要連到經授權的非正式 DDAE 環境，但不需要網際網路。

這是 DDAE-7 本機開發測試包，不是正式相容性認證版本。Windows 11 原生執行與 authenticated DDAE 的驗證證據須另外取得。實際來源與工具鏈記錄在 `build-manifest.json`。

## 套件內容

| 檔案 | 用途 |
|---|---|
| `Prepare.cmd` | 校驗套件、設定輸出權限、首次建立設定檔 |
| `Run-SelfTest.cmd` | 使用合成資料檢查 Parser、加密與本機 TLS 認證流程 |
| `Run-Diagnostics.cmd` | 依 `config.yaml` 執行有時間與容量上限的診斷 |
| `Create-Analysis-Key.cmd` | 在可信任分析主機建立公私鑰 |
| `Review-Results.cmd` | 解密業務內容，同時產生離線 Parser 報告 |
| `Run-Exporter.cmd` | 以獨立 `exporter.yaml` 啟動資源監控 Exporter |
| `Launch.cmd` | 供其他 CMD 呼叫，自動選用 x64／ARM64 執行檔 |
| `config.example.yaml`、`exporter.example.yaml` | 兩種模式的設定範本 |
| `bin/windows-amd64/`、`bin/windows-arm64/` | 各含 `ddae-diagnose.exe` 與 `ddae-exporter.exe` |
| `tools/darwin-arm64/ddae-diagnose` | Apple Silicon Mac 的金鑰、解密與重播工具 |
| `SHA256SUMS`、`build-manifest.json` | 檔案雜湊、建置工具鏈與來源識別 |
| `THIRD-PARTY-NOTICES.txt` | 第三方元件授權聲明 |

## 現場操作

### 解壓縮與準備

1. 將 `Portable-Windows11.zip` 複製到 Windows 11 x64 或 ARM64 電腦。
2. 完整解壓縮，將 `Portable` 放在目前帳號可寫入的本機 NTFS 目錄，例如 `C:\DDAE-Test\Portable`。使用專用資料夾，不要直接使用磁碟根目錄、網路分享、FAT/exFAT 隨身碟、符號連結或 junction。
3. 雙擊 `Prepare.cmd`。成功顯示 `Exit code: 0`，並建立 `config.yaml`、`exporter.yaml`、`secrets`、`trust`、`keys`、`results`。已有設定會保留。
4. 先雙擊 `Run-SelfTest.cmd`。開啟新產生的 `results\<執行編號>\summary.txt`；預期為 `PASS`、退出代碼 `0`。此步驟不需要現場帳密。

準備程序會將資料目錄的存取限制為目前使用者與 SYSTEM。設計上不需要管理員權限，也不會安裝服務、修改登錄、PowerShell 執行原則或防護軟體。若組織的應用程式控管阻擋執行，請依既有流程處理。

`SHA256SUMS` 可檢查檔案是否缺漏或改變，不是數位簽章。請從可信任管道取得套件；保留範本、腳本與程式檔原樣，只編輯準備程序建立的執行期設定。

### 建立帳密檔案

使用記事本分別建立以下檔案。每個檔案只放對應的值，不加引號、欄位名稱或註解。

| 檔案 | 內容範例，請換成實際測試值 |
|---|---|
| `secrets\ddae-username` | `<read-only-username>` |
| `secrets\ddae-password` | `<read-only-password>` |
| `secrets\ddae-client-secret` | `<ddae-client-secret>` |

在記事本選「另存新檔」：檔案類型選「所有檔案」，編碼選 **UTF-8，不含 BOM**。確認沒有自動加上 `.txt`，也不要把表格中的 `<…>` 當成帳密。

檔案上限為 64 KiB；內容須為有效 UTF-8、非空且不含 NUL。讀取時只移除最後一組 CRLF，或最後一個 LF／CR；其他空白和換行會保留。請直接貼上帳密，不要額外加空格或空白行。診斷程式會拒絕 UTF-8 BOM。

### 填入連線設定

用記事本開啟 `config.yaml`，至少修改 `ddae.base_url`。只放 HTTPS origin，不含 API path：

```yaml
ddae:
  base_url: https://ddae.example.invalid
```

這是占位網址，程式會拒絕 `.invalid`；請換成管理人員提供的位址。預設 Ping 為 `/ping`，其他 API 使用 `/v1`。所有相對路徑以**設定檔所在資料夾**為基準；Windows 路徑可使用 `/`，避免 YAML 雙引號中的反斜線跳脫問題。

若 DDAE 使用組織 CA，將 PEM 格式 CA 放到 `trust\ddae-ca.pem`，再設定：

```yaml
ddae:
  tls:
    ca_file: trust/ddae-ca.pem
    insecure_skip_verify: false
```

以上片段是現有欄位的修改示意，不要重複新增第二個 `ddae`。預設使用系統信任庫及指定的 CA；請保留 TLS 驗證。

### 啟用原始內容擷取

只要 HTTP 狀態與 Parser 結果時，保留 `capture.enabled: false`。若要帶回原始業務內容，先依「分析主機操作」建立金鑰，現場只接收公鑰：

1. 將公鑰複製為 `keys\recipient-public.pem`。
2. 在 `config.yaml` 修改以下欄位：

```yaml
capture:
  enabled: true
  recipient_public_key_file: keys/recipient-public.pem
  max_body_bytes: 8388608
  max_total_bytes: 536870912
```

公鑰、加密或權限初始化失敗時，程式會在連線前停止，不會改存明文。私鑰不要放進現場套件，也不要跟結果壓縮檔一起寄送。

### 執行診斷與取回結果

1. 確認已取得非正式環境測試授權，帳號具備所選 API 的唯讀權限。
2. 雙擊 `Run-Diagnostics.cmd`。預設持續五分鐘，每輪完成後等待 30 秒。
3. 保持視窗開啟；需要提早停止時按 Ctrl+C。正常取消會盡力保存已完成記錄；強制結束、關機或磁碟故障可能只留下部分檔案。
4. 開啟最新的 `results\<執行編號>\summary.txt`，再看 `report.json` 與 `steps.jsonl`。
5. 完成資料政策檢查後，將該次的 `transfer.zip` 帶回分析主機。程式不自動上傳資料。

每次建立新目錄，不覆蓋舊結果。結果不自動刪除，請依組織保存期限清理。磁碟須容納加密檔、報告及結果 ZIP；預設擷取上限 512 MiB，ZIP 可能再占用相近容量。

## 設定參考

診斷與一般 Exporter 是獨立設定格式。診斷不接受環境變數覆寫、明文帳密欄位、未知鍵、重複鍵、YAML alias 或多份 YAML document。

| 設定 | 預設／必要值 | 說明 |
|---|---|---|
| `version` | 必填 `1` | 診斷 YAML 格式版本 |
| `ddae.base_url` | 必填 | HTTPS origin，不帶 path、query 或帳密 |
| `ddae.paths.ping_prefix` | `""` | 接上 `/ping`；例如 `/rest/v1` 會產生 `/rest/v1/ping` |
| `ddae.paths.api_prefix` | `/v1` | 接上各業務 API suffix；例如 `/rest/v1` |
| `ddae.credentials.username_file` | 必填 | 帳號檔路徑 |
| `ddae.credentials.password_file` | 必填 | 密碼檔路徑 |
| `ddae.credentials.client_secret_file` | 必填 | Client secret 檔路徑 |
| `ddae.tls.ca_file` | 空字串 | 選用 PEM CA 檔 |
| `ddae.tls.insecure_skip_verify` | `false` | 略過憑證檢查的目標開關 |
| `security.allow_insecure_tls` | `false` | 全域許可；須與上一項同時為 `true` |
| `ddae.request_timeout` | `5s` | 單次 API 操作期限，大於零且不超過一小時 |
| `ddae.retry_max` | `2` | 0～10；沿用正式 Client 重試規則 |
| `checks.ping` | `true` | Ping 檢查 |
| `checks.resources` | `true` | Cluster、node、system lock、shutdown 狀態 |
| `checks.alerts` | `true` | Serviceability issues 清單及所選明細 |
| `checks.serviceability_logs` | `true` | Serviceability events 清單及所選明細 |
| `run.duration` | `5m` | 執行期間，1 秒～1 小時 |
| `run.interval` | `30s` | 輪次間隔，1～300 秒；不重疊執行 |
| `run.max_requests` | `1000` | HTTP 嘗試總數，1～10000，包含認證、刷新與重試 |
| `run.max_details_per_family_per_cycle` | `10` | 每輪、每類明細上限，0～100；0 跳過明細 |
| `run.shutdown_grace_period` | `15s` | 收尾期限，1～30 秒；OS／儲存故障時盡力保存 |
| `capture.enabled` | `false` | 加密保存原始業務 API 內容 |
| `capture.recipient_public_key_file` | 擷取時必填 | PKIX PEM RSA 公鑰，3072～8192 bits |
| `capture.max_body_bytes` | `8388608` | 每個 Body 的 bytes 上限，1～67108864；另受各 API 上限約束 |
| `capture.max_total_bytes` | `536870912` | 加密檔總 bytes 上限，1 MiB～2 GiB，含 framing |
| `output.directory` | `results` | 每次執行結果的父目錄 |
| `output.max_report_bytes` | `16777216` | 安全報告容量預算，1～64 MiB；預留收尾空間，ZIP 另占容量 |

四個 `checks` 至少啟用一個。Prefix 可為空，或以 `/` 開頭且不以 `/` 結尾的合法片段；不要填完整網址、query、fragment、`..` 或百分比跳脫字元。

TLS 兩個開關同時為 `true` 才略過驗證，報告會標示 `insecure_tls: true`；這類結果不能作為 release 的 TLS 驗證證據。優先修正 CA、主機名稱或憑證鏈。

## API 範圍與結果判讀

業務呼叫全部使用 GET；認證只有固定 token path 的 POST。診斷不初始化 Kafka、OpenSearch 或 Exporter outbox，不修改 DDAE 狀態。

| 報告 operation | 預設路徑 |
|---|---|
| `ping` | `/ping` |
| `clusters` | `/v1/ddae-clusters` |
| `nodes` | `/v1/infrastructure-nodes` |
| `lock` | `/v1/system-lock` |
| `power` | `/v1/system-shutdown`，僅讀取狀態 |
| `alert_list`、`alert_detail` | `/v1/serviceability-issues`、`/v1/serviceability-issues/{id}` |
| `serviceability_log_list`、`serviceability_log_detail` | `/v1/serviceability-events`、`/v1/serviceability-events/{id}` |
| `token` | `/auth/realms/ddae/protocol/openid-connect/token`，不受 prefix 設定影響 |

明細 ID 只取自清單，經驗證、去重、排序後按輪次輪替。分別查看 available／selected／successful；清單完整不代表所有明細已檢查，也不會把 `threshold` 猜成分頁參數。

| 結果檔 | 內容 |
|---|---|
| `summary.txt` | 中文摘要、狀態與明細計數 |
| `report.json` | 模式、建置來源、退出碼及檢查結果 |
| `steps.jsonl` | 逐次寫入的 HTTP 狀態、時間、擷取完整性及 Parser 結果；耗時單位 ms |
| `schema-observations.json` | 各 operation 最近的固定類型與 Parser／完整性觀察 |
| `diagnostic.log` | 不含原始資料的生命週期摘要 |
| `manifest.json` | 結果檔 SHA-256；不是數位簽章 |
| `http-capture.ddaecap` | 明確啟用後產生的加密業務 HTTP 記錄 |
| `status.complete` | 完整收尾後才產生；檢查失敗但完整執行也有此標記 |
| `transfer.zip` | 白名單結果檔；不含設定、帳密、CA、私鑰或一般 Exporter 狀態 |

`http_ok`、`decode_ok`、`validation_ok`、`contract_ok` 是不同結果。HTTP 200 不代表 Parser 成功；可解析也不代表清單完整。既有 Parser 行為會保留，例如 cluster 的 `results` envelope 目前與接受陣列的 Parser 不符，診斷不會暗中修正它。

| 退出碼 | 意義 |
|---|---|
| `0` | 已選且可執行的檢查通過，執行完整 |
| `1` | 執行完成，至少一項檢查失敗 |
| `2` | 指令、設定、平台、金鑰或輸入錯誤 |
| `3` | 容量／請求限制、讀取不完整或儲存失敗，未完整完成 |
| `130` | 使用者取消，結果不完整 |

停用或未選取的明細標示 `SKIP` 並附原因。`status.complete` 不存在時，必須視為未完整收尾；磁碟失敗時已寫入的摘要或 ZIP 也可能是部分結果。初始化還無法建立安全目錄時，只有安全的主控台錯誤與非零退出碼。

## 原始內容的保護範圍

加密內容保留業務 GET 的 method、URL、HTTP version、排除認證資訊後的標頭、空 Request Body，以及 Parser 處理前的 Response Body bytes。JSON 不重排，未知欄位與非 JSON 回應也保留。Gzip 回應保存解壓後、Parser 所讀的 bytes，並記錄原回應標頭與 `content_decoded`。

這不是網路封包擷取或完全未遮蔽的 wire 副本。認證 API 原文、Authorization、Proxy-Authorization、Cookie、Set-Cookie 及含已知認證值的標頭資訊不進入加密內容。認證步驟只記安全狀態與時間。

未知業務 Body 仍可能含敏感資訊甚至伺服器回傳的秘密，加密檔不能視為去識別化資料。安全摘要不含原始 URL、ID、訊息、標頭或 Body 雜湊。

每次使用新的 AES-256-GCM 金鑰，由 RSA-OAEP-SHA256 包裝給指定公鑰。各筆記錄與最終索引分別驗證。竄改、錯誤私鑰、缺尾或順序異常會失敗；中斷檔案可分析已通過驗證的記錄，整體仍標示 `INCOMPLETE`。

Body 上限取設定與正式 Client 上限的較小值：資源 4 MiB、清單 8 MiB、明細 1 MiB；HTTP response headers 上限 1 MiB。超限只保留有標示的不完整前綴，不會為補齊內容重送 API。加密不保護已遭入侵的電腦或程序記憶體。

## 分析主機操作

### 建立金鑰

在可信任 Windows 11 分析主機，以 CMD 切換到 `Portable`，使用獨立、專用金鑰資料夾：

```bat
Create-Analysis-Key.cmd --private-key "C:\DDAE-Analysis-Keys\recipient-private.pem" --public-key "C:\DDAE-Analysis-Keys\recipient-public.pem"
```

Apple Silicon Mac 可在自己的 `Portable` 執行：

```bash
./tools/darwin-arm64/ddae-diagnose keygen \
  --private-key "$HOME/DDAE-Analysis-Keys/recipient-private.pem" \
  --public-key "$HOME/DDAE-Analysis-Keys/recipient-public.pem"
```

程式不覆寫金鑰。私鑰為未另加密的 PKCS#8 PEM，需依賴分析主機受限權限與組織金鑰政策，請安全備份。現場只拿公鑰；遺失私鑰就無法解密既有擷取檔。

### 解密與重播

先在可信任分析位置解開現場 `transfer.zip`。Windows 範例：

```bat
Review-Results.cmd --capture "C:\DDAE-Inbox\run\http-capture.ddaecap" --private-key "C:\DDAE-Analysis-Keys\recipient-private.pem" --output "C:\DDAE-Analysis\raw"
```

Mac 範例：

```bash
./tools/darwin-arm64/ddae-diagnose decrypt \
  --capture "$HOME/DDAE-Inbox/run/http-capture.ddaecap" \
  --private-key "$HOME/DDAE-Analysis-Keys/recipient-private.pem" \
  --output "$HOME/DDAE-Analysis/raw"
```

每次建立新目錄，包含 `000001.exchange.json`、`000001.body` 等明文，並產生 Parser 報告。`.body` 是 Response bytes；metadata JSON 的 Body 採 Base64。這些明文不會放進分析結果 `transfer.zip`，也不要貼進公開 Issue、Git 或聊天記錄。

只重跑 Parser、不另存明文，使用 `replay`：

```bat
Launch.cmd replay --capture "C:\DDAE-Inbox\run\http-capture.ddaecap" --private-key "C:\DDAE-Analysis-Keys\recipient-private.pem" --output "C:\DDAE-Analysis\replay"
```

Mac 將 `decrypt` 改成 `replay` 即可。兩者都不連 DDAE、不使用現場帳密，也不執行擷取 URL。建立 Bug 回歸測試前，先把敏感資料改成去識別化的合成 fixture。

## 另行執行一般 Exporter

先修改 `exporter.yaml` 的位址、prefix、CA 與帳密檔路徑，再雙擊 `Run-Exporter.cmd`。這是獨立前景程序，固定資源監控、停用 alerts／serviceability logs，綁定 `127.0.0.1:9469`，不產生原始 HTTP 擷取。

同機開啟 `http://127.0.0.1:9469/metrics` 查看 Prometheus 資料。一般 stdout/stderr 留在主控台，不併入診斷報告；其他設定仍沿用一般 Exporter 的環境變數規則，請先檢查既有 `DDAE_*` 設定。按 Ctrl+C 停止。

## 常見問題

- **準備失敗**：確認完整套件、`SHA256SUMS`、NTFS、目前帳號寫入／ACL 權限，以及路徑不是 junction 或分享。不要修改校驗檔來略過檢查。
- **自我檢查失敗**：看 `summary.txt`。`loopback_tls_auth` 需要允許程式自有的 `127.0.0.1` 短暫連線；此失敗不代表 DDAE 有問題。
- **退出碼 2**：檢查 YAML 拼字、重複鍵、占位網址、帳密檔附檔名／BOM、公鑰與 CA。執行期只編輯 `config.yaml`，不要更改範本。
- **401／403**：請管理人員確認帳密、Client secret 與 API 權限；提供 operation 與狀態碼，不提供 token。
- **404**：確認兩組 prefix；程式不會自動嘗試其他 API path。
- **HTTP 成功但 Parser／contract 失敗**：帶回加密內容，在可信任主機解密／重播，檢查回應形狀、清單完整性及明細 ID。
- **INCOMPLETE**：檢查容量、請求上限、磁碟空間與取消狀態。部分記錄可分析，但不可當完整測試。
- **解密失敗**：確認私鑰與該次公鑰配對，以及檔案傳輸完整性；不要把私鑰交給現場排查。

需要維護者協助時，優先提供安全摘要、建置來源與經核准的結果包。Windows x64／ARM64 原生測試、真實 DDAE 與獨立安全審查須各自留存證據，不能以交叉建置或 self-test 代替。
