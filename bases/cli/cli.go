// Package cli is rubric's entry-point layer. It ports bat's bin/bat application:
// it defines the full command-line surface (clap_app.rs), reads the config file
// and environment, resolves every option into a config.Config, builds the list
// of inputs, opens the pager, and runs the controller.
package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"

	"goforge.dev/rubric/components/assets"
	"goforge.dev/rubric/components/config"
	"goforge.dev/rubric/components/controller"
	inputsrc "goforge.dev/rubric/components/inputsrc"
	"goforge.dev/rubric/components/linerange"
	pager "goforge.dev/rubric/components/pager"
	"goforge.dev/rubric/components/style"
	termdetect "goforge.dev/rubric/components/termdetect"
)

const version = "1.0.0"

// Run parses args (excluding the program name), executes rubric, and returns the
// process exit code.
func Run(args []string) int {
	fs := newFlagSet()

	// Prepend config-file args (BAT_CONFIG_PATH / ~/.config/rubric/config), unless
	// disabled with --no-config, mirroring bat's config precedence.
	allArgs := args
	if !hasFlag(args, "--no-config") {
		if cfgArgs := loadConfigArgs(); len(cfgArgs) > 0 {
			allArgs = append(append([]string{}, cfgArgs...), args...)
		}
	}

	if err := fs.Parse(allArgs); err != nil {
		fmt.Fprintln(os.Stderr, "rubric:", err)
		return 2
	}

	if getBool(fs, "version") {
		fmt.Printf("rubric %s\n", version)
		return 0
	}
	if getString(fs, "completion") != "" {
		return printCompletion(getString(fs, "completion"))
	}
	if getBool(fs, "diagnostic") {
		printDiagnostic(fs)
		return 0
	}
	if getBool(fs, "config-file") {
		fmt.Println(configFilePath())
		return 0
	}
	if getBool(fs, "config-dir") {
		fmt.Println(configDir())
		return 0
	}
	if getBool(fs, "cache-dir") {
		fmt.Println(cacheDir())
		return 0
	}
	if getBool(fs, "generate-config-file") {
		return generateConfigFile()
	}
	if getBool(fs, "acknowledgements") {
		fmt.Println(acknowledgements())
		return 0
	}
	if getBool(fs, "list-themes") {
		for _, t := range assets.ListThemes() {
			fmt.Println(t)
		}
		return 0
	}
	if getBool(fs, "list-languages") {
		listLanguages()
		return 0
	}

	interactive := termdetect.StdoutIsTerminal()
	cfg, err := buildConfig(fs, interactive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rubric:", err)
		return 1
	}

	inputs := buildInputs(fs.Args(), fs)

	// Output: pager when interactive and paging enabled.
	chop := cfg.WrappingMode == config.WrapNever && cfg.Chop
	out := pager.FromMode(cfg.PagingMode, cfg.Pager, interactive, chop,
		cfg.PagingMode == config.PagingAuto || cfg.PagingMode == config.PagingQuitIfOneScreen)
	defer out.Close()

	// Set the terminal title when paging, mirroring bat.
	if cfg.SetTerminalTitle && cfg.PagingMode != config.PagingNever {
		fmt.Fprintf(os.Stdout, "\x1b]2;rubric: %s\x07", strings.Join(fs.Args(), " "))
	}

	ctrl := controller.New(cfg, os.Stdin)
	if err := ctrl.Run(out, inputs); err != nil {
		return 1
	}
	return 0
}

func newFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("rubric", flag.ContinueOnError)
	fs.SortFlags = false

	fs.BoolP("show-all", "A", false, "Show non-printable characters (space, tab, newline, ...)")
	fs.String("nonprintable-notation", "unicode", "Set notation for non-printable characters (unicode|caret)")
	fs.String("binary", "no-printing", "How to treat binary content (no-printing|as-text)")
	fs.CountP("plain", "p", "Show plain style (no decorations); -pp also disables paging")
	fs.StringP("language", "l", "", "Set the language for syntax highlighting")
	fs.String("fallback-syntax", "", "Set the fallback syntax when detection fails")
	fs.StringArrayP("highlight-line", "H", nil, "Highlight the specified line ranges with a different background color")
	fs.String("file-name", "", "Specify the display name for the file")
	fs.BoolP("diff", "d", false, "Only show lines that have been added/removed/modified")
	fs.String("diff-context", "2", "Include N context lines around changes in --diff mode")
	fs.String("tabs", "", "Set the tab width; 0 passes tabs through unchanged")
	fs.String("wrap", "auto", "Specify the text-wrapping mode (auto|never|character)")
	fs.BoolP("chop-long-lines", "S", false, "Truncate long lines instead of wrapping")
	fs.String("terminal-width", "", "Explicitly set the terminal width (or +N / -N delta)")
	fs.BoolP("number", "n", false, "Only show line numbers, no other decorations")
	fs.String("color", "auto", "When to use colors (auto|never|always)")
	fs.String("italic-text", "never", "Use italics in output (always|never)")
	fs.String("decorations", "auto", "When to show the decorations (auto|never|always)")
	fs.BoolP("force-colorization", "f", false, "Force color and decorations")
	fs.String("paging", "auto", "When to use the pager (auto|never|always)")
	fs.BoolP("no-paging", "P", false, "Alias for --paging=never")
	fs.String("pager", "", "Determine the pager to use")
	fs.StringArrayP("map-syntax", "m", nil, "Map a glob pattern to a syntax: '<glob>:<syntax>'")
	fs.StringArray("ignored-suffix", nil, "Ignore extension when guessing the syntax")
	fs.String("theme", "", "Set the highlighting theme")
	fs.String("theme-light", "", "Theme to use when the terminal is light")
	fs.String("theme-dark", "", "Theme to use when the terminal is dark")
	fs.Bool("list-themes", false, "Display all supported highlighting themes")
	fs.BoolP("squeeze-blank", "s", false, "Squeeze consecutive empty lines into one")
	fs.String("squeeze-limit", "", "Set the maximum number of consecutive empty lines to display")
	fs.String("strip-ansi", "auto", "Strip ANSI escapes from input (auto|always|never)")
	fs.String("style", "", "Comma-separated list of style components to show")
	fs.StringArrayP("line-range", "r", nil, "Only print the lines from N to M")
	fs.BoolP("list-languages", "L", false, "Display all supported languages")
	fs.BoolP("unbuffered", "u", false, "(ignored; for cat compatibility)")
	fs.Bool("no-config", false, "Do not use the configuration file")
	fs.Bool("no-custom-assets", false, "Do not load custom assets")
	fs.String("completion", "", "Generate a shell completion script (bash|zsh|fish)")
	fs.Bool("lessopen", false, "Enable the $LESSOPEN preprocessor")
	fs.Bool("no-lessopen", false, "Disable the $LESSOPEN preprocessor")
	fs.Bool("config-file", false, "Show path to the configuration file")
	fs.Bool("generate-config-file", false, "Generate a default configuration file")
	fs.Bool("config-dir", false, "Show rubric's configuration directory")
	fs.Bool("cache-dir", false, "Show rubric's cache directory")
	fs.Bool("diagnostic", false, "Show diagnostic information for bug reports")
	fs.Bool("acknowledgements", false, "Show acknowledgements")
	fs.Bool("quiet-empty", false, "Produce no output when the input is empty")
	fs.Bool("set-terminal-title", false, "Set the terminal title when using a pager")
	fs.BoolP("version", "V", false, "Show version information")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "rubric %s — a cat(1) clone with wings (bat in Go).\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: rubric [OPTIONS] [FILE]...\n\nOptions:\n")
		fs.PrintDefaults()
	}
	return fs
}

