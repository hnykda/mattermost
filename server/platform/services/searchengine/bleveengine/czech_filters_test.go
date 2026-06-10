// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bleveengine

import (
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestCzechStem(t *testing.T) {
	// Expectations match Lucene's CzechStemmer (light stemmer, Dolamic & Savoy).
	for _, tc := range []struct{ in, out string }{
		// case endings
		{"stromy", "strom"},
		{"stromu", "strom"},
		{"stromů", "strom"},
		{"kočkami", "kočk"},
		{"kočka", "kočk"},
		{"kočkách", "kočk"},
		{"vyhledávání", "vyhledáván"},
		{"nakupování", "nakupován"},
		{"český", "česk"},
		{"jazycích", "jazyk"}, // -ích removed, c -> k normalization
		// possessives
		{"karlův", "karl"},
		{"jazykový", "jazyk"}, // -ý removed, then -ov possessive removed
		// normalization
		{"matek", "matk"}, // e* > *
		{"prací", "prak"}, // -í removed, c -> k
		{"dům", "dom"},    // *ů* -> *o*, matches "domy" -> "dom"
		{"les", "ls"},     // e* > * applies even to short words
		// English passes through mostly unchanged
		{"run", "run"},
	} {
		assert.Equal(t, tc.out, string([]byte(string(czechStem([]rune(tc.in))))), "input: %s", tc.in)
	}
}

func TestFoldDiacritics(t *testing.T) {
	for _, tc := range []struct{ in, out string }{
		{"vyhledáván", "vyhledavan"},
		{"kočk", "kock"},
		{"žluťoučký", "zlutoucky"},
		{"strom", "strom"},
		{"běh", "beh"},
	} {
		assert.Equal(t, tc.out, string(foldDiacritics([]byte(tc.in))), "input: %s", tc.in)
	}
}

// analyzeTerms runs text through the named analyzer and returns the produced terms.
func analyzeTerms(t *testing.T, analyzerName, text string) []string {
	t.Helper()
	m := bleve.NewIndexMapping()
	addCustomAnalyzers(m)
	analyzer := m.AnalyzerNamed(analyzerName)
	require.NotNil(t, analyzer)
	tokens := analyzer.Analyze([]byte(text))
	terms := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		terms = append(terms, string(tok.Term))
	}
	return terms
}

func TestCsLightAnalyzer(t *testing.T) {
	cs := func(text string) []string { return analyzeTerms(t, csLightAnalyzerName, text) }

	t.Run("czech inflections converge", func(t *testing.T) {
		assert.Equal(t, cs("strom"), cs("stromy"))
		assert.Equal(t, cs("strom"), cs("stromů"))
		assert.Equal(t, cs("kočka"), cs("kočkami"))
		assert.Equal(t, cs("kočka"), cs("kočkách"))
		assert.Equal(t, cs("matka"), cs("matek"))
	})

	t.Run("diacritics insensitive", func(t *testing.T) {
		assert.Equal(t, cs("vyhledávání"), cs("vyhledavani"))
		assert.Equal(t, cs("kočka"), cs("kocka"))
	})

	t.Run("stop words removed", func(t *testing.T) {
		// "že" is missing from Lucene's Czech stop list (only unaccented "ze"
		// is there), so test with words actually in the list.
		assert.Empty(t, cs("aby tak proč"))
	})

	t.Run("unaccented stop word variants removed", func(t *testing.T) {
		// "nové" is a stop word; its unaccented spelling must behave the same,
		// otherwise unaccented multi-word AND queries can never match.
		assert.Empty(t, cs("nové nove proč proc"))
	})
}

func TestEnCsAnalyzer(t *testing.T) {
	en := func(text string) []string { return analyzeTerms(t, enCsAnalyzerName, text) }

	t.Run("english stemming", func(t *testing.T) {
		assert.Equal(t, en("run"), en("running"))
		assert.Equal(t, en("database"), en("databases"))
		assert.Equal(t, en("user"), en("users"))
		assert.Equal(t, en("problem"), en("problems"))
		assert.Equal(t, en("pony"), en("ponies"))
	})

	t.Run("diacritics folded on exact forms", func(t *testing.T) {
		assert.Equal(t, en("vyhledávání"), en("vyhledavani"))
	})

	t.Run("stop words removed", func(t *testing.T) {
		assert.Empty(t, en("the and a"))
	})
}

// TestSearchPostsCzech exercises the full SearchPosts path against an in-memory
// index, verifying the Message/MessageCs disjunction wiring.
func TestSearchPostsCzech(t *testing.T) {
	index, err := bleve.NewMemOnly(getPostIndexMapping())
	require.NoError(t, err)
	defer index.Close()
	engine := &BleveEngine{PostIndex: index}

	channelID := model.NewId()
	teamID := model.NewId()
	posts := map[string]string{ // post id -> message
		"czech":     "Zasadili jsme nové stromy a běháme kolem nich s kočkami",
		"unaccent":  "dneska resim vyhledavani v mattermostu",
		"accented":  "Vyhledávání funguje skvěle",
		"english":   "the users are running new databases",
		"unrelated": "completely different content here",
	}
	ids := map[string]string{}
	for key, msg := range posts {
		id := model.NewId()
		ids[key] = id
		post := BLVPostFromPost(&model.Post{
			Id:        id,
			ChannelId: channelID,
			Message:   msg,
			CreateAt:  model.GetMillis(),
		}, teamID)
		require.NoError(t, index.Index(post.Id, post))
	}

	channels := model.ChannelList{{Id: channelID}}
	search := func(terms string) []string {
		res, _, appErr := engine.SearchPosts(channels, []*model.SearchParams{{Terms: terms}}, 0, 20)
		require.Nil(t, appErr)
		return res
	}

	for _, tc := range []struct {
		terms   string
		expect  string
		comment string
	}{
		{"strom", "czech", "czech singular query matches plural in post"},
		{"stromů", "czech", "genitive plural query matches nominative plural"},
		{"kočka", "czech", "singular matches instrumental plural kočkami"},
		{"kockami", "czech", "unaccented inflected query matches accented post"},
		{"vyhledávání", "unaccent", "accented query matches unaccented post"},
		{"vyhledavani", "accented", "unaccented query matches accented post"},
		{"user", "english", "english stemming: user matches users"},
		{"database", "english", "english stemming: database matches databases"},
		{"nové stromy", "czech", "multi-word czech AND query"},
		{"nove stromy", "czech", "unaccented multi-word query (nove is a folded stop word)"},
	} {
		results := search(tc.terms)
		assert.Contains(t, results, ids[tc.expect], "%s (terms: %q, results: %v)", tc.comment, tc.terms, results)
		assert.NotContains(t, results, ids["unrelated"], "unrelated post must not match %q", tc.terms)
	}

	// sanity: nonsense doesn't match anything
	assert.Empty(t, search("xyzabc"), "nonsense query should return nothing")
}
