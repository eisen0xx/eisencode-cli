// eisencode — the official CLI for the eisencode API.
// 11 models, one endpoint, your key.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	version       = "0.2.0"
	defaultBase   = "https://eisencode.waivy.dev/api/v1"
	defaultModel  = "E-1-Flash"
	brand         = "eisencode"
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiBlue      = "\033[38;5;33m"
	ansiRed       = "\033[38;5;160m"
	ansiGreen     = "\033[38;5;34m"
	ansiGray      = "\033[38;5;244m"
	ansiClearLine = "\r\033[K"
)

// ---------- config ----------

type Config struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "eisencode", "config.json")
}

func loadConfig() Config {
	cfg := Config{BaseURL: defaultBase, Model: defaultModel}
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if v := os.Getenv("EISENCODE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("EISENCODE_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("EISENCODE_MODEL"); v != "" {
		cfg.Model = v
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return cfg
}

func saveConfig(cfg Config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// ---------- api ----------

type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string { return e.Msg }

func doJSON(cfg Config, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, cfg.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", brand+"-cli/"+version+" ("+runtime.GOOS+"/"+runtime.GOARCH+")")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach eisencode: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return &apiError{Status: resp.StatusCode, Msg: errMessage(data, resp.StatusCode)}
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("bad response: %w", err)
		}
	}
	return nil
}

func errMessage(data []byte, status int) string {
	var e struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	_ = json.Unmarshal(data, &e)
	if e.Message != "" {
		return e.Message
	}
	if len(e.Error) > 0 {
		var s string
		if json.Unmarshal(e.Error, &s) == nil && s != "" {
			return s
		}
		var o struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &o) == nil && o.Message != "" {
			return o.Message
		}
	}
	switch status {
	case 401:
		return "invalid or revoked key — run: eisencode login"
	case 402:
		return "no credits on this key — top up at https://eisencode.waivy.dev/dashboard"
	case 429:
		return "rate limited — slow down (30 req/min per key)"
	}
	return fmt.Sprintf("request failed (http %d)", status)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionReq struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type SSEChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// streamChat posts a completion and prints the stream. Returns full text.
func streamChat(cfg Config, req CompletionReq, showReasoning bool, w io.Writer) (string, error) {
	req.Stream = true
	b, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", brand+"-cli/"+version+" ("+runtime.GOOS+"/"+runtime.GOARCH+")")
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cannot reach eisencode: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", &apiError{Status: resp.StatusCode, Msg: errMessage(data, resp.StatusCode)}
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var full strings.Builder
	inReasoning := false
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var c SSEChunk
		if err := json.Unmarshal([]byte(payload), &c); err != nil {
			continue
		}
		if c.Error != nil {
			return full.String(), errors.New(c.Error.Message)
		}
		for _, ch := range c.Choices {
			if ch.Delta.ReasoningContent != "" && showReasoning {
				if !inReasoning {
					fmt.Fprint(w, ansiGray)
					inReasoning = true
				}
				fmt.Fprint(w, ch.Delta.ReasoningContent)
			}
			if ch.Delta.Content != "" {
				if inReasoning {
					fmt.Fprint(w, ansiReset)
					fmt.Fprint(w, "\n"+ansiBlue+ansiBold+"answer"+ansiReset+"\n")
					inReasoning = false
				}
				full.WriteString(ch.Delta.Content)
				fmt.Fprint(w, ch.Delta.Content)
			}
		}
	}
	if inReasoning {
		fmt.Fprint(w, ansiReset)
	}
	if err := sc.Err(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

// ---------- commands ----------

func cmdHelp() {
	fmt.Printf(`%seisencode%sv%s — the API wall

%sUSAGE%s
  eisencode login [key]          save and verify your API key (asks if omitted)
  eisencode logout               remove the saved key
  eisencode key                  show key, plan, balance
  eisencode provider             show the eisen provider + base url
  eisencode models               list the 11 available models
  eisencode run [prompt]        one-shot completion (reads stdin if no prompt)
  eisencode chat                interactive chat
  eisencode usage               recent requests + balance
  eisencode version             print version

%sOPTIONS%s
  --model <id>                  model to use (default %s)
  --system <text>               system prompt (run/chat)
  --max-tokens <n>              cap response length
  --json                        raw JSON output (run/models/usage)
  --no-stream                   wait for the full answer, print once
  --reasoning                   show the model's reasoning (chat: /reasoning)

%sENV%s
  EISENCODE_API_KEY             key (overrides config)
  EISENCODE_BASE_URL            API base (default %s)
  EISENCODE_MODEL               default model

%sCONFIG%s %s
`, ansiBlue, ansiReset, version,
		ansiBold, ansiReset,
		ansiBold, ansiReset,
		defaultModel,
		ansiBold, ansiReset,
		defaultBase,
		ansiBold, ansiReset, configPath())
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func askKey() string {
	fmt.Printf("%sget a key%s at https://eisencode.waivy.dev/dashboard\n",
		ansiDim, ansiReset)
	fmt.Printf("%spaste your eisencode api key%s › ", ansiBlue, ansiReset)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		if b, err := term.ReadPassword(fd); err == nil {
			fmt.Println()
			return strings.TrimSpace(string(b))
		}
	}
	in := bufio.NewReader(os.Stdin)
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}

func saveVerifiedKey(cfg *Config, key string) error {
	cfg.APIKey = key
	var info struct {
		Service  string  `json:"service"`
		Provider string  `json:"provider"`
		Key      string  `json:"key"`
		Plan     string  `json:"plan"`
		Balance  float64 `json:"balance"`
	}
	if err := doJSON(*cfg, "GET", "/key", nil, &info); err != nil {
		cfg.APIKey = ""
		return err
	}
	if err := saveConfig(*cfg); err != nil {
		return fmt.Errorf("could not save config: %w", err)
	}
	fmt.Printf("%s✓ logged in%s  key %s  plan %s  balance %.2f  provider %s\n",
		ansiGreen, ansiReset, info.Key, info.Plan, info.Balance, info.Provider)
	return nil
}

func cmdLogin(cfg *Config, key string) int {
	key = strings.TrimSpace(key)
	if key == "" {
		if !isTTY() {
			fmt.Fprintln(os.Stderr, ansiRed+"usage: eisencode login <key>"+ansiReset)
			return 2
		}
		key = askKey()
	}
	if key == "" {
		fmt.Fprintln(os.Stderr, ansiRed+"no key entered"+ansiReset)
		return 2
	}
	if err := saveVerifiedKey(cfg, key); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+"key rejected: "+err.Error()+ansiReset)
		return 1
	}
	return 0
}

