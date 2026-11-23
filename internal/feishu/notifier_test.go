package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/report"
)

func TestSendPostsSuccessfulDeploymentReport(t *testing.T) {
	var payload struct {
		MessageType string `json:"msg_type"`
		Card        struct {
			Header struct {
				Template string `json:"template"`
				Title    struct {
					Content string `json:"content"`
				} `json:"title"`
			} `json:"header"`
			Elements []struct {
				Fields []struct {
					Text struct {
						Content string `json:"content"`
					} `json:"text"`
				} `json:"fields"`
				Text *struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"elements"`
		} `json:"card"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("解码消息失败: %v", err)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Send(context.Background(), config.Feishu{Webhook: server.URL, Timeout: time.Second}, report.Report{Status: report.StatusSuccess, CheckedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), Checks: []report.Check{{Name: "deployment", Status: report.StatusSuccess, Summary: "ready"}}}, "uat", "catalog")
	if err != nil {
		t.Fatalf("Send() 错误 = %v", err)
	}
	var text strings.Builder
	for _, element := range payload.Card.Elements {
		for _, field := range element.Fields {
			text.WriteString(field.Text.Content)
		}
		if element.Text != nil {
			text.WriteString(element.Text.Content)
		}
	}
	if payload.MessageType != "interactive" || payload.Card.Header.Template != "green" || !strings.Contains(payload.Card.Header.Title.Content, "成功") {
		t.Fatalf("消息类型 = %q, 标题 = %q (模板 %q), 期望为绿色标题的成功卡片", payload.MessageType, payload.Card.Header.Title.Content, payload.Card.Header.Template)
	}
	if !strings.Contains(text.String(), "uat/catalog") || !strings.Contains(text.String(), "deployment") {
		t.Fatalf("卡片内容 = %q, 期望包含目标 uat/catalog 和检查项 deployment", text.String())
	}
	if !strings.Contains(text.String(), "20:00:00") {
		t.Fatalf("卡片内容 = %q, 期望时间已转换为北京时间 20:00:00", text.String())
	}
}

func TestSendSignsRequestWhenSecretConfigured(t *testing.T) {
	const secret = "signing-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		timestamp := request.URL.Query().Get("timestamp")
		signature := request.URL.Query().Get("sign")
		if timestamp == "" || signature == "" {
			t.Fatal("缺少飞书签名查询参数")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(timestamp + "\n" + secret))
		if signature != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Fatalf("签名 = %q, 期望为合法的 HMAC", signature)
		}
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			t.Fatalf("时间戳 = %q, 期望为 Unix 秒: %v", timestamp, err)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Send(context.Background(), config.Feishu{Webhook: server.URL, Secret: secret, Timeout: time.Second}, report.Report{Status: report.StatusFailed}, "uat", "catalog")
	if err != nil {
		t.Fatalf("Send() 错误 = %v", err)
	}
}

func TestSendReturnsErrorForRejectedWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	err := Send(context.Background(), config.Feishu{Webhook: server.URL, Timeout: time.Second}, report.Report{Status: report.StatusError}, "uat", "catalog")
	if err == nil {
		t.Fatal("Send() 错误为 nil, 期望为 HTTP 拒绝错误")
	}
}
