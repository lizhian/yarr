# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" -> "Write tests for invalid inputs, then make them pass"
- "Fix the bug" -> "Write a test that reproduces it, then make it pass"
- "Refactor X" -> "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] -> verify: [check]
2. [Step] -> verify: [check]
3. [Step] -> verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## 项目说明：yarr

`yarr` 是一个用 Go 编写的 Web RSS/feed 阅读器，发布形态是单个二进制文件。它既可以作为带托盘图标的桌面应用运行，也可以作为个人自托管服务运行，数据存储使用 SQLite。

### 技术栈与依赖

- 模块名：`github.com/nkanaev/yarr`
- Go 版本：`go 1.23.0`，toolchain 为 `go1.23.5`
- 因为存储层使用 `github.com/mattn/go-sqlite3`，所以需要 CGO。
- 依赖已 vendored 到 `vendor/`；除非明确要求，不要编辑 vendored code。
- 默认 build tags 是 `sqlite_foreign_keys sqlite_json`；优先使用 Makefile targets，避免遗漏这些 tags。

### 常用命令

- 运行完整测试：`make test`
- 构建当前平台 CLI 二进制：`make host`
- 启动本地 debug server，并从磁盘读取前端资源：`make serve`
- 执行默认目标，包含测试和 host 构建：`make`

`make serve` 会使用 `debug` build tag 和 `-db local.db` 执行 `go run`。debug build 会从磁盘上的 `src/assets` 读取资源；非 debug build 会通过 `embed.FS` 嵌入 HTML、JavaScript、CSS 和 SVG 资源。

### 目录结构

- `cmd/yarr`：主应用入口，负责 flags、环境变量默认值、日志和 server 启动。
- `cmd/feed2json` 与 `cmd/readability`：辅助命令行工具。
- `src/server`：HTTP server、routing、forms、auth、Fever API、OPML 导入导出。
- `src/storage`：SQLite schema migrations 和持久化逻辑。
- `src/worker`：feed refresh、crawling、favicon lookup、后台清理。
- `src/parser`：RSS、Atom、RDF 和 JSON Feed 解析。
- `src/content`：内容抓取、readability、sanitizing、URL 和 iframe helpers。
- `src/assets`：可嵌入或 debug 时从磁盘服务的 Web UI 资源。
- `src/platform` 与 `src/systray`：OS-specific desktop/tray integration。
- `doc`：构建说明、API 说明、设计理由、changelog、平台细节。
- `etc`：打包脚本、Dockerfiles、安装脚本、图标、示例文件。

### 编码与验证

- 修改应尽量限制在相关 package 内；除非目标行为需要，否则避免跨 package 重写。
- 编辑 Go 文件后运行 `gofmt`。
- 涉及 parser、storage、router、sanitizer、OPML、URL/content 行为时，在相邻 package 中新增或更新聚焦测试。
- 最终验证优先使用 `make test`。迭代窄范围 Go 修改时，可以先运行 `go test -tags "sqlite_foreign_keys sqlite_json" ./path/...`，可行时再做更广验证。
- 不要提交生成的二进制文件、`local.db` 这类本地数据库，或 `out/` build artifacts。

## Agent skills

### Issue tracker

Issues and PRDs for this repo are tracked in GitHub Issues for `lizhian/yarr`. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the default five-label triage vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This repo uses a single-context domain documentation layout. See `docs/agents/domain.md`.