// ensureKey checks for a saved key and, on a terminal, asks for one and saves it.
func ensureKey(cfg *Config) bool {
	if strings.TrimSpace(cfg.APIKey) != "" {
		return true
	}
	if !isTTY() {
		fmt.Fprintln(os.Stderr, ansiRed+"no API key — get one at https://eisencode.waivy.dev/dashboard"+ansiReset)
		fmt.Fprintln(os.Stderr, "  eisencode login sk_eis_...")
		return false
	}
	fmt.Printf("%sno api key yet.%s\n", ansiRed, ansiReset)
	key := askKey()
	if key == "" {
		fmt.Fprintln(os.Stderr, ansiRed+"no key entered — nothing to do"+ansiReset)
		return false
	}
	if err := saveVerifiedKey(cfg, key); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+"key rejected: "+err.Error()+ansiReset)
		return false
	}
	return true
}

func cmdKey(cfg *Config) int {
	if !ensureKey(cfg) {
		return 1
	}
	var info struct {
		Service       string  `json:"service"`
		Provider      string  `json:"provider"`
		Key           string  `json:"key"`
		Plan          string  `json:"plan"`
		Balance       float64 `json:"balance"`
		RequestsMonth int     `json:"requests_month"`
		Active        bool    `json:"active"`
	}
	if err := doJSON(*cfg, "GET", "/key", nil, &info); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	fmt.Printf("%skey%s      %s\n%sservice%s  %s\n%sprovider%s %s\n%splan%s     %s\n%sbalance%s  %.2f credits\n%smonth%s    %d requests\n",
		ansiBold, ansiReset, info.Key,
		ansiBold, ansiReset, info.Service,
		ansiBold, ansiReset, info.Provider,
		ansiBold, ansiReset, info.Plan,
		ansiBold, ansiReset, info.Balance,
		ansiBold, ansiReset, info.RequestsMonth)
	return 0
}

type modelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func cmdModels(cfg *Config, asJSON bool) int {
	if !ensureKey(cfg) {
		return 1
	}
	var ml modelList
	if err := doJSON(*cfg, "GET", "/models", nil, &ml); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(ml, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	for _, m := range ml.Data {
		mark := "  "
		if m.ID == cfg.Model {
			mark = ansiBlue + "→ " + ansiReset
		}
		fmt.Printf("%s%s\n", mark, m.ID)
	}
	return 0
}

func cmdProvider(cfg Config, asJSON bool) int {
	var p struct {
		Object   string   `json:"object"`
		Provider string   `json:"provider"`
		Service  string   `json:"service"`
		BaseURL  string   `json:"base_url"`
		Auth     string   `json:"auth"`
		Models   []string `json:"models"`
		Docs     string   `json:"docs"`
	}
	if err := doJSON(cfg, "GET", "/provider", nil, &p); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	fmt.Printf("%sprovider%s  %s\n%sservice%s   %s\n%sbase url%s  %s\n%sauth%s      %s (openai-compatible)\n%smodels%s    %d\n%slist%s\n",
		ansiBold, ansiReset, p.Provider,
		ansiBold, ansiReset, p.Service,
		ansiBold, ansiReset, p.BaseURL,
		ansiBold, ansiReset, p.Auth,
		ansiBold, ansiReset, len(p.Models),
		ansiBold, ansiReset)
	for _, m := range p.Models {
		fmt.Printf("  %s\n", m)
	}
	fmt.Printf("%sdocs%s      %s\n", ansiBold, ansiReset, p.Docs)
	return 0
}

func readPrompt(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	st, _ := os.Stdin.Stat()
	if st != nil && st.Mode()&os.ModeCharDevice == 0 {
		b, _ := io.ReadAll(os.Stdin)
		return strings.TrimSpace(string(b))
	}
	return ""
}

func cmdRun(cfg *Config, args []string, opts options) int {
	if !ensureKey(cfg) {
		return 1
	}
	prompt := readPrompt(args)
	if prompt == "" {
		fmt.Fprintln(os.Stderr, ansiRed+"no prompt — eisencode run \"your prompt\" (or pipe stdin)"+ansiReset)
		return 2
	}
	msgs := []Message{}
	if opts.system != "" {
		msgs = append(msgs, Message{Role: "system", Content: opts.system})
	}
	msgs = append(msgs, Message{Role: "user", Content: prompt})
	req := CompletionReq{Model: opts.model, Messages: msgs, MaxTokens: opts.maxTokens}
	if opts.noStream {
		var out map[string]any
		if err := doJSON(*cfg, "POST", "/chat/completions", req, &out); err != nil {
			fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
			return 1
		}
		if opts.asJSON {
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return 0
		}
		fmt.Println(extractContent(out))
		return 0
	}
	text, err := streamChat(*cfg, req, opts.reasoning, os.Stdout)
	if err != nil && text == "" {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	fmt.Println()
	return 0
}

func extractContent(out map[string]any) string {
	ch, _ := out["choices"].([]any)
	if len(ch) == 0 {
		return ""
	}
	c0, _ := ch[0].(map[string]any)
	msg, _ := c0["message"].(map[string]any)
	s, _ := msg["content"].(string)
	return s
}

type options struct {
	model      string
	system     string
	maxTokens  int
	asJSON     bool
	noStream   bool
	reasoning  bool
	positional []string
}

func parseFlags(cfg *Config, args []string) options {
	o := options{model: cfg.Model}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch a {
		case "--model", "-m":
			o.model = next()
		case "--system", "-s":
			o.system = next()
		case "--max-tokens":
			fmt.Sscanf(next(), "%d", &o.maxTokens)
		case "--json":
			o.asJSON = true
		case "--no-stream":
			o.noStream = true
		case "--reasoning":
			o.reasoning = true
		default:
			o.positional = append(o.positional, a)
		}
	}
	return o
}

func cmdUsage(cfg *Config, asJSON bool) int {
	if !ensureKey(cfg) {
		return 1
	}
	var info struct {
		Key           string  `json:"key"`
		Balance       float64 `json:"balance"`
		RequestsMonth int     `json:"requests_month"`
		Usage         []struct {
			Model     string `json:"model"`
			LatencyMS int    `json:"latency_ms"`
			Status    string `json:"status"`
			TS        string `json:"ts"`
		} `json:"usage"`
	}
	if err := doJSON(*cfg, "GET", "/key", nil, &info); err != nil {
		fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	fmt.Printf("%s%s%s  balance %.2f  ·  %d requests this month\n\n",
		ansiBold, info.Key, ansiReset, info.Balance, info.RequestsMonth)
	if len(info.Usage) == 0 {
		fmt.Println(ansiDim + "no requests yet" + ansiReset)
		return 0
	}
	for _, u := range info.Usage {
		ts := u.TS
		if len(ts) >= 19 {
			ts = ts[:19]
		}
		fmt.Printf("  %s  %-32s %5dms  %s\n", ts, u.Model, u.LatencyMS, u.Status)
	}
	return 0
}

func cmdChat(cfg *Config, opts options) int {
	if !ensureKey(cfg) {
		return 1
	}
	model := opts.model
	msgs := []Message{}
	if opts.system != "" {
		msgs = append(msgs, Message{Role: "system", Content: opts.system})
	}
	showReasoning := opts.reasoning
	fmt.Printf("%seisencode%s chat — model %s%s%s\n", ansiBlue+ansiBold, ansiReset, ansiBold, model, ansiReset)
	fmt.Println(ansiDim + "/exit  /clear  /model <id>  /reasoning  /help" + ansiReset)
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for {
		fmt.Printf("\n%syou%s › ", ansiBlue, ansiReset)
		if !in.Scan() {
			fmt.Println()
			return 0
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		switch {
		case line == "/exit" || line == "/quit":
			return 0
		case line == "/clear":
			msgs = msgs[:0]
			if opts.system != "" {
				msgs = append(msgs, Message{Role: "system", Content: opts.system})
			}
			fmt.Println(ansiDim + "context cleared" + ansiReset)
			continue
		case line == "/reasoning":
			showReasoning = !showReasoning
			fmt.Printf(ansiDim+"reasoning %v"+ansiReset+"\n", showReasoning)
			continue
		case strings.HasPrefix(line, "/model "):
			model = strings.TrimSpace(strings.TrimPrefix(line, "/model "))
			fmt.Printf(ansiDim+"model → %s"+ansiReset+"\n", model)
			continue
		case line == "/help":
			fmt.Println(ansiDim + "/exit  /clear  /model <id>  /reasoning" + ansiReset)
			continue
		}
		msgs = append(msgs, Message{Role: "user", Content: line})
		fmt.Printf("%s%s%s › ", ansiBlue+ansiBold, brand, ansiReset)
		req := CompletionReq{Model: model, Messages: msgs, MaxTokens: opts.maxTokens}
		text, err := streamChat(*cfg, req, showReasoning, os.Stdout)
		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, ansiRed+err.Error()+ansiReset)
			msgs = msgs[:len(msgs)-1]
			continue
		}
		msgs = append(msgs, Message{Role: "assistant", Content: text})
	}
}

func main() {
	cfg := loadConfig()
	args := os.Args[1:]
	if len(args) == 0 {
		cmdHelp()
		return
	}
	switch args[0] {
	case "help", "-h", "--help":
		cmdHelp()
	case "version", "-v", "--version":
		fmt.Printf("%s %s (%s/%s)\n", brand, version, runtime.GOOS, runtime.GOARCH)
	case "login":
		key := ""
		if len(args) > 1 {
			key = args[1]
		}
		os.Exit(cmdLogin(&cfg, key))
	case "logout":
		cfg.APIKey = ""
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("logged out — key removed from", configPath())
	case "key":
		os.Exit(cmdKey(&cfg))
	case "provider":
		opts := parseFlags(&cfg, args[1:])
		os.Exit(cmdProvider(cfg, opts.asJSON))
	case "models":
		opts := parseFlags(&cfg, args[1:])
		os.Exit(cmdModels(&cfg, opts.asJSON))
	case "usage":
		opts := parseFlags(&cfg, args[1:])
		os.Exit(cmdUsage(&cfg, opts.asJSON))
	case "run":
		opts := parseFlags(&cfg, args[1:])
		os.Exit(cmdRun(&cfg, opts.positional, opts))
	case "chat":
		opts := parseFlags(&cfg, args[1:])
		os.Exit(cmdChat(&cfg, opts))
	default:
		fmt.Fprintf(os.Stderr, "%sunknown command %q%s\n\n", ansiRed, args[0], ansiReset)
		cmdHelp()
		os.Exit(2)
	}
}
