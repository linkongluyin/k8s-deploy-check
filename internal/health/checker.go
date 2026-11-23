package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/report"
)

func Check(ctx context.Context, target config.Health) (report.Check, error) {
	if target.URL == "" {
		return report.Check{Name: "health", Status: report.StatusSuccess, Summary: "未配置"}, nil
	}
	client := &http.Client{
		Timeout: target.Timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	lastFailure := ""
	for attempt := 1; attempt <= target.Retries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
		if err != nil {
			return report.Check{}, fmt.Errorf("构造健康检查请求失败: %w", err)
		}
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest {
				return report.Check{Name: "health", Status: report.StatusSuccess, Summary: fmt.Sprintf("HTTP %d（第 %d 次尝试成功）", response.StatusCode, attempt)}, nil
			}
			lastFailure = fmt.Sprintf("HTTP 状态码=%d", response.StatusCode)
		} else {
			lastFailure = err.Error()
		}
		if attempt < target.Retries {
			select {
			case <-ctx.Done():
				return report.Check{}, ctx.Err()
			case <-time.After(target.RetryInterval):
			}
		}
	}
	return report.Check{Name: "health", Status: report.StatusFailed, Summary: fmt.Sprintf("健康端点在 %d 次尝试后仍不健康", target.Retries), Details: []string{lastFailure}}, nil
}
