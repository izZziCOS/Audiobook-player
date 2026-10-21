package about

import (
	"testing"
	"time"

	"github.com/izzzicos/audiobook-player/online"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

var frankensteinBook = Candidate{Path: key, Title: "Frankenstein (Unabridged)", Author: "Mary Shelley"}

func TestBooksAreMatchedAutomatically(t *testing.T) {
	a, calls, linked := newTestAbout(t)
	a.spacing = 0

	a.MatchAutomatically([]Candidate{frankensteinBook})
	if calls.searches.Load() != 0 {
		t.Fatal("nothing should be sent while look-ups are off")
	}

	a.SetEnabled(true)
	a.MatchAutomatically([]Candidate{frankensteinBook})

	details, found := a.data.Online(key)
	if !found || details.Edition.ASIN != "B00FQRCM9O" || details.Edition.Ratings != 24000 {
		t.Fatalf("the most rated edition should be matched with its full details, got %+v", details)
	}
	if calls.searches.Load() != 1 || calls.lookups.Load() != 1 || len(*linked) != 1 {
		t.Fatalf("searches %d, lookups %d", calls.searches.Load(), calls.lookups.Load())
	}

	a.MatchAutomatically([]Candidate{frankensteinBook})
	if calls.searches.Load() != 1 || calls.lookups.Load() != 1 {
		t.Fatal("a matched book with fresh details shouldn't be looked up again")
	}
}

func TestNotThisBookIsRespected(t *testing.T) {
	a, calls, _ := newTestAbout(t)
	a.spacing = 0
	a.SetEnabled(true)

	a.MatchAutomatically([]Candidate{frankensteinBook})
	a.data.UnlinkOnline(key)
	a.MatchAutomatically([]Candidate{frankensteinBook})

	if _, found := a.data.Online(key); found || calls.searches.Load() != 1 {
		t.Fatal("after Not this book the book shouldn't be matched again automatically")
	}
}

func TestUnlikelyMatchesAreLeftAlone(t *testing.T) {
	a, calls, _ := newTestAbout(t)
	a.spacing = 0
	a.SetEnabled(true)
	moby := Candidate{Path: "/books/Moby-Dick", Title: "Moby-Dick", Author: "Herman Melville"}

	a.MatchAutomatically([]Candidate{moby})
	if _, found := a.data.Online(moby.Path); found {
		t.Fatal("a result with a different title shouldn't be matched")
	}

	a.MatchAutomatically([]Candidate{moby})
	if calls.searches.Load() != 1 {
		t.Fatal("a book Audible had nothing for shouldn't be searched again straight away")
	}

	moby.Title = "Moby-Dick; or, The Whale"
	a.MatchAutomatically([]Candidate{moby})
	if calls.searches.Load() != 2 {
		t.Fatal("a corrected title should be searched again")
	}

	a.now = func() time.Time { return time.Now().Add(searchAgain + time.Hour) }
	a.MatchAutomatically([]Candidate{moby})
	if calls.searches.Load() != 3 {
		t.Fatal("after a month the book should be searched again")
	}
}

func TestOldDetailsAreRefreshedInTheBackground(t *testing.T) {
	a, calls, _ := newTestAbout(t)
	a.spacing = 0
	a.SetEnabled(true)
	a.data.LinkOnline(key, "us", online.Edition{ASIN: "B00FQRCM9O", Title: "Frankenstein", Ratings: 100})

	a.now = func() time.Time { return time.Now().Add(userdata.RefreshOnline + time.Hour) }
	a.MatchAutomatically([]Candidate{frankensteinBook})

	if details, _ := a.data.Online(key); details.Edition.Ratings != 24000 || calls.searches.Load() != 0 {
		t.Fatalf("old details should be refreshed without searching again, got %+v", details.Edition)
	}
}

func TestSameTitle(t *testing.T) {
	for _, match := range [][2]string{
		{"Frankenstein (Unabridged)", "Frankenstein"},
		{"The Hobbit", "The Hobbit, or There and Back Again"},
		{"Dune: Book 1", "Dune"},
	} {
		if !sameTitle(match[0], match[1]) {
			t.Errorf("%q and %q should match", match[0], match[1])
		}
	}
	for _, different := range [][2]string{
		{"Part 1", "Frankenstein"},
		{"Moby-Dick", "Frankenstein"},
		{"The Book", "The Book Thief"},
	} {
		if sameTitle(different[0], different[1]) {
			t.Errorf("%q and %q shouldn't match", different[0], different[1])
		}
	}
}
