package app

import (
	"testing"

	"github.com/example/k8s-deploy-check/internal/report"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		status report.Status
		want   int
	}{
		{report.StatusSuccess, ExitSuccess},
		{report.StatusFailed, ExitVerificationFailed},
		{report.StatusError, ExitRuntimeError},
	}
	for _, test := range tests {
		if got := ExitCode(report.Report{Status: test.status}); got != test.want {
			t.Errorf("ExitCode(%q) = %d, 期望为 %d", test.status, got, test.want)
		}
	}
}
