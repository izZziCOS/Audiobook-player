package tunnel

import (
    "bufio"
    "context"
    "encoding/base64"
    "encoding/hex"
    "errors"
    "fmt"
    "net"
    "net/netip"
    "strconv"
    "strings"

    "golang.zx2c4.com/wireguard/conn"
    "golang.zx2c4.com/wireguard/device"
    "golang.zx2c4.com/wireguard/tun/netstack"
)

const defaultMTU = 1420

type Peer struct {
    PublicKey    string
    PresharedKey string
    Endpoint     string
    AllowedIPs   []netip.Prefix
    Keepalive    int
}

type Config struct {
    PrivateKey string
    Addresses  []netip.Addr
    DNS        []netip.Addr
    MTU        int
    ListenPort int
    Peers      []Peer
}

func Parse(text string) (Config, error) {
    config := Config{MTU: defaultMTU}
    section := ""
    var peer *Peer

    scanner := bufio.NewScanner(strings.NewReader(text))
    for number := 1; scanner.Scan(); number++ {
        line := strings.TrimSpace(scanner.Text())
        if index := strings.IndexAny(line, "#;"); index >= 0 {
            line = strings.TrimSpace(line[:index])
        }
        if line == "" {
            continue
        }

        if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
            section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
            if section == "peer" {
                config.Peers = append(config.Peers, Peer{})
                peer = &config.Peers[len(config.Peers)-1]
            }
            continue
        }

        key, value, found := strings.Cut(line, "=")
        if !found {
            return Config{}, fmt.Errorf("line %d of the WireGuard settings isn't a setting", number)
        }
        key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)

        var err error
        switch {
        case section == "interface" && key == "privatekey":
            config.PrivateKey = value
        case section == "interface" && key == "address":
            config.Addresses, err = addresses(value)
        case section == "interface" && key == "dns":
            config.DNS, err = dnsServers(value)
        case section == "interface" && key == "mtu":
            config.MTU, err = strconv.Atoi(value)
        case section == "interface" && key == "listenport":
            config.ListenPort, err = strconv.Atoi(value)
        case section == "peer" && key == "publickey":
            peer.PublicKey = value
        case section == "peer" && key == "presharedkey":
            peer.PresharedKey = value
        case section == "peer" && key == "endpoint":
            peer.Endpoint = value
        case section == "peer" && key == "allowedips":
            peer.AllowedIPs, err = prefixes(value)
        case section == "peer" && key == "persistentkeepalive":
            peer.Keepalive, err = strconv.Atoi(value)
        }
        if err != nil {
            return Config{}, fmt.Errorf("line %d of the WireGuard settings: %w", number, err)
        }
    }

    return config, config.check()
}

func (c Config) check() error {
    if _, err := keyHex(c.PrivateKey); err != nil {
        return fmt.Errorf("the WireGuard settings need this computer's private key: %w", err)
    }
    if len(c.Addresses) == 0 {
        return errors.New("the WireGuard settings need this computer's address inside the tunnel")
    }
    if len(c.Peers) == 0 {
        return errors.New("the WireGuard settings need the server as a peer")
    }
    for _, peer := range c.Peers {
        if _, err := keyHex(peer.PublicKey); err != nil {
            return fmt.Errorf("the server's public key: %w", err)
        }
        if len(peer.AllowedIPs) == 0 {
            return errors.New("the WireGuard settings need AllowedIPs for the server")
        }
    }

    return nil
}

func list(value string) []string {
    var items []string
    for _, item := range strings.Split(value, ",") {
        if item = strings.TrimSpace(item); item != "" {
            items = append(items, item)
        }
    }

    return items
}

func addresses(value string) ([]netip.Addr, error) {
    var result []netip.Addr
    for _, item := range list(value) {
        if prefix, err := netip.ParsePrefix(item); err == nil {
            result = append(result, prefix.Addr())
            continue
        }
        address, err := netip.ParseAddr(item)
        if err != nil {
            return nil, fmt.Errorf("%q isn't an address", item)
        }
        result = append(result, address)
    }

    return result, nil
}

func dnsServers(value string) ([]netip.Addr, error) {
    var result []netip.Addr
    for _, item := range list(value) {
        if address, err := netip.ParseAddr(item); err == nil {
            result = append(result, address)
        }
    }

    return result, nil
}

