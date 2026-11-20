package service

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Beyoncé & Jay-Z":   "beyonce-jay-z",
		"  The   Local  ":   "the-local",
		"AC/DC":             "ac-dc",
		"!!!":               "artist",
		"Sigur Rós":         "sigur-ros",
		"2 Chainz":          "2-chainz",
		"---Already-Slug--": "already-slug",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextFreeSlug(t *testing.T) {
	if got := nextFreeSlug("a", nil); got != "a" {
		t.Fatal(got)
	}
	if got := nextFreeSlug("a", []string{"a", "a-2", "a-4"}); got != "a-3" {
		t.Fatal(got)
	}
	if !slugHasBase("a-12", "a") || slugHasBase("a-b", "a") || slugHasBase("a-1", "a") {
		t.Fatal("slugHasBase")
	}
}

func TestDiff(t *testing.T) {
	b, a := diff(map[string]any{"x": 1, "y": "same"}, map[string]any{"x": 2, "y": "same"})
	if len(b) != 1 || len(a) != 1 || b["x"] != 1 || a["x"] != 2 {
		t.Fatalf("diff: %v %v", b, a)
	}
}
