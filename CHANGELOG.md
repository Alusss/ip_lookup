# Changelog

## [0.5.0] - 2026-07-23

### Added

- **指标精细化**：按状态码分组计数（2xx/3xx/4xx/5xx）、延迟直方图桶（11 桶）、延迟累计值、运行时间
- **自监控告警引擎**（`backend/monitor.go`）：内置周期检查错误率/P99延迟/限流命中率，超阈值时日志 WARN + Webhook 推送
- Webhook 双格式支持：`generic`（通用 JSON）和 `dingtalk`（钉钉消息）
- 告警冷却机制：同一指标在冷却期内不重复推送，避免告警风暴
- 告警总开关（`monitoring.enabled`），默认关闭，环境变量可覆盖
- 请求 ID 入日志：`loggingMiddleware` 每条日志记录 `request_id`；`logInfo/logWarn/logError` 从 Context 提取并输出
- `MemoryMax=256M` 到 systemd 单元，防止极端场景 OOM
- `CircuitBreaker` 通用熔断器模块（Closed/Open/HalfOpen 状态机），为未来外部依赖预留
- Shell 脚本重试机制：`update-geoip.sh` 和 `update-cloudflare-ip.sh` 下载失败自动重试 3 次，指数退避
- E2E 集成测试：`TestFullMiddlewareChain` 覆盖完整 11 层中间件链的 HTTP 集成验证
- `docs/future-plan.md`：水平扩缩容与健康检查扩展的未来规划

### Changed

- 指标采集点从 `metricsMiddleware` 移至 `loggingMiddleware`：确保被外层中间件（denylist/methodCheck）拒绝的请求也正确计入状态码统计（修复 403/405 等误标为 200 的 bug）
- `metricsMiddleware` 简化为仅跟踪 inflight 连接数
- `config.yaml` 新增 `monitoring` 配置段
- `.env.example` 新增 `MONITORING_*` 环境变量说明

### Fixed

- [BUG] `computeP99` 从高到低迭代方向错误：99% 分位应从低延迟桶开始累计，修复后正确反映真实 P99
- [BUG] `metricsMiddleware` 自建 `responseWriter` 捕获状态码：由于外层中间件（denylist/methodCheck）在其之前已写响应，`rw.statusCode` 始终为 200，与实际响应码不一致。修复后将指标采集移至 `loggingMiddleware`，它是链中最外层，捕获的状态码真实可靠

## [0.4.0] - 2026-07-22

### Added
- GeoIP LRU 缓存（10K 条目）：重复 IP 查询 ~200μs → ~0.1μs，命中率 ~80%
- `realIPMiddleware`：前置提取真实 IP，通过 Context 传递至后续中间件和处理函数
- 前端广告配置合并：`/ad-config` 独立请求取消，广告信息通过主 API 响应头 `X-Ad-*` 传递，减少 1 个 HTTP 请求

### Changed
- `connCounter` 分片化：16 分片替代单全局 `sync.Mutex`，消除 10K+ 并发场景争用
- `PerIPRateLimiter` 改用 `sync.Map`：`LoadOrStore` 原子语义替代 `RWMutex` 双重检查锁定
- 全局速率默认值提升：1000 req/s → 5000 req/s
- `Config` 结构体重构：可热加载字段提取为 `ConfigValues` 嵌入，消除 `*cfg = *newCfg` 的 mutex 拷贝问题
- 前端：移除 `fetchAdConfig()`，广告数据从 `fetchIp()` 响应头读取

### Fixed
- [并发安全] `Config.reload()` 中 `*cfg = *newCfg` 拷贝 `sync.RWMutex` — 分离为 `ConfigValues` 安全拷贝
- [质量] 移除 `formatApiResponse`、`formatWebAdResponse` 死代码
- [质量] 移除 `main_test.go` 中未使用的 `sync/atomic`、`golang.org/x/time/rate` 导入

## [0.3.0] - 2026-07-22

### Added
- `Content-Security-Policy` 头（`_headers`），覆盖 `default-src/connect-src/style-src/script-src/frame-ancestors`
- `metricsMiddleware`：在中间件链中注入 `IncRequestsTotal` / `IncInflight` / `DecInflight`，修复指标计数器从未被增量的缺陷
- 黑名单 CIDR 支持：`denylistMiddleware` 中 CIDR 段匹配逻辑已修复并生效

