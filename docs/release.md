# 发布与 CI/CD

## 版本规范

遵循 [SemVer](https://semver.org/)：

- **主版本**：不兼容的 API 修改
- **次版本**：向下兼容的功能新增
- **修订号**：向下兼容的问题修复

版本号写入 Go 编译参数：

```bash
go build -ldflags="-X main.version=v1.2.3" -o ip-lookup .
```

## CI/CD

### GitHub Actions

`.github/workflows/ci.yml`，触发条件 `on: [pull_request]`：

1. Checkout 代码
2. 安装 Go 1.23
3. 执行 `bash scripts/verify.sh`

失败阻断合并。

### 验证脚本

`scripts/verify.sh` 包含：
1. `go test ./...` 全绿
2. `golangci-lint run`（如安装）
3. `docker build -t ip-lookup:test .`（如安装 Docker）
4. `systemd-analyze security deploy/systemd/ip-lookup.service` 评分 ≤ 3.0
5. 文件完整性检查（检查所有关键文件是否存在）

---

## 灰度发布策略

### 蓝绿部署

1. 新版本在备用环境部署
2. 修改 Cloudflare Load Balancer 或 DNS 切流
3. 观察 30 分钟（错误率、延迟、限流命中率）
4. 回滚：DNS 切回旧环境

### 金丝雀发布

1. 新版本先 10% 流量
2. 观察 30 分钟
3. 逐步增加至 100%

---

## 回滚机制

- **DNS 回滚**：Cloudflare DNS 指向旧版本
- **Docker 回滚**：`docker-compose up -d` 指定上一版本 tag
- **systemd 回滚**：替换二进制后 `systemctl restart ip-lookup`

```bash
# systemd 回滚
cp /usr/local/bin/ip-lookup.bak /usr/local/bin/ip-lookup
systemctl restart ip-lookup
```

## CHANGELOG 规范

每个版本包含：

```
## [1.0.1] - 2026-07-22

### Fixed
- 修复 X-Forwarded-For 解析空指针
- 修复 IPv6 监听地址格式

### Changed
- 提升默认单 IP 限流至 10 req/min
```
