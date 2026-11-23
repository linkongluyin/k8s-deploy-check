# k8s-deploy-check MVP 设计

## 目标

`k8s-deploy-check` 在 Jenkins 发布后使用 Kubernetes 官方 `client-go` 验证一个 Deployment 是否真正可用，并给出可机器解析或可阅读的失败报告。

## 范围

- 等待 Deployment 的 `ObservedGeneration`、更新副本和可用副本达到期望值。
- 使用 Deployment 的标签选择器列出 Pod，诊断非 Running、未 Ready、等待/终止原因和异常重启次数。
- 可选调用 HTTP 健康地址，支持请求超时和固定间隔重试。
- 输出统一报告（`text` 或 `json`），并返回固定退出码。

不包含发布、重启、回滚、多 Deployment 编排、事件流、告警通知或 CRD 支持；这些职责留给 Jenkins/Kubernetes 和后续独立迭代。

## 命令和配置

MVP 只提供一个命令：

```text
k8s-deploy-check --namespace uat --deployment orders --health-url http://orders/actuator/health
```

也可传入 `--config configs/example.yaml`。命令行的非空参数覆盖配置文件同名值。kubeconfig 未指定时按 `client-go` 的默认加载规则处理；在集群内运行时使用 ServiceAccount。

配置包含 Kubernetes 连接参数、Deployment 标识、轮询总超时/间隔、Pod 最大允许重启数，以及可选 HTTP 检查的 URL、单次超时、重试次数和重试间隔。

## 架构

```text
cmd/k8s-deploy-check  参数解析、进程退出
        │
internal/app           编排检查流程与退出码映射
 ├── internal/config   配置加载、合并、校验
 ├── internal/kubernetes client-go 客户端、Deployment/Pod 检查
 ├── internal/health   HTTP 重试检查
 └── internal/report   统一领域报告与 JSON/文本渲染
```

`app` 只依赖检查结果，不了解 client-go 的对象细节；`kubernetes` 不写终端输出；`report` 不发起网络调用。为保持 MVP 简洁，不引入 repository/service 接口层：每个实现只有一个时，接口只会增加间接层。

## 检查流程

1. 载入配置并校验必填项及时间/次数边界。
2. 创建 `client-go` 客户端，在总超时内轮询 Deployment 副本状态。
3. 无论 Deployment 成功或失败，读取匹配 Pod 并收集诊断信息。
4. 配置了健康 URL 时执行 HTTP 检查；每次请求都受单次超时限制。
5. 汇总报告：任一业务检查失败为 `failed`；无法建立 Kubernetes 客户端或读取资源为 `error`。

## 退出码

| 代码 | 含义 |
| --- | --- |
| `0` | 所有启用的检查成功 |
| `2` | 参数或配置无效 |
| `3` | 发布验证失败（副本、Pod 或健康检查不符合要求） |
| `4` | 运行错误（Kubernetes/API/网络调用无法完成） |

报告写到标准输出，日志写到标准错误；因此 JSON 可安全交给 Jenkins 或其他程序消费。

## 测试策略

- 配置校验：必填字段、非法时长、非法重试数。
- Deployment 状态判定与 Pod 诊断：只测纯函数，避免为 MVP 构造 API server。
- HTTP 重试：`httptest` 覆盖最终成功和最终失败。
- 报告渲染及退出码：覆盖 JSON 有效性、文本摘要和关键映射。

真实集群连通性由 Jenkins 示例在目标环境中验证；本项目不在单元测试中依赖集群。

## 后续扩展点

当需求真实出现时再增加：多个目标的清单执行、Ingress/Service 端到端探测、Kubernetes Events 和日志尾部、失败时自动回滚、Prometheus/企业通知。
