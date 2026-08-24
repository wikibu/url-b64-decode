# Release 发布流水线实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 url-b64-decode 建立自动化发布流水线:推 `v*` tag 后自动测试、交叉编译 6 平台二进制、生成校验和与 changelog 并发布到 GitHub Releases,同时补上基础 CI。

**Architecture:** 三块组成——① `main.go` 加 `-version` flag,构建时由 `-ldflags` 注入版本信息;② `.goreleaser.yaml` 描述构建矩阵与产物格式;③ 两个 GitHub Actions workflow:ci.yml(push/PR 跑测试)和 release.yml(tag 触发,先测试后发布)。发布决策全部由 GoReleaser OSS v2 承担,workflow 只负责触发与注入 GITHUB_TOKEN。

**Tech Stack:** Go 1.26(仅标准库)、GoReleaser v2(OSS)、GitHub Actions(checkout/setup-go v7、goreleaser-action v7)。

**Spec:** `docs/superpowers/specs/2026-08-25-release-pipeline-design.md`

## Global Constraints

- Go 1.26,仅标准库——不引入任何 Go 第三方依赖
- stdout 纪律:程序正常输出只进 stdout,错误进 stderr;`-version` 输出到 stdout 且退出码 0
- 退出码约定:0 成功 / 1 运行错误 / 2 参数错误
- TDD:先写失败测试,再写实现
- 提交信息用 conventional commits(`feat:` / `fix:` / `docs:` / `chore:` / `ci:`),正文末尾加 `Co-Authored-By: Claude <noreply@anthropic.com>` trailer
- 平台矩阵固定为 linux / darwin / windows × amd64 / arm64,共 6 个二进制;`CGO_ENABLED=0`
- 工作环境:Windows + Git Bash,命令用 POSIX shell 语法
- 仓库:`git@github.com:wikibu/url-b64-decode.git`,主分支 `main`,直接推 main(个人仓库,无 PR 流程)

---

### Task 1: `-version` flag(TDD)与文档更新

**Files:**
- Modify: `main.go`(在 `func main()` 上方加变量与函数;`main()` 内加 flag 与分支)
- Modify: `main_test.go`(文件末尾追加测试)
- Modify: `README.md`(使用方法 flags 块、新增安装小节)

**Interfaces:**
- Consumes: 无
- Produces: 包级变量 `version`、`commit`、`date`(Task 2 的 ldflags 会注入 `main.version` / `main.commit` / `main.date`,名字必须一字不差)

- [ ] **Step 1: 写失败测试**

在 `main_test.go` 文件末尾追加:

```go
func TestVersionLine(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{"release build", "v1.0.0", "6a07da1", "2026-08-25T10:00:00Z",
			"url-b64-decode v1.0.0 (commit 6a07da1, built 2026-08-25T10:00:00Z)"},
		{"dev build", "dev", "none", "unknown",
			"url-b64-decode dev (commit none, built unknown)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionLine(tt.version, tt.commit, tt.date); got != tt.want {
				t.Fatalf("versionLine() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

运行:`go test ./... -count=1`
预期:编译失败,报 `undefined: versionLine`

- [ ] **Step 3: 最小实现**

在 `main.go` 中 `func main()` 上方加入:

```go
// version, commit, and date are set via -ldflags at release build time;
// the defaults identify a local development build.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionLine formats the -version output line.
func versionLine(version, commit, date string) string {
	return fmt.Sprintf("url-b64-decode %s (commit %s, built %s)", version, commit, date)
}
```

在 `main()` 内做两处修改。① 与现有三个 flag 定义并列处加:

```go
	showVersion := flag.Bool("version", false, "show version and exit")
```

② `flag.Parse()` 之后、`if flag.NArg() != 1` 检查之前加:

```go
	if *showVersion {
		fmt.Println(versionLine(version, commit, date))
		os.Exit(0)
	}
```

- [ ] **Step 4: 运行测试确认通过**

运行:`go test ./... -count=1`
预期:全部 PASS(包括原有 TestDecodeBase64UTF8、TestFetch)

- [ ] **Step 5: 静态检查与行为验证**

运行:`go vet ./...`
预期:无输出

运行:`go run . -version`
预期输出:`url-b64-decode dev (commit none, built unknown)`

运行:`go run -ldflags "-X main.version=v9.9.9 -X main.commit=abc1234 -X main.date=2026-01-01T00:00:00Z" . -version`
预期输出:`url-b64-decode v9.9.9 (commit abc1234, built 2026-01-01T00:00:00Z)`

- [ ] **Step 6: 更新 README**

① `使用方法` 小节的 Flags 块末尾加一行(与其他行对齐):

```
  -version              显示版本信息并退出
