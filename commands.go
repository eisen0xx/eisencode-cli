package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------- sessions ----------

func cmdSessions(cfg *Config, args []string) int {
	sub := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "list":
		return sessionsList(cfg, args)
	case "show":
		return sessionsShow(cfg, args)
	case "rm", "delete":
		return sessionsRemove(args)
	case "rename":
		return sessionsRename(args)
	case "export":
		return sessionsExport(cfg, args)
	default:
		fmt.Fprintf(os.Stderr, "%sunknown sessions command %q%s\n", ansiRed, sub, ansiReset)
		fmt.Fprintln(os.Stderr, "  sessions  sessions show <id>  sessions rm <id>  sessions rename <id> <title>  sessions export <id>")
		return 2
	}
}

func sessionsList(cfg *Config, args []string) int {
	source, asJSON, limit := "eisen", false, 50
	dir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "--in":
			if i+1 < len(args) {
				i++
				source = args[i]
			}
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 < len(args) {
				i++
				limit, _ = strconv.Atoi(args[i])
			}
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = resolveOCDir(args[i])
			}
		}
	}
	type row struct {
		Source  string `json:"source"`
		ID      string `json:"id"`
		Title   string `json:"title"`
		Updated string `json:"updated"`
	}
	rows := []row{}
	if source == "eisen" || source == "all" {
		sessions, _ := listSessions()
		for _, s := range sessions {
			if len(rows) >= limit {
				break
			}
			rows = append(rows, row{"eisen", s.ID, s.Title,
				time.UnixMilli(s.Updated).Local().Format("2006-01-02 15:04")})
		}
	}
	if source == "opencode" || source == "all" {
		for _, s := range opencodeSessionRows(*cfg, dir, limit) {
			rows = append(rows, row{"opencode", s.ID, s.Title, s.When})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Updated > rows[j].Updated })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	if asJSON {
		b, _ := json.MarshalIndent(map[string]any{"data": rows}, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if len(rows) == 0 {
		switch source {
		case "opencode":
			fmt.Println(ansiDim + "no opencode sessions here — pass --dir <project> to search another project" + ansiReset)
		default:
			fmt.Println(ansiDim + "no sessions yet — start one with: eisencode chat" + ansiReset)
		}
		return 0
	}
	for _, r := range rows {
		src := ansiBlue + "eisen" + ansiReset + "   "
		if r.Source == "opencode" {
			src = ansiGray + "opencode" + ansiReset
		}
		fmt.Printf("%s  %-32s  %-16s  %s\n", src, r.ID, r.Updated, r.Title)
	}
	return 0
}

func sessionsShow(cfg *Config, args []string) int {
	var id, dir string
	asJSON, asMD := false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--md", "--markdown":
			asMD = true
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = resolveOCDir(args[i])
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				id = args[i]
			}
		}
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: eisencode sessions show <id> [--md|--json] [--dir PATH]")
		return 2
	}
	if strings.HasPrefix(id, "ses_") {
		doc, err := exportOpenCodeSession(*cfg, id, dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
			return 1
		}
		if asJSON {
			b, _ := json.MarshalIndent(doc, "", "  ")
			fmt.Println(string(b))
			return 0
		}
		fmt.Printf("%s%s%s\n\n", ansiBold, doc.Info.Title, ansiReset)
		fmt.Print(ocExportText(doc))
		return 0
	}
	s, err := loadSession(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if asMD {
		fmt.Print(sessionMarkdown(s))
		return 0
	}
	fmt.Printf("%s%s%s  %s\n", ansiBold, s.Title, ansiReset, ansiDim+s.ID+ansiReset)
	for _, m := range s.Messages {
		if m.Role == "system" {
			continue
		}
		who := ansiBlue + "you" + ansiReset
		if m.Role != "user" {
			who = ansiBright + brand + ansiReset
		}
		fmt.Printf("\n%s\n%s\n", who, m.Content)
	}
	return 0
}

func sessionsRemove(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: eisencode sessions rm <id>")
		return 2
	}
	if err := deleteSession(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	fmt.Println("deleted", args[0])
	return 0
}

