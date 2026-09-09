# 跨電腦接續開發

同一個 Git revision 應包含開發所需的指令、規格、核准紀錄、程式與測試。
從另一台電腦 clone 後，請從 repository root 開始，依 `HARNESS/ACTIVE_CHANGE`
指定的 change 接續工作。詳細進度以該 change 的執行計畫為準。

## Git 應保存的內容

| 路徑 | 用途 |
|---|---|
| `AGENTS.md`、子目錄的 `AGENTS.md` | 共用 Agent 工作規則與局部覆寫 |
| `PROJECT.md`、`PLANS.md`、`CODE_REVIEW.md` | 專案背景、計畫格式與審查要求 |
| `HARNESS/` | 完整命令、函式庫、schemas、tests、templates 與版本紀錄 |
| `HARNESS/config.env`、`HARNESS/ACTIVE_CHANGE` | 必要驗證 stages、專案政策與 active change；不是機密環境設定 |
| `SPECIFICATION.md`、`SPEC_CHANGELOG.md` | 各版本需求、驗收條件與變更歷史 |
| `DEVELOPMENT_PLAN.md`、`TEST_PLAN.md`、`TRACEABILITY.md` | 核准及 digest、測試設計、需求追蹤狀態 |
| `plans/`、`docs/decisions/` | 執行進度、發現、後續工作與架構決策 |
| `*.example`、deployment 範本、sanitized fixtures | 不含真實環境憑證的可重用輸入 |
| `cmd/`、`internal/`、`integration/`、`scripts/`、`go.mod`、`go.sum` | 對應規格的原始碼、測試與鎖定依賴 |
| `.gitattributes`、`.gitignore`、`.github/` | 跨平台換行、資料邊界與 CI 工作流程 |

`.agents/skills/` 與 `.codex/` 下明確允許的 `config.toml`、`agents/`、`rules/`、
`skills/`、`environments/` 可以保存經審查的共用設定。這些路徑目前不必存在；
新增時應使用可攜路徑，且不得放入機密或個人帳號資料。
不要把家目錄的整個 `.codex` 複製進 repository。個人聊天紀錄、本機登入與個人
skills 不屬於這份 Git 交付；接續工作所需的決策與進度應寫入專案文件。

## 本機保留、不提交的內容

- `.env`、本機 `config.yaml`、`*.local.*`、秘密檔案、私鑰與真實憑證。
- `bin/`、`dist/`、`output/`、`coverage/`、`test-results/`、dependency caches。
- Exporter state/outbox databases、診斷 captures、解密資料及私人測試結果。
- 舊 Portable 診斷工具已移除；既有本機診斷資料仍不得提交。

即使 capture 已加密，`*.ddaecap` 仍不提交。`.gitignore` 不會移除已追蹤的機密，
也不會自動 commit 或 push。確認變更時應檢查 `git diff --cached`，不要只看
檔案是否被忽略。既有核准紀錄保留原樣；沒有測試證據的狀態不能補寫成通過。

## 新電腦初始化

Harness 支援 macOS 與 Linux 的 Bash 3.2+。Windows 開發請使用 WSL，並在 WSL
內安裝 Git、Go、ripgrep 與 C compiler。`go test -race` 需要受支援平台及可用的 C compiler。

```bash
git clone https://github.com/crispkid/Dell-DDAE-Metrics-Exporter.git
cd Dell-DDAE-Metrics-Exporter
git status --short
go version
go mod download
./HARNESS/harness.sh doctor
./HARNESS/harness.sh instructions:doctor
./HARNESS/harness.sh governance:approved
./HARNESS/harness.sh test
```

使用 `go.mod` 與 `PROJECT.md` 宣告的 Go toolchain（目前 `go1.26.6`）。
初次下載依賴需要網路；security/supply-chain tools 的固定版本與安裝方式在兩份
README 的 Development／開發章節。不要在換機時重新採用 Harness baseline、
覆寫 `PROJECT.md` 範本，或重新產生已核准的 specification/plan/test bytes。

Git checkout 會保留 shell 腳本的 executable bit；`.gitattributes` 使 shell
及規格文件維持 LF，避免 Windows `core.autocrlf` 改變核准 digest。優先使用
`git clone`，下載來源 ZIP 的方式可能失去 executable bit 與 Git revision 資訊。

接續開發前先閱讀 `AGENTS.md`、`PROJECT.md`、`HARNESS/ACTIVE_CHANGE`、
該 change 的 `SPECIFICATION.md` 章節、`DEVELOPMENT_PLAN.md` 核准紀錄及
`plans/<change-id>.md`。再依測試設計與追蹤表確認工作範圍。

產品交付時執行：

```bash
./HARNESS/harness.sh sdd:check
./HARNESS/harness.sh verify
```

`governance:approved` 驗證既有核准與 digest；`sdd:check`／`verify` 另要求
完成度及驗證證據。尚未完成的 trace row 或缺少 DDAE、Kafka、OpenSearch、
原生 Windows 環境時，後兩者可能失敗；必須依實際結果接續，不得改弱 gates。
生成的 `test-results/` 不隨 Git 搬移，新電腦須重跑適用驗證。

## 提交與換機前檢查

這次修正讓以前被忽略的文件可以正常加入 Git。首次提交時須一併納入完整
`HARNESS/`、根目錄治理文件、`plans/`、`docs/decisions/`，以及它們依賴的
程式與測試變更，避免只提交規格卻缺少對應實作。

```bash
git status --short --untracked-files=all
git ls-files AGENTS.md PROJECT.md HARNESS SPECIFICATION.md DEVELOPMENT_PLAN.md TEST_PLAN.md TRACEABILITY.md plans docs/decisions
git ls-files -ci --exclude-standard
git diff --cached --check
git diff --cached --stat
```

`git ls-files` 應列出上述共用檔案；`git ls-files -ci --exclude-standard` 應沒有
被排除的本機產物卻仍受到追蹤。確認 staged 內容完整且不含機密後才 commit、push，
另一台電腦才能 clone/pull 到相同內容。個別電腦若有額外 global excludes，使用
`git check-ignore -v --no-index <path>` 查出規則來源。
