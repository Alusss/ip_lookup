# ADR 0002: 选择 Caddy 作为 Web 服务器（同时保留 Nginx 配置作为备选）

## 状态

已采纳

## 上下文

源站需要反向代理与 TLS 终止。选项：
- **Caddy**：自动 HTTPS、内置 `trusted_proxies` 指令、配置简洁
- **Nginx**：生态成熟、`set_real_ip_from` 指令、更广泛的使用基础

## 决策

**Caddy 为主，Nginx 为备选**。两者均产出完整配置，部署时可二选一。

## 理由

1. Caddy 自动 HTTPS（ZeroSSL/LetsEncrypt）大幅降低运维成本
2. `trusted_proxies static { import /etc/caddy/cloudflare-trusted.conf }` 支持文件导入，适合 CF CIDR 自动同步
3. 配置语法比 Nginx 更简洁，减少人为错误
4. 内置 `layer4` 限流模块可做第二道防线

## 后果

- Caddy 需要 `cloudflare` DNS 插件（需自行编译或使用 xcaddy）
- 运维团队如只熟悉 Nginx，可选择 Nginx 部署
- 提供两套完整配置，部署脚本 `update-cloudflare-ip.sh` 同时产出两套片段
