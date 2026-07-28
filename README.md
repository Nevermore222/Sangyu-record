# 桑榆录（Sangyu Record）

面向线下采集人员的 AI 老年人回忆录自动化项目。当前仓库实现第一阶段基础闭环：创建老人项目、生成采集计划、上传录音和照片、执行可持久化工作流，并输出带内部来源映射的 PDF。

> 当前语音转写、照片理解、记忆整理与写作均为确定性模拟实现，用于验证工程流程，不代表真实 AI 生成质量。

## 技术结构

- Go API 与异步 Worker：业务状态、工作流编排、对象存储和 PDF 产物
- PostgreSQL：项目、物料、工作流节点与产物元数据
- Redis / Asynq：可重试的异步任务队列
- MinIO：不可变原始物料与生成文件
- Headless Chromium：HTML 到 PDF
- Node.js Skill Runner：隔离的多语言 Skill HTTP 契约示例
- 微信原生小程序：工作人员采集与处理界面

## 环境要求

- Windows PowerShell 5.1 或更高版本
- Go 1.26.x（标准安装路径 `C:\Program Files\Go` 也可自动识别）
- Node.js 与 npm
- Docker Desktop，且 Docker Engine 已启动
- 微信开发者工具（仅手工验证小程序时需要）

## 一键验收

在仓库根目录运行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
```

脚本会构建并启动完整 Compose 栈、执行数据库迁移、运行 Go/Node/TypeScript 测试，再创建真实测试项目，上传两类测试物料，等待工作流完成并校验下载文件以 `%PDF-` 开头。成功后服务会继续运行，方便小程序联调。

已经构建过镜像时可跳过构建：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1 -SkipBuild
```

## 手动启动

复制 `.env.example` 为本地环境配置参考。Docker Compose 已内置开发环境变量，不要求创建 `.env`：

```powershell
docker compose -f deploy/local/compose.yaml up -d --build --wait
$env:GOOSE_DRIVER = 'postgres'
$env:GOOSE_DBSTRING = 'postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
```

若不在 Docker 内运行 Go 进程，先加载 `.env.example` 中的变量，然后分别执行：

```powershell
go run ./cmd/api
go run ./cmd/worker
```

## 小程序

1. 在微信开发者工具中导入 `miniapp/project.config.json`。
2. 开发阶段关闭“不校验合法域名、web-view（业务域名）、TLS 版本以及 HTTPS 证书”。
3. 确认 `miniapp/env.ts` 指向 `http://localhost:8080`。
4. 在“工具 → 构建 npm”生成 `miniprogram_npm`。
5. 创建项目，分别录音/选择照片并上传，启动自动处理，完成后下载 PDF。

如需 CLI 自动编译或截图，还需在开发者工具“设置 → 安全设置”中手动开启服务端口。这是开发者工具的本机安全开关，仓库脚本不会代替用户修改。

## 本地端点

- API 健康检查：<http://localhost:8080/healthz>
- MinIO API：<http://localhost:9000>
- MinIO 控制台：<http://localhost:9001>（`sangyu` / `sangyu-local-secret`）
- Mock Skill Runner：<http://localhost:8090>
- Chromium DevTools：<http://localhost:9222>

## 常用验证

```powershell
go test ./...
go vet ./...
npm --prefix skills/mock-memoir test
npm --prefix miniapp test
npm --prefix miniapp run typecheck
```

设计说明见 `docs/superpowers/specs/2026-07-28-ai-memoir-platform-design.md`，首期实现计划见 `docs/superpowers/plans/2026-07-28-foundation-vertical-slice.md`。
