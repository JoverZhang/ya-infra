package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the same HTTP routes, collectors and SQLite store used in deployment.
func TestInboxRecovery(t *testing.T) {
	fail := false
	scoped := true
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("GitHub write attempted: %s", r.Method)
		}
		if fail {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if scoped {
			w.Header().Set("X-OAuth-Scopes", "repo, notifications")
		} else {
			w.Header().Set("X-OAuth-Scopes", "notifications")
		}
		switch r.URL.Path {
		case "/user":
			w.Write([]byte(`{"login":"fixture-user"}`))
		case "/notifications":
			w.Write([]byte(`[{"reason":"mention","updated_at":"2025-01-01T00:00:00Z","repository":{"full_name":"example/project"},"subject":{"title":"Discuss change","type":"PullRequest","url":"https://api.github.com/repos/example/project/pulls/7"}}]`))
		case "/search/issues":
			w.Write([]byte(`{"total_count":1,"incomplete_results":false,"items":[{"title":"Discuss change","html_url":"https://github.com/example/project/pull/7","updated_at":"2025-01-01T00:00:00Z"}]}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	file := filepath.Join(t.TempDir(), "inbox.sqlite")
	store, err := openStore(file)
	if err != nil {
		t.Fatal(err)
	}
	app := &application{db: store, config: configuration{Password: "fixture-password", Sites: []siteConfig{}}, github: githubClient{base: api.URL, token: "fixture-token", client: api.Client()}}
	app.collect(context.Background())
	handler := app.routes()
	req := httptest.NewRequest("GET", "/api/items", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatalf("private items disclosed: %d", res.Code)
	}
	login := httptest.NewRequest("POST", "/login", strings.NewReader("password=fixture-password"))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, login)
	if res.Code != 303 || len(res.Result().Cookies()) == 0 {
		t.Fatalf("login: %d", res.Code)
	}
	cookie := res.Result().Cookies()[0]
	readItems := func() []item {
		t.Helper()
		req = httptest.NewRequest("GET", "/api/items", nil)
		req.AddCookie(cookie)
		res = httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("items: %d", res.Code)
		}
		var data struct{ Items []item }
		if err = json.Unmarshal(res.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data.Items
	}
	items := readItems()
	if len(items) != 1 || len(items[0].Reasons) != 4 {
		t.Fatalf("duplicates not merged: %+v", items)
	}
	action := httptest.NewRequest("POST", "/api/state", strings.NewReader(`{"id":"`+items[0].ID+`","read":true,"snooze_hours":1}`))
	action.AddCookie(cookie)
	action.Header.Set("Content-Type", "application/json")
	action.Header.Set("Origin", "https://untrusted.example")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, action)
	if res.Code != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	action = httptest.NewRequest("POST", "/api/state", strings.NewReader(`{"id":"`+items[0].ID+`","read":true,"snooze_hours":1}`))
	action.AddCookie(cookie)
	action.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, action)
	if res.Code != 204 {
		t.Fatalf("state: %d", res.Code)
	}
	store.Close()
	store, err = openStore(file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app.db = store
	items = readItems()
	if !items[0].Read || items[0].SnoozedUntil == 0 {
		t.Fatal("state lost on restart")
	}
	scoped = false
	app.collect(context.Background())
	items = readItems()
	if len(items) != 2 {
		t.Fatal("insufficient scopes silently accepted")
	}
	scoped = true
	fail = true
	app.collect(context.Background())
	items = readItems()
	if len(items) != 2 {
		t.Fatalf("failed collector erased existing task: %+v", items)
	}
	fail = false
	app.collect(context.Background())
	items = readItems()
	for _, it := range items {
		if it.ID == "source:github" && it.Status != "resolved" {
			t.Fatal("source recovery not resolved")
		}
	}
	t.Log(`{"authentication":"required","github":"read-only","dedup":"pass","restart":"persisted","outage":"retained","recovery":"resolved","csrf":"refused"}`)
}

func TestSiteFaultLifecycle(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "current")
	errorFile := filepath.Join(dir, "sync-error")
	hash := strings.Repeat("a", 40)
	if err := os.Symlink(hash, link); err != nil {
		t.Fatal(err)
	}
	originOK := true
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !originOK {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte("site"))
	}))
	defer origin.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"sha":"` + hash + `"}`)) }))
	defer api.Close()
	db, err := openStore(filepath.Join(dir, "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &application{db: db, github: githubClient{base: api.URL, token: "fixture", client: api.Client()}}
	site := siteConfig{ID: "docs", Label: "Documentation", Repository: "example/docs", Branch: "pages", Current: link, SyncError: errorFile, Origin: origin.URL, Public: origin.URL}
	items, err := app.checkSite(context.Background(), site)
	if err != nil || len(items) != 0 {
		t.Fatalf("healthy site: %v %+v", err, items)
	}
	if err = os.WriteFile(errorFile, []byte("PRIVATE URL AND TOKEN MUST NOT APPEAR"), 0600); err != nil {
		t.Fatal(err)
	}
	originOK = false
	items, err = app.checkSite(context.Background(), site)
	if err != nil || len(items) != 1 || len(items[0].Reasons) != 3 {
		t.Fatalf("fault: %v %+v", err, items)
	}
	encoded, _ := json.Marshal(items)
	if strings.Contains(string(encoded), "PRIVATE") {
		t.Fatal("raw sync diagnostics disclosed")
	}
	if err = app.save("site:docs", items, nil); err != nil {
		t.Fatal(err)
	}
	os.Remove(errorFile)
	originOK = true
	items, err = app.checkSite(context.Background(), site)
	if err != nil || len(items) != 0 {
		t.Fatal("recovery not detected")
	}
	if err = app.save("site:docs", items, nil); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = db.QueryRow("SELECT status FROM items WHERE id='site:docs'").Scan(&status); err != nil || status != "resolved" {
		t.Fatal("fault did not resolve")
	}
	t.Log(`{"site":"origin/public/sync checked","privacy":"raw errors withheld","recovery":"resolved"}`)
}

func TestGitHubRetryDeadline(t *testing.T) {
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	}))
	defer api.Close()
	client := githubClient{base: api.URL, token: "fixture", client: api.Client()}
	var value struct{}
	if client.get(context.Background(), "/user", &value) == nil {
		t.Fatal("rate limit not reported")
	}
	if client.get(context.Background(), "/repos/example/site/commits/pages", &value) == nil {
		t.Fatal("deferred request not reported")
	}
	if requests != 1 {
		t.Fatalf("issued %d requests before retry deadline", requests)
	}
}

func TestPartialSSOCoverage(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-GitHub-SSO", "partial-results; organizations=123")
		w.Write([]byte(`{"total_count":0,"items":[]}`))
	}))
	defer api.Close()
	client := githubClient{base: api.URL, token: "fixture", client: api.Client()}
	var value struct{}
	if client.get(context.Background(), "/search/issues?q=example", &value) == nil {
		t.Fatal("partial SSO coverage accepted as complete")
	}
}

func TestAssignmentNotification(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo, notifications")
		switch r.URL.Path {
		case "/user":
			w.Write([]byte(`{"login":"fixture"}`))
		case "/search/issues":
			w.Write([]byte(`{"total_count":0,"items":[]}`))
		case "/notifications":
			w.Write([]byte(`[{"reason":"assign","updated_at":"2025-01-01T00:00:00Z","repository":{"full_name":"example/project"},"subject":{"title":"Assigned change","type":"PullRequest","url":"https://api.github.com/repos/example/project/pulls/8"}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	client := githubClient{base: api.URL, token: "fixture", client: api.Client()}
	items, err := client.tasks(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("assigned PR missing: %v %+v", err, items)
	}
}
