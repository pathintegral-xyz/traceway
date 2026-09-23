package notifications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeishuAdapterValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		key  string
		want bool
	}{
		{"valid", "https://open.feishu.cn/open-apis/bot/v2/hook/abc", "secret", true},
		{"lark", "https://open.larksuite.com/open-apis/bot/v2/hook/abc", "secret", true},
		{"missing secret", "https://open.feishu.cn/open-apis/bot/v2/hook/abc", "", false},
		{"http", "http://open.feishu.cn/open-apis/bot/v2/hook/abc", "secret", false},
		{"other host", "https://example.com/open-apis/bot/v2/hook/abc", "secret", false},
		{"host suffix", "https://open.feishu.cn.evil.test/open-apis/bot/v2/hook/abc", "secret", false},
		{"missing hook", "https://open.feishu.cn/open-apis/bot/v2/hook/", "secret", false},
		{"query", "https://open.feishu.cn/open-apis/bot/v2/hook/abc?x=1", "secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := (&FeishuAdapter{WebhookURL: tc.url, SigningSecret: tc.key}).Validate() == nil
			if got != tc.want {
				t.Fatalf("Validate() success = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFeishuAdapterSend(t *testing.T) {
	secret := "signing-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		var payload struct {
			Timestamp int64  `json:"timestamp"`
			Sign      string `json:"sign"`
			MsgType   string `json:"msg_type"`
			Content   struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if time.Since(time.Unix(payload.Timestamp, 0)) > time.Minute {
			t.Errorf("invalid timestamp: %d", payload.Timestamp)
		}
		mac := hmac.New(sha256.New, []byte(fmt.Sprintf("%d\n%s", payload.Timestamp, secret)))
		if payload.Sign != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Error("invalid signature")
		}
		if payload.MsgType != "text" || payload.Content.Text != "Error detected\nStack details\nhttps://traceway.example/issues/1" {
			t.Errorf("unexpected message: %+v", payload)
		}
		fmt.Fprint(w, `{"code":0,"msg":"success"}`)
	}))
	defer server.Close()

	a := &FeishuAdapter{WebhookURL: server.URL, SigningSecret: secret}
	if err := a.Send(context.Background(), Message{Subject: "Error detected", Body: "Stack details", URL: "https://traceway.example/issues/1"}); err != nil {
		t.Fatal(err)
	}
}

func TestFeishuAdapterRejectsApplicationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":19021,"msg":"signature verification failed"}`)
	}))
	defer server.Close()

	a := &FeishuAdapter{WebhookURL: server.URL, SigningSecret: "wrong"}
	if err := a.Send(context.Background(), Message{Subject: "test"}); err == nil || !strings.Contains(err.Error(), "19021") {
		t.Fatalf("Send() error = %v, want application error", err)
	}
}