### Changed
- `ready` 状态标志从裸 `bool` 升级为 `sync/atomic.Bool`，消除多 goroutine 间 data race
- `connCounter` 重构为 `TryAcquire` / `Release` 原子操作，消除 `IsOverLimit` + `Inc` 之间的 TOCTOU 竞态
- `Config.reload()` 热重载时保留 `watcher`/`onReload` 字段，回调函数在锁外安全调用
- `parseCIDRList` / `parseList` 合并为 `parseCommaList`，消除 100% 重复逻辑
- `bodyRejectionMiddleware` 修复为先检查 `ContentLength` 再关闭 `Body`
- `json.NewEncoder(w).Encode(resp)` 返回值增加 error check 和日志记录
- `os.MkdirAll` 增加错误检查和 fallback 日志
- `i18n.js` 全局变量封装到 IIFE，仅暴露必要接口
- `IPExtractor` 移除未使用的 `watcher` 字段

### Fixed
- [并发安全] `ready` 状态读写无同步 — 改用 `atomic.Bool`
- [并发安全] `connCounter` TOCTOU 竞态 — 原子化 `TryAcquire/Release`
- [并发安全] `Config.reload()` watcher/onReload 被 struct 整体覆盖丢失
- [功能] `ispDBPath()` 返回 City 路径而非 ISP 路径 — 现在正确映射 `-ISP.mmdb`
- [功能] `bodyRejectionMiddleware` 先关 Body 再验 ContentLength — 拒绝逻辑形同虚设
- [功能] `denylistMiddleware` CIDR 检测条件 `HasSuffix(denied, "/")` 永远为 false — 改为 `Contains`
- [可观测] `IncRequestsTotal` / `IncInflight` / `DecInflight` 从未被调用 — 注入 metricsMiddleware
- [健壮性] `json.Encode`、`os.MkdirAll` 返回值未检查 — 增加 error check 和 log
- [质量] `parseCIDRList` / `parseList` 完全相同 — 合并为 `parseCommaList`
- [质量] `IPExtractor.watcher` 字段赋值但不读 — 移除
- [质量] `i18n.js` 全局变量污染 — IIFE 封装
- [安全] CSP 头缺失 — 添加正向安全策略
- [测试] `TestDetectLanguage` 调用 `detectLanguage()` 缺少 Config receiver — 修复

## [0.2.0] - 2026-07-22

### Added
- 广告配置拆分：`api_ad_*`（API 接口广告）与 `web_ad_*`（前端页面顶栏广告）独立控制
- 后端 `GET /ad-config` 端点，前端动态拉取广告配置实现完全可配置化
- 后端双栈监听：同时监听 IPv4 `127.0.0.1:8080` 和 IPv6 `[::1]:8081`
- JSON API 支持：携带 `Accept: application/json` 时返回结构化 JSON（含 IP、版本、GeoIP 信息）
- GeoIP 集成（GeoLite2）：免费开源地理位置数据库，fsnotify 热加载，自动更新脚本
- 前端 IPv4 + IPv6 并行自动检测，页面加载时同时展示双栈结果
- 知识页：`/docs/what-is-ipv6`、`/docs/ipv6-test-guide`（SEO 长尾内容）
- `/privacy` 隐私政策页面
- 错误信息精细化（错误消息统一管理，按 HTTP 状态码分化）
- `robots.txt` 移至仓库根目录（修复 Cloudflare Pages 发布位置错误）
- GeoIP 自动更新脚本 `deploy/scripts/update-geoip.sh`
- 新增后端模块：`geoip.go`（GeoIP 查询与热加载）、`errors.go`（错误消息集中管理）

### Changed
- 后端配置字段重构：`ad_enabled` → `api_ad_enabled`，新增 `web_ad_enabled` 系列
- `.env.example` / `VARIABLES.md` 全面更新
- 前端自动检测 IPv6（无需手动点击按钮），失败时展示无 IPv6 提示
- `openapi.yaml` 更新至 v1.1（新增 `/ad-config`、JSON schema）

### Fixed
- 后端仅监听 IPv4 的问题，现在双栈同时监听
- `robots.txt` 位置错误（`frontend/` → 仓库根目录）
