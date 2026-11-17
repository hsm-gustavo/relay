package responseview

import (
	"strings"
	"testing"
)

func TestSearchAndNavigation(t *testing.T) {
	result := Find("Ada ADA äda ÄDA", "ada")
	if len(result.Matches) != 2 || result.Matches[0] != (Match{0, 3}) || result.Matches[1] != (Match{4, 7}) {
		t.Fatalf("matches = %+v", result.Matches)
	}
	unicode := Find("🙂 ÄDA café CAFÉ", "café")
	if len(unicode.Matches) != 2 || unicode.Matches[0].Start != 6 || unicode.Matches[1].Start != 11 {
		t.Fatalf("Unicode offsets = %+v", unicode.Matches)
	}
	if result := Find("Σ σ ς", "σ"); len(result.Matches) != 3 {
		t.Fatal("Unicode simple case folding failed")
	}
	if Navigate(0, -1, 2) != 1 || Navigate(1, 1, 2) != 0 || Navigate(-1, -1, 2) != 1 || Navigate(0, 1, 0) != -1 {
		t.Fatal("navigation did not wrap")
	}
	if len(Find("anything", "").Matches) != 0 || len(Find("abc", "missing").Matches) != 0 {
		t.Fatal("unexpected matches")
	}
}

func TestSearchLimits(t *testing.T) {
	if result := Find(strings.Repeat("a", MaxMatches+1), "a"); len(result.Matches) != MaxMatches || !result.Limited {
		t.Fatal("result limit missing")
	}
	if result := Find(strings.Repeat("a", TextLimit)+"needle", "needle"); len(result.Matches) != 0 || !result.Limited {
		t.Fatal("search exceeded text limit")
	}
	if result := Find("text", strings.Repeat("x", MaxQueryRunes+1)); !result.Limited || result.Note == "" {
		t.Fatal("query limit missing")
	}
}

func TestRawWindowsPreserveTextAndBoundGrid(t *testing.T) {
	for _, text := range []string{strings.Repeat("long minified line", 1000), strings.Repeat("line\n", 1000), strings.Repeat("\tname: 🙂\n", 1000), "hello\r\nworld"} {
		var reconstructed strings.Builder
		start := 0
		for {
			page := Window(text, start)
			if page.End <= page.Start && page.More {
				t.Fatal("window did not advance")
			}
			if len([]rune(page.Text)) > PageRunes {
				t.Fatal("page is unbounded")
			}
			reconstructed.WriteString(page.Text)
			if !page.More {
				break
			}
			start = page.End
		}
		if reconstructed.String() != text {
			t.Fatal("Raw navigation rewrote or dropped characters")
		}
	}
	page := Window("prefix needle suffix", 7)
	if !strings.HasPrefix(page.Text, "needle") {
		t.Fatal("match window did not start at requested rune")
	}
}

func BenchmarkLargeResponsePreparation(b *testing.B) {
	r := response("text/html", strings.Repeat("<html>source text</html>", 100000))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		Prepare(r)
	}
}
