# 桑榆录（Sangyu Record）

面向线下采集人员的 AI 老年人回忆录自动化项目。当前仓库负责采集计划、音频与照片上传、持久化工作流、外部能力编排、结果校验、PDF 生成和交付。

语音转写、照片理解、共同记忆检索和回忆录写作不在本项目内实现。它们分别通过 Media、Knowledge、Agent 三类可替换的异步 HTTP Provider API 接入。`providers/mock` 只是本地测试替身，不代表真实模型、知识库或智能体的生成质量。

## 系统边界

- Go API：项目、素材、工作流状态、Provider 回调和 PDF 下载接口
- Go Worker：Provider 提交与轮询、结果规范化、节点推进和本地 PDF 渲染
- PostgreSQL：项目、素材、工作流、Provider job/attempt 和产物元数据的权威数据源
- Redis / Asynq：工作流节点与 Provider poll 队列
- MinIO：原始素材、Provider 原始响应和 PDF 产物
- Headless Chromium：HTML 转 PDF
- 微信原生小程序：工作人员采集与处理界面

外部 Provider 不能连接应用数据库或队列、不能持有对象存储凭据，也不能决定下一工作流节点。Media Provider 仅可获得单个素材的 15 分钟只读 URL；Knowledge 和 Agent Provider 只接收结构化数据。

## 本地启动

环境要求：Windows PowerShell 5.1+、Go 1.26.x、Node.js/npm 和已启动的 Docker Desktop。

一键启动后端并完成小程序全链路验收：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/miniapp-local.ps1
```

该命令会验证两条真实链路：基础单走访流程，以及包含员工隔离、两次走访分析、重复成书幂等和最终 PDF 的工作人员小程序流程。服务会保持运行，随后可直接导入微信开发者工具。

只运行后端纵向验证：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
```

已构建过镜像时可跳过构建：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1 -SkipBuild
```

脚本会启动 Compose、执行数据库迁移、运行 Go/Node/TypeScript 测试，创建测试项目并上传音频和照片。验收会确认 Media、Knowledge、Agent 三类 Provider job 均已落库，原始响应与规范化输出分别保存，七个节点只推进一次，最终项目为 `completed` 且下载内容以 `%PDF-` 开头。

手动启动：

```powershell
docker compose -f deploy/local/compose.yaml up -d --build --wait
$env:GOOSE_DRIVER = 'postgres'
$env:GOOSE_DBSTRING = 'postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
```

## Provider 配置

本机直接运行 Go 进程时，按 `.env.example` 设置以下变量：

```text
MEDIA_PROVIDER_URL=http://localhost:8090
MEDIA_PROVIDER_TOKEN=local-media-token
KNOWLEDGE_PROVIDER_URL=http://localhost:8090
KNOWLEDGE_PROVIDER_TOKEN=local-knowledge-token
AGENT_PROVIDER_URL=http://localhost:8090
AGENT_PROVIDER_TOKEN=local-agent-token
PROVIDER_ALLOWED_HOSTS=localhost:8090
PROVIDER_CALLBACK_BASE_URL=http://localhost:8080
PROVIDER_CALLBACK_SECRET=sangyu-local-callback-secret
PROVIDER_POLL_INTERVAL=2s
```

`PROVIDER_ALLOWED_HOSTS` 是逗号分隔的精确 `host:port` allowlist。Provider 回调地址为 `/v1/provider-callbacks/{kind}/{jobID}`，请求需携带：

- `X-Sangyu-Timestamp`：RFC3339 时间戳，与服务器时间差不能超过 5 分钟
- `X-Sangyu-Signature`：`HMAC-SHA256(secret, timestamp + "." + exact_body)` 的小写十六进制值

回调验签在 JSON 解码前完成；回调和轮询最终汇合到同一个幂等终态消费路径。

## 常用验证

```powershell
go test ./...
go vet ./...
npm --prefix providers/mock test
npm --prefix miniapp test
npm --prefix miniapp run typecheck
```

本地端点：

- API 健康检查：<http://localhost:8080/healthz>
- Mock Provider：<http://localhost:8090>
- MinIO API：<http://localhost:9000>
- MinIO 控制台：<http://localhost:9001>（`sangyu` / `sangyu-local-secret`）
- Chromium DevTools：<http://localhost:9222>

Provider 边界设计见 `docs/superpowers/specs/2026-07-30-external-provider-boundary-design.md`，实现计划见 `docs/superpowers/plans/2026-07-30-provider-boundary-refactor.md`。

## 小程序

1. 在微信开发者工具中导入 `miniapp/project.config.json`。
2. 开发阶段关闭“校验合法域名、web-view（业务域名）、TLS 版本以及 HTTPS 证书”。
3. 确认 `miniapp/env.ts` 指向 `http://localhost:8080`。
4. 在“工具 -> 构建 npm”生成 `miniprogram_npm`。
5. 创建项目、上传录音与照片、启动处理并下载 PDF。

完整的小程序本地联调、生产 HTTPS、微信合法域名、体验版发布和备份说明见 [`docs/deployment-miniapp.md`](docs/deployment-miniapp.md)。生产模板位于 `deploy/production`，固定使用微信登录，不暴露 PostgreSQL、Redis、MinIO、Chromium 或 Worker 端口。
