package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateRequiresTarget(t *testing.T) {
	config := Default()
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() 错误为 nil, 期望为缺少必填目标错误")
	}
}

func TestLoadAppliesOverrides(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("kubernetes:\n  namespace: default\n  deployment: catalog\ncheck:\n  timeout: 30s\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := Load(path, Overrides{Namespace: "uat", Timeout: 45 * time.Second})
	if err != nil {
		t.Fatalf("Load() 错误 = %v", err)
	}
	if config.Kubernetes.Namespace != "uat" || config.Kubernetes.Deployment != "catalog" {
		t.Fatalf("目标 = %s/%s, 期望为 uat/catalog", config.Kubernetes.Namespace, config.Kubernetes.Deployment)
	}
	if config.Check.Timeout != 45*time.Second {
		t.Fatalf("timeout = %s, 期望为 45s", config.Check.Timeout)
	}
}

func TestValidateRejectsInvalidHealthRetries(t *testing.T) {
	config := Default()
	config.Kubernetes.Namespace = "uat"
	config.Kubernetes.Deployment = "catalog"
	config.Health.URL = "https://catalog.example/health"
	config.Health.Retries = 0
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() 错误为 nil, 期望为 retries 校验错误")
	}
}

func TestLoadKeepsDefaultMaxRestartsWithoutOverride(t *testing.T) {
	config, err := Load("", Overrides{Namespace: "uat", Deployment: "catalog"})
	if err != nil {
		t.Fatalf("Load() 错误 = %v", err)
	}
	if config.Check.MaxRestarts != 3 {
		t.Fatalf("maxRestarts = %d, 期望为 3", config.Check.MaxRestarts)
	}
}

func TestLoadAllowsZeroMaxRestartsOverride(t *testing.T) {
	maxRestarts := 0
	config, err := Load("", Overrides{Namespace: "uat", Deployment: "catalog", MaxRestarts: &maxRestarts})
	if err != nil {
		t.Fatalf("Load() 错误 = %v", err)
	}
	if config.Check.MaxRestarts != 0 {
		t.Fatalf("maxRestarts = %d, 期望为 0", config.Check.MaxRestarts)
	}
}

func TestValidateRejectsInvalidFeishuWebhook(t *testing.T) {
	config := Default()
	config.Kubernetes.Namespace = "uat"
	config.Kubernetes.Deployment = "catalog"
	config.Feishu.Webhook = "not-a-url"
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() 错误为 nil, 期望为非法飞书 webhook 错误")
	}
}
