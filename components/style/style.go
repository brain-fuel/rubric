// Package style ports bat's style module: the StyleComponent enum, the
// StyleComponents set queried by the printer, and the StyleComponentList
// parser that understands override / "+add" / "-remove" semantics.
package style

import (
	"fmt"
	"strings"
)

// Component is a single style element.
type Component int

const (
	Auto Component = iota
	Changes
	Grid
	Rule
	Header
	HeaderFilename
	HeaderFilesize
	LineNumbers
	Snip
	Full
	Default
	Plain
)

// ParseComponent parses one style name.
func ParseComponent(s string) (Component, error) {
	switch s {
	case "auto":
		return Auto, nil
	case "changes":
		return Changes, nil
	case "grid":
		return Grid, nil
	case "rule":
		return Rule, nil
	case "header":
		return Header, nil
	case "header-filename":
		return HeaderFilename, nil
	case "header-filesize":
		return HeaderFilesize, nil
	case "numbers":
		return LineNumbers, nil
	case "snip":
		return Snip, nil
	case "full":
		return Full, nil
	case "default":
		return Default, nil
	case "plain":
		return Plain, nil
	default:
		return 0, fmt.Errorf("Unknown style '%s'", s)
	}
}

// expand returns the concrete sub-components a Component stands for.
func (c Component) expand(interactive bool) []Component {
	switch c {
	case Auto:
		if interactive {
			return Default.expand(interactive)
		}
		return Plain.expand(interactive)
	case Changes:
		return []Component{Changes}
	case Grid:
		return []Component{Grid}
	case Rule:
		return []Component{Rule}
	case Header:
		return []Component{HeaderFilename}
	case HeaderFilename:
		return []Component{HeaderFilename}
	case HeaderFilesize:
		return []Component{HeaderFilesize}
	case LineNumbers:
		return []Component{LineNumbers}
	case Snip:
		return []Component{Snip}
	case Full:
		return []Component{Changes, Grid, HeaderFilename, HeaderFilesize, LineNumbers, Snip}
	case Default:
		return []Component{Changes, Grid, HeaderFilename, LineNumbers, Snip}
	case Plain:
		return nil
	default:
		return nil
	}
}

// Components is the resolved set of active style elements.
type Components struct {
	set map[Component]bool
}

// NewComponents builds a set from the given elements.
func NewComponents(cs []Component) Components {
	m := make(map[Component]bool, len(cs))
	for _, c := range cs {
		m[c] = true
	}
	return Components{set: m}
}

func (sc Components) has(c Component) bool { return sc.set[c] }

func (sc Components) Changes() bool        { return sc.has(Changes) }
func (sc Components) Grid() bool           { return sc.has(Grid) }
func (sc Components) Rule() bool           { return sc.has(Rule) }
func (sc Components) HeaderFilename() bool { return sc.has(HeaderFilename) }
func (sc Components) HeaderFilesize() bool { return sc.has(HeaderFilesize) }
func (sc Components) Header() bool         { return sc.HeaderFilename() || sc.HeaderFilesize() }
func (sc Components) Numbers() bool        { return sc.has(LineNumbers) }
func (sc Components) Snip() bool           { return sc.has(Snip) }

// Plain reports whether no decoration components are active.
func (sc Components) Plain() bool {
	for c := range sc.set {
		if c != Plain {
			return false
		}
	}
	return true
}

// Insert adds a component.
func (sc *Components) Insert(c Component) {
	if sc.set == nil {
		sc.set = map[Component]bool{}
	}
	sc.set[c] = true
}

// Clear empties the set.
func (sc *Components) Clear() { sc.set = map[Component]bool{} }

type action int

const (
	actOverride action = iota
	actAdd
	actRemove
)

func extractAction(s string) (action, string) {
	if s == "" {
		return actOverride, s
	}
	switch s[0] {
	case '-':
		return actRemove, s[1:]
	case '+':
		return actAdd, s[1:]
	default:
		return actOverride, s
	}
}

type entry struct {
	act action
	cmp Component
}

// ComponentList is a parsed --style list that may contain +/- prefixes.
type ComponentList struct {
	entries []entry
}

// ParseComponentList parses a comma-separated --style value.
func ParseComponentList(s string) (ComponentList, error) {
	var list ComponentList
	for _, part := range strings.Split(s, ",") {
		act, name := extractAction(part)
		cmp, err := ParseComponent(name)
		if err != nil {
			return ComponentList{}, err
		}
		list.entries = append(list.entries, entry{act, cmp})
	}
	return list, nil
}

// DefaultComponentList is the implicit "default" style list.
func DefaultComponentList() ComponentList {
	return ComponentList{entries: []entry{{actOverride, Default}}}
}

func (l ComponentList) containsOverride() bool {
	for _, e := range l.entries {
		if e.act == actOverride {
			return true
		}
	}
	return false
}

func (l ComponentList) expandInto(set map[Component]bool, interactive bool) {
	for _, e := range l.entries {
		sub := e.cmp.expand(interactive)
		switch e.act {
		case actOverride, actAdd:
			for _, c := range sub {
				set[c] = true
			}
		case actRemove:
			for _, c := range sub {
				delete(set, c)
			}
		}
	}
}

// ToComponents folds an ordered list of ComponentLists into the final set.
// A later list that contains an override clears all previous components; a list
// of only +/- entries merges into the accumulated set. When withDefault is set,
// the Auto style seeds the initial set.
func ToComponents(lists []ComponentList, interactive, withDefault bool) Components {
	set := map[Component]bool{}
	if withDefault {
		for _, c := range Auto.expand(interactive) {
			set[c] = true
		}
	}
	for _, list := range lists {
		if list.containsOverride() {
			set = map[Component]bool{}
		}
		list.expandInto(set, interactive)
	}
	return Components{set: set}
}
