# 开发指南

## 前置要求

- Go 1.23+

## 快速开始

```bash
git clone https://github.com/{{ORG}}/ip-lookup
cd ip-lookup

# 后端开发
cd backend
cp config.yaml config.local.yaml
# 编辑配置...
go run . -config config.local.yaml

# 测试
go test ./... -count=1 -v

# 前端（纯静态，直接浏览器打开或用 serve）
cd ../frontend
npx serve .
```

## Makefile 命令

```bash
make build    # 构建二进制
make test     # 运行测试
make run      # 构建并运行
make clean    # 清理
make lint     # golangci-lint
make docker   # Docker 镜像
make verify   # 完整验证
```

## 项目结构

```
ip-lookup/
├── backend/            # Go 后端
│   ├── main.go         # 入口、双栈监听、优雅退出
│   ├── config.go       # 配置管理（YAML + 环境变量 + fsnotify 热加载）
│   ├── cache.go        # 通用并发 LRU 缓存（GeoIP 查询缓存底层）
│   ├── handler.go      # HTTP 处理器（/ /all /ad-config /health /readyz /metrics）
│   ├── ip_extract.go   # IP 提取与信任链（CF CIDR 热加载 + 兜底定时器 + cf_only 校验）
│   ├── ratelimit.go    # 限流（Token Bucket，单 IP + 全局，可开关/选模式）
│   ├── ad.go           # 广告逻辑（API 广告 + Web 广告配置）
│   ├── geoip.go        # GeoIP 查询（GeoLite2 City+ASN + fsnotify 热加载 + 配置热重载）
│   ├── metrics.go      # Prometheus 指标（含状态码/延迟分布/运行时间）
│   ├── middleware.go   # HTTP 中间件链（requestID → realIP → recovery → securityHeaders → metrics(连接数) → methodCheck → bodyRejection → denylist → connLimit → cors → logging(计数+延迟+状态码)）
│   ├── errors.go       # 错误消息集中管理
│   ├── monitor.go      # 自监控告警引擎（阈值检查 + Webhook 推送）
│   ├── main_test.go    # 单元测试 + E2E 集成测试
│   └── config.yaml     # 默认配置
├── frontend/           # 前端静态文件
│   ├── index.html      # 主页面（IPv4 + IPv6 并行展示）
│   ├── privacy.html    # 隐私政策
│   ├── js/
│   │   ├── i18n.js     # 国际化
│   │   └── app.js      # 应用逻辑（状态机、节流、双栈检测、广告响应头解析）
│   ├── docs/
│   │   ├── what-is-ipv6.html      # 知识页
│   │   └── ipv6-test-guide.html   # 知识页
│   └── sitemap.xml
├── deploy/             # 部署配置
│   ├── systemd/        # systemd service（安全加固）
│   ├── caddy/          # Caddyfile（自动 HTTPS）
│   ├── nginx/          # nginx.conf（备选）
│   ├── scripts/        # 部署脚本
│   └── nftables/       # 防火墙规则
├── docker/             # Docker 配置
├── docs/               # 文档
│   ├── adr/            # 架构决策记录
│   ├── architecture.md
│   ├── deployment.md
│   ├── development.md
│   ├── operation.md
│   ├── security.md
│   ├── seo.md
│   ├── release.md
│   ├── product.md
│   ├── privacy.md
│   └── future-plan.md
├── api/                # API 规范
│   └── openapi.yaml
├── scripts/            # 工具脚本
│   ├── install-systemd.sh
│   └── verify.sh
├── _headers            # Cloudflare Pages 头配置
├── _redirects          # Cloudflare Pages 重定向
├── robots.txt          # SEO
└── Makefile
```

## 贡献规范

1. 分支从 `main` 创建
2. 提交信息遵循 Conventional Commits
3. 提交前运行 `make verify`
4. PR 需通过 CI
