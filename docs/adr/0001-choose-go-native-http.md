# ADR 0001: 使用 Go 标准库 net/http

## 状态

已采纳

## 上下文

项目需要高性能、低资源占用的 HTTP 服务器来处理 IP 查询请求。主要需求：
- 极简路由（仅 `/`, `/health`, `/readyz`, `/metrics`）
- 显式超时控制 (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`)
- 优雅退出 (`Shutdown`)
- 无需框架中间件

## 决策

使用 Go 标准库 `net/http` 的 `http.Server`，不接受第三方 HTTP 框架。

## 理由

1. 路由极简，不需要 `gin`/`echo`/`chi` 等框架
2. `http.Server` 原生支持 `ReadTimeout`/`WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes`
3. `srv.Shutdown()` 原生支持优雅退出
4. 减少依赖 == 减少 CVE 攻击面 == 更小的二进制
5. `log/slog` 标准库已满足结构化日志需求

## 后果

- 需要自行实现限流中间件（Token Bucket）
- 需要自行实现 metrics 收集（`expvar` 或简单的 Prometheus 计数器）
- 代码量略增但可控
