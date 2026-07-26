# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`url-b64-decode`：自用 Go CLI 工具。GET 请求指定 URL，将纯 base64 响应体解码为 UTF-8 文本打印到 stdout。仅使用 Go 标准库，禁止引入第三方依赖。

设计文档在 `docs/superpowers/specs/`，实现计划在 `docs/superpowers/plans/`。

## 常用命令

```bash
go build                                 # 产出 ./url-b64-decode（已 gitignore）
go test ./... -count=1                   # 全部测试（-count=1 跳过缓存）
go test -run TestDecodeBase64UTF8 -v     # 单个测试函数
go test -run 'TestFetch/4xx' -v          # 单个子测试（子测试名中空格用下划线）
go vet ./...
```

## 架构

单模块单文件 `main.go`，三层结构，测试在 `main_test.go`：

- `decodeBase64UTF8(raw)` — 去首尾空白后解码：先试 `base64.StdEncoding`，失败再试 `base64.URLEncoding`（顺序是约定，勿改）；结果必须是合法 UTF-8。错误信息带输入前 100 字节（`head()`）。
- `fetch(client, url, retries, retryWait)` — 带重试的 GET。重试语义：总尝试次数 = retries+1；网络错误/超时/5xx 可重试，**4xx 立即失败不重试**（`doGet` 的 `retryable` 返回值承载这个区分）；负数 retries 钳制为 0。
- `main()` — flag 解析（`-timeout` 10s / `-retries` 3 / `-retry-wait` 1s）与串接。

## 约定

- 成功时 stdout 只输出解码结果（方便管道）；所有错误走 stderr。退出码：参数错误 2，运行错误 1。
- 测试用 `httptest` 起真实本地服务器并断言实际请求次数（atomic 计数器），不 mock http.Client；重试测试传 `retryWait=0` 保持快速。
- 开发流程为 TDD：先加失败测试再改实现。
