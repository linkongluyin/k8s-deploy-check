package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/k8s-deploy-check/internal/app"
)

func TestRunRejectsExplicitInvalidCheckValues(t *testing.T) {
	tests := [][]string{
		{"--namespace", "uat", "--deployment", "catalog", "--timeout=0s"},
		{"--namespace", "uat", "--deployment", "catalog", "--interval=-1s"},
		{"--namespace", "uat", "--deployment", "catalog", "--max-restarts=-1"},
		{"--namespace", "uat", "--deployment", "catalog", "--health-retries=0"},
		{"--namespace", "uat", "--deployment", "catalog", "--feishu-webhook", "not-a-url"},
		{"--namespace", "uat", "--deployment", "catalog", "--feishu-timeout=0s"},
	}
	for _, arguments := range tests {
		if got := run(arguments, &bytes.Buffer{}, &bytes.Buffer{}); got != app.ExitInvalidConfig {
			t.Errorf("run(%v) = %d, 期望为 %d", arguments, got, app.ExitInvalidConfig)
		}
	}
}

func TestRunAutoLoadsDefaultConfigWhenPresent(t *testing.T) {
	directory := t.TempDir()
	contents := "kubernetes:\n  namespace: uat\n  deployment: catalog\ncheck:\n  timeout: bogus\n"
	if err := os.WriteFile(filepath.Join(directory, defaultConfigFiles[0]), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })

	var stderr bytes.Buffer
	if got := run(nil, &bytes.Buffer{}, &stderr); got != app.ExitInvalidConfig {
		t.Fatalf("run() = %d, 期望为 %d", got, app.ExitInvalidConfig)
	}
	if !strings.Contains(stderr.String(), "解析 check.timeout 失败") {
		t.Fatalf("stderr = %q, 期望为自动加载配置的解析错误", stderr.String())
	}
}
