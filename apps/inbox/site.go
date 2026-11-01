package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (a *application) checkSite(ctx context.Context, s siteConfig) ([]item, error) {
	var reasons []string
	if s.SyncError != "" {
		if info, e := os.Stat(s.SyncError); e == nil && info.Size() > 0 {
			reasons = append(reasons, "Repository synchronization failed")
		} else if e != nil && !os.IsNotExist(e) {
			reasons = append(reasons, "Synchronization status is unavailable")
		}
	}
	for _, probe := range []struct{ label, raw string }{{"Local page", s.Origin}, {"Public page", s.Public}, {"Sync process", s.SyncURL}} {
		if probe.raw == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, "GET", probe.raw, nil)
		if err != nil {
			return nil, errors.New("invalid page probe URL")
		}
		res, err := a.github.client.Do(req)
		if err != nil {
			reasons = append(reasons, probe.label+" is unreachable")
			continue
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			reasons = append(reasons, fmt.Sprintf("%s returned HTTP %d", probe.label, res.StatusCode))
		}
	}
	link, err := os.Readlink(s.Current)
	if err != nil {
		reasons = append(reasons, "No published revision is available")
	} else {
		var head struct {
			SHA string `json:"sha"`
		}
		err = a.github.get(ctx, "/repos/"+s.Repository+"/commits/"+url.PathEscape(s.Branch), &head)
		if err != nil {
			reasons = append(reasons, "Upstream revision is unknown: "+err.Error())
		} else if filepath.Base(link) != head.SHA {
			reasons = append(reasons, "Published revision is behind the selected branch")
		}
	}
	if len(reasons) == 0 {
		return nil, nil
	}
	return []item{{ID: "site:" + s.ID, Title: s.Label, URL: s.Public, Reasons: reasons, Version: strings.Join(reasons, ";"), Status: "active"}}, nil
}
