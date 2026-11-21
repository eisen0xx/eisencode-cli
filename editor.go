package main

// A small raw-mode line editor with configurable keybinds and history.
//
// It is used only for the interactive `chat` prompt when stdin is a terminal.
// Piped input keeps using a plain scanner, so scripts and non-interactive use
// behave exactly as before.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

var errEditorEOF = errors.New("eof")
var errEditorCancel = errors.New("cancel")

var defaultKeybinds = map[string]string{
	"send":          "enter",
	"cancel":        "ctrl+c",
	"eof":           "ctrl+d",
	"clear_screen":  "ctrl+l",
	"history_prev":  "up",
	"history_next":  "down",
	"delete_word":   "ctrl+w",
	"kill_line":     "ctrl+k",
	"kill_to_start": "ctrl+u",
	"beginning":     "ctrl+a",
	"end":           "ctrl+e",
	"backspace":     "backspace",
	"delete":        "delete",
}

var keybindHelp = map[string]string{
	"send":          "send the message",
	"cancel":        "clear the line, or leave chat when empty",
	"eof":           "leave chat when the line is empty",
	"clear_screen":  "clear the terminal, keep the session",
	"history_prev":  "previous prompt",
	"history_next":  "next prompt",
	"delete_word":   "delete the word before the cursor",
	"kill_line":     "delete to end of line",
	"kill_to_start": "delete to start of line",
	"beginning":     "move to start of line",
	"end":           "move to end of line",
	"backspace":     "delete one character",
	"delete":        "delete under the cursor",
}

func keybindOrder() []string {
	keys := make([]string, 0, len(keybindHelp))
	for k := range keybindHelp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func effectiveKeybinds(cfg Config) map[string]string {
	binds := map[string]string{}
	for k, v := range defaultKeybinds {
		binds[k] = v
	}
	for k, v := range cfg.Keybinds {
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.ToLower(strings.TrimSpace(v))
		if _, ok := defaultKeybinds[k]; ok && v != "" {
			binds[k] = v
		}
	}
	return binds
}

type Editor struct {
	prompt  string
	binds   map[string]string
	history []string
	out     io.Writer
	plain   *bufio.Reader
}

func newEditor(cfg Config, prompt string, out io.Writer) *Editor {
	return &Editor{
		prompt:  prompt,
		binds:   effectiveKeybinds(cfg),
		history: loadHistory(),
		out:     out,
	}
}

func (e *Editor) action(key string) string {
	for action, bind := range e.binds {
		if bind == key {
			return action
		}
	}
	return ""
}

// readLine returns the submitted line. EOF/cancel are distinct so chat can
// decide whether to exit or just clear.
func (e *Editor) readLine() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return e.readLinePlain()
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return e.readLinePlain()
	}
	defer term.Restore(fd, old)

	buf := []rune{}
	pos := 0
	histIdx := len(e.history)
	saved := ""
	redraw := func() {
		fmt.Fprint(e.out, "\r\x1b[K"+e.prompt+string(buf))
		if pos < len(buf) {
			fmt.Fprintf(e.out, "\x1b[%dD", len(buf)-pos)
		}
	}
	redraw()
	reader := bufio.NewReader(os.Stdin)
	for {
		key, err := readKey(reader)
		if err != nil {
			fmt.Fprint(e.out, "\r\n")
			return "", errEditorEOF
		}
		switch e.action(key) {
		case "send":
			fmt.Fprint(e.out, "\r\n")
			line := string(buf)
			e.remember(line)
			return line, nil
		case "cancel":
			if len(buf) == 0 {
				fmt.Fprint(e.out, "\r\n")
				return "", errEditorCancel
			}
			buf, pos = buf[:0], 0
			redraw()
			continue
		case "eof":
			if len(buf) == 0 {
				fmt.Fprint(e.out, "\r\n")
				return "", errEditorEOF
			}
			if pos < len(buf) {
				buf = append(buf[:pos], buf[pos+1:]...)
				redraw()
			}
			continue
		case "clear_screen":
			fmt.Fprint(e.out, "\x1b[H\x1b[2J")
			redraw()
			continue
		case "history_prev":
			if len(e.history) > 0 && histIdx > 0 {
				if histIdx == len(e.history) {
					saved = string(buf)
				}
				histIdx--
				buf = []rune(e.history[histIdx])
				pos = len(buf)
				redraw()
			}
			continue
		case "history_next":
			if histIdx < len(e.history) {
				histIdx++
				if histIdx == len(e.history) {
					buf = []rune(saved)
				} else {
					buf = []rune(e.history[histIdx])
				}
				pos = len(buf)
				redraw()
			}
			continue
		case "beginning":
			pos = 0
			redraw()
			continue
		case "end":
			pos = len(buf)
			redraw()
			continue
		case "delete_word":
			start := pos
			for start > 0 && buf[start-1] == ' ' {
				start--
			}
			for start > 0 && buf[start-1] != ' ' {
				start--
			}
			buf = append(buf[:start], buf[pos:]...)
			pos = start
			redraw()
			continue
		case "kill_line":
			buf = buf[:pos]
			redraw()
			continue
		case "kill_to_start":
			buf = append([]rune{}, buf[pos:]...)
			pos = 0
			redraw()
			continue
		case "backspace":
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				redraw()
			}
			continue
		case "delete":
			if pos < len(buf) {
				buf = append(buf[:pos], buf[pos+1:]...)
				redraw()
			}
			continue
		}
		switch key {
		case "left":
			if pos > 0 {
				pos--
				redraw()
			}
		case "right":
			if pos < len(buf) {
				pos++
				redraw()
			}
		case "tab":
			if completed, changed := completeChatLine(string(buf)); changed {
				buf = []rune(completed)
				pos = len(buf)
				redraw()
			}
		default:
			if r, ok := printableRune(key); ok {
				buf = append(buf, 0)
				copy(buf[pos+1:], buf[pos:])
				buf[pos] = r
				pos++
				redraw()
			}
		}
	}
}

