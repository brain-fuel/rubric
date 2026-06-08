package controller

import (
	"bytes"
	"strings"
	"testing"

	"goforge.dev/gat/components/config"
	inputsrc "goforge.dev/gat/components/inputsrc"
	"goforge.dev/gat/components/linerange"
	"goforge.dev/gat/components/style"
)

func baseConfig() config.Config {
	cfg := config.Default()
	cfg.TermWidth = 80
	cfg.TabWidth = 4
	cfg.WrappingMode = config.WrapNever
	cfg.Chop = false
	cfg.ColoredOutput = false
	return cfg
}

func render(t *testing.T, cfg config.Config, content string) string {
	t.Helper()
	var buf bytes.Buffer
	ctrl := New(cfg, strings.NewReader(""))
	in := inputsrc.FromBytes([]byte(content)).WithName("test.txt")
	if err := ctrl.Run(&buf, []inputsrc.Input{in}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return buf.String()
}

func TestNumbersDecoration(t *testing.T) {
	cfg := baseConfig()
	cfg.StyleComponents = style.NewComponents([]style.Component{style.LineNumbers})
	out := render(t, cfg, "alpha\nbeta\ngamma\n")
	wantLines := []string{"   1 alpha", "   2 beta", "   3 gamma"}
	for _, w := range wantLines {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q:\n%s", w, out)
		}
	}
}

func TestLineRangeFiltersLines(t *testing.T) {
	cfg := baseConfig()
	cfg.StyleComponents = style.NewComponents([]style.Component{style.LineNumbers})
	cfg.VisibleLines = config.VisibleLines{
		Ranges: linerange.NewLineRanges([]linerange.LineRange{linerange.New(2, 3)}),
	}
	out := render(t, cfg, "one\ntwo\nthree\nfour\n")
	if strings.Contains(out, "one") || strings.Contains(out, "four") {
		t.Errorf("range should exclude lines 1 and 4:\n%s", out)
	}
	if !strings.Contains(out, "two") || !strings.Contains(out, "three") {
		t.Errorf("range should include lines 2 and 3:\n%s", out)
	}
}

func TestCatModePreservesContent(t *testing.T) {
	cfg := baseConfig()
	cfg.LoopThrough = true
	content := "no\tdecoration\nhere\n"
	out := render(t, cfg, content)
	if out != content {
		t.Errorf("cat mode must pass content through unchanged: got %q want %q", out, content)
	}
}

func TestGridHeaderStructure(t *testing.T) {
	cfg := baseConfig()
	cfg.StyleComponents = style.NewComponents([]style.Component{style.Grid, style.HeaderFilename, style.LineNumbers})
	out := render(t, cfg, "x\n")
	if !strings.Contains(out, "File: test.txt") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "┬") || !strings.Contains(out, "┼") || !strings.Contains(out, "┴") {
		t.Errorf("missing grid corners:\n%s", out)
	}
}

func TestShowNonprintable(t *testing.T) {
	cfg := baseConfig()
	cfg.ShowNonprintable = true
	cfg.NonprintableNotation = config.NotationUnicode
	cfg.StyleComponents = style.NewComponents([]style.Component{style.Plain})
	out := render(t, cfg, "a b\tc\n")
	if !strings.Contains(out, "·") { // space marker
		t.Errorf("expected space marker ·:\n%q", out)
	}
	if !strings.Contains(out, "␊") { // newline marker
		t.Errorf("expected newline marker ␊:\n%q", out)
	}
}
