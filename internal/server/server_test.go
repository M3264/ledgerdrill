package server

import (
	"context"
	"ledgerdrill/internal/lab"
	"ledgerdrill/internal/store"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessAndOriginGuards(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	p := lab.NewProxy(db)
	engine := lab.NewEngine(db, p, "http://localhost:8098", "a-test-token", context.Background())
	s, e := New(db, engine, p, "a-test-token")
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		path, method, token, origin string
		status                      int
	}{{"/api/config", "GET", "", "", 200}, {"/api/runs", "GET", "", "", 401}, {"/api/runs", "GET", "a-test-token", "", 200}, {"/api/sessions", "POST", "a-test-token", "http://evil.invalid", 403}, {"/api/sessions", "POST", "a-test-token", "http://example.com", 201}, {"/api/runs", "POST", "a-test-token", "http://example.com", 400}}
	for _, c := range cases {
		body := `{"scenario":"lost_confirmation"}`
		if c.path == "/api/runs" && c.method == "POST" {
			body = `{"scenario":"invalid"}`
		}
		r := httptest.NewRequest(c.method, c.path, strings.NewReader(body))
		r.Host = "example.com"
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%s %s got %d want %d: %s", c.method, c.path, w.Code, c.status, w.Body.String())
		}
	}
}
func TestInterruptedRunIsNeverAutomaticallyReplayed(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := lab.Run{ID: "unfinished", Status: "running", Clients: []lab.ClientResult{{ID: "naive", Transactions: []lab.Evidence{{Hash: "original-hash"}}}}}
	db.Put("run", r.ID, r)
	engine := lab.NewEngine(db, lab.NewProxy(db), "http://localhost:8098", "", context.Background())
	if e = engine.Recover(); e != nil {
		t.Fatal(e)
	}
	db.Get("run", r.ID, &r)
	if r.Status != "interrupted" || r.Clients[0].Transactions[0].Hash != "original-hash" || engine.Busy() {
		t.Fatal("interrupted state lost or resumed automatically")
	}
}