func prefixes(value string) ([]netip.Prefix, error) {
    var result []netip.Prefix
    for _, item := range list(value) {
        prefix, err := netip.ParsePrefix(item)
        if err != nil {
            return nil, fmt.Errorf("%q isn't an address range", item)
        }
        result = append(result, prefix)
    }

    return result, nil
}

func keyHex(key string) (string, error) {
    raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
    if err != nil || len(raw) != 32 {
        return "", errors.New("it isn't a WireGuard key")
    }

    return hex.EncodeToString(raw), nil
}

func (c Config) uapi(ctx context.Context) (string, error) {
    var settings strings.Builder

    private, _ := keyHex(c.PrivateKey)
    fmt.Fprintf(&settings, "private_key=%s\n", private)
    if c.ListenPort > 0 {
        fmt.Fprintf(&settings, "listen_port=%d\n", c.ListenPort)
    }
    settings.WriteString("replace_peers=true\n")

    for _, peer := range c.Peers {
        public, _ := keyHex(peer.PublicKey)
        fmt.Fprintf(&settings, "public_key=%s\n", public)
        if peer.PresharedKey != "" {
            shared, err := keyHex(peer.PresharedKey)
            if err != nil {
                return "", fmt.Errorf("the preshared key: %w", err)
            }
            fmt.Fprintf(&settings, "preshared_key=%s\n", shared)
        }
        if peer.Endpoint != "" {
            endpoint, err := resolve(ctx, peer.Endpoint)
            if err != nil {
                return "", err
            }
            fmt.Fprintf(&settings, "endpoint=%s\n", endpoint)
        }
        if peer.Keepalive > 0 {
            fmt.Fprintf(&settings, "persistent_keepalive_interval=%d\n", peer.Keepalive)
        }
        settings.WriteString("replace_allowed_ips=true\n")
        for _, prefix := range peer.AllowedIPs {
            fmt.Fprintf(&settings, "allowed_ip=%s\n", prefix)
        }
    }

    return settings.String(), nil
}

func resolve(ctx context.Context, endpoint string) (string, error) {
    host, port, err := net.SplitHostPort(endpoint)
    if err != nil {
        return "", fmt.Errorf("the server's endpoint %q needs an address and a port", endpoint)
    }
    if address, err := netip.ParseAddr(host); err == nil {
        return netip.AddrPortFrom(address, mustPort(port)).String(), nil
    }

    found, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
    if err != nil || len(found) == 0 {
        return "", fmt.Errorf("the server's address %q couldn't be looked up: %w", host, err)
    }
    best := found[0]
    for _, address := range found {
        if address.Is4() {
            best = address
            break
        }
    }

    return netip.AddrPortFrom(best.Unmap(), mustPort(port)).String(), nil
}

func mustPort(port string) uint16 {
    value, _ := strconv.ParseUint(port, 10, 16)
    return uint16(value)
}

type Tunnel struct {
    device *device.Device
    net    *netstack.Net
}

func Start(ctx context.Context, config Config) (*Tunnel, error) {
    settings, err := config.uapi(ctx)
    if err != nil {
        return nil, err
    }

    mtu := config.MTU
    if mtu <= 0 {
        mtu = defaultMTU
    }
    tunDevice, network, err := netstack.CreateNetTUN(config.Addresses, config.DNS, mtu)
    if err != nil {
        return nil, fmt.Errorf("starting the WireGuard tunnel: %w", err)
    }

    wireguard := device.NewDevice(tunDevice, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
    if err := wireguard.IpcSet(settings); err != nil {
        wireguard.Close()
        return nil, fmt.Errorf("setting up the WireGuard tunnel: %w", err)
    }
    if err := wireguard.Up(); err != nil {
        wireguard.Close()
        return nil, fmt.Errorf("starting the WireGuard tunnel: %w", err)
    }

    return &Tunnel{device: wireguard, net: network}, nil
}

func (t *Tunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
    return t.net.DialContext(ctx, network, address)
}

func (t *Tunnel) Listen(address netip.AddrPort) (net.Listener, error) {
    return t.net.ListenTCPAddrPort(address)
}

func (t *Tunnel) Close() {
    t.device.Close()
}
