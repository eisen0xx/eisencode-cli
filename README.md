# eisencode-cli

The official CLI for **eisencode** — 12 models, one OpenAI-style endpoint, your key.

```
https://eisencode.waivy.dev/api/v1
```

## Install

**Linux / macOS**
```bash
curl -fsSL https://eisencode.waivy.dev/install.sh | bash
```

**Windows (PowerShell)**
```powershell
irm https://eisencode.waivy.dev/install.ps1 | iex
```

Or grab a binary from [Releases](https://github.com/eisen0xx/eisencode-cli/releases):

| OS | Arch | File |
|---|---|---|
| Linux | x86_64 | `eisencode-linux-amd64` |
| Linux | arm64 | `eisencode-linux-arm64` |
| macOS | Intel | `eisencode-darwin-amd64` |
| macOS | Apple Silicon | `eisencode-darwin-arm64` |
| Windows | x86_64 | `eisencode-windows-amd64.exe` |

## Quick start

```bash
eisencode login sk_eis_your_key     # from https://eisencode.waivy.dev/dashboard
eisencode run "explain raft in 3 lines"
eisencode chat
```

## Commands

| Command | What it does |
|---|---|
| `eisencode login <key>` | verify + save your key |
| `eisencode logout` | remove the saved key |
| `eisencode key` | key, plan, balance |
| `eisencode models` | list the 12 models |
| `eisencode run [prompt]` | one-shot completion (streams, reads stdin if no prompt) |
| `eisencode chat` | interactive chat (`/model`, `/clear`, `/reasoning`, `/exit`) |
| `eisencode usage` | recent requests + balance |
| `eisencode version` | version |

Flags: `--model <id>`, `--system <text>`, `--max-tokens <n>`, `--json`, `--no-stream`, `--reasoning`.

## Config

`~/.config/eisencode/config.json` (Linux), `~/Library/Application Support/eisencode/config.json` (macOS), `%AppData%\eisencode\config.json` (Windows). Written with `0600`.

Environment overrides:

```bash
export EISENCODE_API_KEY=sk_eis_...
export EISENCODE_BASE_URL=https://eisencode.waivy.dev/api/v1
export EISENCODE_MODEL=eisenberg/glm-5.3-flash
```

## Use with any OpenAI client

```python
from openai import OpenAI
client = OpenAI(base_url="https://eisencode.waivy.dev/api/v1", api_key="sk_eis_...")
print(client.chat.completions.create(
    model="eisenberg/glm-5.3-flash",
    messages=[{"role": "user", "content": "hi"}],
).choices[0].message.content)
```

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o eisencode .
```
