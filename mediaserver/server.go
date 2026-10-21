package mediaserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hirochachacha/go-smb2"

	"github.com/izzzicos/audiobook-player/tunnel"
	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	smbPort          = "445"
	directTimeout    = 3 * time.Second
	tunnelTimeout    = 15 * time.Second
	connectTimeout   = 30 * time.Second
	operationTimeout = 20 * time.Second
	checkEvery       = 30 * time.Second
	firstRetry       = 5 * time.Second
	longestRetry     = time.Minute

	statusNoSuchFile       = 0xC000000F
	statusAccessDenied     = 0xC0000022
	statusNameNotFound     = 0xC0000034
	statusPathNotFound     = 0xC000003A
	statusLogonFailure     = 0xC000006D
	statusBadNetworkName   = 0xC00000CC
	statusObjectNameExists = 0xC0000035
)

type Settings struct {
	Address  string `json:"address"`
	Share    string `json:"share"`
	Folder   string `json:"folder,omitempty"`
	Username string `json:"username"`
}

type State int

const (
	Disconnected State = iota
	Connecting
	Connected
)

type Status struct {
	State   State
	Route   string
	Problem string
}

type Server struct {
	OnChange func()

	settings  Settings
	password  string
	wireguard *tunnel.Config

	mu      sync.Mutex
	tcp     net.Conn
	session *smb2.Session
	share   *smb2.Share
	tunnel  *tunnel.Tunnel
	status  Status

	wake    chan struct{}
	stop    chan struct{}
	stopped chan struct{}
}

func New(settings Settings, password string, wireguard *tunnel.Config) *Server {
	return &Server{
		settings:  settings,
		password:  password,
		wireguard: wireguard,
		wake:      make(chan struct{}, 1),
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

func (s *Server) Start() {
	go s.run()
}

func (s *Server) Close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	<-s.stopped
}

func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.status
}

func (s *Server) setStatus(status Status) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()

	if s.OnChange != nil {
		s.OnChange()
	}
}

func (s *Server) run() {
	defer close(s.stopped)
	defer s.disconnect()

	stopping, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-s.stop
		cancel()
	}()

	retry := firstRetry
	for {
		s.setStatus(Status{State: Connecting})
		route, err := s.connect(stopping)
		if stopping.Err() != nil {
			return
		}
		if err != nil {
			s.setStatus(Status{State: Disconnected, Problem: Explain(err)})
			if !s.pause(retry) {
				return
			}
			retry = min(retry*2, longestRetry)
			continue
		}

		retry = firstRetry
		s.setStatus(Status{State: Connected, Route: route})
		if !s.watch() {
			return
		}
	}
}

func (s *Server) pause(wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-s.stop:
		return false
	case <-s.wake:
		return true
	case <-timer.C:
		return true
	}
}

func (s *Server) watch() bool {
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return false
		case <-s.wake:
			if !s.Connected() {
				return true
			}
		case <-ticker.C:
			share, err := s.current()
			if err == nil {
				checked, cancel := limited(share)
				_, err = checked.Stat(s.remote(""))
				cancel()
			}
			if err != nil && !isMissing(err) {
				s.broken(err)
				return true
			}
		}
	}
}

func limited(share *smb2.Share) (*smb2.Share, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	return share.WithContext(ctx), cancel
}

func (s *Server) host() (string, string) {
	address := strings.TrimSpace(s.settings.Address)
	if host, port, err := net.SplitHostPort(address); err == nil {
		return host, port
	}

	return strings.Trim(address, "[]"), smbPort
}

func (s *Server) dial(ctx context.Context) (net.Conn, string, error) {
	host, port := s.host()
	address := net.JoinHostPort(host, port)

	direct, directErr := (&net.Dialer{Timeout: directTimeout}).DialContext(ctx, "tcp", address)
	if directErr == nil {
		s.closeTunnel()
		return direct, "on your home network", nil
	}
	if s.wireguard == nil {
		return nil, "", directErr
	}

	s.mu.Lock()
	current := s.tunnel
	s.mu.Unlock()
	if current == nil {
		started, err := tunnel.Start(ctx, *s.wireguard)
		if err != nil {
			return nil, "", err
		}
		s.mu.Lock()
		s.tunnel, current = started, started
		s.mu.Unlock()
	}

	dialCtx, cancel := context.WithTimeout(ctx, tunnelTimeout)
	defer cancel()
	connection, err := current.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, "", fmt.Errorf("%w (also tried through WireGuard: %v)", directErr, err)
	}

	return connection, "through WireGuard", nil
}

