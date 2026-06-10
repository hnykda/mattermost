// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bleveengine

import (
	"unicode"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
	"golang.org/x/text/unicode/norm"
)

const (
	CzechLightStemmerName = "stemmer_cs_light"
	FoldDiacriticsName    = "fold_diacritics"
)

func init() {
	registry.RegisterTokenFilter(CzechLightStemmerName,
		func(config map[string]interface{}, cache *registry.Cache) (analysis.TokenFilter, error) {
			return &czechLightStemmerFilter{}, nil
		})
	registry.RegisterTokenFilter(FoldDiacriticsName,
		func(config map[string]interface{}, cache *registry.Cache) (analysis.TokenFilter, error) {
			return &foldDiacriticsFilter{}, nil
		})
}

// czechLightStemmerFilter is a Go port of Lucene's CzechStemmer (Apache License 2.0,
// org.apache.lucene.analysis.cz.CzechStemmer), implementing the light stemming
// algorithm from Dolamic & Savoy, "Indexing and stemming approaches for the Czech
// language" (https://portal.acm.org/citation.cfm?id=1598600).
//
// Input tokens must be lowercase with diacritical marks intact, so this filter has
// to run after to_lower and before fold_diacritics in the analyzer chain. It also
// has to run before stemmer_porter: porter's y→i rule rewrites Czech -y plurals
// ("stromy"→"stromi"), which the Czech suffix rules would then misinterpret.
type czechLightStemmerFilter struct{}

func (f *czechLightStemmerFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	for _, token := range input {
		token.Term = []byte(string(czechStem([]rune(string(token.Term)))))
	}
	return input
}

func czechStem(s []rune) []rune {
	s = czechRemoveCase(s)
	s = czechRemovePossessives(s)
	if len(s) > 0 {
		s = czechNormalize(s)
	}
	return s
}

func runesEndWith(s []rune, suffix string) bool {
	suf := []rune(suffix)
	if len(s) < len(suf) {
		return false
	}
	off := len(s) - len(suf)
	for i, r := range suf {
		if s[off+i] != r {
			return false
		}
	}
	return true
}

func czechRemoveCase(s []rune) []rune {
	n := len(s)

	if n > 7 && runesEndWith(s, "atech") {
		return s[:n-5]
	}

	if n > 6 &&
		(runesEndWith(s, "ětem") ||
			runesEndWith(s, "etem") ||
			runesEndWith(s, "atům")) {
		return s[:n-4]
	}

	if n > 5 {
		for _, suf := range []string{
			"ech", "ich", "ích", "ého", "ěmi", "emi", "ému", "ěte", "ete",
			"ěti", "eti", "ího", "iho", "ími", "ímu", "imu", "ách", "ata",
			"aty", "ých", "ama", "ami", "ové", "ovi", "ými",
		} {
			if runesEndWith(s, suf) {
				return s[:n-3]
			}
		}
	}

	if n > 4 {
		for _, suf := range []string{
			"em", "es", "ém", "ím", "ům", "at", "ám", "os", "us", "ým", "mi", "ou",
		} {
			if runesEndWith(s, suf) {
				return s[:n-2]
			}
		}
	}

	if n > 3 {
		switch s[n-1] {
		case 'a', 'e', 'i', 'o', 'u', 'ů', 'y', 'á', 'é', 'í', 'ý', 'ě':
			return s[:n-1]
		}
	}

	return s
}

func czechRemovePossessives(s []rune) []rune {
	n := len(s)
	if n > 5 &&
		(runesEndWith(s, "ov") ||
			runesEndWith(s, "in") ||
			runesEndWith(s, "ův")) {
		return s[:n-2]
	}
	return s
}

func czechNormalize(s []rune) []rune {
	n := len(s)

	if runesEndWith(s, "čt") { // čt -> ck
		s[n-2] = 'c'
		s[n-1] = 'k'
		return s
	}

	if runesEndWith(s, "št") { // št -> sk
		s[n-2] = 's'
		s[n-1] = 'k'
		return s
	}

	switch s[n-1] {
	case 'c', 'č': // [cč] -> k
		s[n-1] = 'k'
		return s
	case 'z', 'ž': // [zž] -> h
		s[n-1] = 'h'
		return s
	}

	if n > 1 && s[n-2] == 'e' {
		s[n-2] = s[n-1] // e* > *
		return s[:n-1]
	}

	if n > 2 && s[n-2] == 'ů' {
		s[n-2] = 'o' // *ů* -> *o*
		return s
	}

	return s
}

// foldDiacriticsFilter strips combining diacritical marks (NFD-decompose, drop
// Unicode Mn runes, NFC-recompose), so "vyhledávání" and "vyhledavani" index to
// the same term. Czech is commonly typed without diacritics, so folding both the
// indexed text and the query makes accented and unaccented forms match.
type foldDiacriticsFilter struct{}

func (f *foldDiacriticsFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	for _, token := range input {
		token.Term = foldDiacritics(token.Term)
	}
	return input
}

func foldDiacritics(in []byte) []byte {
	decomposed := norm.NFD.String(string(in))
	out := make([]rune, 0, len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		out = append(out, r)
	}
	return []byte(norm.NFC.String(string(out)))
}
