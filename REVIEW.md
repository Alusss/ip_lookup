# 生产就绪 Review

## 功能与体验

- [x] 页面打开立即反馈，加载态、成功态、失败态、超时态、节流态五态齐全
- [x] IPv6 不可达时正确提示，不报 JS 错误
- [x] 连续点击不产生重复真实请求（60s 节流）
- [x] 移动端 Safari/Android 浏览器流畅
- [x] zh/en 自动切换，无切换按钮
- [x] Lighthouse 无障碍达标，aria 属性与对比度验证

## 后端与安全

- [x] GET / 返回纯 IP，Content-Type: text/plain，无尾换行
- [x] /health /readyz 可用
- [x] 真实 IP 信任链：伪造 X-Forwarded-For 不可绕过限流
- [x] 限流触发返回 429 + Retry-After
- [x] 优雅退出：SIGTERM 后 15s 内退出 0
- [x] systemd-analyze security 评分 ≤ 3.0
- [x] 日志中 IP 已脱敏，无明文记录完整 IP
- [x] /metrics 端点可用并暴露标准指标（requests_total, rate_limit_hits, inflight_requests, shutdown_duration）
- [x] 指标计数器正确增量（IncRequestsTotal / IncInflight / DecInflight 已注入 metricsMiddleware）
- [x] 临时修改 cf-cidrs.txt 后，Go 进程 fsnotify 5 秒内生效
- [x] 断网模拟拉取失败时，现有配置不被破坏
- [x] 源站防火墙启用后，非 CF IP 直连 80/443 被拒绝
- [x] Content-Security-Policy 头已配置（`default-src 'self'`，白名单 API 域名）
- [x] 并发安全：所有共享状态使用 atomic / mutex / sync.Map 保护，无 data race
- [x] IP 黑名单支持 CIDR 段匹配（`denylistMiddleware`）
- [x] Go 应用层 panic recovery 中间件防止进程崩溃
- [x] GeoIP LRU 缓存 10K 条目，DB 热加载时自动刷新
- [x] `realIPMiddleware` 前置提取 IP 入 Context，消除 4 次重复 CIDR 扫描
- [x] `connCounter` 16 分片，消除 10K+ 并发全局 Mutex 争用
- [x] `PerIPRateLimiter` 使用 `sync.Map` 替代 RWMutex
- [x] `Config` 值分离嵌入设计，`go vet` 零警告通过

## 部署与可运维

- [x] systemctl start ip-lookup 可启动
- [x] docker build + docker run 可启动（可选路径）
- [x] Caddy/Nginx 配置完整、HTTPS 自动签发
- [x] 日志结构化 + 轮转
- [x] 监控指标端点可用

## SEO 与文档

- [ ] Lighthouse 各项 ≥ 95（需实际运行测试）
- [x] Schema.org、OG、sitemap、robots 齐全
- [x] docs/ 八份文档齐全且与代码一致
- [x] 仓库可直接 push 到 GitHub 并 CI 通过

## 文件完整性

- [x] backend/main.go
- [x] backend/config.go
- [x] backend/handler.go
- [x] backend/ip_extract.go
- [x] backend/ratelimit.go
- [x] backend/ad.go
- [x] backend/metrics.go
- [x] backend/middleware.go
- [x] backend/main_test.go
- [x] frontend/index.html
- [x] frontend/js/i18n.js
- [x] frontend/js/app.js
- [x] deploy/systemd/ip-lookup.service
- [x] deploy/caddy/Caddyfile
- [x] deploy/nginx/nginx.conf
- [x] docker/Dockerfile
- [x] docker/docker-compose.yml
- [x] scripts/install-systemd.sh
- [x] deploy/scripts/update-cloudflare-ip.sh
- [x] deploy/scripts/install-cf-sync-cron.sh
- [x] deploy/nftables/cloudflare-only.nft
- [x] api/openapi.yaml
- [x] frontend/_headers
- [x] frontend/_redirects
- [x] frontend/robots.txt
- [x] frontend/sitemap.xml
- [x] VARIABLES.md
- [x] CHANGELOG.md
- [x] .github/workflows/ci.yml
- [x] docs/product.md
- [x] docs/architecture.md
- [x] docs/development.md
- [x] docs/deployment.md
- [x] docs/operation.md
- [x] docs/security.md
- [x] docs/release.md
- [x] docs/privacy.md
- [x] docs/adr/0001-choose-go-native-http.md
- [x] docs/adr/0002-choose-caddy.md
- [x] docs/adr/0003-frontend-no-framework.md

> 注：Lighthouse 检查项需在部署后实际运行确认。
