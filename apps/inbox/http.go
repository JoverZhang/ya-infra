package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *application) session() string {
	m := hmac.New(sha256.New, []byte(a.config.Password))
	m.Write([]byte("inbox-session-v1"))
	return hex.EncodeToString(m.Sum(nil))
}
func (a *application) authorized(r *http.Request) bool {
	c, err := r.Cookie("inbox-session")
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.session())) == 1
}
func sameOrigin(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Host == r.Host
}
func (a *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if a.db.PingContext(r.Context()) != nil {
			http.Error(w, "store unavailable", 503)
			return
		}
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if r.ParseForm() != nil || subtle.ConstantTimeCompare([]byte(r.Form.Get("password")), []byte(a.config.Password)) != 1 {
			http.Error(w, "invalid password", 401)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "inbox-session", Value: a.session(), Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"})
		http.Redirect(w, r, "/", 303)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Inbox · Sign in</title><body style="font:18px system-ui;background:#10151c;color:#e8eef5;max-width:420px;margin:12vh auto;padding:24px"><h1>Personal inbox</h1><p>Sign in to view your tasks and deployment status.</p><form method="post" action="/login"><input type="password" name="password" autocomplete="current-password" required placeholder="Access password"><button>Sign in</button></form></body></html>`))
			return
		}
		data, err := assets.ReadFile("index.html")
		if err != nil {
			http.Error(w, "page unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			http.Error(w, "authentication required", 401)
			return
		}
		items := []item{}
		rows, err := a.db.QueryContext(r.Context(), "SELECT id,source,title,url,reasons,version,status,read,snoozed FROM items ORDER BY status,source,id")
		if err != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var it item
			var reasons string
			if rows.Scan(&it.ID, &it.Source, &it.Title, &it.URL, &reasons, &it.Version, &it.Status, &it.Read, &it.SnoozedUntil) != nil {
				http.Error(w, "store unavailable", 500)
				return
			}
			if json.Unmarshal([]byte(reasons), &it.Reasons) != nil {
				http.Error(w, "store unavailable", 500)
				return
			}
			items = append(items, it)
		}
		if rows.Err() != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		rows.Close()
		observations := []observation{}
		rows, err = a.db.QueryContext(r.Context(), "SELECT source,ok,checked,success,detail FROM observations ORDER BY source")
		if err != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var o observation
			if rows.Scan(&o.Source, &o.OK, &o.Checked, &o.Success, &o.Detail) != nil {
				http.Error(w, "store unavailable", 500)
				return
			}
			observations = append(observations, o)
		}
		if rows.Err() != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Items   []item        `json:"items"`
			Sources []observation `json:"sources"`
		}{items, observations})
	})
	mux.HandleFunc("POST /api/state", func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			http.Error(w, "authentication required", 401)
			return
		}
		if !sameOrigin(r) || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "forbidden", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var change struct {
			ID     string `json:"id"`
			Read   bool   `json:"read"`
			Snooze int    `json:"snooze_hours"`
		}
		if json.NewDecoder(r.Body).Decode(&change) != nil || change.ID == "" || change.Snooze < 0 || change.Snooze > 168 {
			http.Error(w, "invalid state", 400)
			return
		}
		until := int64(0)
		if change.Snooze > 0 {
			until = time.Now().Add(time.Duration(change.Snooze) * time.Hour).Unix()
		}
		result, err := a.db.ExecContext(r.Context(), "UPDATE items SET read=?,snoozed=? WHERE id=?", change.Read, until, change.ID)
		if err != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "store unavailable", 500)
			return
		}
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(204)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; form-action 'self'")
		mux.ServeHTTP(w, r)
	})
}
