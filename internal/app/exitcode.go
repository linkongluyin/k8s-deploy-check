package app

import "github.com/example/k8s-deploy-check/internal/report"

const (
	ExitSuccess            = 0
	ExitInvalidConfig      = 2
	ExitVerificationFailed = 3
	ExitRuntimeError       = 4
)

func ExitCode(result report.Report) int {
	switch result.Status {
	case report.StatusSuccess:
		return ExitSuccess
	case report.StatusFailed:
		return ExitVerificationFailed
	default:
		return ExitRuntimeError
	}
}
