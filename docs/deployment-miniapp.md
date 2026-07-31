# 小程序与生产部署

## 本地联调

在 Windows PowerShell 中执行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/miniapp-local.ps1
```

脚本会检查 Go、Node/npm、Docker Desktop，构建并启动本地 Compose，执行迁移、Go 测试、Provider 测试、小程序测试与类型检查，并跑完单走访基础链路和双走访工作人员链路。服务会保持运行。

随后在微信开发者工具中导入 `D:\Sangyu-record\miniapp\project.config.json`，选择“工具 -> 构建 npm”。仅在本地开发时关闭合法域名和 HTTPS 校验。开发登录固定为“本地采集员”。

## 生产前提

准备一台可运行 Docker Compose 的 Linux 服务器，并将两个域名解析到服务器：

- `API_DOMAIN`：Go API、微信登录和 Provider 回调。
- `FILES_DOMAIN`：音频、照片上传和 PDF 下载的私有签名链接。

服务器需要允许入站 TCP `80/443` 和 UDP `443`。PostgreSQL、Redis、MinIO、Chromium、API 与 Worker 不直接发布端口。Caddy 自动申请和续期证书。

## 配置与启动

```bash
cp deploy/production/.env.production.example deploy/production/.env.production
# 编辑全部 CHANGE_ME、域名、微信凭据、员工访问策略和 Provider 配置
docker compose --env-file deploy/production/.env.production \
  -f deploy/production/compose.yaml config
docker compose --env-file deploy/production/.env.production \
  -f deploy/production/compose.yaml up -d --build --wait
```

`migrate` 容器在 API 和 Worker 启动前执行 Goose。生产固定使用 `AUTH_MODE=wechat`。默认只有 `STAFF_OPENID_ALLOWLIST` 中的员工可登录；体验阶段可设置 `STAFF_AUTO_ENROLL=true`，使通过微信体验成员审批的用户在首次登录时自动建立独立工作人员账号。正式公开发布前应恢复为 `false`，并使用明确的工作人员白名单或后续管理后台。`PROVIDER_ALLOWED_HOSTS` 必须是 Provider 的精确 `host:port` 列表；回调地址为 `https://API_DOMAIN/v1/provider-callbacks/{kind}/{jobID}`。

`POSTGRES_PASSWORD` 与 `REDIS_PASSWORD` 会进入连接 URI，只能使用 `A-Z a-z 0-9 . _ ~ -`。`FILES_DOMAIN` 仅放行签名对象上传/下载所需的 GET、HEAD、PUT，并拒绝 MinIO 管理、健康和指标路径；MinIO 控制台不对公网开放。

API 与 Worker 使用 `S3_ACCESS_KEY/S3_SECRET_KEY` 对应的 Bucket 限定账号；`MINIO_ROOT_USER/MINIO_ROOT_PASSWORD` 仅提供给 MinIO 和一次性初始化容器。轮换业务账号时，应先更新生产环境文件并重建服务，再用 MinIO 管理工具删除旧账号。

## 微信后台域名

在微信公众平台的小程序服务器域名中配置：

- request 合法域名：`https://API_DOMAIN`
- uploadFile 合法域名：`https://FILES_DOMAIN`
- downloadFile 合法域名：`https://FILES_DOMAIN`

本项目上传使用 `wx.request` PUT 到签名地址，仍需确保 `FILES_DOMAIN` 同时处于 request 合法域名允许范围。生产发布前将 `miniapp/env.ts` 的 `API_BASE_URL` 改为 `https://API_DOMAIN`，将 `AUTH_MODE` 改为 `wechat`，并将 `miniapp/project.config.json` 的 `appid` 替换为正式小程序 AppID。

在微信开发者工具中构建 npm、真机预览并验证登录、录音权限、照片选择、微信文件导入、草稿恢复、走访报告、成书进度和 PDF 打开，然后上传体验版，完成成员验收后再提交审核。

## 运维与备份

- 每日备份 PostgreSQL，并定期做恢复演练。
- 对 MinIO 数据卷做增量备份；数据库与对象存储必须在同一恢复点范围内。
- 备份 `deploy/production/.env.production` 到受控密钥系统，不要提交到 Git。
- 上线后检查 `https://API_DOMAIN/healthz`、Caddy 证书、Worker 日志、Provider 回调验签和失败队列。
- 升级前先备份，再运行同一条 `docker compose ... up -d --build --wait`；迁移容器会幂等执行未应用迁移。
