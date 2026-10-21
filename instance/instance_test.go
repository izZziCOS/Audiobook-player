package instance

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func socketPath(t *testing.T) string {
	t.Helper()

	directory, err := os.MkdirTemp("", "ab")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })

	return filepath.Join(directory, "s")
}

func TestSecondInstanceForwardsItsArguments(t *testing.T) {
	path := socketPath(t)

	server, forwarded, err := Claim(path, nil)
	if err != nil || forwarded || server == nil {
		t.Fatalf("the first instance should own the socket: %v, %v, %v", server, forwarded, err)
	}
	defer server.Close()

	received := make(chan []string, 1)
	go server.Serve(func(args []string) {
		received <- args
	})

	second, forwarded, err := Claim(path, []string{"/books/Dune.m4b"})
	if err != nil || !forwarded || second != nil {
		t.Fatalf("the second instance should hand over to the first: %v, %v, %v", second, forwarded, err)
	}

	select {
	case args := <-received:
		if !slices.Equal(args, []string{"/books/Dune.m4b"}) {
			t.Fatalf("received %v", args)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first instance never received the arguments")
	}

	if _, forwarded, _ := Claim(path, nil); !forwarded {
		t.Fatal("starting again with no arguments should still hand over")
	}
}

func TestStaleSocketIsReplaced(t *testing.T) {
	path := socketPath(t)

	if err := os.WriteFile(path, []byte("left over from a crash"), 0o600); err != nil {
		t.Fatal(err)
	}

	server, forwarded, err := Claim(path, nil)
	if err != nil || forwarded || server == nil {
		t.Fatalf("a stale socket file should be replaced: %v, %v, %v", server, forwarded, err)
	}
	server.Close()

	server, forwarded, err = Claim(path, nil)
	if err != nil || forwarded || server == nil {
		t.Fatalf("after the first instance quits, a new one should take over: %v, %v, %v", server, forwarded, err)
	}
	server.Close()
}
