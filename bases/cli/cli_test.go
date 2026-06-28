package cli

import (
	"strings"
	"testing"
)

func TestBuildConfigRejectsInvalidChoiceFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "color", args: []string{"--color=sometimes"}, want: `invalid --color "sometimes"`},
		{name: "wrap", args: []string{"--wrap=sideways"}, want: `invalid --wrap "sideways"`},
		{name: "paging", args: []string{"--paging=maybe"}, want: `invalid --paging "maybe"`},
		{name: "strip ansi", args: []string{"--strip-ansi=maybe"}, want: `invalid --strip-ansi "maybe"`},
		{name: "decorations", args: []string{"--decorations=maybe"}, want: `invalid --decorations "maybe"`},
		{name: "binary", args: []string{"--binary=maybe"}, want: `invalid --binary "maybe"`},
		{name: "notation", args: []string{"--nonprintable-notation=maybe"}, want: `invalid --nonprintable-notation "maybe"`},
		{name: "italic", args: []string{"--italic-text=maybe"}, want: `invalid --italic-text "maybe"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFlagSet()
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err := buildConfig(fs, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestBuildConfigRejectsInvalidIntegerFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "tabs text", args: []string{"--tabs=two"}, want: `invalid --tabs "two"`},
		{name: "tabs negative", args: []string{"--tabs=-1"}, want: `invalid --tabs "-1"`},
		{name: "squeeze", args: []string{"--squeeze-blank", "--squeeze-limit=nope"}, want: `invalid --squeeze-limit "nope"`},
		{name: "diff", args: []string{"--diff", "--diff-context=nope"}, want: `invalid --diff-context "nope"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFlagSet()
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err := buildConfig(fs, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestBuildConfigAcceptsDocumentedChoiceFlags(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse([]string{
		"--color=never",
		"--wrap=character",
		"--paging=never",
		"--strip-ansi=auto",
		"--decorations=always",
		"--binary=as-text",
		"--nonprintable-notation=caret",
		"--italic-text=never",
		"--tabs=0",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := buildConfig(fs, false); err != nil {
		t.Fatalf("buildConfig returned error: %v", err)
	}
}
