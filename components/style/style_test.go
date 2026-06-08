package style

import "testing"

func comps(t *testing.T, s string) Components {
	t.Helper()
	list, err := ParseComponentList(s)
	if err != nil {
		t.Fatalf("ParseComponentList(%q): %v", s, err)
	}
	return ToComponents([]ComponentList{list}, false, false)
}

func TestParseListError(t *testing.T) {
	for _, s := range []string{"not-a-component", "grid,not-a-component", "numbers,-not-a-component"} {
		if _, err := ParseComponentList(s); err == nil {
			t.Errorf("ParseComponentList(%q) expected error", s)
		}
	}
}

func TestToComponentsBasic(t *testing.T) {
	c := comps(t, "grid,numbers")
	if !c.Grid() || !c.Numbers() {
		t.Fatalf("expected grid+numbers")
	}
}

func TestToComponentsRemovesNegated(t *testing.T) {
	c := comps(t, "grid,numbers,-grid")
	if c.Grid() || !c.Numbers() {
		t.Fatalf("expected numbers only, got grid=%v numbers=%v", c.Grid(), c.Numbers())
	}
}

func TestFullExpands(t *testing.T) {
	c := comps(t, "full")
	if !c.Grid() || !c.Numbers() || !c.HeaderFilename() || !c.HeaderFilesize() || !c.Snip() || !c.Changes() {
		t.Fatalf("full should expand to all components")
	}
}

func TestPrecedenceOverride(t *testing.T) {
	c := ToComponents([]ComponentList{
		mustList(t, "grid"),
		mustList(t, "numbers"),
	}, false, false)
	if c.Grid() {
		t.Fatalf("later override should clear grid")
	}
	if !c.Numbers() {
		t.Fatalf("expected numbers")
	}
}

func TestPrecedenceMerge(t *testing.T) {
	c := ToComponents([]ComponentList{
		mustList(t, "grid,header"),
		mustList(t, "-grid"),
		mustList(t, "+numbers"),
	}, false, false)
	if c.Grid() || !c.HeaderFilename() || !c.Numbers() {
		t.Fatalf("merge precedence wrong: grid=%v header=%v numbers=%v", c.Grid(), c.HeaderFilename(), c.Numbers())
	}
}

func TestDefaultBuildsOnAuto(t *testing.T) {
	c := ToComponents([]ComponentList{mustList(t, "-numbers")}, true, true)
	if c.Numbers() {
		t.Fatalf("numbers should be removed from auto default")
	}
	if !c.Grid() {
		t.Fatalf("auto default should include grid on interactive terminal")
	}
}

func mustList(t *testing.T, s string) ComponentList {
	t.Helper()
	l, err := ParseComponentList(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
