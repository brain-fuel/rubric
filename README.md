# rubric

`rubric` is **bat written in Go** — a `cat(1)` clone with wings. It ports
[`sharkdp/bat`](https://github.com/sharkdp/bat) (reference: v0.26.1 in
`ref_impl/bat`) to Go, aiming for feature parity while tracking the latest
upstream behavior (e.g. no phantom git-change column, `Size:` in the `full`
style) rather than the quirks of older releases.

Syntax highlighting and themes are provided by
[`chroma`](https://github.com/alecthomas/chroma) (the Go analogue of Rust's
`syntect`): ~296 languages and ~74 themes out of the box.

## Build & run

```sh
go build -o rubric ./projects/rubric
./rubric src/main.go
./rubric --style=full --theme=GitHub README.md
git diff | ./rubric -l diff
```

## Features

- Syntax highlighting (language auto-detection by name, extension and content)
- Themes (`--theme`, `--list-themes`; bat theme names aliased to chroma styles)
- Decorations: line numbers, grid, header (filename/size), rule, snip
  (`--style`, with `+add`/`-remove`/override list semantics)
- Git integration: `+`/`~`/`‾`/`_` change markers (`--style=changes`) via the
  `git` CLI; `--diff` / `--diff-context` to show only changed regions
- Line ranges (`-r`, supports `N`, `N:M`, `N:`, `:M`, `N:+K`, `N:-K`, `N::C`,
  `N:M:C`, and end-relative `:-K`) and line highlighting (`-H`)
- Wrapping (`--wrap`, character wrapping with a continuation gutter) and
  `--chop-long-lines`
- Non-printable rendering (`-A`, `--nonprintable-notation=unicode|caret`)
- Tab expansion (`--tabs`), squeeze blank lines (`-s`/`--squeeze-limit`)
- Paging through `less` (`--paging`, `--pager`, `-P`); auto when interactive
- Syntax mapping (`-m '<glob>:<syntax>'`), ignored suffixes (`--ignored-suffix`)
- Config file (`$BAT_CONFIG_PATH` or `~/.config/rubric/config`), `--no-config`
- Env vars: `BAT_THEME`, `BAT_STYLE`, `BAT_PAGER`, `PAGER`, `NO_COLOR`,
  `COLORTERM`, `COLUMNS`
- Reads files, multiple files (concatenation with per-file headers), and stdin
- UTF-16 (LE/BE) and UTF-8 BOM decoding; binary detection + `--binary=as-text`
- `--strip-ansi`, man-page overstrike stripping, word/character wrapping
- Light/dark theme switching (`--theme-light`/`--theme-dark`, `COLORFGBG`)
- Shell completions (`--completion bash|zsh|fish`), `--acknowledgements`,
  `--diagnostic`, `--config-file`/`--config-dir`/`--cache-dir`/
  `--generate-config-file`, `$LESSOPEN` preprocessing, `--set-terminal-title`

### Intentional divergence from bat

bat's `cache` subcommand (`--build`/`--clear`/`--source`/`--target`) builds and
caches custom `.sublime-syntax` / `.tmTheme` assets. rubric uses chroma's bundled
syntaxes and themes, so there is no asset-cache step; `--no-custom-assets` and
`--no-config` are accepted, and `--cache-dir` still reports a path for tooling
compatibility.

## Architecture (goforge / Polylith)

The workspace is organized with [`goforge`](https://goforge.dev) into single-
responsibility **components** (each an interface package + private `internal`
boundary), an entry-point **base**, and a deployable **project**. Run
`goforge check` to validate boundaries, `goforge info` for the brick matrix.

| Brick | Role (bat module it ports) |
|-------|----------------------------|
| `config` | `config.rs` — Config struct, option enums, syntax mapping |
| `style` | `style.rs` — StyleComponent set + list parsing |
| `linerange` | `line_range.rs` — range parsing & membership |
| `inputsrc` | `input.rs` — file / stdin / bytes sources |
| `termdetect` | terminal width, tty, truecolor, color enablement |
| `assets` | `assets.rs` — chroma lexer/theme lookup, styled segments |
| `decorations` | `decorations.rs` — gutter elements |
| `printer` | `printer.rs` — interactive + simple printers |
| `pager` | `output.rs` — stdout vs `less` |
| `gitdiff` | `diff.rs` — per-line git changes |
| `controller` | `controller.rs` — orchestration |
| `cli` (base) | `bin/bat` — flag surface, config building, wiring |
| `rubric` (project) | the `rubric` binary |

## Tests

```sh
go test ./...
goforge check
```

Unit tests cover line-range parsing, the style-list precedence rules, and
end-to-end rendering (numbers, ranges, grid/header, cat mode, `-A`).
