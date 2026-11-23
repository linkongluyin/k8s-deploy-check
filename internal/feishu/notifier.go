package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/k8s-deploy-check/internal/config"
	"github.com/example/k8s-deploy-check/internal/report"
)

func Send(ctx context.Context, target config.Feishu, result report.Report, namespace, deployment string) error {
	webhook, err := signedWebhook(target.Webhook, target.Secret, time.Now())
	if err != nil {
		return err
	}
	payload, err := json.Marshal(interactivePayload{MessageType: "interactive", Card: card(result, namespace, deployment)})
	if err != nil {
		return fmt.Errorf("编码飞书消息失败: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("构造飞书请求失败: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: target.Timeout}).Do(request)
	if err != nil {
		return fmt.Errorf("发送飞书告警失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("飞书 webhook 返回了 HTTP %d", response.StatusCode)
	}
	return nil
}

func signedWebhook(raw, secret string, now time.Time) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("解析飞书 webhook 地址失败: %w", err)
	}
	if secret == "" {
		return parsed.String(), nil
	}
	timestamp := fmt.Sprint(now.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	query := parsed.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// 飞书卡片消息结构定义，见 https://open.feishu.cn/document/uAjLw4CM/ukzMukzM7
type interactivePayload struct {
	MessageType string      `json:"msg_type"`
	Card        cardPayload `json:"card"`
}

type cardPayload struct {
	Config   cardConfig    `json:"config"`
	Header   cardHeader    `json:"header"`
	Elements []cardElement `json:"elements"`
}

type cardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type cardHeader struct {
	Template string   `json:"template"`
	Title    cardText `json:"title"`
}

type cardText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type cardElement struct {
	Tag    string      `json:"tag,omitempty"`
	Fields []cardField `json:"fields,omitempty"`
	Text   *cardText   `json:"text,omitempty"`
}

type cardField struct {
	IsShort bool     `json:"is_short"`
	Text    cardText `json:"text"`
}

// beijing 用于飞书卡片的时间展示；内部数据（JSON 报告等）保持 UTC。
var beijing = time.FixedZone("UTC+8", 8*60*60)

func card(result report.Report, namespace, deployment string) cardPayload {
	checkedAt := result.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	title, template := "⚠️ Kubernetes 发布验证错误", "grey"
	switch result.Status {
	case report.StatusSuccess:
		title, template = "✅ Kubernetes 发布验证成功", "green"
	case report.StatusFailed:
		title, template = "❌ Kubernetes 发布验证失败", "red"
	}
	elements := []cardElement{
		{Tag: "div", Fields: []cardField{
			{IsShort: true, Text: cardText{Tag: "lark_md", Content: fmt.Sprintf("**目标**\n%s/%s", namespace, deployment)}},
			{IsShort: true, Text: cardText{Tag: "lark_md", Content: fmt.Sprintf("**时间**\n%s（北京时间）", checkedAt.In(beijing).Format("2006-01-02 15:04:05"))}},
		}},
		{Tag: "hr"},
	}
	for _, check := range result.Checks {
		var text strings.Builder
		fmt.Fprintf(&text, "%s **%s** - %s", statusIcon(check.Status), check.Name, check.Summary)
		for _, detail := range check.Details {
			fmt.Fprintf(&text, "\n  - %s", detail)
		}
		elements = append(elements, cardElement{Tag: "div", Text: &cardText{Tag: "lark_md", Content: text.String()}})
	}
	return cardPayload{Config: cardConfig{WideScreenMode: true}, Header: cardHeader{Template: template, Title: cardText{Tag: "plain_text", Content: title}}, Elements: elements}
}

func statusIcon(status report.Status) string {
	switch status {
	case report.StatusSuccess:
		return "✅"
	case report.StatusFailed:
		return "❌"
	default:
		return "⚠️"
	}
}
