package about

import (
    "net/http"
    "os"
    "net/http/httptest"
    "net/url"
    "path/filepath"
    "strings"
    "sync/atomic"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"
    "fyne.io/fyne/v2/widget"

    "github.com/izzzicos/audiobook-player/online"
    "github.com/izzzicos/audiobook-player/ui/internal/testbooks"
    "github.com/izzzicos/audiobook-player/ui/userdata"
)

const products = `{"products": [
    {"asin": "B09JKZFVS9", "title": "Frankenstein", "authors": [{"name": "Mary Shelley"}], "narrators": [{"name": "Jamie Bell"}],
     "release_date": "2021-10-21", "rating": {"overall_distribution": {"average_rating": 4.5, "num_ratings": 770}}},
    {"asin": "B00FQRCM9O", "title": "Frankenstein", "authors": [{"name": "Mary Shelley"}], "narrators": [{"name": "Dan Stevens"}],
     "release_date": "2013-10-29", "runtime_length_min": 515, "publisher_summary": "<p>An uncanny performance.</p>",
     "rating": {"overall_distribution": {"average_rating": 4.48, "num_ratings": 23383}}}
]}`

type fake struct {
    searches atomic.Int32
    lookups  atomic.Int32
    query    atomic.Value
}

func newTestAbout(t *testing.T) (*About, *fake, *[]string) {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    calls := &fake{}
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.HasSuffix(r.URL.Path, "/products") {
            calls.searches.Add(1)
            calls.query.Store(r.URL.RawQuery)
            w.Write([]byte(products))
            return
        }
        calls.lookups.Add(1)
        if strings.HasSuffix(r.URL.Path, "/B09JKZFVS9") {
            w.Write([]byte(`{"product": {"asin": "B09JKZFVS9", "title": "Frankenstein", "narrators": [{"name": "Jamie Bell"}], "rating": {"overall_distribution": {"average_rating": 4.5, "num_ratings": 770}}}}`))
            return
        }
        w.Write([]byte(`{"product": {"asin": "B00FQRCM9O", "title": "Frankenstein", "rating": {"overall_distribution": {"average_rating": 4.6, "num_ratings": 24000}}}}`))
    }))
    t.Cleanup(server.Close)

    window := app.NewWindow("Audiobook Player")
    a := New(window, app.Preferences(), userdata.New(userdata.FileIn(t.TempDir())), func(work func()) {
        work()
    })
    a.client.BaseURL = func(online.Store) string { return server.URL }

    var linked []string
    a.OnLinked = func(key string) {
        linked = append(linked, key)
    }

    return a, calls, &linked
}

const key = "/books/Frankenstein"

var frankenstein = Book{Title: "Frankenstein (Unabridged)", Author: "Mary Shelley"}

func TestNothingIsSentUntilLookupsAreTurnedOn(t *testing.T) {
    a, calls, _ := newTestAbout(t)

    v := a.open(key, frankenstein)
    if a.Enabled() || v.primary.Text != "Turn on Audible look-ups" || !strings.Contains(v.message.Text, "sent to Audible") {
        t.Fatalf("look-ups should start off and explain what is sent, got %q", v.message.Text)
    }
    if calls.searches.Load() != 0 || calls.lookups.Load() != 0 {
        t.Fatal("nothing should be sent to Audible yet")
    }

    test.Tap(v.primary)
    if !a.Enabled() || !a.enabled.Checked || calls.searches.Load() != 1 {
        t.Fatal("turning look-ups on should save the choice and search straight away")
    }
}

