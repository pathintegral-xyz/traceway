package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type FeishuAdapter struct {
	WebhookURL    string `json:"webhookUrl"`
	SigningSecret string `json:"signingSecret"`
}

func (a *FeishuAdapter) Type() string { return "feishu" }

func (a *FeishuAdapter) Validate() error {
	u, err := url.Parse(a.WebhookURL)
	if err != nil || u.Scheme != "https" || (u.Hostname() != "open.feishu.cn" && u.Hostname() != "open.larksuite.com") || u.Port() != "" || !strings.HasPrefix(u.Path, "/open-apis/bot/v2/hook/") || strings.TrimPrefix(u.Path, "/open-apis/bot/v2/hook/") == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("a Feishu custom bot HTTPS webhook URL is required")
	}
	if a.SigningSecret == "" {
		return fmt.Errorf("Feishu signing secret is required")
	}
	return nil
}

func feishuSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (a *FeishuAdapter) Send(ctx context.Context, msg Message) error {
	timestamp := time.Now().Unix()
	message := msg.Subject
	if msg.Body != "" {
		message += "\n" + msg.Body
	}
	if msg.URL != "" {
		message += "\n" + msg.URL
	}
	payload := struct {
		Timestamp int64  `json:"timestamp"`
		Sign      string `json:"sign"`
		MsgType   string `json:"msg_type"`
		Content   struct {
			Text string `json:"text"`
		} `json:"content"`
	}{Timestamp: timestamp, Sign: feishuSign(strconv.FormatInt(timestamp, 10), a.SigningSecret), MsgType: "text"}
	payload.Content.Text = message
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Feishu payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create Feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("Feishu request failed: %w", err)
	}
	defer resp.Body.Close()
	resultBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("failed to read Feishu response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Feishu returned status %d", resp.StatusCode)
	}
	var result struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(resultBody, &result); err != nil || result.Code == nil {
		return fmt.Errorf("invalid Feishu response")
	}
	if *result.Code != 0 {
		return fmt.Errorf("Feishu rejected message (code %d): %s", *result.Code, result.Msg)
	}
	return nil
}
