# eisencode-cli

The official CLI for **eisencode** — 11 models from the **eisen** provider, one OpenAI-style endpoint, your key.

```
https://eisencode.waivy.dev/api/v1
```

It is a **standalone, universal** client. It keeps its own config and its own
sessions and does **not** depend on [opencode](https://opencode.ai). When you
want, it can hand a session over to opencode (one command) or include opencode
sessions in a search.

- own state dir: `~/.config/eisencode/` (override with `EISENCODE_HOME`)
- own sessions: `eis_<10 hex>` — importable into opencode as `ses_...`
- search scopes: `eisen` (default), `opencode`, or `all`
- raw-mode line editor with configurable keybinds, history and `/` completion
- shell completion for bash, zsh, fish and PowerShell

## Install

**Linux / macOS**
```bash
curl -fsSL https://eisencode.waivy.dev/install.sh | bash
```

**Windows (PowerShell)**
```powershell
irm https://eisencode.waivy.dev/install.ps1 | iex
```

Both installers finish with a `version` check and print the matching
completion command. Or grab a binary from
[Releases](https://github.com/eisen0xx/eisencode-cli/releases):

| OS | Arch | File |
|---|---|---|
| Linux | x86_64 | `eisencode-linux-amd64` |
| Linux | arm64 | `eisencode-linux-arm64` |
| macOS | Intel | `eisencode-darwin-amd64` |
| macOS | Apple Silicon | `eisencode-darwin-arm64` |
| Windows | x86_64 | `eisencode-windows-amd64.exe` |

## Quick start

```bash
eisencode login                # prompts for your key, verifies it, saves it
eisencode run "explain raft in 3 lines"
eisencode chat
```

## Commands

| Command | What it does |
|---|---|
| `eisencode login [key]` | verify + save your key (asks interactively if omitted) |
| `eisencode logout` | remove the saved key |
| `eisencode key` | key, provider, plan, balance |
| `eisencode provider` | the eisen provider + base url + models |
| `eisencode models` | list the 11 models |
| `eisencode run [prompt]` | one-shot completion (streams, reads stdin if no prompt) |
| `eisencode chat` | interactive chat with isolated sessions |
| `eisencode sessions [list]` | your sessions — eisen by default, `--source opencode\|all` |
| `eisencode sessions show <id>` | print a session (`--md`, `--json`) |
| `eisencode sessions rm <id>` | delete an eisen session |
| `eisencode sessions rename <id> <title>` | retitle an eisen session |
| `eisencode sessions export <id>` | write an opencode-importable JSON (`--out FILE\|-`, `--import`) |
| `eisencode search <query>` | search sessions — `--in eisen\|opencode\|all` |
| `eisencode keys` | list the chat keybinds |
| `eisencode completion <shell>` | bash, zsh, fish or powershell |
| `eisencode logo` | print the EISEN CODE wordmark |
| `eisencode usage` | recent requests + balance |
| `eisencode version` | version |

Global flags: `--model <id>`, `--system <text>`, `--max-tokens <n>`, `--json`,
`--no-stream`, `--reasoning`, and for `sessions`/`search`: `--source`,
`--in`, `--limit N`, `--dir PATH`.

## Sessions (isolated)

Sessions are stored by the CLI itself:

```
$EISENCODE_HOME/sessions/eis_<id>.json
```

Nothing is written to opencode's database. In `chat`:

| command | effect |
|---|---|
| `/sessions` | list your 10 most recent eisen sessions |
| `/new` | start a fresh session (keeps the current model) |
| `/title <text>` | retitle the session |
| `/export` | write `eis_<id>.opencode.json` in the current dir |
| `/search <query>` | search your eisen sessions |
| `/model [id]` | show or switch the model |
| `/clear` | clear the conversation, keep the session id |
| `/exit`, `/quit` | leave (session is saved) |

Chat also supports `--session <id>` to resume, `--new` to force a fresh one and
`--no-save` for a throwaway conversation. A session is persisted after every
answer.

## Keybinds

Chat uses a raw-mode line editor, so it does not need readline. Defaults:

| key | action |
|---|---|
| `enter` | send |
| `ctrl+c` | clear the line, or leave chat when the line is empty |
| `ctrl+d` | leave chat when the line is empty |
| `ctrl+l` | clear the screen, keep the session |
| `up` / `down` | prompt history |
| `left` / `right` | move the cursor |
| `ctrl+a` / `ctrl+e` | start / end of line |
| `ctrl+w` | delete the previous word |
| `ctrl+k` / `ctrl+u` | delete to end / start of line |
| `tab` | complete `/commands` |

Override any of them in the config (action names are the ones `eisencode keys`
lists):

```json
{
  "keybinds": {
    "send": "enter",
    "cancel": "ctrl+c",
    "history_prev": "up",
    "history_next": "down"
  }
}
```

`eisencode keys --json` prints the effective map.

## Search scopes

```bash
eisencode search "raft"                  # eisen sessions only (default)
eisencode search "raft" --in opencode    # opencode sessions only
eisencode search "raft" --in all         # both, tagged by source
eisencode search "raft" --in all --dir ~/code/project   # opencode project dir
```

opencode scopes its sessions per project directory. Pass `--dir` to search a
different project; without it the current directory is used. `sessions list`
accepts the same flag.

## Hand a session to opencode

```bash
eisencode sessions export eis_1a2b3c4d5e            # writes eis_...opencode.json
eisencode sessions export eis_1a2b3c4d5e --import   # writes + runs the import
```

The exported document uses opencode's import schema (`ses_...` id, sha1
`projectID`, `info` + `messages`) and is accepted by:

```bash
opencode session import eis_1a2b3c4d5e.opencode.json --directory /path/to/project
```

Use `--out -` to print the JSON instead of writing a file. Importing never
mutates the original session; the eisen copy stays where it is.

## Completion

```bash
eisencode completion bash   >> ~/.bashrc
eisencode completion zsh    >  "${fpath[1]}/_eisencode"
eisencode completion fish   >  ~/.config/fish/completions/eisencode.fish
eisencode completion powershell | Out-File -Append $PROFILE
```

The installers print the command for your shell.

## Config

`~/.config/eisencode/config.json` (Linux), `~/Library/Application Support/eisencode/config.json` (macOS), `%AppData%\eisencode\config.json` (Windows). Written with `0600`.

```json
{
  "api_key": "sk_eis_...",
  "base_url": "https://eisencode.waivy.dev/api/v1",
  "model": "E-1-Flash",
  "keybinds": {},
  "opencode_bin": ""
}
```

Environment overrides:

```bash
export EISENCODE_API_KEY=sk_eis_...
export EISENCODE_BASE_URL=https://eisencode.waivy.dev/api/v1
export EISENCODE_MODEL=E-1-Flash
export EISENCODE_HOME=~/.config/eisencode    # private state dir (config + sessions)
export EISENCODE_OPENCODE_BIN=/path/to/opencode
```

## Use with any OpenAI client

```python
from openai import OpenAI
client = OpenAI(base_url="https://eisencode.waivy.dev/api/v1", api_key="sk_eis_...")
print(client.chat.completions.create(
    model="E-1-Flash",
    messages=[{"role": "user", "content": "hi"}],
).choices[0].message.content)
```

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o eisencode .
```

Cross-compile the release matrix with `./build.sh` (or the commands in the
release workflow). The binary has no runtime dependencies beyond the OS.
