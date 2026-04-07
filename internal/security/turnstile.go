package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type TurnstileVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) (bool, error)
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type CloudflareTurnstileVerifier struct {
	secret string
	client HTTPClient
}

type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func NewCloudflareTurnstileVerifier(secret string, client HTTPClient) *CloudflareTurnstileVerifier {
	return &CloudflareTurnstileVerifier{secret: strings.TrimSpace(secret), client: client}
}

func (v *CloudflareTurnstileVerifier) Verify(ctx context.Context, token string, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}
	if v.secret == "" {
		return false, fmt.Errorf("turnstile secret is not configured")
	}

	values := url.Values{}
	values.Set("secret", v.secret)
	values.Set("response", token)
	if strings.TrimSpace(remoteIP) != "" {
		values.Set("remoteip", strings.TrimSpace(remoteIP))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(values.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("turnstile verify unexpected status: %s", resp.Status)
	}

	var payload turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, err
	}

	return payload.Success, nil
}
