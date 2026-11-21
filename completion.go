package main

import (
	"fmt"
	"os"
)

var cliCommands = []string{
	"login", "logout", "key", "provider", "models", "run", "chat",
	"sessions", "search", "keys", "completion", "logo", "usage", "version", "help",
}

func cmdCompletion(shell string) int {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	case "powershell", "pwsh":
		fmt.Print(powershellCompletion)
	case "", "help":
		fmt.Fprintln(os.Stderr, "usage: eisencode completion <bash|zsh|fish|powershell>")
		return 2
	default:
		fmt.Fprintf(os.Stderr, "%sunknown shell %q%s\n", ansiRed, shell, ansiReset)
		return 2
	}
	return 0
}

var bashCompletion = `# bash completion for eisencode
_eisencode() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  local cmds="login logout key provider models run chat sessions search keys completion logo usage version help"
  case "$prev" in
    --model|-m) COMPREPLY=( $(compgen -W "E-1-Flash beam deepseek-v4.1-flash flint-40b hy4-flash iceman longcat-2.5 mimo-v2.6-flash nemotron-3.5-lightning nemotron-3.5-super-vl tidepool-alpha" -- "$cur") ); return ;;
    --in) COMPREPLY=( $(compgen -W "eisen opencode all" -- "$cur") ); return ;;
    --source) COMPREPLY=( $(compgen -W "eisen opencode all" -- "$cur") ); return ;;
    completion) COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "$cur") ); return ;;
    sessions) COMPREPLY=( $(compgen -W "list show rm rename export" -- "$cur") ); return ;;
  esac
  COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
}
complete -F _eisencode eisencode
`

var zshCompletion = `#compdef eisencode
_eisencode() {
  local -a cmds models scopes
  cmds=(login logout key provider models run chat sessions search keys completion logo usage version help)
  models=(E-1-Flash beam deepseek-v4.1-flash flint-40b hy4-flash iceman longcat-2.5 mimo-v2.6-flash nemotron-3.5-lightning nemotron-3.5-super-vl tidepool-alpha)
  scopes=(eisen opencode all)
  case $words[2] in
    completion) _values 'shell' bash zsh fish powershell ;;
    sessions) _values 'subcommand' list show rm rename export ;;
    *) _arguments \
         '--model[model id]:model:($models)' \
         '--system[system prompt]:text:' \
         '--max-tokens[max output]:n:' \
         '--json[raw json]' \
         '--no-stream[wait for full answer]' \
         '--reasoning[show reasoning]' \
         '--in[search scope]:scope:($scopes)' \
         '1:command:($cmds)' ;;
  esac
}
_eisencode "$@"
`

var fishCompletion = `# fish completion for eisencode
complete -c eisencode -f
complete -c eisencode -n "__fish_use_subcommand" -a "login logout key provider models run chat sessions search keys completion logo usage version help"
complete -c eisencode -n "__fish_seen_subcommand_from sessions" -a "list show rm rename export"
complete -c eisencode -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"
complete -c eisencode -l model -d "model id"
complete -c eisencode -l system -d "system prompt"
complete -c eisencode -l max-tokens -d "max output tokens"
complete -c eisencode -l json -d "raw json"
complete -c eisencode -l no-stream -d "wait for full answer"
complete -c eisencode -l reasoning -d "show reasoning"
complete -c eisencode -l in -x -a "eisen opencode all" -d "search scope"
`

var powershellCompletion = `Register-ArgumentCompleter -Native -CommandName eisencode -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $cmds = 'login','logout','key','provider','models','run','chat','sessions','search','keys','completion','logo','usage','version','help'
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
