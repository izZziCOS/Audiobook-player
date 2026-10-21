package userdata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/izzzicos/audiobook-player/online"
)

func TestOnlineDetailsSurviveARestart(t *testing.T) {
	file := FileIn(t.TempDir())
	data := New(file)
	data.SetEdit("/books/Frankenstein", BookEdit{Title: "Frankenstein"})
	edition := online.Edition{ASIN: "B00FQRCM9O", Title: "Frankenstein", Rating: 4.5, Ratings: 23383}

	data.LinkOnline("/books/Frankenstein", "uk", edition)

	restarted := New(file)
	restarted.Load(testPreferences(t))
	details, found := restarted.Online("/books/Frankenstein")
	if !found || details.Edition.Ratings != 23383 || details.Store != "uk" || details.Stale(time.Now()) {
		t.Fatalf("after a restart the details are %+v, %v", details, found)
	}
	if edit := restarted.Edit("/books/Frankenstein"); edit.Title != "Frankenstein" || edit.ASIN != "B00FQRCM9O" || edit.Store != "uk" {
		t.Fatalf("linking should keep the other corrections, got %+v", edit)
	}
	if !details.Stale(time.Now().Add(RefreshOnline + time.Hour)) {
		t.Fatal("details should be refreshed after a while")
	}

	restarted.UnlinkOnline("/books/Frankenstein")
	if _, found := restarted.Online("/books/Frankenstein"); found || len(restarted.online) != 0 {
		t.Fatal("unlinking should forget the details")
	}
	if restarted.Edit("/books/Frankenstein").Title != "Frankenstein" {
		t.Fatal("unlinking should keep the other corrections")
	}
}

func TestDamagedOnlineFileStartsEmpty(t *testing.T) {
	file := FileIn(t.TempDir())
	data := New(file)
	data.LinkOnline("/books/Dune", "us", online.Edition{ASIN: "B002V1OF70", Title: "Dune"})
	os.WriteFile(data.onlineFile(), []byte("{broken"), 0o644)

	restarted := New(file)
	restarted.Load(testPreferences(t))
	if _, found := restarted.Online("/books/Dune"); found {
		t.Fatal("a damaged file should just mean looking the book up again")
	}
	if restarted.Edit("/books/Dune").ASIN != "B002V1OF70" {
		t.Fatal("the chosen edition is kept in the library, so it isn't lost")
	}
}

func TestDownloadsFollowMovedBooks(t *testing.T) {
	data := New("")
	old, moved := filepath.Join("nas", "Hobbit"), filepath.Join("nas", "Tolkien", "Hobbit")
	data.SetDownload(old, Download{Folder: "abc", Complete: true, Parts: []DownloadPart{{Source: filepath.Join(old, "1.mp3"), File: "1.mp3", Done: true}}})

	data.Move(old, moved)

	download, found := data.Download(moved)
	if _, left := data.Download(old); left || !found || download.Parts[0].Source != filepath.Join(moved, "1.mp3") {
		t.Fatalf("got %+v", download)
	}
}
