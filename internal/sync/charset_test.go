package sync

import "testing"

func TestUnquotePhrase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`"火山引擎"`, "火山引擎"},
		{`火山引擎`, "火山引擎"},
		{`Anaconda`, "Anaconda"},
		{`""`, ""},
		{`"`, `"`},                      // lone quote — not a quoted-string
		{`"a"b"`, `a"b`},                // unescaped inner quote tolerated
		{`"say \"hi\""`, `say "hi"`},    // escaped quotes unescaped
		{`"back\\slash"`, `back\slash`}, // escaped backslash
		{`  "  spaced  "  `, "spaced"},  // outer + inner whitespace trimmed
		{``, ``},
	}
	for _, c := range cases {
		if got := unquotePhrase(c.in); got != c.want {
			t.Errorf("unquotePhrase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDecodeAddressName(t *testing.T) {
	// decodeAddressName = MIME-decode then unquote.
	cases := []struct {
		in, want string
	}{
		{`"火山引擎"`, "火山引擎"},
		{`Anaconda`, "Anaconda"},
		{`=?UTF-8?B?54Gr5bGx5byV5pOO?=`, "火山引擎"}, // bare encoded-word, no quotes
	}
	for _, c := range cases {
		if got := decodeAddressName(c.in); got != c.want {
			t.Errorf("decodeAddressName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