func (e *Editor) readLinePlain() (string, error) {
	if e.plain == nil {
		e.plain = bufio.NewReader(os.Stdin)
	}
	line, err := e.plain.ReadString('\n')
	if err != nil && line == "" {
		return "", errEditorEOF
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (e *Editor) remember(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if n := len(e.history); n > 0 && e.history[n-1] == line {
		return
	}
	e.history = append(e.history, line)
	if len(e.history) > 500 {
		e.history = e.history[len(e.history)-500:]
	}
	saveHistory(e.history)
}

func loadHistory() []string {
	b, err := os.ReadFile(historyPath())
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	if len(out) > 500 {
		out = out[len(out)-500:]
	}
	return out
}

func saveHistory(lines []string) {
	if err := os.MkdirAll(stateRoot(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(historyPath(), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// readKey decodes one key press into a canonical name (ctrl+x, up, enter, …).
func readKey(r *bufio.Reader) (string, error) {
	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch b {
	case '\r', '\n':
		return "enter", nil
	case 0x7f, 0x08:
		return "backspace", nil
	case 0x00:
		return "ctrl+space", nil
	case 0x1b:
		return readEscape(r)
	}
	if b < 0x20 {
		return "ctrl+" + string(rune('a'+b-1)), nil
	}
	if b < utf8.RuneSelf {
		return string(rune(b)), nil
	}
	rest := []byte{b}
	for !utf8.FullRune(rest) && len(rest) < 4 {
		n, err := r.ReadByte()
		if err != nil {
			break
		}
		rest = append(rest, n)
	}
	if rn, _ := utf8.DecodeRune(rest); rn != utf8.RuneError {
		return string(rn), nil
	}
	return "", nil
}

func readEscape(r *bufio.Reader) (string, error) {
	b, err := r.ReadByte()
	if err != nil {
		return "escape", nil
	}
	if b != '[' && b != 'O' {
		return "escape", nil
	}
	seq := []byte{}
	for {
		n, err := r.ReadByte()
		if err != nil {
			return "escape", nil
		}
		seq = append(seq, n)
		if (n >= 'A' && n <= 'Z') || (n >= 'a' && n <= 'z') || n == '~' {
			break
		}
		if len(seq) > 8 {
			return "escape", nil
		}
	}
	switch string(seq) {
	case "A":
		return "up", nil
	case "B":
		return "down", nil
	case "C":
		return "right", nil
	case "D":
		return "left", nil
	case "H", "1~", "7~":
		return "home", nil
	case "F", "4~", "8~":
		return "end", nil
	case "3~":
		return "delete", nil
	}
	return "escape", nil
}

func printableRune(key string) (rune, bool) {
	if key == "" {
		return 0, false
	}
	r, size := utf8.DecodeRuneInString(key)
	if r == utf8.RuneError || size != len(key) {
		return 0, false
	}
	if r < 0x20 || r == 0x7f {
		return 0, false
	}
	return r, true
}

var chatCommands = []string{
	"/exit", "/quit", "/clear", "/model", "/reasoning", "/help",
	"/sessions", "/new", "/title", "/export", "/search",
}

func completeChatLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "/") || strings.Contains(line, " ") {
		return line, false
	}
	var matches []string
	for _, c := range chatCommands {
		if strings.HasPrefix(c, line) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return line, false
	}
	prefix := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	if prefix == line && len(matches) == 1 {
		prefix += " "
	}
	return prefix, prefix != line
}
