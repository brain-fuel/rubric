package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	flag "github.com/spf13/pflag"

	"goforge.dev/gat/components/assets"
)

// --- config / cache directories ----------------------------------------

func configDir() string {
	if d := os.Getenv("BAT_CONFIG_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "gat")
	}
	return filepath.Join(base, "gat")
}

func configFilePath() string {
	if p := os.Getenv("BAT_CONFIG_PATH"); p != "" {
		return p
	}
	return filepath.Join(configDir(), "config")
}

func cacheDir() string {
	if d := os.Getenv("BAT_CACHE_PATH"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".cache", "gat")
	}
	return filepath.Join(base, "gat")
}

const defaultConfigTemplate = `# This is gat's configuration file. Each line either contains a comment or
# a command-line option that you want to set as a default. For example:
#
#   --theme="Monokai Extended"
#   --style="numbers,changes,header"
#   --italic-text=always
#   --map-syntax "*.conf:INI"
`

func generateConfigFile() int {
	path := configFilePath()
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "gat: configuration file already exists at %s\n", path)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gat:", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gat:", err)
		return 1
	}
	fmt.Printf("Success! Config file written to %s\n", path)
	return 0
}

// --- diagnostic --------------------------------------------------------

func printDiagnostic(fs *flag.FlagSet) {
	fmt.Println("#### gat diagnostic")
	fmt.Println()
	fmt.Printf("**gat version**: %s\n", version)
	fmt.Printf("**Go version**: %s\n", runtime.Version())
	fmt.Printf("**OS / arch**: %s / %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("**Syntaxes**: %d, **Themes**: %d\n", len(assets.ListLanguages()), len(assets.ListThemes()))
	fmt.Printf("**Config file**: %s\n", configFilePath())
	fmt.Printf("**Config dir**: %s\n", configDir())
	fmt.Printf("**Cache dir**: %s\n", cacheDir())
	fmt.Println()
	fmt.Println("**Environment**:")
	for _, k := range []string{"BAT_THEME", "BAT_STYLE", "BAT_PAGER", "BAT_CONFIG_PATH",
		"PAGER", "TERM", "COLORTERM", "COLORFGBG", "NO_COLOR", "LESS", "LESSOPEN"} {
		if v, ok := os.LookupEnv(k); ok {
			fmt.Printf("- `%s=%s`\n", k, v)
		}
	}
}

// --- acknowledgements --------------------------------------------------

func acknowledgements() string {
	return strings.TrimSpace(`
gat is a Go port of bat (https://github.com/sharkdp/bat) by David Peter and
contributors, licensed under MIT OR Apache-2.0.

Syntax highlighting and themes are provided by chroma
(https://github.com/alecthomas/chroma) by Alec Thomas, licensed under MIT.

Terminal width handling uses golang.org/x/term and
github.com/mattn/go-runewidth. Flag parsing uses github.com/spf13/pflag.

This software bundles syntax and theme definitions derived from those projects;
see their respective licenses for the full acknowledgements.`)
}

// --- shell completions -------------------------------------------------

func printCompletion(shell string) int {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "gat: no completion for shell %q (try bash, zsh, fish)\n", shell)
		return 1
	}
	return 0
}

const bashCompletion = `# bash completion for gat
_gat() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="--show-all --plain --language --highlight-line --file-name --diff \
--diff-context --tabs --wrap --chop-long-lines --terminal-width --number \
--color --italic-text --decorations --force-colorization --paging --no-paging \
--pager --map-syntax --ignored-suffix --theme --theme-light --theme-dark \
--list-themes --squeeze-blank --squeeze-limit --strip-ansi --style --line-range \
--list-languages --unbuffered --no-config --completion --config-file \
--config-dir --cache-dir --diagnostic --acknowledgements --help --version"
    case "${prev}" in
        --language|-l) COMPREPLY=( $(compgen -W "$(gat --list-languages 2>/dev/null | cut -d: -f1)" -- "${cur}") ); return 0 ;;
        --theme) COMPREPLY=( $(compgen -W "$(gat --list-themes 2>/dev/null)" -- "${cur}") ); return 0 ;;
        --color|--paging|--decorations) COMPREPLY=( $(compgen -W "auto never always" -- "${cur}") ); return 0 ;;
        --wrap) COMPREPLY=( $(compgen -W "auto never character" -- "${cur}") ); return 0 ;;
    esac
    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
    COMPREPLY=( $(compgen -f -- "${cur}") )
}
complete -F _gat gat
`

const zshCompletion = `#compdef gat
# zsh completion for gat
_gat() {
    _arguments -s \
        '(-A --show-all)'{-A,--show-all}'[Show non-printable characters]' \
        '(-p --plain)'{-p,--plain}'[Show plain style]' \
        '(-l --language)'{-l,--language}'[Set the language]:language:($(gat --list-languages 2>/dev/null | cut -d: -f1))' \
        '(-n --number)'{-n,--number}'[Show line numbers only]' \
        '--theme[Set the theme]:theme:($(gat --list-themes 2>/dev/null))' \
        '--color[When to use colors]:when:(auto never always)' \
        '--paging[When to page]:when:(auto never always)' \
        '--wrap[Wrapping mode]:mode:(auto never character)' \
        '--style[Style components]:style:' \
        '(-r --line-range)'{-r,--line-range}'[Only print given lines]:range:' \
        '(-L --list-languages)'{-L,--list-languages}'[List languages]' \
        '--list-themes[List themes]' \
        '(-h --help)'{-h,--help}'[Show help]' \
        '(-V --version)'{-V,--version}'[Show version]' \
        '*:file:_files'
}
_gat "$@"
`

const fishCompletion = `# fish completion for gat
complete -c gat -s A -l show-all -d 'Show non-printable characters'
complete -c gat -s p -l plain -d 'Show plain style'
complete -c gat -s n -l number -d 'Show line numbers only'
complete -c gat -s l -l language -d 'Set the language' -x -a '(gat --list-languages 2>/dev/null | cut -d: -f1)'
complete -c gat -l theme -d 'Set the theme' -x -a '(gat --list-themes 2>/dev/null)'
complete -c gat -l color -d 'When to use colors' -x -a 'auto never always'
complete -c gat -l paging -d 'When to page' -x -a 'auto never always'
complete -c gat -l wrap -d 'Wrapping mode' -x -a 'auto never character'
complete -c gat -s r -l line-range -d 'Only print given lines'
complete -c gat -s L -l list-languages -d 'List languages'
complete -c gat -l list-themes -d 'List themes'
complete -c gat -s h -l help -d 'Show help'
complete -c gat -s V -l version -d 'Show version'
`
