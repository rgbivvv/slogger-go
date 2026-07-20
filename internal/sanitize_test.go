package internal

import "testing"

func TestSanitizeKeepsRelativeImgSrc(t *testing.T) {
	p := SanitizePolicy()
	in := `<img src="public/img/house1.jpg" alt="house1">`
	got := SanitizeHTML(in, p)
	want := `<img src="public/img/house1.jpg" alt="house1">`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
