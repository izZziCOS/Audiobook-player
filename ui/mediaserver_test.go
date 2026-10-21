//go:build smbserver

package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/izzzicos/audiobook-player/vfs"
)

func TestTheAppWithTheRealMediaServer(t *testing.T) {
	ui := newTestUI(t)
	ui.remote.OnConnected = nil
	t.Cleanup(ui.remote.Close)

	ui.remote.Fill("127.0.0.1:1445", "audiobooks", "Books", "tester", "secret")
	ui.remote.Save()

	root := ui.remote.Root()
	deadline := time.Now().Add(30 * time.Second)
	for !vfs.Connected(root) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !vfs.Connected(root) {
		t.Fatal("the app should connect to the server")
	}

	ui.importer.SetFolder(root)
	hobbit := filepath.Join(root, "Tolkien", "The Hobbit")
	if len(ui.shelf.Cards()) != 2 {
		t.Fatalf("the server's books should be in the library, got %d", len(ui.shelf.Cards()))
	}
	if card, _ := ui.shelf.Card(hobbit); card.Title != "The Hobbit" {
		t.Fatalf("got %q", card.Title)
	}

	if err := ui.playFromShelf(hobbit); err != nil || ui.player.Book() == nil {
		t.Fatalf("a book should play straight from the server, got %v", err)
	}
	ui.player.MoveTo(filepath.Join(hobbit, "Part 2.flac"), 0.5)
	if !closeTo(partPosition(ui), 0.5) {
		t.Fatalf("seeking within a part on the server should work, at %v", partPosition(ui))
	}

	ui.offline.Start(hobbit)
	if ui.offline.Local(hobbit) == nil {
		t.Fatal("a book on the server should download for offline listening")
	}
}