func sessionsRename(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: eisencode sessions rename <id> <title>")
		return 2
	}
	s, err := loadSession(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	s.Title = strings.Join(args[1:], " ")
	if err := saveSession(s); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	fmt.Println("renamed →", s.Title)
	return 0
}

func sessionsExport(cfg *Config, args []string) int {
	var id, out, directory string
	asMD, asJSON, doImport := false, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out", "-o":
			if i+1 < len(args) {
				i++
				out = args[i]
			}
		case "--md", "--markdown":
			asMD = true
		case "--json":
			asJSON = true
		case "--import":
			doImport = true
		case "--directory":
			if i+1 < len(args) {
				i++
				directory = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				id = args[i]
			}
		}
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "usage: eisencode sessions export <id> [--out FILE|-] [--md] [--json] [--import]")
		return 2
	}
	s, err := loadSession(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if asMD {
		writeTextExport(sessionMarkdown(s), out)
		return 0
	}
	if asJSON {
		b, _ := json.MarshalIndent(s, "", "  ")
		writeTextExport(string(b)+"\n", out)
		return 0
	}
	path, err := writeOpenCodeExport(s, out, directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if path != "-" {
		fmt.Printf("wrote %s\n", path)
	}
	if doImport {
		msg, err := importIntoOpenCode(*cfg, path, directory)
		if err != nil {
			fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
			return 1
		}
		fmt.Println(msg)
		return 0
	}
	if path != "-" {
		fmt.Printf("%simport it:%s  opencode session import %s\n", ansiDim, ansiReset, path)
	}
	return 0
}

func writeTextExport(text, out string) {
	if out == "" || out == "-" {
		fmt.Print(text)
		return
	}
	if err := os.WriteFile(out, []byte(text), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return
	}
	fmt.Printf("wrote %s\n", out)
}

// ---------- search ----------

func cmdSearch(cfg *Config, args []string) int {
	scope, asJSON, limit := "eisen", false, 25
	dir := ""
	var words []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--in", "--source", "--scope":
			if i+1 < len(args) {
				i++
				scope = args[i]
			}
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 < len(args) {
				i++
				limit, _ = strconv.Atoi(args[i])
			}
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = resolveOCDir(args[i])
			}
		default:
			words = append(words, args[i])
		}
	}
	query := strings.TrimSpace(strings.Join(words, " "))
	if query == "" {
		fmt.Fprintln(os.Stderr, "usage: eisencode search <query> [--in eisen|opencode|all] [--limit N] [--json] [--dir PATH]")
		return 2
	}
	hits := []SearchHit{}
	if scope == "eisen" || scope == "all" {
		hits = append(hits, searchSessions(query, limit)...)
	}
	if scope == "opencode" || scope == "all" {
		searchOpenCode(*cfg, query, limit, dir, &hits)
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].When > hits[j].When })
	if asJSON {
		b, _ := json.MarshalIndent(map[string]any{"query": query, "scope": scope, "data": hits}, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if len(hits) == 0 {
		fmt.Printf("%sno matches for %q (%s)%s\n", ansiDim, query, scope, ansiReset)
		return 0
	}
	for _, h := range hits {
		src := ansiBlue + h.Source + ansiReset
		if h.Source == "opencode" {
			src = ansiGray + h.Source + ansiReset
		}
		fmt.Printf("%-22s %-32s %-16s %s\n", src, h.ID, h.When, h.Title)
		if h.Snippet != "" && h.Snippet != h.Title {
			fmt.Printf("  %s%s%s\n", ansiDim, h.Snippet, ansiReset)
		}
	}
	return 0
}

// ---------- keybinds ----------

func cmdKeys(cfg *Config, asJSON bool) int {
	binds := effectiveKeybinds(*cfg)
	if asJSON {
		b, _ := json.MarshalIndent(binds, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	fmt.Printf("%s%s keybinds%s %s(chat) — override in %s%s\n\n",
		ansiBold, brand, ansiReset, ansiDim, configPath(), ansiReset)
	for _, action := range keybindOrder() {
		fmt.Printf("  %-16s %-14s %s\n", binds[action], action, ansiDim+keybindHelp[action]+ansiReset)
	}
	return 0
}

// ---------- path helper for exports ----------

func defaultExportName(id string) string {
	return filepath.Join(".", id+".opencode.json")
}
