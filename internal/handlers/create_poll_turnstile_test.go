package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createPollForm(token string) url.Values {
	v := url.Values{}
	v.Set("title", "Lunch spot")
	v.Set("description", "Friday team lunch")
	v.Set("candidate_1", "Tacos")
	v.Set("candidate_2", "Pho")
	v.Set("candidate_3", "Pizza")
	if token != "" {
		v.Set("cf-turnstile-response", token)
	}
	return v
}

func postCreatePoll(h *Handler, body url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.CreatePollHandler(w, req)
	return w
}

func setupCreateTestHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	h, db := setupTestHandler(t)
	h.TemplateCache["home.html"] = h.TemplateCache["vote.html"]
	return h, db
}

func TestCreatePollHandler_MissingTurnstileRejected(t *testing.T) {
	h, db := setupCreateTestHandler(t)
	defer db.Close()

	w := postCreatePoll(h, createPollForm(""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "verification challenge") {
		t.Fatalf("body = %q, want verification error", w.Body.String())
	}
	if got := pollCount(t, db); got != 0 {
		t.Fatalf("poll count = %d, want 0", got)
	}
}

func TestCreatePollHandler_InvalidTurnstileRejected(t *testing.T) {
	h, db := setupCreateTestHandler(t)
	defer db.Close()
	h.Turnstile = stubTurnstile{ok: false}

	w := postCreatePoll(h, createPollForm("bad-token"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "Verification failed") {
		t.Fatalf("body = %q, want verification failed error", w.Body.String())
	}
	if got := pollCount(t, db); got != 0 {
		t.Fatalf("poll count = %d, want 0", got)
	}
}

func TestCreatePollHandler_TurnstileErrorRejected(t *testing.T) {
	h, db := setupCreateTestHandler(t)
	defer db.Close()
	h.Turnstile = stubTurnstile{err: errors.New("network down")}

	w := postCreatePoll(h, createPollForm("token"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := pollCount(t, db); got != 0 {
		t.Fatalf("poll count = %d, want 0", got)
	}
}

func TestCreatePollHandler_ValidTurnstileCreatesPoll(t *testing.T) {
	h, db := setupCreateTestHandler(t)
	defer db.Close()
	h.Turnstile = stubTurnstile{ok: true}

	w := postCreatePoll(h, createPollForm("token-ok"))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); !strings.HasSuffix(loc, "/manage") {
		t.Fatalf("Location = %q, want manage page", loc)
	}
	if got := pollCount(t, db); got != 1 {
		t.Fatalf("poll count = %d, want 1", got)
	}
}

func TestCreatePollHandler_DisabledTurnstileSkipsCheck(t *testing.T) {
	h, db := setupCreateTestHandler(t)
	defer db.Close()
	h.DisableTurnstile = true
	h.Turnstile = nil

	w := postCreatePoll(h, createPollForm(""))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if got := pollCount(t, db); got != 1 {
		t.Fatalf("poll count = %d, want 1", got)
	}
}

// Renders the real home template to confirm the widget is present and the
// entered values survive a failed submission.
func TestCreatePollHandler_RealTemplatePreservesValuesAndShowsWidget(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()

	wd, _ := os.Getwd()
	root, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	cache, err := newTemplateCache()
	if err != nil {
		t.Fatalf("template cache: %v", err)
	}
	h.TemplateCache = cache
	h.Turnstile = stubTurnstile{ok: false}

	w := postCreatePoll(h, createPollForm("bad-token"))
	body := w.Body.String()

	for _, want := range []string{
		`class="cf-turnstile" data-sitekey="site-key"`,
		"challenges.cloudflare.com/turnstile/v0/api.js",
		`value="Lunch spot"`,
		"Friday team lunch",
		`value="Tacos"`,
		`value="Pizza"`,
		"Verification failed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if pollCount(t, db) != 0 {
		t.Fatal("poll should not be created")
	}
}
