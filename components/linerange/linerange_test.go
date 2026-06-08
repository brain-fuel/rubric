package linerange

import "testing"

func mustParse(t *testing.T, s string) LineRange {
	t.Helper()
	r, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", s, err)
	}
	return r
}

func wantBounds(t *testing.T, s string, lo, hi RangeBound) {
	t.Helper()
	r := mustParse(t, s)
	if r.Lower != lo || r.Upper != hi {
		t.Fatalf("Parse(%q) = {%+v,%+v}, want {%+v,%+v}", s, r.Lower, r.Upper, lo, hi)
	}
}

func TestParseSingle(t *testing.T) { wantBounds(t, "40", abs(40), abs(40)) }

func TestParsePlus(t *testing.T) { wantBounds(t, "40:+10", abs(40), abs(50)) }

func TestParsePlusOverflow(t *testing.T) {
	r := mustParse(t, "9223372036854775807:+1")
	if r.Lower != abs(maxLine) || r.Upper != abs(maxLine) {
		t.Fatalf("overflow not saturated: %+v", r)
	}
}

func TestParseMinus(t *testing.T) {
	wantBounds(t, "40:-10", abs(30), abs(40))
	wantBounds(t, "5:-4", abs(1), abs(5))
	wantBounds(t, "5:-5", abs(0), abs(5))
	wantBounds(t, "5:-100", abs(0), abs(5))
}

func TestParseContextSingle(t *testing.T) { wantBounds(t, "35::5", abs(30), abs(40)) }

func TestParseContextRange(t *testing.T) {
	wantBounds(t, "30:40:2", abs(28), abs(42))
	wantBounds(t, "40:50:80", abs(0), abs(130))
	wantBounds(t, "5::10", abs(0), abs(15))
	wantBounds(t, "50::0", abs(50), abs(50))
	wantBounds(t, "30:40:0", abs(30), abs(40))
}

func TestParseEndRelative(t *testing.T) {
	wantBounds(t, ":-3", abs(minLine), fromEnd(3))
	wantBounds(t, "-3:", fromEnd(3), abs(maxLine))
}

func TestParseFail(t *testing.T) {
	for _, s := range []string{
		"40:50:80:90", "-2:5", ":40:", "abc:def",
		"40:+z", "40:+-10", "40:+",
		"40:-z", "40:-+10", "40:-",
		"40::z", "::5", "40::", "30:40:z", "30::40:5",
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", s)
		}
	}
}

func TestIsInsideOffset(t *testing.T) {
	r := mustParse(t, "-3:") // last 4 lines (offset 3 from end, inclusive)
	if !r.IsInside(98, 100) || r.IsInside(96, 100) {
		t.Fatalf("offset range membership wrong")
	}
}
