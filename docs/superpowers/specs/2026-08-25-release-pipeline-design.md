# Release 发布流水线设计文档

日期：2026-08-25
状态：已确认

## 目标

为 url-b64-decode 建立自动化发布流水线：推 `v*` tag 后，GitHub Actions 自动跑测试、交叉编译全平台二进制、生成校验和与 changelog，并发布到 GitHub Releases。同时补上基础 CI，每次 push/PR 自动验证。

## 需求边界

- 目标平台：主流 64 位——linux / darwin / windows × amd64 / arm64，共 6 个二进制；不含 32 位与嵌入式架构
- 二进制需可自查版本（`-version` flag，构建时注入版本信息）
- 发布前必须自动通过测试，坏版本发不出去
- 仅个人使用：不做 Homebrew formula、deb/rpm 包、Docker 镜像、代码签名（cosign），产物校验和即够
- 本地开发流程不变，只增加"打 tag"这一步

## 方案选型

已评估并选定 **GoReleaser OSS（v2）+ GitHub Actions**。

| 方案 | 配置量 | 校验和/归档/Changelog | 外部工具 | 结论 |
|------|--------|------------------------|----------|------|
| GoReleaser + Actions | ~55 行（两个文件） | 全自动 | GoReleaser | **选定** |
| 纯 Actions 手写 matrix | ~100 行 | 全部手写维护 | 1 个第三方 action | 排除：重复造轮子 |
| 本地脚本 + gh CLI | ~50 行 | 手写 | 本机依赖 | 排除：绕过 CI 把关、不可复现、本机未装 gh |

选定理由：GoReleaser OSS 对公开仓库免费，是 Go 社区事实标准（约 5 万仓库在用）；交叉编译、归档命名、校验和、changelog 均为内置行为，配置文件短且稳定。构建工具不属于 Go 代码依赖，不违反"仅标准库"约定。

## 整体结构

新增 3 个文件，改动 1 个文件：

| 文件 | 作用 |
|------|------|
| `main.go`（改） | 加 `version` / `commit` / `date` 变量与 `-version` flag |
| `.goreleaser.yaml`（新） | 发布配置：构建矩阵、归档、校验和、changelog |
| `.github/workflows/ci.yml`（新） | push/PR 时跑 `go vet` + `go test` + `go build` |
| `.github/workflows/release.yml`（新） | `v*` tag 触发：先测试，通过后用 GoReleaser 构建发布 |

发布流程：

1. 开发者改代码、合并到 main
2. 打 `v*` tag 并推送
3. `release.yml` 触发：job 1（test）跑完整测试套件；失败则整个发布中止
4. job 2（goreleaser，`needs: test`）：交叉编译 6 个二进制 → 打包归档 → 生成 checksums.txt → 从提交记录生成 changelog → 创建 GitHub Release

## 版本注入（-version）

`main.go` 新增包级变量，构建时由 `-ldflags` 注入：

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

行为约定：

- `-version` 命中时打印版本行到 **stdout**，退出码 **0**
- 不要求 URL 参数；优先级高于一切（同时给了 URL 也只打印版本）
- 版本行格式：`url-b64-decode <版本> (commit <短哈希>, built <日期>)`
  - 发布构建：`url-b64-decode v1.0.0 (commit 6a07da1, built 2026-08-25T10:00:00Z)`
  - 本地未注入：`url-b64-decode dev (commit none, built unknown)`
- 格式化抽为纯函数 `versionLine(version, commit, date string) string`，便于单元测试；`main()` 中的 flag 分支不做进程级测试

测试（按项目 TDD 约定，先写测试）：

- `main_test.go` 加表驱动测试 `TestVersionLine`：正式值、dev 默认值两类用例
- 顺序：加测试跑红 → 实现跑绿 → `go vet` + 全量测试

## CI workflow（ci.yml）

- 触发：`push` 到 main、`pull_request`
- 权限：`contents: read`
- 步骤：`actions/checkout` → `actions/setup-go`（`go-version-file: go.mod`）→ `go vet ./...` → `go test ./... -count=1` → `go build .`

## Release workflow（release.yml）

- 触发：`push: tags: ["v*"]`
- 权限：workflow 级 `contents: read`，`goreleaser` job 单独 `contents: write`（最小权限原则）
- job `test`：与 ci.yml 相同的 vet/test/build 三连
- job `goreleaser`（`needs: test`）：
  - `actions/checkout`，`fetch-depth: 0`（changelog 需要完整历史）
  - `actions/setup-go`（`go-version-file: go.mod`）
  - `goreleaser/goreleaser-action`，`distribution: goreleaser`、`version: "~> v2"`、`args: release --clean`
  - `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`

action 版本以实施时的最新稳定版为准（设计时为 checkout / setup-go v7、goreleaser-action v7）。

## GoReleaser 配置（.goreleaser.yaml）

```yaml
version: 2

builds:
  - main: .
    binary: url-b64-decode
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w
      - -X main.version={{.Tag}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]

checksum:
  name_template: checksums.txt
```

要点：

- `-trimpath` 可复现构建；`-s -w` 去符号表减小体积
- 版本注入用 `{{.Tag}}`（含 `v` 前缀，如 `v1.0.0`），`-version` 直接打印，无需对 `dev` 默认值做特殊处理；归档文件名仍用 `{{.Version}}`（不带 `v`）
- changelog 用 v2 默认行为（按 tag 区间自动生成）

## 发布产物

以 v1.0.0 为例：

```
url-b64-decode_1.0.0_linux_amd64.tar.gz
url-b64-decode_1.0.0_linux_arm64.tar.gz
url-b64-decode_1.0.0_darwin_amd64.tar.gz
url-b64-decode_1.0.0_darwin_arm64.tar.gz
url-b64-decode_1.0.0_windows_amd64.zip
url-b64-decode_1.0.0_windows_arm64.zip
checksums.txt
```

Release notes 为自动生成的 changelog（得益于现有 `feat:` / `fix:` / `docs:` 提交规范）。

## 错误处理与失败恢复

- 测试失败：发布中止，不产生 Release
- GoReleaser 中途失败（如 GitHub API 抖动）：重跑 workflow 即可，`--clean` 保证幂等
- Release 创建到一半需要重来：删 tag（`git push origin :refs/tags/vX.Y.Z`）重打，无不可恢复状态
- 非 `v` 前缀的 tag 不触发发布（有意为之）

## 测试与验收

1. 本地：`go test ./... -count=1` 全绿、`go vet ./...` 通过、`go build` 后运行 `-version` 确认输出 dev 版本行
2. 推送代码后打第一个 tag，观察 ci.yml 与 release.yml 全绿
3. 检查 Release 页面：6 个归档 + checksums.txt + changelog 齐全
4. 下载当前平台产物，核对校验和，解包运行 `-version` 确认注入的版本号

## 待定项

- 首发版本号：建议 `v1.0.0`（工具已稳定在用）；如需发布流程试运行期可用 `v0.1.0`。实施时确定。
