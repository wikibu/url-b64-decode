# url-b64-decode 设计文档

日期：2026-07-26
状态：已确认

## 目标

一个自用的命令行工具：传入一个 URL，GET 请求后拿到纯 base64 编码的响应体（内容为 UTF-8 文本），解码后打印到终端。

## 需求边界

- 响应体是纯 base64 字符串（不需要 JSON 解析等预处理）
- 普通 GET 请求，无需认证、自定义 Header 或代理
- 需要超时和重试控制
- 仅个人使用，无需考虑分发打包

## 语言选型

**Go**。理由：标准库完整覆盖 HTTP 客户端、base64 解码、超时控制；`go build` 产出零依赖单二进制，放入 PATH 长期可用；代码量与脚本语言相当。

已评估并排除：Python（需管理运行时/依赖环境）、Shell 脚本（错误处理能力弱，不适合长期维护）。

## 命令行接口

```
url-b64-decode [flags] <URL>

Flags:
  -timeout duration     单次请求超时（默认 10s）
  -retries int          失败重试次数（默认 3）
  -retry-wait duration  重试间隔（默认 1s）
```

## 项目结构

单模块单文件 `main.go`，仅用 Go 标准库（`flag`、`net/http`、`encoding/base64`、`unicode/utf8` 等）。fetch 和 decode 拆成独立函数，便于单元测试。

## 数据流

1. 解析命令行参数，校验 URL 参数存在
2. 发起带超时的 GET 请求
3. 校验 HTTP 状态码为 2xx
4. 读取响应体，去除首尾空白（兼容尾部换行）
5. base64 解码：先尝试标准编码（StdEncoding），失败后尝试 URL-safe 编码（URLEncoding），兼容两种常见变体
6. 校验解码结果为合法 UTF-8
7. 将结果打印到 stdout

## 错误处理

- 网络错误、超时、5xx 状态码：按 `-retries` 重试，间隔 `-retry-wait`
- 4xx 状态码：不重试，直接报错（客户端错误重试无意义）
- 无效 base64 / 非 UTF-8 内容：报错并附上原始内容前 100 个字符，便于排查
- 所有错误输出到 stderr，退出码非 0
- 成功时 stdout 只输出解码结果，便于管道使用

## 测试

- decode 函数：表驱动测试（标准 base64、URL-safe base64、无效输入、非 UTF-8 结果、带首尾空白）
- fetch/重试逻辑：用 `httptest` 起本地服务器，覆盖成功、5xx 重试后成功、4xx 不重试、超时等场景
