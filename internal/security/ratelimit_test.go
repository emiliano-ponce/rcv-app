package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)

	if !limiter.Allow("127.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("127.0.0.1") {
		t.Fatal("second request should be allowed")
	}
	if limiter.Allow("127.0.0.1") {
		t.Fatal("third request should be limited")
	}
}

func TestWrapWithRateLimit(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := WrapWithRateLimit(limiter, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req1 := httptest.NewRequest(http.MethodPost, "/polls", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	w1 := httptest.NewRecorder()
	handler(w1, req1)
	if w1.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", w1.Code, http.StatusNoContent)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/polls", nil)
	req2.RemoteAddr = "127.0.0.1:12346"
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}
}
