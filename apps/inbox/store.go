package main

import (
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func openStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS items(id TEXT PRIMARY KEY,source TEXT NOT NULL,title TEXT NOT NULL,url TEXT NOT NULL,reasons TEXT NOT NULL,version TEXT NOT NULL,status TEXT NOT NULL,read INTEGER NOT NULL DEFAULT 0,snoozed INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS observations(source TEXT PRIMARY KEY,ok INTEGER NOT NULL,checked INTEGER NOT NULL,success INTEGER NOT NULL,detail TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func (a *application) save(source string, items []item, collectionErr error) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	detail := "OK"
	ok := collectionErr == nil
	if !ok {
		detail = collectionErr.Error()
	} else {
		// Resolve only after a complete successful snapshot. Preserve cached tasks on failure.
		for _, it := range items {
			reasons, _ := json.Marshal(it.Reasons)
			_, err = tx.Exec(`INSERT INTO items(id,source,title,url,reasons,version,status) VALUES(?,?,?,?,?,?,'active') ON CONFLICT(id) DO UPDATE SET title=excluded.title,url=excluded.url,reasons=excluded.reasons,read=CASE WHEN items.status='resolved' OR items.version!=excluded.version THEN 0 ELSE items.read END,version=excluded.version,status='active'`, it.ID, source, it.Title, it.URL, string(reasons), it.Version)
			if err != nil {
				return err
			}
		}
		args := []any{source}
		marks := []string{}
		for _, it := range items {
			args = append(args, it.ID)
			marks = append(marks, "?")
		}
		query := "UPDATE items SET status='resolved',read=0 WHERE source=? AND status='active'"
		if len(marks) > 0 {
			query += " AND id NOT IN (" + strings.Join(marks, ",") + ")"
		}
		if _, err = tx.Exec(query, args...); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`INSERT INTO observations VALUES(?,?,?,?,?) ON CONFLICT(source) DO UPDATE SET ok=excluded.ok,checked=excluded.checked,success=CASE WHEN excluded.ok THEN excluded.checked ELSE observations.success END,detail=excluded.detail`, source, ok, now, func() int64 {
		if ok {
			return now
		}
		return 0
	}(), detail)
	if err != nil {
		return err
	}
	id := "source:" + source
	if ok {
		_, err = tx.Exec("UPDATE items SET read=CASE WHEN status='active' THEN 0 ELSE read END,status='resolved' WHERE id=?", id)
	} else {
		reasons, _ := json.Marshal([]string{detail})
		_, err = tx.Exec(`INSERT INTO items(id,source,title,url,reasons,version,status) VALUES(?,? ,?,'',?,?,'active') ON CONFLICT(id) DO UPDATE SET reasons=excluded.reasons,read=CASE WHEN items.status='resolved' OR items.version!=excluded.version THEN 0 ELSE items.read END,version=excluded.version,status='active'`, id, "health:"+source, "Source unavailable: "+source, string(reasons), detail)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
