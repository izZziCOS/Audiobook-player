//go:build smbserver

package mediaserver

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "net"
    "net/netip"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "golang.org/x/crypto/curve25519"

    "github.com/izzzicos/audiobook-player/audio"
    "github.com/izzzicos/audiobook-player/library"
    "github.com/izzzicos/audiobook-player/tunnel"
    "github.com/izzzicos/audiobook-player/vfs"
)

var testServer = Settings{Address: "127.0.0.1:1445", Share: "audiobooks", Folder: "Books", Username: "tester"}

func waitFor(t *testing.T, server *Server, want State) Status {
    t.Helper()

    deadline := time.Now().Add(40 * time.Second)
    for time.Now().Before(deadline) {
        status := server.Status()
        if status.State == want || (want == Connected && status.Problem != "") {
            return status
        }
        time.Sleep(50 * time.Millisecond)
    }

    t.Fatalf("the server never got to state %d, status %+v", want, server.Status())
    return Status{}
}

func mounted(t *testing.T, server *Server) string {
    t.Helper()

    root := filepath.Join(t.TempDir(), "not-on-disk", "Media server")
    vfs.Mount(root, server)
    t.Cleanup(func() { vfs.Unmount(root) })

    return root
}

func TestABookFromTheRealServer(t *testing.T) {
    server := New(testServer, "secret", nil)
    server.Start()
    defer server.Close()

    if status := waitFor(t, server, Connected); status.State != Connected || status.Route != "on your home network" {
        t.Fatalf("got %+v", status)
    }
    root := mounted(t, server)

    books, err := library.FindBooks(root)
    if err != nil || len(books) != 2 {
        t.Fatalf("got %v, %v", books, err)
    }

    hobbit := filepath.Join(root, "Tolkien", "The Hobbit")
    book, err := library.OpenBook(hobbit)
    if err != nil || len(book.Parts) != 2 {
        t.Fatalf("got %+v, %v", book, err)
    }

    file, err := vfs.Open(book.Parts[1])
    if err != nil {
        t.Fatal(err)
    }
    streamer, format, err := audio.Decode(file)
    if err != nil {
        t.Fatal(err)
    }
    samples, total := make([][2]float64, 4096), 0
    for {
        n, ok := streamer.Stream(samples)
        total += n
        if !ok {
            break
        }
    }
    streamer.Close()
    if total == 0 || total != streamer.Len() || format.SampleRate == 0 {
        t.Fatalf("decoded %d of %d samples", total, streamer.Len())
    }

    if seconds, err := audio.Duration(filepath.Join(root, "Herbert", "Dune.m4b")); err != nil || seconds <= 0 {
        t.Fatalf("an M4B on the server should be measurable, got %v, %v", seconds, err)
    }

    sync := filepath.Join(root, ".audiobook-player-sync", "laptop.json")
    for _, content := range []string{`{"first":true}`, `{"second":true}`} {
        if err := vfs.WriteAtomic(sync, []byte(content)); err != nil {
            t.Fatal(err)
        }
    }
    if data, err := vfs.ReadFile(sync); err != nil || string(data) != `{"second":true}` {
        t.Fatalf("got %q, %v", data, err)
    }
    if _, err := vfs.Stat(filepath.Join(root, "No such book")); !errors.Is(err, fs.ErrNotExist) {
        t.Fatalf("a missing file should be reported as missing, got %v", err)
    }
}

func TestWrongDetailsAreExplained(t *testing.T) {
    for _, test := range []struct {
        settings Settings
        password string
        problem  string
    }{
        {testServer, "wrong", "username or password"},
        {Settings{Address: testServer.Address, Share: "nope", Username: "tester"}, "secret", "no shared folder"},
        {Settings{Address: testServer.Address, Share: "audiobooks", Folder: "Missing", Username: "tester"}, "secret", "folder inside"},
    } {
        server := New(test.settings, test.password, nil)
        server.Start()
        status := waitFor(t, server, Connected)
        server.Close()
        if status.State == Connected || !strings.Contains(status.Problem, test.problem) {
            t.Errorf("settings %+v gave %+v, want a message about %s", test.settings, status, test.problem)
        }
    }
}

