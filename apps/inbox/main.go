package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed index.html
var assets embed.FS

type configuration struct {
	Listen    string       `yaml:"listen"`
	Database  string       `yaml:"database"`
	TokenFile string       `yaml:"github_token_file"`
	Password  string       `yaml:"password"`
	Sites     []siteConfig `yaml:"sites"`
}
type siteConfig struct {
	ID         string `yaml:"id"`
	Label      string `yaml:"label"`
	Repository string `yaml:"repository"`
	Branch     string `yaml:"branch"`
	Current    string `yaml:"current_link"`
	SyncError  string `yaml:"sync_error_file"`
	SyncURL    string `yaml:"sync_url"`
	Origin     string `yaml:"origin_url"`
	Public     string `yaml:"public_url"`
}
type item struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"`
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	Reasons      []string `json:"reasons"`
	Version      string   `json:"version"`
	Status       string   `json:"status"`
	Read         bool     `json:"read"`
	SnoozedUntil int64    `json:"snoozed_until"`
}
type observation struct {
	Source  string `json:"source"`
	OK      bool   `json:"ok"`
	Checked int64  `json:"checked"`
	Success int64  `json:"success"`
	Detail  string `json:"detail"`
}
type application struct {
	db     *sql.DB
	config configuration
	github githubClient
}

func main() {
	if err := run(); err != nil {
		log.Print("inbox stopped: ", err)
		os.Exit(1)
	}
}
func run() error {
	path := os.Getenv("INBOX_CONFIG")
	if path == "" {
		path = "/run/secrets/inbox-config"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("read configuration: check secret mount")
	}
	var cfg configuration
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if dec.Decode(&cfg) != nil {
		return errors.New("invalid configuration fields or types")
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:18580"
	}
	if cfg.Database == "" {
		cfg.Database = "/data/inbox.sqlite"
	}
	if len(cfg.Password) < 16 {
		return errors.New("configure an access password of at least 16 characters")
	}
	ids := map[string]bool{}
	for _, s := range cfg.Sites {
		if s.ID == "" || ids[s.ID] || s.Label == "" || s.Current == "" || s.Origin == "" || s.Repository == "" || s.Branch == "" {
			return errors.New("sites need unique IDs, labels, repository, branch, current_link and origin_url")
		}
		ids[s.ID] = true
		for _, raw := range []string{s.Origin, s.Public} {
			if raw != "" {
				u, e := url.Parse(raw)
				if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
					return errors.New("site probes require HTTP(S) URLs without credentials")
				}
			}
		}
	}
	db, err := openStore(cfg.Database)
	if err != nil {
		return fmt.Errorf("open persistent store: %w", err)
	}
	defer db.Close()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	app := &application{db: db, config: cfg, github: githubClient{base: "https://api.github.com", client: client}}
	if cfg.TokenFile != "" {
		token, e := os.ReadFile(cfg.TokenFile)
		if e != nil {
			return errors.New("read GitHub token: check secret mount")
		}
		app.github.token = strings.TrimSpace(string(token))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			app.collect(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(app.github.interval()):
			}
		}
	}()
	server := &http.Server{Addr: cfg.Listen, Handler: app.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if e := server.Shutdown(shutdown); e != nil {
			log.Print("HTTP shutdown incomplete")
		}
	}()
	log.Print("inbox listening")
	err = server.ListenAndServe()
	stop()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (a *application) collect(ctx context.Context) {
	items, err := a.github.tasks(ctx)
	if e := a.save("github", items, err); e != nil {
		log.Print("store GitHub observation failed")
	}
	for _, site := range a.config.Sites {
		items, err := a.checkSite(ctx, site)
		if e := a.save("site:"+site.ID, items, err); e != nil {
			log.Print("store site observation failed")
		}
	}
}
