# k8s-deploy-check

Kubernetes 发布后的单目标验证 CLI。它使用官方 `client-go` 读取 Deployment 和 Pod，不调用 `kubectl`；可选地验证 HTTP 健康地址。

## 能力

- 等待 Deployment 的 generation、更新副本、Ready 副本和 Available 副本达到目标。
- 列出 Deployment 标签选择器（包含 `matchExpressions`）匹配的 Pod，诊断非 Running、未 Ready、等待/终止原因和超限重启。
- 对可选 HTTP 地址执行超时受控的固定间隔重试；接受 `200–399`。
- 验证结束后（无论成功或失败）可向飞书自定义机器人 webhook 推送结果告警，使用交互式卡片消息（成功绿色 / 失败红色 / 错误灰色标题栏，含目标、时间与各项检查项详情），支持签名校验。
- 统一 `text`/`json` 报告和适合 Jenkins 的退出码。

## 构建与测试

```bash
make test
make vet
make build
```

二进制位于 `bin/k8s-deploy-check`。首次运行会下载 Go 依赖。

## 使用

配置文件示例见 [`configs/example.yaml`](configs/example.yaml)。未指定 `--config` 时，工具按顺序自动查找：当前目录的 `config.yaml` → 系统目录 `/etc/k8s-deploy-check/config.yaml`（适配二进制装到 `/usr/bin` 的部署方式）；都不存在则纯参数运行。命令行非空参数仍然覆盖配置文件：

```bash
# 当前目录有 config.yaml 时，无需任何参数即可运行
./bin/k8s-deploy-check

# 或显式指定配置文件
./bin/k8s-deploy-check --config configs/example.yaml
./bin/k8s-deploy-check \
  --namespace uat \
  --deployment catalog \
  --health-url http://catalog.uat.svc.cluster.local:8080/actuator/health \
  --output json \
  --verbose
```

命令行的非空参数覆盖配置文件。`--kubeconfig` 和 `--context` 可指定集群连接；两者都未提供时，工具优先使用集群内 ServiceAccount，再使用本地默认 kubeconfig。日志始终写到标准错误，统一为 `k8s-deploy-check: 日期 时间 中文消息` 格式（参数校验、加载配置、渲染报告、飞书推送等错误日志始终输出，进度日志需 `--verbose`）；报告始终写到标准输出，因此 JSON 可直接重定向归档。

| 参数 | 说明 |
| --- | --- |
| `--config` | YAML 配置文件路径，未指定时自动读取当前目录的 `config.yaml` |
| `--namespace` / `--deployment` | 必填目标，可由配置文件提供 |
| `--timeout` / `--interval` | Deployment 轮询总超时和轮询间隔 |
| `--max-restarts` | 单容器允许的最大重启数，`0` 表示不允许重启 |
| `--health-url` | 可选 HTTP 健康地址 |
| `--health-timeout` / `--health-retries` / `--health-retry-interval` | HTTP 请求超时、总尝试次数、两次尝试间隔 |
| `--feishu-webhook` | 可选飞书自定义机器人 webhook，配置后无论成功或失败都会推送告警 |
| `--feishu-secret` | 飞书机器人签名校验密钥，留空则不签名 |
| `--feishu-timeout` | 飞书推送请求超时 |
| `--output text|json` | 报告格式，默认 `text` |
| `--verbose` | 输出进度日志到标准错误 |

## 退出码

| 代码 | 含义 |
| --- | --- |
| `0` | 所有启用检查成功 |
| `2` | 参数或配置不合法 |
| `3` | Deployment、Pod 或健康检查未通过 |
| `4` | Kubernetes 配置/API 或运行时调用失败 |

## Kubernetes 权限

运行身份至少需要目标 namespace 中 `apps/deployments` 的 `get`，以及 core `pods` 的 `get,list`。示例中没有包含 RBAC 清单，避免工具擅自授予生产权限；请将这些最小权限合并到现有发布 ServiceAccount。

## Jenkins

见 [`docs/jenkins.md`](docs/jenkins.md)。检查失败会返回非零退出码，报告可作为构建产物归档。

## 结构

```text
cmd/k8s-deploy-check/    CLI 入口
internal/config/         配置加载与校验
internal/app/            检查编排与退出码
internal/kubernetes/     client-go Deployment/Pod 检查
internal/health/         HTTP 健康重试
internal/feishu/         飞书成功/失败告警推送
internal/report/         文本与 JSON 报告
```

设计取舍与边界见 [`docs/design/2026-11-10-k8s-deploy-check.md`](docs/design/2026-11-10-k8s-deploy-check.md)。
