package remote

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "strings"
    "sync"
    "testing"
    "time"

    "fyne.io/fyne/v2/test"

    "github.com/izzzicos/audiobook-player/ui/kit"
    "github.com/izzzicos/audiobook-player/vfs"
)

type mainThread struct {
    mu      sync.Mutex
    waiting []func()
}

func (m *mainThread) post(work func()) {
    m.mu.Lock()
    m.waiting = append(m.waiting, work)
    m.mu.Unlock()
}

func (m *mainThread) runUntil(t *testing.T, done func() bool) {
    t.Helper()

    deadline := time.Now().Add(30 * time.Second)
    for time.Now().Before(deadline) {
        m.mu.Lock()
        waiting := m.waiting
        m.waiting = nil
        m.mu.Unlock()
        for _, work := range waiting {
            work()
        }
        if done() {
            return
        }
        time.Sleep(20 * time.Millisecond)
    }
    t.Fatal("timed out")
}

func newTestConnection(t *testing.T) (*Connection, Memory, *mainThread) {
    t.Helper()

    app := test.NewApp()
    t.Cleanup(app.Quit)

    secrets := Memory{}
    main := &mainThread{}
    c := New(app.NewWindow("Settings"), app.Preferences(), &kit.Picker{}, secrets, t.TempDir())
    c.onMain = main.post
    t.Cleanup(c.Close)

    return c, secrets, main
}

func wireGuardFile() string {
    key := func() string {
        raw := make([]byte, 32)
        rand.Read(raw)
        return base64.StdEncoding.EncodeToString(raw)
    }

    return fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = 10.8.0.2/32\n[Peer]\nPublicKey = %s\nEndpoint = 127.0.0.1:9\nAllowedIPs = 192.168.1.0/24\n", key(), key())
}

func TestSavingKeepsSecretsInTheKeychain(t *testing.T) {
    c, secrets, main := newTestConnection(t)

    c.address.SetText(" 127.0.0.1:1 ")
    c.share.SetText("/audiobooks/")
    c.folder.SetText("Books")
    c.username.SetText("listener")
    c.passwordBox.SetText("hunter2")
    c.wireGuard = wireGuardFile()
    c.save()

    stored := c.prefs.String(preferenceSettings)
    if strings.Contains(stored, "hunter2") || strings.Contains(stored, "PrivateKey") || !strings.Contains(stored, `"share":"audiobooks"`) {
        t.Fatalf("only non-secret details belong in the preferences, got %s", stored)
    }
    if secrets[passwordKey] != "hunter2" || !strings.Contains(secrets[wireGuardKey], "PrivateKey") {
        t.Fatal("the password and WireGuard key should be in the keychain")
    }
    if !vfs.IsRemote(c.Root()) {
        t.Fatal("the server should be attached as soon as it's set up")
    }

    again := New(c.window, c.prefs, c.picker, secrets, t.TempDir())
    again.onMain = main.post
    t.Cleanup(again.Close)
    again.Load()
    if again.address.Text != "127.0.0.1:1" || again.passwordBox.Text != "hunter2" || again.wireGuard == "" {
        t.Fatal("the details should come back after a restart")
    }

    c.Forget()
    if c.prefs.String(preferenceSettings) != "" || len(secrets) != 0 || vfs.IsRemote(c.Root()) || c.status.Text != "Not set up." {
        t.Fatal("forgetting should remove everything")
    }
}

func TestAnUnreachableServerIsExplained(t *testing.T) {
    c, _, main := newTestConnection(t)
    c.address.SetText("127.0.0.1:1")
    c.share.SetText("audiobooks")
    c.save()

    main.runUntil(t, func() bool { return strings.Contains(c.status.Text, "Trying again") })
    if !c.useLibrary.Disabled() || !strings.Contains(c.status.Text, "couldn't be reached") {
        t.Fatalf("the status says %q", c.status.Text)
    }

    started := time.Now()
    c.Close()
    if time.Since(started) > 2*time.Second {
        t.Fatal("closing shouldn't wait for an unreachable server")
    }
}

func TestClosingDoesntWaitForTheTunnel(t *testing.T) {
    c, _, _ := newTestConnection(t)
    c.address.SetText("127.0.0.1:1")
    c.share.SetText("audiobooks")
    c.wireGuard = wireGuardFile()
    c.save()
    time.Sleep(4 * time.Second)

    started := time.Now()
    c.Close()
    if time.Since(started) > 2*time.Second {
        t.Fatalf("closing waited %v for a tunnel that can't connect", time.Since(started))
    }
}

func TestConnectionFiles(t *testing.T) {
    c, _, _ := newTestConnection(t)
    c.address.SetText("127.0.0.1:2")
    c.share.SetText("audiobooks")
    c.username.SetText("listener")
    c.passwordBox.SetText("hunter2")
    c.wireGuard = wireGuardFile()
    c.save()

    data, err := c.Export()
    if err != nil || strings.Contains(string(data), "hunter2") || !strings.Contains(string(data), "PrivateKey") {
        t.Fatalf("the file should carry everything but the password, got %s, %v", data, err)
    }

    other, _, _ := newTestConnection(t)
    if err := other.Import(data); err != nil {
        t.Fatal(err)
    }
    if other.address.Text != "127.0.0.1:2" || other.share.Text != "audiobooks" || other.username.Text != "listener" || other.passwordBox.Text != "" || other.wireGuard == "" {
        t.Fatal("importing should fill in everything except the password")
    }

    if err := other.Import([]byte(`{"hello": "world"}`)); err == nil {
        t.Fatal("other files should be refused")
    }
}
