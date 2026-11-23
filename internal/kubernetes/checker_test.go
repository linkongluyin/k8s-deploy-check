package kubernetes

import (
	"errors"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDeploymentReady(t *testing.T) {
	desired := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 7},
		Spec:       appsv1.DeploymentSpec{Replicas: &desired},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 7,
			UpdatedReplicas:    2,
			ReadyReplicas:      2,
			AvailableReplicas:  2,
		},
	}
	if ready, _ := deploymentReady(deployment); !ready {
		t.Fatal("deploymentReady() = false, 期望为 true")
	}
}

func TestDeploymentReadyAllowsZeroReplicas(t *testing.T) {
	desired := int32(0)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &desired},
		Status:     appsv1.DeploymentStatus{ObservedGeneration: 1},
	}
	if ready, _ := deploymentReady(deployment); !ready {
		t.Fatal("deploymentReady() = false, 期望为零副本时为 true")
	}
}

func TestDiagnosePodsDetectsCrashLoopAndRestarts(t *testing.T) {
	pods := []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Name: "catalog-123"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "catalog",
				RestartCount: 4,
				State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}}
	check := diagnosePods(pods, 3)
	if check.Status != "failed" {
		t.Fatalf("状态 = %q, 期望为 failed", check.Status)
	}
	if !strings.Contains(strings.Join(check.Details, " "), "CrashLoopBackOff") {
		t.Fatalf("详情 = %v, 期望包含崩溃循环原因", check.Details)
	}
}

func TestSelectorForDeploymentSupportsMatchExpressions(t *testing.T) {
	deployment := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"catalog"}}}}}}
	selector, err := selectorForDeployment(deployment)
	if err != nil {
		t.Fatalf("selectorForDeployment() 错误 = %v", err)
	}
	if !selector.Matches(labels.Set{"app": "catalog"}) || selector.Matches(labels.Set{"app": "orders"}) {
		t.Fatalf("选择器 = %q, 期望只匹配 catalog", selector.String())
	}
}

func TestCheckDeploymentReturnsAPIErrors(t *testing.T) {
	desired := int32(1)
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "catalog", Namespace: "uat", Generation: 1}, Spec: appsv1.DeploymentSpec{Replicas: &desired}, Status: appsv1.DeploymentStatus{ObservedGeneration: 1}}
	client := clientset.NewSimpleClientset(deployment)
	gets := 0
	client.PrependReactor("get", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets > 1 {
			return true, nil, errors.New("API unavailable")
		}
		return false, nil, nil
	})
	checker := &Checker{client: client, namespace: "uat", deployment: "catalog", timeout: time.Second, interval: time.Millisecond}
	if _, err := checker.CheckDeployment(t.Context()); err == nil {
		t.Fatal("CheckDeployment() 错误为 nil, 期望为 API 错误")
	}
}
