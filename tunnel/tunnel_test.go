package tunnel

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/curve25519"
)

func keyPair(t *testing.T) (private, public string) {
	t.Helper()

	secret := make([]byte, 32)
	rand.Read(secret)
	secret[0] &= 248
	secret[31] = secret[31]&127 | 64
	shared, err := curve25519.X25519(secret, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}

	return base64.StdEncoding.EncodeToString(secret), base64.StdEncoding.EncodeToString(shared)
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.LocalAddr().(*net.UDPAddr).Port
}

func TestReachingTheServerThroughTheTunnel(t *testing.T) {
	serverPrivate, serverPublic := keyPair(t)
	laptopPrivate, laptopPublic := keyPair(t)
	port := freePort(t)

	server, err := Parse(fmt.Sprintf(`
[Interface]
PrivateKey = %s
Address = 10.77.0.1/24
ListenPort = %d

[Peer]
PublicKey = %s
AllowedIPs = 10.77.0.2/32
`, serverPrivate, port, laptopPublic))
	if err != nil {
		t.Fatal(err)
	}

	laptop, err := Parse(fmt.Sprintf(`
# the file a WireGuard server hands out
[Interface]
PrivateKey = %s
Address = 10.77.0.2/32
DNS = 10.77.0.1

[Peer]
PublicKey = %s
Endpoint = localhost:%d
AllowedIPs = 10.77.0.0/24, 192.168.1.0/24
PersistentKeepalive = 25
`, laptopPrivate, serverPublic, port))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	home, err := Start(ctx, server)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()

	listener, err := home.Listen(netip.MustParseAddrPort("10.77.0.1:445"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		line, _ := bufio.NewReader(connection).ReadString('\n')
		fmt.Fprintf(connection, "server got %s", line)
	}()

	away, err := Start(ctx, laptop)
	if err != nil {
		t.Fatal(err)
	}
	defer away.Close()

	connection, err := away.DialContext(ctx, "tcp", "10.77.0.1:445")
	if err != nil {
		t.Fatalf("the server should be reachable through the tunnel: %v", err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(10 * time.Second))

	fmt.Fprintf(connection, "hello\n")
	reply, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil || reply != "server got hello\n" {
		t.Fatalf("got %q, %v", reply, err)
	}
}

func TestBrokenSettingsAreExplained(t *testing.T) {
	_, public := keyPair(t)
	for settings, problem := range map[string]string{
		"[Interface]\nAddress = 10.0.0.2/32\n":                                                                  "private key",
		"[Interface]\nPrivateKey = " + public + "\n":                                                            "address",
		"[Interface]\nPrivateKey = " + public + "\nAddress = 10.0.0.2/32\n":                                     "peer",
		"[Interface]\nPrivateKey = nope\nAddress = 10.0.0.2/32\n":                                               "private key",
		"[Interface]\nPrivateKey = " + public + "\nAddress = nonsense\n":                                        "isn't an address",
		"[Interface]\nPrivateKey = " + public + "\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = " + public + "\n": "AllowedIPs",
	} {
		if _, err := Parse(settings); err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("settings %q gave %v, want a message about %s", settings, err, problem)
		}
	}
}