func TestFindingAndLinkingAnEdition(t *testing.T) {
    a, calls, linked := newTestAbout(t)
    a.SetEnabled(true)
    a.prefs.SetString(preferenceStore, "uk")

    var chosen online.Edition
    f := a.openFinder(key, frankenstein, func(store online.Store, edition online.Edition) {
        if store.Code != "uk" {
            t.Errorf("searched the %s store, want the one chosen in Settings", store.Name)
        }
        chosen = edition
        a.data.LinkOnline(key, store.Code, edition)
    })

    raw, _ := calls.query.Load().(string)
    if query, _ := url.ParseQuery(raw); query.Get("title") != "Frankenstein" || query.Get("author") != "Mary Shelley" {
        t.Fatalf("the search should leave out words like Unabridged, sent %q", raw)
    }
    if len(f.results) != 2 || f.results[0].edition.ASIN != "B00FQRCM9O" || !strings.Contains(f.status.Text, "most rated first") && !strings.Contains(f.status.Text, "most people rated") {
        t.Fatalf("the most rated edition should be first, got %+v, %q", f.results, f.status.Text)
    }
    f.list.Select(0)
    if chosen.ASIN != "B00FQRCM9O" {
        t.Fatal("choosing a result should link it")
    }

    lookups := calls.lookups.Load()
    v := a.open(key, frankenstein)
    if v.rating.Text != "★ 4.5 · 23,383 ratings on Audible" || !strings.Contains(v.facts.Text, "Dan Stevens") || !strings.Contains(v.facts.Text, "29 October 2013") {
        t.Fatalf("the details show %q and %q", v.rating.Text, v.facts.Text)
    }
    if v.description.Text != "An uncanny performance." || !v.openLink.Visible() || v.primary.Text != "Change edition…" {
        t.Fatal("a linked book should show its description and offer to change or open it")
    }
    if calls.lookups.Load() != lookups {
        t.Fatal("fresh details shouldn't be fetched again")
    }

    test.Tap(v.unlink)
    if _, found := a.data.Online(key); found || v.primary.Text != "Find on Audible…" || len(*linked) != 1 {
        t.Fatalf("Not this book should remove the link, OnLinked calls %v", *linked)
    }
}

func TestOldDetailsAreRefreshed(t *testing.T) {
    a, calls, linked := newTestAbout(t)
    a.SetEnabled(true)
    a.data.LinkOnline(key, "us", online.Edition{ASIN: "B00FQRCM9O", Title: "Frankenstein", Rating: 4.5, Ratings: 23383})

    a.now = func() time.Time { return time.Now().Add(userdata.RefreshOnline + time.Hour) }

    v := a.open(key, frankenstein)
    if calls.lookups.Load() != 1 || v.rating.Text != "★ 4.6 · 24,000 ratings on Audible" || len(*linked) != 1 {
        t.Fatalf("old details should be refreshed, showing %q", v.rating.Text)
    }
}

func TestTheEditionInTheFilesComesFirst(t *testing.T) {
    a, calls, _ := newTestAbout(t)
    a.SetEnabled(true)

    untagged := filepath.Join(t.TempDir(), "Frankenstein")
    testbooks.CopyFile(t, "speech-16bit-mono.wav", filepath.Join(untagged, "Part 1.wav"))
    f := a.openFinder(untagged, frankenstein, func(online.Store, online.Edition) {})
    if calls.lookups.Load() != 0 || f.results[0].tagged {
        t.Fatal("without an ASIN in the files, only the search should run")
    }

    tagged := filepath.Join(t.TempDir(), "Frankenstein.mp3")
    os.WriteFile(tagged, id3WithASIN("B09JKZFVS9"), 0o644)
    f = a.openFinder(tagged, frankenstein, func(online.Store, online.Edition) {})
    if calls.lookups.Load() != 1 || len(f.results) != 2 || !f.results[0].tagged || f.results[0].edition.ASIN != "B09JKZFVS9" {
        t.Fatalf("the edition named in the files should come first, once, got %+v", f.results)
    }
    if !strings.Contains(f.status.Text, "named in this book's files") || !strings.Contains(f.results[0].heading(), "matches this book's files") {
        t.Fatalf("the match should be explained, got %q", f.status.Text)
    }
}

func id3WithASIN(asin string) []byte {
    data := append([]byte{0}, "ASIN\x00"+asin...)
    frame := append([]byte("TXXX"), 0, 0, 0, byte(len(data)), 0, 0)
    frame = append(frame, data...)

    return append([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, byte(len(frame))}, frame...)
}

func TestChoosingAnEditionFetchesItsFullDetails(t *testing.T) {
    a, calls, linked := newTestAbout(t)
    a.SetEnabled(true)

    v := a.open(key, frankenstein)
    v.find()
    top := a.window.Canvas().Overlays().Top()
    if top == nil {
        t.Fatal("the search should open")
    }
    var list *widget.List
    for _, object := range test.LaidOutObjects(top) {
        if found, ok := object.(*widget.List); ok {
            list = found
        }
    }
    list.Select(0)

    details, found := a.data.Online(key)
    if !found || calls.lookups.Load() != 1 || details.Edition.Ratings != 24000 || len(*linked) != 1 {
        t.Fatalf("choosing should fetch and save the full details, got %+v", details)
    }
    if v.rating.Text != "★ 4.6 · 24,000 ratings on Audible" {
        t.Fatalf("the open About shows %q", v.rating.Text)
    }
}
