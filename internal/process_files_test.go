package internal

import "testing"

func TestParseFilenameEpoch(t *testing.T) {
	date, epoch, ok := ParseFilenameEpoch("20260126_1769475135_hello")
	if !ok || date != "20260126" || epoch != 1769475135 {
		t.Fatalf("got date=%q epoch=%d ok=%v", date, epoch, ok)
	}
	if _, _, ok := ParseFilenameEpoch("bad"); ok {
		t.Fatal("expected failure for bad stem")
	}
}
