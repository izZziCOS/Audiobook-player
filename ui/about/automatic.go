package about

import (
	"context"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"

	"github.com/izzzicos/audiobook-player/online"
)

const (
	requestSpacing = 2 * time.Second
	searchAgain    = 30 * 24 * time.Hour
)

type Candidate struct {
	Path   string
	Title  string
	Author string
}

type task struct {
	candidate Candidate
	query     string
	refresh   string
	store     online.Store
}

func searchQuery(candidate Candidate) string {
	return searchTitle(candidate.Title) + "\x00" + strings.TrimSpace(candidate.Author)
}

func (a *About) MatchAutomatically(candidates []Candidate) {
	if !a.Enabled() || a.matching {
		return
	}

	now := a.now()
	store := a.store()
	var tasks []task
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Title) == "" {
			continue
		}

		if details, linked := a.data.Online(candidate.Path); linked {
			if details.Stale(now) {
				tasks = append(tasks, task{candidate: candidate, refresh: details.Edition.ASIN, store: online.StoreFor(details.Store)})
			}
			continue
		}

		if a.data.Edit(candidate.Path).NoAudible {
			continue
		}

		query := searchQuery(candidate)
		searched := a.data.Searched(candidate.Path)
		if searched.Query == query && now.Sub(time.Unix(searched.At, 0)) < searchAgain {
			continue
		}

		tasks = append(tasks, task{candidate: candidate, query: query, store: store})
	}
	if len(tasks) == 0 {
		return
	}

	a.matching = true
	client, spacing := a.client, a.spacing

	a.async(func() {
		for i, current := range tasks {
			if i > 0 {
				time.Sleep(spacing)
			}

			edition, found, err := a.fetch(client, current)
			fyne.Do(func() {
				a.applyMatch(current, edition, found, err)
			})
		}

		fyne.Do(func() {
			a.matching = false
		})
	})
}

func (a *About) fetch(client *online.Client, current task) (online.Edition, bool, error) {
	ctx := context.Background()

	if current.refresh != "" {
		edition, err := client.Lookup(ctx, current.store, current.refresh)
		return edition, err == nil, err
	}

	if asin := taggedASIN(current.candidate.Path); asin != "" {
		if edition, err := client.Lookup(ctx, current.store, asin); err == nil {
			return edition, true, nil
		}
	}

	editions, err := client.Search(ctx, current.store, searchTitle(current.candidate.Title), current.candidate.Author)
	if err != nil {
		return online.Edition{}, false, err
	}
	if len(editions) == 0 || !sameTitle(current.candidate.Title, editions[0].Title) {
		return online.Edition{}, false, nil
	}

	full, err := client.Lookup(ctx, current.store, editions[0].ASIN)
	if err != nil {
		return editions[0], true, nil
	}

	return full, true, nil
}

func (a *About) applyMatch(current task, edition online.Edition, found bool, err error) {
	path := current.candidate.Path

	switch {
	case err != nil:
		fyne.LogError("Looking a book up on Audible automatically failed", err)
		return
	case current.refresh != "":
		if details, linked := a.data.Online(path); linked && details.Edition.ASIN == current.refresh {
			a.data.UpdateOnline(current.store.Code, edition)
		}
	case !found:
		a.data.SetSearched(path, current.query)
		return
	default:
		if _, linked := a.data.Online(path); linked || a.data.Edit(path).NoAudible {
			return
		}
		a.data.LinkOnline(path, current.store.Code, edition)
	}

	if a.OnLinked != nil {
		a.OnLinked(path)
	}
}

func significantWords(title string) map[string]bool {
	words := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len([]rune(word)) >= 3 && !commonWords[word] {
			words[word] = true
		}
	}

	return words
}

var commonWords = map[string]bool{
	"the": true, "and": true, "for": true, "part": true, "book": true, "volume": true,
	"unabridged": true, "abridged": true, "audiobook": true, "edition": true, "chapter": true,
}

func sameTitle(ours, theirs string) bool {
	want := significantWords(searchTitle(ours))
	if len(want) == 0 {
		return false
	}

	for word := range significantWords(theirs) {
		if want[word] {
			return true
		}
	}

	return false
}