```

② 在 `环境要求` 与 `构建` 小节之间插入:

```markdown
## 安装

预编译二进制见 [GitHub Releases](https://github.com/wikibu/url-b64-decode/releases)：下载对应平台/架构的压缩包（Linux、macOS 为 `.tar.gz`，Windows 为 `.zip`），解包后将二进制放入 PATH 即可。
```

- [ ] **Step 7: 提交**

```bash
git add main.go main_test.go README.md
git commit -m "$(cat <<'EOF'
feat: add -version flag with build-time version injection

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: GoReleaser 配置

**Files:**
- Create: `.goreleaser.yaml`

**Interfaces:**
- Consumes: Task 1 产出的 `main.version` / `main.commit` / `main.date` 变量名
- Produces: `.goreleaser.yaml`,Task 3 的 release.yml 通过 goreleaser-action 调用它

- [ ] **Step 1: 写配置文件**

创建 `.goreleaser.yaml`:

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
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: checksums.txt
```

- [ ] **Step 2: 本地校验配置**

运行:`go run github.com/goreleaser/goreleaser/v2@latest check`
预期:退出码 0,无错误(首次运行会编译 GoReleaser,需几分钟,输出可含 `• checking config` 日志行)

若因网络等原因无法运行:记录"跳过本地校验,由首次 tag 发布在 CI 中验证",继续下一步。

- [ ] **Step 3: 提交**

```bash
git add .goreleaser.yaml
git commit -m "$(cat <<'EOF'
chore: add GoReleaser config for multi-platform releases

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: GitHub Actions workflows 并推送验证

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: Task 2 的 `.goreleaser.yaml`(goreleaser-action 默认读取仓库根目录的该文件)
- Produces: 可用的 CI 与发布流水线;Task 4 依赖其全绿

- [ ] **Step 1: 写 ci.yml**

创建 `.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./... -count=1
      - name: Build
        run: go build .
```

- [ ] **Step 2: 写 release.yml**

创建 `.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./... -count=1
      - name: Build
        run: go build .

  goreleaser:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "$(cat <<'EOF'
ci: add test and release workflows

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: 推送并验证 CI**

运行:`git push origin main`

打开(或抓取) <https://github.com/wikibu/url-b64-decode/actions>:
预期:ci 工作流运行并全绿。若失败,读日志修复后重新提交推送,直到绿。

---

### Task 4: 首次发布与端到端验收

**Files:** 无代码改动(纯发布操作与验收)

**Interfaces:**
- Consumes: Task 1–3 全部产物;要求 main 上 CI 为绿

- [ ] **Step 1: 与用户确认首发版本号**

问用户:`v1.0.0`(设计文档推荐,工具已稳定在用)还是 `v0.1.0`(发布流程试运行)?以下命令以 `v1.0.0` 为例,按用户选择替换。

- [ ] **Step 2: 打 tag 并推送**

```bash
git tag v1.0.0
git push origin v1.0.0
```

- [ ] **Step 3: 观察 release 工作流**

打开 <https://github.com/wikibu/url-b64-decode/actions>:
预期:test job 先跑,绿后 goreleaser job 跑,全绿。
若中途失败:修复后重跑 workflow(`--clean` 幂等);若 Release 已创建一半,删 tag 重来:

```bash
git tag -d v1.0.0 && git push origin :refs/tags/v1.0.0
```

- [ ] **Step 4: 验收 Release 页面**

打开 <https://github.com/wikibu/url-b64-decode/releases/tag/v1.0.0>,确认:

- 6 个归档齐全:`url-b64-decode_1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz`、`url-b64-decode_1.0.0_windows_{amd64,arm64}.zip`
- `checksums.txt` 存在
- Release notes 含自动生成的 changelog

- [ ] **Step 5: 下载、校验、运行**

以 windows_amd64 为例(按执行环境替换平台):

```bash
curl -LO https://github.com/wikibu/url-b64-decode/releases/download/v1.0.0/url-b64-decode_1.0.0_windows_amd64.zip
curl -LO https://github.com/wikibu/url-b64-decode/releases/download/v1.0.0/checksums.txt
grep url-b64-decode_1.0.0_windows_amd64.zip checksums.txt | sha256sum -c -
```

预期:输出 `OK`。

解包(资源管理器或 `tar -xf url-b64-decode_1.0.0_windows_amd64.zip` 等可用工具),运行:

```
.\url-b64-decode.exe -version
```

预期输出:`url-b64-decode v1.0.0 (commit <短哈希>, built <日期>)`——版本号为 `v1.0.0` 而非 `dev`,即注入成功。