func keys(t *testing.T) (string, string) {
    secret := make([]byte, 32)
    rand.Read(secret)
    secret[0] &= 248
    secret[31] = secret[31]&127 | 64
    public, _ := curve25519.X25519(secret, curve25519.Basepoint)
    return base64.StdEncoding.EncodeToString(secret), base64.StdEncoding.EncodeToString(public)
}

func TestTheServerThroughWireGuard(t *testing.T) {
    homePrivate, homePublic := keys(t)
    laptopPrivate, laptopPublic := keys(t)
    udp, _ := net.ListenPacket("udp", "127.0.0.1:0")
    port := udp.LocalAddr().(*net.UDPAddr).Port
    udp.Close()

    home, err := tunnel.Parse(fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = 10.88.0.1/24\nListenPort = %d\n[Peer]\nPublicKey = %s\nAllowedIPs = 10.88.0.2/32\n", homePrivate, port, laptopPublic))
    if err != nil {
        t.Fatal(err)
    }
    homeTunnel, err := tunnel.Start(context.Background(), home)
    if err != nil {
        t.Fatal(err)
    }
    defer homeTunnel.Close()

    listener, err := homeTunnel.Listen(netip.MustParseAddrPort("10.88.0.1:445"))
    if err != nil {
        t.Fatal(err)
    }
    defer listener.Close()
    go func() {
        for {
            inside, err := listener.Accept()
            if err != nil {
                return
            }
            go func() {
                defer inside.Close()
                samba, err := net.Dial("tcp", testServer.Address)
                if err != nil {
                    return
                }
                defer samba.Close()
                go io.Copy(samba, inside)
                io.Copy(inside, samba)
            }()
        }
    }()

    laptop, err := tunnel.Parse(fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = 10.88.0.2/32\n[Peer]\nPublicKey = %s\nEndpoint = 127.0.0.1:%d\nAllowedIPs = 10.88.0.0/24\n", laptopPrivate, homePublic, port))
    if err != nil {
        t.Fatal(err)
    }

    away := testServer
    away.Address = "10.88.0.1"
    server := New(away, "secret", &laptop)
    server.Start()
    defer server.Close()

    status := waitFor(t, server, Connected)
    if status.State != Connected || status.Route != "through WireGuard" {
        t.Fatalf("away from home the server should be reached through WireGuard, got %+v", status)
    }

    books, err := library.FindBooks(mounted(t, server))
    if err != nil || len(books) != 2 {
        t.Fatalf("through the tunnel got %v, %v", books, err)
    }
}

func TestLosingAndRegainingTheConnection(t *testing.T) {
    server := New(testServer, "secret", nil)
    server.Start()
    defer server.Close()
    waitFor(t, server, Connected)
    root := mounted(t, server)

    server.broken(io.ErrUnexpectedEOF)
    started := time.Now()
    _, err := vfs.Stat(filepath.Join(root, "Tolkien"))
    if !errors.Is(err, vfs.ErrNotConnected) || time.Since(started) > 100*time.Millisecond || vfs.Connected(root) {
        t.Fatalf("while disconnected, files should fail at once, got %v after %v", err, time.Since(started))
    }

    deadline := time.Now().Add(30 * time.Second)
    for !server.Connected() && time.Now().Before(deadline) {
        time.Sleep(50 * time.Millisecond)
    }
    if status := server.Status(); status.State != Connected {
        t.Fatalf("the connection should come back by itself, got %+v", status)
    }
    if _, err := vfs.Stat(filepath.Join(root, "Tolkien")); err != nil {
        t.Fatalf("after reconnecting, files should work again: %v", err)
    }
}
