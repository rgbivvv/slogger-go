package internal

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello_world"},
		{"20260126_1769475135 title", "20260126_1769475135_title"},
		{"  Foo---Bar  ", "foo_bar"},
		{"Café", "caf"}, // non-ASCII stripped
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
