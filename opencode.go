package main

// Bridge to the opencode CLI, used only when the user explicitly asks to
// hand a session over or to include opencode sessions in a search. The
// eisencode CLI itself stores everything on its own side.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func opencodeBin(cfg Config) string {
	if v := strings.TrimSpace(cfg.OpenCodeBin); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("EISENCODE_OPENCODE_BIN")); v != "" {
		return v
	}
	if p, err := exec.LookPath("opencode"); err == nil {
		return p
	}
	return ""
}

func opencodeMissing(cfg Config) bool {
	if opencodeBin(cfg) != "" {
		return false
	}
	fmt.Fprintln(os.Stderr, ansiDim+"opencode not found on PATH — skipping opencode sessions"+ansiReset)
	return true
}

// runOpenCode executes opencode with an optional working directory. opencode
// scopes its session list to the current project, so the directory matters.
func runOpenCode(cfg Config, dir string, args ...string) ([]byte, error) {
	bin := opencodeBin(cfg)
	if bin == "" {
		return nil, fmt.Errorf("opencode not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}

type ocSession struct {
	ID    string
	Title string
	When  string
}

// listOpenCodeSessions uses the machine-readable list so titles containing tabs
// or unusual spacing cannot break parsing.
func listOpenCodeSessions(cfg Config, dir string, limit int) ([]ocSession, error) {
	if limit <= 0 {
		limit = 100
	}
	out, err := runOpenCode(cfg, dir, "session", "list", "--format", "json", "-n", fmt.Sprint(limit))
	if err != nil {
		return nil, err
	}
	var raw []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Updated int64  `json:"updated"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("could not parse opencode sessions: %w", err)
	}
	rows := make([]ocSession, 0, len(raw))
	for _, r := range raw {
		if !strings.HasPrefix(r.ID, "ses_") {
			continue
		}
		when := ""
		if r.Updated > 0 {
			when = time.UnixMilli(r.Updated).Local().Format("2006-01-02 15:04")
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = "untitled"
		}
		rows = append(rows, ocSession{ID: r.ID, Title: title, When: when})
	}
	return rows, nil
}

type ocExport struct {
	Info struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Model struct {
			ID         string `json:"id"`
			ProviderID string `json:"providerID"`
			Variant    string `json:"variant"`
		} `json:"model"`
		Time struct {
			Created int64 `json:"created"`
			Updated int64 `json:"updated"`
		} `json:"time"`
		Location struct {
			Directory string `json:"directory"`
		} `json:"location"`
	} `json:"info"`
	Messages []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"messages"`
}

func exportOpenCodeSession(cfg Config, id, dir string) (*ocExport, error) {
	out, err := runOpenCode(cfg, dir, "session", "export", id)
	if err != nil {
		return nil, err
	}
	var doc ocExport
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("could not parse export: %w", err)
	}
	return &doc, nil
}

func ocExportText(doc *ocExport) string {
	var b strings.Builder
	for _, m := range doc.Messages {
		if m.Text != "" {
			b.WriteString(m.Text)
			b.WriteString("\n")
		}
		for _, c := range m.Content {
			if c.Type == "text" && c.Text != "" {
				b.WriteString(c.Text)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func searchOpenCode(cfg Config, query string, limit int, dir string, into *[]SearchHit) {
	if opencodeMissing(cfg) {
		return
	}
	rows, err := listOpenCodeSessions(cfg, dir, 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sopencode sessions unavailable: %v%s\n", ansiDim, err, ansiReset)
		return
	}
	budget := limit
	for _, row := range rows {
		if len(*into) >= limit {
			return
		}
		if sn, ok := matchSnippet(row.Title, query); ok {
			*into = append(*into, SearchHit{
				Source: "opencode", ID: row.ID, Title: row.Title, When: row.When, Snippet: sn,
			})
			continue
		}
		if budget <= 0 {
			continue
		}
		doc, err := exportOpenCodeSession(cfg, row.ID, dir)
		if err != nil {
			continue
		}
		budget--
		if sn, ok := matchSnippet(ocExportText(doc), query); ok {
			title := row.Title
			if title == "" {
				title = doc.Info.Title
			}
			*into = append(*into, SearchHit{
				Source: "opencode", ID: row.ID, Title: title, When: row.When, Snippet: sn,
			})
		}
	}
}

// opencodeSessionID returns an id in the shape opencode's importer accepts.
func opencodeSessionID() string {
	b := make([]byte, 13)
	_, _ = rand.Read(b)
	return "ses_" + hex.EncodeToString(b)
}

// projectIDFor derives the same 40-hex project id opencode uses for a directory.
func projectIDFor(directory string) string {
	sum := sha1.Sum([]byte(directory))
	return hex.EncodeToString(sum[:])
}

func updatedAt(s *Session) int64 {
	if s.Updated > 0 {
		return s.Updated
	}
	return s.Created
}

// opencodeImportDoc builds the {info, messages} document accepted by
// `opencode session import`.
func opencodeImportDoc(s *Session, directory string) map[string]any {
	now := time.Now().UnixMilli()
	created := s.Created
	if created == 0 {
		created = now
	}
	msgs := make([]map[string]any, 0, len(s.Messages))
	for _, m := range s.Messages {
		if m.Role == "system" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		ts := m.Time
		if ts == 0 {
			ts = created
		}
		if m.Role == "user" {
			msgs = append(msgs, map[string]any{
				"id":    newMessageID(),
				"time":  map[string]any{"created": ts},
				"text":  m.Content,
				"files": []any{},
				"type":  "user",
			})
			continue
		}
		msgs = append(msgs, map[string]any{
			"id":    newMessageID(),
			"time":  map[string]any{"created": ts, "completed": ts + 1},
			"type":  "assistant",
			"agent": "build",
			"model": map[string]any{
				"id": s.Model, "providerID": "eisencode", "variant": "default",
			},
			"content": []map[string]any{{"type": "text", "text": m.Content}},
		})
	}
	if directory == "" {
		directory, _ = os.Getwd()
	}
	return map[string]any{
		"info": map[string]any{
			"id":        opencodeSessionID(),
			"projectID": projectIDFor(directory),
			"model": map[string]any{
				"id": s.Model, "providerID": "eisencode", "variant": "default",
			},
			"cost": 0,
			"tokens": map[string]any{
				"input": 0, "output": 0, "reasoning": 0,
				"cache": map[string]any{"read": 0, "write": 0},
			},
			"outcome":  "succeeded",
			"time":     map[string]any{"created": created, "updated": updatedAt(s)},
			"title":    s.Title,
			"location": map[string]any{"directory": directory},
		},
		"messages": msgs,
	}
}

func writeOpenCodeExport(s *Session, path, directory string) (string, error) {
	doc := opencodeImportDoc(s, directory)
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if path == "-" {
		_, err := os.Stdout.Write(append(b, '\n'))
		return "-", err
	}
	if path == "" {
		path = defaultExportName(s.ID)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func importIntoOpenCode(cfg Config, path, directory string) (string, error) {
	if opencodeBin(cfg) == "" {
		return "", fmt.Errorf("opencode not found on PATH — install it first, or run: opencode session import %s", path)
	}
	args := []string{"session", "import", path}
	if directory != "" {
		args = append(args, "--directory", directory)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, opencodeBin(cfg), args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("%s", text)
	}
	if text == "" {
		text = "imported into opencode"
	}
	return text, nil
}

// opencodeSessionRows is used by `sessions --source opencode`.
func opencodeSessionRows(cfg Config, dir string, limit int) []ocSession {
	rows, err := listOpenCodeSessions(cfg, dir, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sopencode sessions unavailable: %v%s\n", ansiDim, err, ansiReset)
		return nil
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// resolveOCDir turns a --dir value into a usable working directory.
func resolveOCDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
