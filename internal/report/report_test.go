package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderJSON(t *testing.T) {
	result := Report{Status: StatusSuccess, Checks: []Check{{Name: "deployment", Status: StatusSuccess, Summary: "ready"}}}
	var output bytes.Buffer
	if err := Render(&output, result, "json"); err != nil {
		t.Fatalf("Render() 错误 = %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("Render() 输出不是合法 JSON: %v", err)
	}
	if decoded.Status != StatusSuccess {
		t.Fatalf("JSON 状态 = %q, 期望为 %q", decoded.Status, StatusSuccess)
	}
}

func TestRenderTextIncludesFailedCheck(t *testing.T) {
	result := Report{Status: StatusFailed, Checks: []Check{{Name: "health", Status: StatusFailed, Summary: "HTTP 503", Details: []string{"attempts=3"}}}}
	var output bytes.Buffer
	if err := Render(&output, result, "text"); err != nil {
		t.Fatalf("Render() 错误 = %v", err)
	}
	if !strings.Contains(output.String(), "health: failed - HTTP 503") {
		t.Fatalf("Render() = %q, 期望包含失败的检查项", output.String())
	}
}
