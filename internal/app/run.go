package app

import (
	"context"
	"log"
	"time"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/health"
	"github.com/example/k8s-deploy-check/internal/kubernetes"
	"github.com/example/k8s-deploy-check/internal/report"
)

func Run(ctx context.Context, config config.Config, logger *log.Logger) report.Report {
	result := report.Report{CheckedAt: time.Now().UTC()}
	checker, err := kubernetes.NewClient(config)
	if err != nil {
		return finish(result, report.Check{Name: "kubernetes", Status: report.StatusError, Summary: err.Error()})
	}
	if logger != nil {
		logger.Printf("正在检查 Deployment %s/%s", config.Kubernetes.Namespace, config.Kubernetes.Deployment)
	}
	deployment, err := checker.CheckDeployment(ctx)
	if err != nil {
		result.Checks = append(result.Checks, report.Check{Name: "deployment", Status: report.StatusError, Summary: err.Error()})
	} else {
		result.Checks = append(result.Checks, deployment)
	}
	pods, err := checker.DiagnosePods(ctx)
	if err != nil {
		result.Checks = append(result.Checks, report.Check{Name: "pods", Status: report.StatusError, Summary: err.Error()})
	} else {
		result.Checks = append(result.Checks, pods)
	}
	if config.Health.URL != "" {
		if logger != nil {
			logger.Printf("正在检查 HTTP 健康地址")
		}
		healthCheck, err := health.Check(ctx, config.Health)
		if err != nil {
			result.Checks = append(result.Checks, report.Check{Name: "health", Status: report.StatusError, Summary: err.Error()})
		} else {
			result.Checks = append(result.Checks, healthCheck)
		}
	}
	return finish(result)
}

func finish(result report.Report, checks ...report.Check) report.Report {
	result.Checks = append(result.Checks, checks...)
	result.Status = report.StatusSuccess
	for _, check := range result.Checks {
		if check.Status == report.StatusError {
			result.Status = report.StatusError
			return result
		}
		if check.Status == report.StatusFailed {
			result.Status = report.StatusFailed
		}
	}
	return result
}
