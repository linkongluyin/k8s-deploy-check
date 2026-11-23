# k8s-deploy-check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个可编译的 Go CLI，用 `client-go` 验证单个 Kubernetes Deployment 与可选 HTTP 健康地址。

**Architecture:** `cmd` 负责进程入口；`app` 编排独立的 Kubernetes、HTTP 与报告包；配置和状态判定保持为可直接单测的纯逻辑。使用 Go 标准 `flag` 包，避免为一个命令引入 CLI 框架。

**Tech Stack:** Go 1.24、Kubernetes `client-go`、`sigs.k8s.io/yaml`、Go `httptest`。

**Spec:** `docs/design/2026-11-10-k8s-deploy-check.md`

## Global Constraints

- 只通过 Kubernetes 官方 `client-go` 访问集群，禁止调用 `kubectl` 或 shell。
- 保持单目标 MVP；不实现发布、重启、回滚或通知。
- 报告仅写标准输出，日志仅写标准错误。
- 退出码固定为 `0`、`2`、`3`、`4`。

## Review Focus

- Deployment 缩容到 `0` 副本时应视为成功，不能因零值误判。
- `matchExpressions` 标签选择器必须能列出 Pod，不能只处理 `matchLabels`。
- HTTP 健康地址返回重定向或非 2xx 时的接受策略必须与 README 一致。
- 轮询取消/超时后仍须输出已有的 Pod 诊断。
- JSON 模式不得混入日志或多余文本。

### Task 1: 配置、报告和命令参数

**Files:**
- Create: `go.mod`, `internal/config/config.go`, `internal/config/config_test.go`
- Create: `internal/report/report.go`, `internal/report/report_test.go`, `internal/app/exitcode.go`
- Create: `cmd/k8s-deploy-check/main.go`, `configs/example.yaml`

**Interfaces:**
- Produces: `config.Load(path string, overrides config.Overrides) (config.Config, error)`；`Config.Validate() error`；`report.Render(io.Writer, report.Report, format string) error`。

- [ ] 写配置和报告的失败测试，验证必填 Kubernetes 参数、非法值、JSON 输出和退出码映射。
- [ ] 运行 `go test ./internal/config ./internal/report ./internal/app`，确认测试因缺失实现失败。
- [ ] 实现最小配置加载/合并/校验、报告渲染、退出码和标准库 `flag` 参数解析。
- [ ] 再次运行同一命令，确认通过。

### Task 2: Kubernetes Deployment 与 Pod 诊断

**Files:**
- Create: `internal/kubernetes/checker.go`, `internal/kubernetes/checker_test.go`

**Interfaces:**
- Consumes: `config.Config` 和 `report.Check`。
- Produces: `kubernetes.NewClient(config.Config) (*kubernetes.Checker, error)`；`Checker.CheckDeployment(context.Context) (report.Check, error)`；`Checker.DiagnosePods(context.Context) (report.Check, error)`。

- [ ] 写 Deployment 就绪判定、零副本、重启阈值及异常容器状态的失败测试。
- [ ] 运行 `go test ./internal/kubernetes`，确认测试失败。
- [ ] 使用 `client-go` 实现客户端创建、轮询 Deployment、标签选择器 Pod 列表和纯函数诊断。
- [ ] 运行 `go test ./internal/kubernetes`，确认通过。

### Task 3: HTTP 检查与流程编排

**Files:**
- Create: `internal/health/checker.go`, `internal/health/checker_test.go`
- Create: `internal/app/run.go`, `internal/app/run_test.go`
- Modify: `cmd/k8s-deploy-check/main.go`

**Interfaces:**
- Consumes: `config.Config`、`kubernetes.Checker` 与 `report.Report`。
- Produces: `health.Check(context.Context, config.Health) (report.Check, error)`；`app.Run(context.Context, config.Config) (report.Report, int)`。

- [ ] 用 `httptest` 写出「首次失败后成功」和「重试耗尽」的失败测试；为编排退出码写失败测试。
- [ ] 运行 `go test ./internal/health ./internal/app`，确认失败。
- [ ] 实现固定间隔重试和流程编排；命令将渲染后的报告输出并以映射退出。
- [ ] 运行 `go test ./...` 与 `go build ./cmd/k8s-deploy-check`，确认通过。

### Task 4: 使用文档与 Jenkins 示例

**Files:**
- Create: `README.md`, `docs/jenkins.md`, `Makefile`

**Interfaces:**
- Consumes: 已完成的 CLI 参数、配置格式、退出码和报告格式。

- [ ] 记录本地构建、集群权限、配置、所有退出码、JSON 使用方式和 Jenkins 后置检查示例。
- [ ] 运行 `go test ./...`、`go vet ./...`、`go build ./cmd/k8s-deploy-check`，确认通过。
