# url-b64-decode

一个自用的 Go 命令行工具：GET 请求指定 URL，将**纯 base64 编码的响应体**解码为 UTF-8 文本并打印到 stdout。仅使用 Go 标准库，零第三方依赖。

## 功能特性

- 自动兼容两种 base64 变体：先尝试标准编码（`StdEncoding`），失败后再试 URL-safe 编码（`URLEncoding`）
- 自动去除响应体首尾空白（兼容尾部换行）
- 校验解码结果为合法 UTF-8，避免输出乱码二进制
- 带超时控制与失败重试
- 成功时 stdout 只输出解码结果，方便管道（`|`）串接；所有错误信息走 stderr

## 环境要求

- Go 1.26+（仅标准库，无其他依赖）

## 构建

```bash
go build          # 产出 ./url-b64-decode 二进制
```

## 使用方法

```
url-b64-decode [flags] <URL>

Flags:
  -timeout duration     单次请求超时（默认 10s）
  -retries int          失败重试次数（默认 3）
  -retry-wait duration  重试间隔（默认 1s）
```

示例：

```bash
# 基本用法
url-b64-decode https://example.com/data.b64

# 管道使用（成功时 stdout 只有解码结果）
url-b64-decode https://example.com/config.b64 | jq .

# 自定义超时与重试
url-b64-decode -timeout 30s -retries 5 -retry-wait 2s https://example.com/data.b64
```

## 重试语义

| 情况 | 行为 |
|------|------|
| 网络错误、超时、5xx | 重试，总尝试次数 = `-retries` + 1，间隔 `-retry-wait` |
| 4xx | **立即失败，不重试**（客户端错误重试无意义） |
| 负数 `-retries` | 钳制为 0（只请求一次） |

## 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 1 | 运行错误（请求失败、无效 base64、非 UTF-8 内容等） |
| 2 | 参数错误（如缺少 URL） |

解码失败的错误信息会附带原始输入的前 100 个字符，便于排查。

## 项目结构

单模块单文件实现，测试与实现分离：

```
├── main.go           # 实现（decodeBase64UTF8 / fetch / main 三层）
├── main_test.go      # 测试（httptest 真实本地服务器，不 mock http.Client）
├── go.mod            # 模块 url-b64-decode
└── docs/superpowers/
    ├── specs/        # 设计文档
    └── plans/        # 实现计划
```

核心函数：

- `decodeBase64UTF8(raw)` — 去首尾空白后解码（先 Std 后 URL-safe），并校验 UTF-8 合法性
- `fetch(client, url, retries, retryWait)` — 带重试的 GET，返回 2xx 响应体
- `main()` — flag 解析与串接

## 开发

开发流程为 TDD：先加失败测试，再改实现。

```bash
go test ./... -count=1                   # 运行全部测试（-count=1 跳过缓存）
go test -run TestDecodeBase64UTF8 -v     # 运行单个测试函数
go test -run 'TestFetch/4xx' -v          # 运行单个子测试（子测试名中空格用下划线）
go vet ./...                             # 静态检查
```

## 文档

- 设计文档：[docs/superpowers/specs/2026-07-26-url-b64-decode-design.md](docs/superpowers/specs/2026-07-26-url-b64-decode-design.md)
- 实现计划：[docs/superpowers/plans/2026-07-26-url-b64-decode.md](docs/superpowers/plans/2026-07-26-url-b64-decode.md)