func (s *Server) connect(stopping context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(stopping, connectTimeout)
	defer cancel()

	connection, route, err := s.dial(ctx)
	if err != nil {
		return "", err
	}

	dialer := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: s.settings.Username, Password: s.password}}
	session, err := dialer.DialContext(ctx, connection)
	if err != nil {
		connection.Close()
		return "", err
	}

	host, _ := s.host()
	share, err := session.WithContext(ctx).Mount(`\\` + host + `\` + s.settings.Share)
	if err != nil {
		session.Logoff()
		connection.Close()
		return "", err
	}

	if info, err := share.WithContext(ctx).Stat(s.remote("")); err != nil || !info.IsDir() {
		share.Umount()
		session.Logoff()
		connection.Close()
		if err == nil {
			err = fmt.Errorf("%q on the server isn't a folder", s.settings.Folder)
		}
		return "", err
	}

	s.mu.Lock()
	s.tcp, s.session, s.share = connection, session, share
	s.mu.Unlock()

	return route, nil
}

func (s *Server) disconnect() {
	s.mu.Lock()
	share, session, connection := s.share, s.session, s.tcp
	s.share, s.session, s.tcp = nil, nil, nil
	s.mu.Unlock()

	if share != nil {
		share.Umount()
	}
	if session != nil {
		session.Logoff()
	}
	if connection != nil {
		connection.Close()
	}
	s.closeTunnel()
}

func (s *Server) closeTunnel() {
	s.mu.Lock()
	current := s.tunnel
	s.tunnel = nil
	s.mu.Unlock()

	if current != nil {
		current.Close()
	}
}

func (s *Server) broken(err error) {
	s.mu.Lock()
	share, session, connection := s.share, s.session, s.tcp
	s.share, s.session, s.tcp = nil, nil, nil
	wasConnected := s.status.State == Connected
	if wasConnected {
		s.status = Status{State: Disconnected, Problem: Explain(err)}
	}
	s.mu.Unlock()

	if connection != nil {
		connection.Close()
	}
	if share != nil {
		go func() {
			share.Umount()
			session.Logoff()
		}()
	}
	if wasConnected && s.OnChange != nil {
		s.OnChange()
	}

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Server) Reconnect() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Server) current() (*smb2.Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.share == nil {
		return nil, vfs.ErrNotConnected
	}

	return s.share, nil
}

func (s *Server) remote(name string) string {
	joined := path.Join(strings.Trim(s.settings.Folder, "/\\"), name)
	if joined == "." {
		return ""
	}

	return joined
}

func (s *Server) failed(err error) error {
	var response *smb2.ResponseError
	if errors.As(err, &response) {
		switch response.Code {
		case statusNoSuchFile, statusNameNotFound, statusPathNotFound:
			return fmt.Errorf("%w: %v", fs.ErrNotExist, err)
		case statusAccessDenied:
			return fmt.Errorf("%w: %v", fs.ErrPermission, err)
		case statusObjectNameExists:
			return fmt.Errorf("%w: %v", fs.ErrExist, err)
		}
		return err
	}
	if errors.Is(err, vfs.ErrNotConnected) || !connectionLost(err) {
		return err
	}

	s.broken(err)
	return fmt.Errorf("%w: %v", vfs.ErrNotConnected, err)
}

func connectionLost(err error) bool {
	var transport *smb2.TransportError
	var deadline *smb2.ContextError
	var network net.Error

	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrPermission), errors.Is(err, fs.ErrExist):
		return false
	case errors.As(err, &transport), errors.As(err, &deadline), errors.As(err, &network):
		return true
	default:
		return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
			errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	}
}

func isMissing(err error) bool {
	var response *smb2.ResponseError
	if errors.As(err, &response) {
		return response.Code == statusNameNotFound || response.Code == statusPathNotFound || response.Code == statusNoSuchFile
	}

	return errors.Is(err, fs.ErrNotExist)
}

func (s *Server) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.share != nil
}

func (s *Server) Open(name string) (vfs.File, error) {
	share, err := s.current()
	if err != nil {
		return nil, err
	}

	file, err := share.Open(s.remote(name))
	if err != nil {
		return nil, s.failed(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, s.failed(err)
	}

	return newCachedFile(s, file, info), nil
}

func (s *Server) Stat(name string) (fs.FileInfo, error) {
	share, err := s.current()
	if err != nil {
		return nil, err
	}

	share, cancel := limited(share)
	defer cancel()

	info, err := share.Stat(s.remote(name))
	if err != nil {
		return nil, s.failed(err)
	}

	return info, nil
}

func (s *Server) ReadDir(name string) ([]fs.DirEntry, error) {
	share, err := s.current()
	if err != nil {
		return nil, err
	}

	share, cancel := limited(share)
	defer cancel()

	infos, err := share.ReadDir(s.remote(name))
	if err != nil {
		return nil, s.failed(err)
	}

	entries := make([]fs.DirEntry, len(infos))
	for i, info := range infos {
		entries[i] = fs.FileInfoToDirEntry(info)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return entries, nil
}

func (s *Server) WriteAtomic(name string, data []byte) error {
	share, err := s.current()
	if err != nil {
		return err
	}
	share, cancel := limited(share)
	defer cancel()

	target := s.remote(name)
	if directory := path.Dir(target); directory != "." {
		if err := share.MkdirAll(directory, 0o755); err != nil {
			return s.failed(err)
		}
	}

	random := make([]byte, 6)
	rand.Read(random)
	temporary := target + ".tmp-" + hex.EncodeToString(random)

	file, err := share.Create(temporary)
	if err != nil {
		return s.failed(err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		share.Remove(temporary)
		return s.failed(err)
	}

	if err := share.Remove(target); err != nil && !isMissing(err) {
		share.Remove(temporary)
		return s.failed(err)
	}
	if err := share.Rename(temporary, target); err != nil {
		share.Remove(temporary)
		return s.failed(err)
	}

	return nil
}

func Explain(err error) string {
	var response *smb2.ResponseError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &response) && response.Code == statusLogonFailure:
		return "The server didn't accept the username or password."
	case errors.As(err, &response) && response.Code == statusBadNetworkName:
		return "The server has no shared folder with that name."
	case errors.As(err, &response) && response.Code == statusAccessDenied:
		return "This account isn't allowed to open that shared folder."
	case errors.Is(err, fs.ErrNotExist):
		return "The folder inside the shared folder wasn't found."
	default:
		return "The server couldn't be reached: " + err.Error()
	}
}
