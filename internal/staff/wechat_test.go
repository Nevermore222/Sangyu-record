package staff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPExchangerReturnsOpenID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != "app-id" || r.URL.Query().Get("js_code") != "temporary-code" || r.URL.Query().Get("grant_type") != "authorization_code" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"openid":"collector-openid","session_key":"secret"}`))
	}))
	defer server.Close()
	exchanger := NewHTTPExchanger(server.URL, "app-id", "app-secret", server.Client())
	openID, err := exchanger.Exchange(context.Background(), "temporary-code")
	if err != nil || openID != "collector-openid" {
		t.Fatalf("openid = %q, err = %v", openID, err)
	}
}

func TestHTTPExchangerRejectsWechatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer server.Close()
	exchanger := NewHTTPExchanger(server.URL, "app-id", "app-secret", server.Client())
	if _, err := exchanger.Exchange(context.Background(), "bad-code"); err == nil {
		t.Fatal("WeChat error was accepted")
	}
}