func buildConfig(fs *flag.FlagSet, interactive bool) (config.Config, error) {
	cfg := config.Default()
	var err error

	plainCount, _ := fs.GetCount("plain")
	plain := plainCount > 0
	number := getBool(fs, "number")
	showAll := getBool(fs, "show-all")
	force := getBool(fs, "force-colorization")

	// Colored output.
	switch getString(fs, "color") {
	case "always":
		cfg.ColoredOutput = true
	case "never":
		cfg.ColoredOutput = false
	default: // auto
		cfg.ColoredOutput = interactive && termdetect.ColorEnabled()
	}
	if force {
		cfg.ColoredOutput = true
		interactive = true
	}
	cfg.TrueColor = termdetect.TrueColor()
	cfg.UseItalicText = getString(fs, "italic-text") == "always"

	// Non-printable.
	cfg.ShowNonprintable = showAll
	if getString(fs, "nonprintable-notation") == "caret" {
		cfg.NonprintableNotation = config.NotationCaret
	} else {
		cfg.NonprintableNotation = config.NotationUnicode
	}
	if getString(fs, "binary") == "as-text" {
		cfg.Binary = config.BinaryAsText
	}

	// Terminal width.
	cfg.TermWidth = resolveTermWidth(getString(fs, "terminal-width"))

	// Tab width.
	cfg.TabWidth = 4
	if tw := getString(fs, "tabs"); tw != "" {
		if n, err := strconv.Atoi(tw); err == nil {
			cfg.TabWidth = n
		}
	}

	// Wrapping.
	termWidthExplicit := getString(fs, "terminal-width") != ""
	switch getString(fs, "wrap") {
	case "never":
		cfg.WrappingMode = config.WrapNever
		cfg.Chop = true
	case "word":
		cfg.WrappingMode = config.WrapWord
	case "character":
		cfg.WrappingMode = config.WrapCharacter
	default: // auto: wrap when interactive or when a width was set explicitly
		if interactive || termWidthExplicit {
			cfg.WrappingMode = config.WrapCharacter
		} else {
			cfg.WrappingMode = config.WrapNever
		}
	}
	if getBool(fs, "chop-long-lines") {
		cfg.WrappingMode = config.WrapNever
		cfg.Chop = true
	}

	// Paging.
	cfg.PagingMode = resolvePaging(fs, interactive)
	cfg.Pager = getString(fs, "pager")

	// Theme + light/dark selection.
	cfg.Theme = getString(fs, "theme")
	if cfg.Theme == "" {
		cfg.Theme = themeFromEnv()
	}
	cfg.ThemeLight = getString(fs, "theme-light")
	cfg.ThemeDark = getString(fs, "theme-dark")
	cfg.DarkBackground = detectDarkBackground()

	// Language / fallback.
	cfg.Language = getString(fs, "language")
	cfg.FallbackSyntax = getString(fs, "fallback-syntax")

	// Squeeze.
	if getBool(fs, "squeeze-blank") {
		cfg.SqueezeLines = 1
		if sl := getString(fs, "squeeze-limit"); sl != "" {
			if n, err := strconv.Atoi(sl); err == nil {
				cfg.SqueezeLines = n
			}
		}
	}

	// Strip ANSI.
	switch getString(fs, "strip-ansi") {
	case "always":
		cfg.StripAnsi = config.StripAlways
	case "never":
		cfg.StripAnsi = config.StripNever
	default:
		cfg.StripAnsi = config.StripAuto
	}

	cfg.QuietEmpty = getBool(fs, "quiet-empty")
	cfg.SetTerminalTitle = getBool(fs, "set-terminal-title")
	cfg.Unbuffered = getBool(fs, "unbuffered")
	cfg.UseLessOpen = getBool(fs, "lessopen") && !getBool(fs, "no-lessopen")

	// Style components.
	cfg.StyleComponents, err = resolveStyle(fs, interactive, plain, number, showAll)
	if err != nil {
		return cfg, err
	}

	// Decorations override.
	switch getString(fs, "decorations") {
	case "never":
		cfg.StyleComponents = style.NewComponents([]style.Component{style.Plain})
	case "always":
		// keep resolved components
	}

	// Tab width default: bat uses 0 (pass tabs through) in plain+no-paging mode,
	// otherwise 4.
	if getString(fs, "tabs") == "" && cfg.StyleComponents.Plain() && cfg.PagingMode == config.PagingNever {
		cfg.TabWidth = 0
	}

	// Plain (cat) mode: only when nothing requires per-line processing.
	if cfg.StyleComponents.Plain() && !cfg.ColoredOutput && !cfg.ShowNonprintable &&
		cfg.WrappingMode == config.WrapNever && cfg.SqueezeLines == 0 &&
		cfg.StripAnsi != config.StripAlways && cfg.HighlightedLines.Ranges.Empty() &&
		!cfg.VisibleLines.DiffMode && len(getStringArray(fs, "line-range")) == 0 {
		cfg.LoopThrough = true
	}

	// Syntax mappings + ignored suffixes.
	for _, s := range getStringArray(fs, "ignored-suffix") {
		cfg.SyntaxMapping.AddIgnoredSuffix(s)
	}
	maps := getStringArray(fs, "map-syntax")
	for i := len(maps) - 1; i >= 0; i-- { // later args take precedence
		parts := strings.SplitN(maps[i], ":", 2)
		if len(parts) != 2 {
			return cfg, fmt.Errorf("invalid syntax mapping %q; expected '<glob>:<syntax>'", maps[i])
		}
		cfg.SyntaxMapping.Insert(parts[0], config.MappingTarget{MapTo: parts[1]})
	}

	// Visible lines / diff.
	if getBool(fs, "diff") {
		cfg.VisibleLines = config.VisibleLines{DiffMode: true, DiffContext: 2}
		if dc := getString(fs, "diff-context"); dc != "" {
			if n, err := strconv.Atoi(dc); err == nil {
				cfg.VisibleLines.DiffContext = n
			}
		}
		cfg.StyleComponents.Insert(style.Changes)
	} else if ranges := getStringArray(fs, "line-range"); len(ranges) > 0 {
		var lrs []linerange.LineRange
		for _, r := range ranges {
			lr, err := linerange.Parse(r)
			if err != nil {
				return cfg, err
			}
			lrs = append(lrs, lr)
		}
		cfg.VisibleLines = config.VisibleLines{Ranges: linerange.NewLineRanges(lrs)}
	}

	// Highlight lines.
	if hl := getStringArray(fs, "highlight-line"); len(hl) > 0 {
		var lrs []linerange.LineRange
		for _, r := range hl {
			lr, err := linerange.Parse(r)
			if err != nil {
				return cfg, err
			}
			lrs = append(lrs, lr)
		}
		cfg.HighlightedLines = linerange.HighlightedLineRanges{Ranges: linerange.NewLineRanges(lrs)}
	}

	return cfg, nil
}

