package main

// Session storage for the eisencode CLI.
//
// Everything here lives under the CLI's own directory and never touches
// opencode's storage. Moving a session into opencode is an explicit export
// (see opencode.go), so the two tools stay isolated until you ask for a handoff.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sessionSource = "eisen"

type SessionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

type Session struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Model    string           `json:"model"`
	System   string           `json:"system,omitempty"`
	Source   string           `json:"source"`
	Created  int64            `json:"created"`
	Updated  int64            `json:"updated"`
	Messages []SessionMessage `json:"messages"`
}

// stateRoot is the CLI's private directory. EISENCODE_HOME overrides it, which
// keeps tests (and throwaway profiles) completely separate from a real install.
func stateRoot() string {
	if v := strings.TrimSpace(os.Getenv("EISENCODE_HOME")); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "eisencode")
}

func sessionsDir() string { return filepath.Join(stateRoot(), "sessions") }
func historyPath() string { return filepath.Join(stateRoot(), "history") }

func newSessionID() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("eis_%012x", time.Now().UnixNano())
	}
	return "eis_" + hex.EncodeToString(b)
}

func newMessageID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "msg_" + hex.EncodeToString(b)
}

func sessionTitle(prompt string) string {
	line := strings.TrimSpace(prompt)
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return "untitled"
	}
	r := []rune(line)
	if len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return line
}

func saveSession(s *Session) error {
	if s.Source == "" {
		s.Source = sessionSource
	}
	if s.ID == "" {
		s.ID = newSessionID()
	}
	now := time.Now().UnixMilli()
	if s.Created == 0 {
		s.Created = now
	}
	s.Updated = now
	if err := os.MkdirAll(sessionsDir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(sessionsDir(), s.ID+".json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func loadSession(id string) (*Session, error) {
	p, err := resolveSessionPath(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("corrupt session %s: %w", filepath.Base(p), err)
	}
	return &s, nil
}

// resolveSessionPath accepts a full id or an unambiguous prefix (>= 4 chars).
func resolveSessionPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("session id required")
	}
	if !strings.ContainsAny(id, `/\`) {
		direct := filepath.Join(sessionsDir(), id+".json")
		if _, err := os.Stat(direct); err == nil {
			return direct, nil
		}
	}
	entries, err := os.ReadDir(sessionsDir())
	if err != nil {
		return "", fmt.Errorf("no sessions yet")
	}
	var matches []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.HasPrefix(name, id) {
			matches = append(matches, name)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no session matching %q", id)
	case 1:
		return filepath.Join(sessionsDir(), matches[0]+".json"), nil
	}
	sort.Strings(matches)
	return "", fmt.Errorf("%q is ambiguous: %s", id, strings.Join(matches, ", "))
}

func listSessions() ([]Session, error) {
	entries, err := os.ReadDir(sessionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(sessionsDir(), e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(b, &s) == nil {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out, nil
}

func deleteSession(id string) error {
	p, err := resolveSessionPath(id)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// ---------- search ----------

type SearchHit struct {
	Source  string `json:"source"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	When    string `json:"when"`
	Snippet string `json:"snippet"`
}

func matchSnippet(text, query string) (string, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return "", false
	}
	low := strings.ToLower(text)
	i := strings.Index(low, q)
	if i < 0 {
		return "", false
	}
	const radius = 48
	start := i - radius
	if start < 0 {
		start = 0
	}
	end := i + len(q) + radius
	if end > len(text) {
		end = len(text)
	}
	snip := strings.Join(strings.Fields(text[start:end]), " ")
	if start > 0 {
		snip = "…" + snip
	}
	if end < len(text) {
		snip += "…"
	}
	return snip, true
}

func searchSessions(query string, limit int) []SearchHit {
	sessions, _ := listSessions()
	var hits []SearchHit
	for _, s := range sessions {
		if len(hits) >= limit {
			break
		}
		snippet := ""
		ok := false
		if _, found := matchSnippet(s.Title, query); found {
			snippet, ok = s.Title, true
		}
		if !ok {
			for _, m := range s.Messages {
				if sn, found := matchSnippet(m.Content, query); found {
					snippet, ok = sn, true
					break
				}
			}
		}
		if !ok {
			continue
		}
		hits = append(hits, SearchHit{
			Source:  sessionSource,
			ID:      s.ID,
			Title:   s.Title,
			When:    time.UnixMilli(s.Updated).Local().Format("2006-01-02 15:04"),
			Snippet: snippet,
		})
	}
	return hits
}

// ---------- transcript rendering ----------

func sessionMarkdown(s *Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "- session: `%s`\n- model: `%s`\n- created: %s\n\n---\n\n",
		s.ID, s.Model, time.UnixMilli(s.Created).Local().Format(time.RFC3339))
	for _, m := range s.Messages {
		switch m.Role {
		case "system":
			continue
		case "user":
			b.WriteString("## you\n\n")
		default:
			b.WriteString("## eisencode\n\n")
		}
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}
