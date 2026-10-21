package online

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const searchFixture = `{"products": [
    {"asin": "B09JKZFVS9", "title": "Frankenstein", "authors": [{"name": "Mary Shelley"}], "narrators": [{"name": "Jamie Bell"}],
     "release_date": "2021-10-21", "runtime_length_min": 490,
     "merchandising_summary": "<p>Short.</p>",
     "rating": {"overall_distribution": {"average_rating": 4.5168, "num_ratings": 770}}},
    {"asin": "B00FQRCM9O", "title": "Frankenstein", "authors": [{"name": "Mary Shelley"}], "narrators": [{"name": "Dan Stevens"}],
     "release_date": "2013-10-29", "runtime_length_min": 515,
     "publisher_summary": "<p>Narrator Dan Stevens <i>(Downton Abbey)</i> presents&nbsp;an uncanny performance ( <i>1818</i> ).</p><p>Second &amp; last.</p><ul><li>One</li><li>Two</li></ul>",
     "rating": {"overall_distribution": {"average_rating": 4.48, "num_ratings": 23383}}},
    {"asin": "B000000000", "title": ""}
]}`

func fakeAudible(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewClient()
	client.BaseURL = func(Store) string { return server.URL }
	return client
}

func TestSearchPutsTheMostRatedEditionFirst(t *testing.T) {
	var query map[string][]string
	client := fakeAudible(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		if r.URL.Path != "/1.0/catalog/products" || !strings.HasPrefix(r.Header.Get("User-Agent"), "AudiobookPlayer/") {
			t.Errorf("unexpected request %s with agent %q", r.URL.Path, r.Header.Get("User-Agent"))
		}
		w.Write([]byte(searchFixture))
	})

	editions, err := client.Search(context.Background(), StoreFor("us"), " Frankenstein ", "Mary Shelley")
	if err != nil {
		t.Fatal(err)
	}
	if query["title"][0] != "Frankenstein" || query["author"][0] != "Mary Shelley" {
		t.Fatalf("searched for %v", query)
	}
	if len(editions) != 2 || editions[0].ASIN != "B00FQRCM9O" || editions[1].ASIN != "B09JKZFVS9" {
		t.Fatalf("got %+v, want the two real editions, most rated first", editions)
	}

	best := editions[0]
	if best.Narrators[0] != "Dan Stevens" || best.Minutes != 515 || best.Year() != "2013" {
		t.Fatalf("got %+v", best)
	}
	if want := "Narrator Dan Stevens (Downton Abbey) presents an uncanny performance (1818).\nSecond & last.\n• One\n• Two"; best.Description != want {
		t.Fatalf("description %q, want %q", best.Description, want)
	}
	if best.Stars() != "★ 4.5 · 23k" || best.RatingText() != "★ 4.5 · 23,383 ratings on Audible" {
		t.Fatalf("rating shown as %q and %q", best.Stars(), best.RatingText())
	}
	if !strings.Contains(Edition{Rating: 5, Ratings: 3}.RatingText(), "few ratings") {
		t.Fatal("a rating from few people should be marked as unreliable")
	}
	if best.Link(StoreFor("uk")) != "https://www.audible.co.uk/pd/B00FQRCM9O" {
		t.Fatalf("link %q", best.Link(StoreFor("uk")))
	}
}

func TestLookup(t *testing.T) {
	client := fakeAudible(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1.0/catalog/products/B00FQRCM9O":
			w.Write([]byte(`{"product": {"asin": "B00FQRCM9O", "title": "Frankenstein", "rating": {"overall_distribution": {"average_rating": 4.5, "num_ratings": 23383}}}}`))
		case "/1.0/catalog/products/B09JKZFVS9":
			w.Write([]byte(`{"product": {"asin": "B09JKZFVS9"}}`))
		case "/1.0/catalog/products/B071V2RNK3":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	store := StoreFor("de")

	if edition, err := client.Lookup(context.Background(), store, "B00FQRCM9O"); err != nil || edition.Ratings != 23383 {
		t.Fatalf("got %+v, %v", edition, err)
	}
	if _, err := client.Lookup(context.Background(), store, "B09JKZFVS9"); err == nil || !strings.Contains(err.Error(), "Germany") {
		t.Fatalf("a book the store doesn't sell should say so, got %v", err)
	}
	if _, err := client.Lookup(context.Background(), store, "B071V2RNK3"); err == nil || !strings.Contains(err.Error(), "too many requests") {
		t.Fatalf("got %v", err)
	}
	if _, err := client.Lookup(context.Background(), store, "../../x"); err == nil {
		t.Fatal("only real product numbers should be looked up")
	}
}

func TestUnreachableAudible(t *testing.T) {
	client := NewClient()
	client.BaseURL = func(Store) string { return "http://127.0.0.1:1" }

	_, err := client.Search(context.Background(), StoreFor("us"), "Dune", "")
	if !errors.Is(err, ErrUnreachable) || err.Error() != ErrUnreachable.Error() {
		t.Fatalf("got %v, want a message about the connection", err)
	}
	var cause *net.OpError
	if !errors.As(err, &cause) {
		t.Fatal("the underlying network error should be kept for the log")
	}
}

func TestCounts(t *testing.T) {
	for count, want := range map[int]string{7: "7", 999: "999", 1000: "1k", 1250: "1.2k", 23383: "23k", 1_200_000: "1.2M", 3_000_000: "3M"} {
		if got := ShortCount(count); got != want {
			t.Errorf("ShortCount(%d) = %q, want %q", count, got, want)
		}
	}
	if got := GroupedCount(1234567); got != "1,234,567" {
		t.Errorf("GroupedCount gave %q", got)
	}
	if StoreFor("nowhere").Code != "us" || StoreNamed("Germany").Host != "api.audible.de" {
		t.Error("stores should be found by code and name, with the US store as the fallback")
	}
}
