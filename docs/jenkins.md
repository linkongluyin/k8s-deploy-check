# Jenkins 调用示例

部署命令与发布验证应分开：前者发起变更，后者确认工作负载和应用已可用。`k8s-deploy-check` 本身不执行 `kubectl`。

目标命名空间 / Deployment、健康检查、飞书告警等全部配置由运行目录的 `config.yaml` 提供（工具自动加载，该文件不入库，避免 webhook 泄露）。Jenkinsfile 只需调用二进制：

```groovy
stage('部署验证') {
    when { expression { return params.RESTART } }
    steps {
        script {
            // 配置全部来自 workspace 的 config.yaml（namespace、deployment、健康检查、飞书 webhook）
            sh "./bin/k8s-deploy-check"
        }
    }
}
```

说明：

- 验证结束后工具**直接向飞书推送交互卡片**（成功绿色 / 失败红色 / 错误灰色），无需任何中间文件；文本报告和进度日志会上 Jenkins 控制台；
- 验证失败（非零退出码）会让 Jenkins Stage 失败，同时飞书收到红色失败卡片；
- 如需在构建产物中留档，可改为 `sh "./bin/k8s-deploy-check --output json > deploy-check-report.json"` 并加 `archiveArtifacts` 归档；
- 应将构建运行账号配置为可读取目标命名空间的 Deployment 与 Pod；集群内运行时可使用对应 ServiceAccount。
