package staff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const WeChatCode2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"

type HTTPExchanger struct {
	endpoint  string
	appID     string
	appSecret string
	client    *http.Client
}

func NewHTTPExchanger(endpoint, appID, appSecret string, client *http.Client) *HTTPExchanger {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPExchanger{endpoint: endpoint, appID: appID, appSecret: appSecret, client: client}
}

func (e *HTTPExchanger) Exchange(ctx context.Context, code string) (string, error) {
	endpoint, err := url.Parse(e.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse WeChat endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("appid", e.appID)
	query.Set("secret", e.appSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create WeChat request: %w", err)
	}
	response, err := e.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange WeChat code: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return "", fmt.Errorf("WeChat code exchange returned status %d", response.StatusCode)
	}

	var payload struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return "", fmt.Errorf("decode WeChat response: %w", err)
	}
	if payload.ErrCode != 0 {
		return "", fmt.Errorf("WeChat code exchange failed (%d): %s", payload.ErrCode, payload.ErrMsg)
	}
	if payload.OpenID == "" {
		return "", errors.New("WeChat code exchange returned no openid")
	}
	return payload.OpenID, nil
}
