package shelf

import (
    "path/filepath"
    "strings"
    "unicode"

    "golang.org/x/text/runes"
    "golang.org/x/text/transform"
    "golang.org/x/text/unicode/norm"

    "github.com/izzzicos/audiobook-player/library"
)

func matchesSearch(query string, fields ...string) bool {
    text := foldForSearch(strings.Join(fields, " "))

    for _, word := range strings.Fields(foldForSearch(query)) {
        if !strings.Contains(text, word) {
            return false
        }
    }

    return true
}

func foldForSearch(text string) string {
    withoutAccents := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
    folded, _, err := transform.String(withoutAccents, text)
    if err != nil {
        folded = text
    }

    return strings.ToLower(library.CleanName(folded))
}

func (entry *shelfEntry) matches(query string) bool {
    return matchesSearch(
        query,
        entry.title,
        entry.author,
        entry.original.Title,
        entry.original.Author,
        filepath.Base(entry.path),
        filepath.Base(filepath.Dir(entry.path)),
    )
}
