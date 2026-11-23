package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/report"
)

func TestCheckRetriesUntilSuccessful(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	check, err := Check(context.Background(), config.Health{URL: server.URL, Timeout: time.Second, Retries: 2, RetryInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("Check() 错误 = %v", err)
	}
	if check.Status != report.StatusSuccess || requests.Load() != 2 {
		t.Fatalf("结果 = %#v, 请求数 = %d, 期望为两次请求后成功", check, requests.Load())
	}
}

func TestCheckReportsFailureAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	check, err := Check(context.Background(), config.Health{URL: server.URL, Timeout: time.Second, Retries: 2, RetryInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("Check() 错误 = %v", err)
	}
	if check.Status != report.StatusFailed {
		t.Fatalf("状态 = %q, 期望为 failed", check.Status)
	}
}

func TestCheckAcceptsRedirectWithoutFollowingIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			writer.Header().Set("Location", "/unhealthy")
			writer.WriteHeader(http.StatusFound)
			return
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	check, err := Check(context.Background(), config.Health{URL: server.URL + "/redirect", Timeout: time.Second, Retries: 1, RetryInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("Check() 错误 = %v", err)
	}
	if check.Status != report.StatusSuccess {
		t.Fatalf("状态 = %q, 期望为 HTTP 302 时判定成功", check.Status)
	}
}
