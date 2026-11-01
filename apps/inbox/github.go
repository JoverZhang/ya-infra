package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type githubClient struct {
	base, token string
	client      *http.Client
	poll        time.Duration
}

func (g *githubClient) interval() time.Duration {
	if g.poll > 60*time.Second {
		return g.poll
	}
	return 60 * time.Second
}
func (g *githubClient) get(ctx context.Context, path string, out any) error {
	if g.token == "" {
		return errors.New("GitHub token is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", g.base+path, nil)
	if err != nil {
		return errors.New("invalid GitHub request")
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "personal-inbox")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := g.client.Do(req)
	if err != nil {
		return errors.New("GitHub request failed")
	}
	defer res.Body.Close()
	if seconds, e := strconv.Atoi(res.Header.Get("X-Poll-Interval")); e == nil && time.Duration(seconds)*time.Second > g.poll {
		g.poll = time.Duration(seconds) * time.Second
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("GitHub HTTP %d", res.StatusCode)
	}
	if json.NewDecoder(res.Body).Decode(out) != nil {
		return errors.New("invalid GitHub response")
	}
	return nil
}
func (g *githubClient) tasks(ctx context.Context) ([]item, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := g.get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	if user.Login == "" {
		return nil, errors.New("GitHub account is unavailable")
	}
	merged := map[string]*item{}
	add := func(title, raw, reason, version string) error {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil {
			return errors.New("invalid GitHub item link")
		}
		sum := sha256.Sum256([]byte(raw))
		id := "github:" + hex.EncodeToString(sum[:])
		it := merged[id]
		if it == nil {
			it = &item{ID: id, Title: title, URL: raw, Status: "active"}
			merged[id] = it
		}
		found := false
		for _, r := range it.Reasons {
			if r == reason {
				found = true
			}
		}
		if !found {
			it.Reasons = append(it.Reasons, reason)
		}
		if version > it.Version {
			it.Version = version
		}
		return nil
	}
	for _, query := range []struct{ filter, reason string }{{"is:pr is:open review-requested:", "Review requested"}, {"is:issue is:open assignee:", "Assigned issue"}, {"is:pr is:open author:", "Changes requested"}} {
		filter := query.filter + user.Login
		if query.reason == "Changes requested" {
			filter += " review:changes_requested"
		}
		for page := 1; page <= 10; page++ {
			var data struct {
				Total      int  `json:"total_count"`
				Incomplete bool `json:"incomplete_results"`
				Items      []struct {
					Title   string `json:"title"`
					URL     string `json:"html_url"`
					Updated string `json:"updated_at"`
				} `json:"items"`
			}
			path := "/search/issues?q=" + url.QueryEscape(filter) + "&per_page=100&page=" + strconv.Itoa(page)
			if err := g.get(ctx, path, &data); err != nil {
				return nil, err
			}
			if data.Incomplete || data.Total > 1000 {
				return nil, errors.New("GitHub search coverage is incomplete")
			}
			for _, it := range data.Items {
				if err := add(it.Title, it.URL, query.reason, it.Updated); err != nil {
					return nil, err
				}
			}
			if page*100 >= data.Total {
				break
			}
		}
	}
	for page := 1; page <= 100; page++ {
		var data []struct {
			Reason     string                            `json:"reason"`
			Updated    string                            `json:"updated_at"`
			Subject    struct{ Title, Type, URL string } `json:"subject"`
			Repository struct {
				Name string `json:"full_name"`
			} `json:"repository"`
		}
		if err := g.get(ctx, "/notifications?all=false&per_page=100&page="+strconv.Itoa(page), &data); err != nil {
			return nil, err
		}
		for _, n := range data {
			if n.Reason != "mention" && n.Reason != "team_mention" && n.Reason != "review_requested" && n.Reason != "comment" {
				continue
			}
			kind := ""
			if n.Subject.Type == "PullRequest" {
				kind = "pull"
			} else if n.Subject.Type == "Issue" {
				kind = "issues"
			} else {
				continue
			}
			subject, err := url.Parse(n.Subject.URL)
			if err != nil {
				return nil, errors.New("invalid notification reference")
			}
			parts := strings.Split(strings.Trim(subject.Path, "/"), "/")
			if len(parts) != 5 || parts[0] != "repos" || parts[1]+"/"+parts[2] != n.Repository.Name {
				return nil, errors.New("invalid notification reference")
			}
			if _, err = strconv.Atoi(parts[4]); err != nil {
				return nil, errors.New("invalid notification number")
			}
			raw := "https://github.com/" + n.Repository.Name + "/" + kind + "/" + parts[4]
			if err = add(n.Subject.Title, raw, "Unread "+strings.ReplaceAll(n.Reason, "_", " "), n.Updated); err != nil {
				return nil, err
			}
		}
		if len(data) < 100 {
			break
		}
		if page == 100 {
			return nil, errors.New("GitHub notifications coverage is incomplete")
		}
	}
	result := make([]item, 0, len(merged))
	for _, it := range merged {
		result = append(result, *it)
	}
	return result, nil
}