func resolveStyle(fs *flag.FlagSet, interactive, plain, number, showAll bool) (style.Components, error) {
	if plain {
		return style.NewComponents([]style.Component{style.Plain}), nil
	}
	if number {
		return style.NewComponents([]style.Component{style.LineNumbers}), nil
	}

	styleStr := getString(fs, "style")
	if styleStr == "" {
		styleStr = os.Getenv("BAT_STYLE")
	}
	if styleStr == "" {
		// Default style is "auto": expands to the default set on an interactive
		// terminal, and to plain when output is redirected.
		return style.ToComponents(nil, interactive, true), nil
	}
	list, err := style.ParseComponentList(styleStr)
	if err != nil {
		return style.Components{}, err
	}
	return style.ToComponents([]style.ComponentList{list}, interactive, false), nil
}

func resolvePaging(fs *flag.FlagSet, interactive bool) config.PagingMode {
	if getBool(fs, "no-paging") {
		return config.PagingNever
	}
	pc, _ := fs.GetCount("plain")
	if pc >= 2 { // -pp disables paging
		return config.PagingNever
	}
	switch getString(fs, "paging") {
	case "always":
		return config.PagingAlways
	case "never":
		return config.PagingNever
	default: // auto
		if interactive {
			return config.PagingQuitIfOneScreen
		}
		return config.PagingNever
	}
}

func resolveTermWidth(arg string) int {
	cur := termdetect.Width()
	if arg == "" {
		return cur
	}
	if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
		if delta, err := strconv.Atoi(arg); err == nil {
			w := cur + delta
			if w < 1 {
				w = 1
			}
			return w
		}
		return cur
	}
	if n, err := strconv.Atoi(arg); err == nil && n > 0 {
		return n
	}
	return cur
}

// detectDarkBackground guesses whether the terminal has a dark background. It
// honors COLORFGBG (set by some terminals as "fg;bg") and otherwise assumes a
// dark background, matching bat's default assumption.
func detectDarkBackground() bool {
	fgbg := os.Getenv("COLORFGBG")
	if fgbg == "" {
		return true
	}
	parts := strings.Split(fgbg, ";")
	bg := parts[len(parts)-1]
	if n, err := strconv.Atoi(strings.TrimSpace(bg)); err == nil {
		// ANSI colors 0-6 and 8 are dark; 7 and 9-15 are light.
		return n < 7 || n == 8
	}
	return true
}

func themeFromEnv() string {
	if t := os.Getenv("BAT_THEME"); t != "" {
		return t
	}
	return assets.DefaultThemeName
}

func buildInputs(files []string, fs *flag.FlagSet) []inputsrc.Input {
	name := getString(fs, "file-name")
	if len(files) == 0 {
		in := inputsrc.StdIn()
		if name != "" {
			in = in.WithName(name)
		}
		return []inputsrc.Input{in}
	}
	var inputs []inputsrc.Input
	for _, f := range files {
		var in inputsrc.Input
		if f == "-" {
			in = inputsrc.StdIn()
		} else {
			in = inputsrc.OrdinaryFile(f)
		}
		if name != "" {
			in = in.WithName(name)
		}
		inputs = append(inputs, in)
	}
	return inputs
}

func listLanguages() {
	langs := assets.ListLanguages()
	sort.Slice(langs, func(i, j int) bool { return langs[i].Name < langs[j].Name })
	for _, l := range langs {
		exts := strings.TrimPrefix(strings.Join(l.Extensions, ","), "")
		fmt.Printf("%s:%s\n", l.Name, exts)
	}
}

// loadConfigArgs reads rubric's config file and returns it as argv tokens.
func loadConfigArgs() []string {
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return nil
	}
	return parseConfigTokens(string(data))
}

// parseConfigTokens parses a bat-style config file: one option per line,
// '#' comments, shell-like quoting.
func parseConfigTokens(content string) []string {
	var args []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args = append(args, tokenize(line)...)
	}
	return args
}

func tokenize(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range line {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// --- small flag helpers -------------------------------------------------

func getBool(fs *flag.FlagSet, name string) bool   { v, _ := fs.GetBool(name); return v }
func getString(fs *flag.FlagSet, name string) string {
	v, _ := fs.GetString(name)
	return v
}
func getStringArray(fs *flag.FlagSet, name string) []string {
	v, _ := fs.GetStringArray(name)
	return v
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

var _ io.Writer
