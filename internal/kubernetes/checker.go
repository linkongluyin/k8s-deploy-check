package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/report"
)

type Checker struct {
	client      kubernetes.Interface
	namespace   string
	deployment  string
	timeout     time.Duration
	interval    time.Duration
	maxRestarts int
}

func NewClient(config config.Config) (*Checker, error) {
	restConfig, err := clientConfig(config.Kubernetes)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}
	return &Checker{client: client, namespace: config.Kubernetes.Namespace, deployment: config.Kubernetes.Deployment, timeout: config.Check.Timeout, interval: config.Check.Interval, maxRestarts: config.Check.MaxRestarts}, nil
}

func (checker *Checker) CheckDeployment(ctx context.Context) (report.Check, error) {
	var last report.Check
	err := wait.PollUntilContextTimeout(ctx, checker.interval, checker.timeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := checker.client.AppsV1().Deployments(checker.namespace).Get(ctx, checker.deployment, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("获取 Deployment %s/%s 失败: %w", checker.namespace, checker.deployment, err)
		}
		ready, summary := deploymentReady(deployment)
		last = report.Check{Name: "deployment", Status: report.StatusFailed, Summary: summary}
		if ready {
			last.Status = report.StatusSuccess
		}
		return ready, nil
	})
	if err != nil && last.Name == "" {
		return report.Check{}, err
	}
	if last.Name == "" {
		return report.Check{}, fmt.Errorf("Deployment 检查未返回状态")
	}
	if err != nil {
		if !errors.Is(err, wait.ErrWaitTimeout) && !errors.Is(err, context.DeadlineExceeded) {
			return report.Check{}, err
		}
		last.Details = append(last.Details, "发布在超时前未达到就绪状态")
	}
	return last, nil
}

func (checker *Checker) DiagnosePods(ctx context.Context) (report.Check, error) {
	deployment, err := checker.client.AppsV1().Deployments(checker.namespace).Get(ctx, checker.deployment, metav1.GetOptions{})
	if err != nil {
		return report.Check{}, fmt.Errorf("获取 Deployment（用于 Pod 诊断）失败: %w", err)
	}
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	if desired == 0 {
		return report.Check{Name: "pods", Status: report.StatusSuccess, Summary: "Deployment 期望副本数为 0"}, nil
	}
	selector, err := selectorForDeployment(deployment)
	if err != nil {
		return report.Check{}, fmt.Errorf("构造 Deployment Pod 选择器失败: %w", err)
	}
	pods, err := checker.client.CoreV1().Pods(checker.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return report.Check{}, fmt.Errorf("列出 Deployment Pod 失败: %w", err)
	}
	return diagnosePods(pods.Items, checker.maxRestarts), nil
}

func clientConfig(target config.Kubernetes) (*rest.Config, error) {
	if target.Kubeconfig == "" && target.Context == "" {
		if inCluster, err := rest.InClusterConfig(); err == nil {
			return inCluster, nil
		}
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if target.Kubeconfig != "" {
		rules.ExplicitPath = target.Kubeconfig
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: target.Context})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("加载 Kubernetes 客户端配置失败: %w", err)
	}
	return restConfig, nil
}

func deploymentReady(deployment *appsv1.Deployment) (bool, string) {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	status := deployment.Status
	summary := fmt.Sprintf("已观察代次=%d 代次=%d 已更新=%d 就绪=%d 可用=%d 期望=%d 不可用=%d", status.ObservedGeneration, deployment.Generation, status.UpdatedReplicas, status.ReadyReplicas, status.AvailableReplicas, desired, status.UnavailableReplicas)
	ready := status.ObservedGeneration >= deployment.Generation && status.UpdatedReplicas == desired && status.ReadyReplicas == desired && status.AvailableReplicas == desired && status.UnavailableReplicas == 0
	return ready, summary
}

func selectorForDeployment(deployment *appsv1.Deployment) (labels.Selector, error) {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return selector, nil
}

func diagnosePods(pods []corev1.Pod, maxRestarts int) report.Check {
	check := report.Check{Name: "pods", Status: report.StatusSuccess, Summary: fmt.Sprintf("%d 个 Pod 健康", len(pods))}
	if len(pods) == 0 {
		check.Status = report.StatusFailed
		check.Summary = "没有 Pod 匹配 Deployment 选择器"
		return check
	}
	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			check.Details = append(check.Details, fmt.Sprintf("Pod %s 阶段=%s", pod.Name, pod.Status.Phase))
		}
		checkContainerStatuses(&check, pod.Name, pod.Status.InitContainerStatuses, maxRestarts, false)
		checkContainerStatuses(&check, pod.Name, pod.Status.ContainerStatuses, maxRestarts, true)
	}
	if len(check.Details) > 0 {
		check.Status = report.StatusFailed
		check.Summary = "Pod 诊断发现不健康状态"
	}
	return check
}

func checkContainerStatuses(check *report.Check, podName string, statuses []corev1.ContainerStatus, maxRestarts int, requireReady bool) {
	for _, status := range statuses {
		prefix := fmt.Sprintf("Pod %s 容器 %s", podName, status.Name)
		if status.RestartCount > int32(maxRestarts) {
			check.Details = append(check.Details, fmt.Sprintf("%s 重启次数=%d 超过上限=%d", prefix, status.RestartCount, maxRestarts))
		}
		if status.State.Waiting != nil && status.State.Waiting.Reason != "" {
			check.Details = append(check.Details, fmt.Sprintf("%s 等待原因=%s", prefix, status.State.Waiting.Reason))
		}
		if status.State.Terminated != nil && status.State.Terminated.Reason != "Completed" {
			check.Details = append(check.Details, fmt.Sprintf("%s 终止原因=%s", prefix, strings.TrimSpace(status.State.Terminated.Reason)))
		}
		if requireReady && !status.Ready {
			check.Details = append(check.Details, fmt.Sprintf("%s 未就绪", prefix))
		}
	}
}
