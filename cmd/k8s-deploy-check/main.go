package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/example/k8s-deploy-check/internal/app"
	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/feishu"
	"github.com/example/k8s-deploy-check/internal/report"
)

// defaultConfigFiles 是未指定 --config 时的自动查找顺序：
// 先当前目录（本地/Jenkins workspace 场景），后系统目录（二进制装到 /usr/bin 的场景）。
var defaultConfigFiles = [2]string{"config.yaml", "/etc/k8s-deploy-check/config.yaml"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	logger := log.New(stderr, "k8s-deploy-check: ", log.LstdFlags)
	flags := flag.NewFlagSet("k8s-deploy-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "YAML 配置文件路径，未指定时自动读取当前目录的 config.yaml")
	kubeconfig := flags.String("kubeconfig", "", "kubeconfig 文件路径")
	contextName := flags.String("context", "", "kubeconfig 上下文名称")
	namespace := flags.String("namespace", "", "Kubernetes 命名空间")
	deployment := flags.String("deployment", "", "Kubernetes Deployment 名称")
	timeout := flags.Duration("timeout", 0, "Deployment 发布等待超时")
	interval := flags.Duration("interval", 0, "Deployment 轮询间隔")
	maxRestarts := flags.Int("max-restarts", -1, "单容器允许的最大重启次数")
	healthURL := flags.String("health-url", "", "可选的 HTTP 健康检查地址")
	healthTimeout := flags.Duration("health-timeout", 0, "HTTP 健康检查请求超时")
	healthRetries := flags.Int("health-retries", 0, "HTTP 健康检查总尝试次数")
	retryInterval := flags.Duration("health-retry-interval", 0, "HTTP 健康检查两次尝试间隔")
	feishuWebhook := flags.String("feishu-webhook", "", "飞书自定义机器人 webhook，配置后成功与失败均推送告警")
	feishuSecret := flags.String("feishu-secret", "", "飞书机器人签名校验密钥")
	feishuTimeout := flags.Duration("feishu-timeout", 0, "飞书告警推送请求超时")
	format := flags.String("output", "text", "报告输出格式：text 或 json")
	verbose := flags.Bool("verbose", false, "输出进度日志到标准错误")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "用法: k8s-deploy-check [参数]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return app.ExitSuccess
		}
		return app.ExitInvalidConfig
	}
	if *format != "text" && *format != "json" {
		logger.Print("参数校验失败: --output 只支持 text 或 json")
		return app.ExitInvalidConfig
	}
	if (flagWasSet(flags, "timeout") && *timeout <= 0) ||
		(flagWasSet(flags, "interval") && *interval <= 0) ||
		(flagWasSet(flags, "max-restarts") && *maxRestarts < 0) ||
		(flagWasSet(flags, "health-timeout") && *healthTimeout <= 0) ||
		(flagWasSet(flags, "health-retries") && *healthRetries < 1) ||
		(flagWasSet(flags, "health-retry-interval") && *retryInterval <= 0) ||
		(flagWasSet(flags, "feishu-timeout") && *feishuTimeout <= 0) {
		logger.Print("参数校验失败: 显式指定的超时、间隔和重试参数必须为正数")
		return app.ExitInvalidConfig
	}
	overrides := config.Overrides{Kubeconfig: *kubeconfig, Context: *contextName, Namespace: *namespace, Deployment: *deployment, Timeout: *timeout, Interval: *interval, HealthURL: *healthURL, HealthTimeout: *healthTimeout, HealthRetries: *healthRetries, RetryInterval: *retryInterval, FeishuWebhook: *feishuWebhook, FeishuSecret: *feishuSecret, FeishuTimeout: *feishuTimeout}
	if *maxRestarts >= 0 {
		overrides.MaxRestarts = maxRestarts
	}
	if *configPath == "" {
		for _, candidate := range defaultConfigFiles {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
			*configPath = candidate
			if *verbose {
				logger.Printf("未指定 --config，已自动加载 %s", candidate)
			}
			break
		}
	}
	target, err := config.Load(*configPath, overrides)
	if err != nil {
		logger.Printf("加载配置失败: %v", err)
		return app.ExitInvalidConfig
	}
	var progress *log.Logger
	if *verbose {
		progress = logger
	}
	result := app.Run(context.Background(), target, progress)
	if err := report.Render(stdout, result, *format); err != nil {
		logger.Printf("输出报告失败: %v", err)
		return app.ExitRuntimeError
	}
	if target.Feishu.Webhook != "" {
		if progress != nil {
			progress.Printf("正在推送飞书告警（结果状态: %s）", result.Status)
		}
		if err := feishu.Send(context.Background(), target.Feishu, result, target.Kubernetes.Namespace, target.Kubernetes.Deployment); err != nil {
			logger.Printf("飞书告警推送失败: %v", err)
		}
	}
	return app.ExitCode(result)
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(current *flag.Flag) {
		found = found || current.Name == name
	})
	return found
}
