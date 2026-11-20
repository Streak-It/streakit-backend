package service

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const maxSlugBase = 200

// slugify turns "Beyoncé & Jay-Z" into "beyonce-jay-z".
func slugify(name string) string {
	// Strip diacritics: decompose, drop combining marks, recompose.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	plain, _, err := transform.String(t, name)
	if err != nil {
		plain = name
	}

	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(plain) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugBase {
		slug = strings.Trim(slug[:maxSlugBase], "-")
	}
	if slug == "" {
		slug = "artist"
	}
	return slug
}

// nextFreeSlug returns base, or base-2, base-3, … — the first not taken.
func nextFreeSlug(base string, taken []string) string {
	set := make(map[string]struct{}, len(taken))
	for _, s := range taken {
		set[s] = struct{}{}
	}
	if _, ok := set[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		cand := base + "-" + strconv.Itoa(n)
		if _, ok := set[cand]; !ok {
			return cand
		}
	}
}

// slugHasBase reports whether slug is base or base-<n>.
func slugHasBase(slug, base string) bool {
	if slug == base {
		return true
	}
	suffix, ok := strings.CutPrefix(slug, base+"-")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(suffix)
	return err == nil && n >= 2
}
