package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusError   Status = "error"
)

type Check struct {
	Name    string   `json:"name"`
	Status  Status   `json:"status"`
	Summary string   `json:"summary"`
	Details []string `json:"details,omitempty"`
}

type Report struct {
	Status    Status    `json:"status"`
	CheckedAt time.Time `json:"checkedAt"`
	Checks    []Check   `json:"checks"`
}

func Render(writer io.Writer, result Report, format string) error {
	if format == "json" {
		return json.NewEncoder(writer).Encode(result)
	}
	if format != "text" {
		return fmt.Errorf("不支持的输出格式 %q", format)
	}
	_, err := fmt.Fprintf(writer, "status: %s\n", result.Status)
	if err != nil {
		return err
	}
	for _, check := range result.Checks {
		if _, err = fmt.Fprintf(writer, "%s: %s - %s\n", check.Name, check.Status, check.Summary); err != nil {
			return err
		}
		for _, detail := range check.Details {
			if _, err = fmt.Fprintf(writer, "  - %s\n", strings.TrimSpace(detail)); err != nil {
				return err
			}
		}
	}
	return nil
}
