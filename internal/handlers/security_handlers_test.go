package handlers

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egp/rcv-app/internal/security"
	_ "modernc.org/sqlite"
)

type stubTurnstile struct {
	ok  bool
	err error
}

func (s stubTurnstile) Verify(_ context.Context, _ string, _ string) (bool, error) {
	return s.ok, s.err
}

func setupTestHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	migrationsPath, err := testMigrationsPath()
	if err != nil {
		t.Fatalf("resolve migrations path: %v", err)
	}
	if err := runTestMigrations(db, migrationsPath); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	tpl := template.Must(template.New("base").Parse(`{{define "base"}}{{template "content" .}}{{end}}{{define "content"}}{{.Error}}{{end}}`))

	h := &Handler{
		DB:                db,
		TemplateCache:     map[string]*template.Template{"vote.html": tpl},
		Turnstile:         stubTurnstile{ok: true},
		TurnstileKey:      "site-key",
		AllowDevMultiVote: false,
	}

	return h, db
}

func runTestMigrations(db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(sqlBytes)) == "" {
			continue
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return err
		}
	}

	return nil
}

func testMigrationsPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	return filepath.Join(repoRoot, "db", "migrations"), nil
}

func seedPollWithCandidates(t *testing.T, db *sql.DB, key string) {
	t.Helper()

	res, err := db.Exec(`INSERT INTO polls (key, title, description) VALUES (?, ?, ?)`, key, "Poll", "")
	if err != nil {
		t.Fatalf("insert poll: %v", err)
	}
	pollID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("poll id: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)`, pollID, "A", 1); err != nil {
		t.Fatalf("insert candidate A: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)`, pollID, "B", 2); err != nil {
		t.Fatalf("insert candidate B: %v", err)
	}
}

func ballotCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ballots`).Scan(&n); err != nil {
		t.Fatalf("count ballots: %v", err)
	}
	return n
}

func pollCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM polls`).Scan(&n); err != nil {
		t.Fatalf("count polls: %v", err)
	}
	return n
}

func TestCreatePollHandler_HoneypotBlocksSubmission(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()

	body := url.Values{}
	body.Set("title", "Team Lunch")
	body.Set("description", "desc")
	body.Set("candidate_1", "A")
	body.Set("candidate_2", "B")
	body.Set("candidate_3", "C")
	body.Set("website", "https://spam.example")

	req := httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.CreatePollHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := pollCount(t, db); got != 0 {
		t.Fatalf("poll count = %d, want 0", got)
	}
}

func TestCreatePollHandler_RateLimit(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()

	limiter := security.NewRateLimiter(1, time.Minute)
	routed := security.WrapWithRateLimit(limiter, h.CreatePollHandler)

	newReq := func() *http.Request {
		body := url.Values{}
		body.Set("title", "Team Lunch")
		body.Set("candidate_1", "A")
		body.Set("candidate_2", "B")
		body.Set("candidate_3", "C")
		req := httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "127.0.0.1:12000"
		return req
	}

	w1 := httptest.NewRecorder()
	routed(w1, newReq())
	if w1.Code != http.StatusSeeOther {
		t.Fatalf("first status = %d, want %d", w1.Code, http.StatusSeeOther)
	}

	w2 := httptest.NewRecorder()
	routed(w2, newReq())
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}
}

func TestSubmitBallotHandler_MissingTurnstileRejected(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()
	seedPollWithCandidates(t, db, "abcd1234")

	body := url.Values{}
	body.Set("rankings", "1,2")

	req := httptest.NewRequest(http.MethodPost, "/polls/abcd1234/vote", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("key", "abcd1234")
	w := httptest.NewRecorder()

	h.SubmitBallotHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := ballotCount(t, db); got != 0 {
		t.Fatalf("ballot count = %d, want 0", got)
	}
}

func TestSubmitBallotHandler_SetsCookieOnSuccess(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()
	seedPollWithCandidates(t, db, "abcd1234")
	h.Turnstile = stubTurnstile{ok: true}

	body := url.Values{}
	body.Set("rankings", "1,2")
	body.Set("cf-turnstile-response", "token-ok")

	req := httptest.NewRequest(http.MethodPost, "/polls/abcd1234/vote", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("key", "abcd1234")
	w := httptest.NewRecorder()

	h.SubmitBallotHandler(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if got := ballotCount(t, db); got != 1 {
		t.Fatalf("ballot count = %d, want 1", got)
	}
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == voteCookieName("abcd1234") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected vote cookie to be set")
	}
}

func TestSubmitBallotHandler_BlocksRepeatVoteByCookie(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()
	seedPollWithCandidates(t, db, "abcd1234")
	h.Turnstile = stubTurnstile{ok: true}

	body := url.Values{}
	body.Set("rankings", "1,2")
	body.Set("cf-turnstile-response", "token-ok")

	req := httptest.NewRequest(http.MethodPost, "/polls/abcd1234/vote", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("key", "abcd1234")
	req.AddCookie(&http.Cookie{Name: voteCookieName("abcd1234"), Value: "voted"})
	w := httptest.NewRecorder()

	h.SubmitBallotHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := ballotCount(t, db); got != 0 {
		t.Fatalf("ballot count = %d, want 0", got)
	}
}

func TestSubmitBallotHandler_DevMultiVoteAllowsUnlimited(t *testing.T) {
	h, db := setupTestHandler(t)
	defer db.Close()
	seedPollWithCandidates(t, db, "abcd1234")
	h.Turnstile = stubTurnstile{ok: true}
	h.AllowDevMultiVote = true

	postVote := func() int {
		body := url.Values{}
		body.Set("rankings", "1,2")
		body.Set("cf-turnstile-response", "token-ok")
		req := httptest.NewRequest(http.MethodPost, "/polls/abcd1234/vote", strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("key", "abcd1234")
		req.AddCookie(&http.Cookie{Name: voteCookieName("abcd1234"), Value: "voted"})
		w := httptest.NewRecorder()
		h.SubmitBallotHandler(w, req)
		return w.Code
	}

	if status := postVote(); status != http.StatusSeeOther {
		t.Fatalf("first status = %d, want %d", status, http.StatusSeeOther)
	}
	if status := postVote(); status != http.StatusSeeOther {
		t.Fatalf("second status = %d, want %d", status, http.StatusSeeOther)
	}

	if got := ballotCount(t, db); got != 2 {
		t.Fatalf("ballot count = %d, want 2", got)
	}
}
