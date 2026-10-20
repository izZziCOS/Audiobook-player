package instance

import (
    "bufio"
    "encoding/json"
    "io"
    "net"
    "os"
    "time"
)

const (
    timeout      = 2 * time.Second
    messageLimit = 64 << 10
)

type message struct {
    Args []string `json:"args"`
}

type Server struct {
    listener net.Listener
    path     string
}

func Claim(path string, args []string) (*Server, bool, error) {
    if forward(path, args) {
        return nil, true, nil
    }

    os.Remove(path)
    listener, err := net.Listen("unix", path)
    if err != nil {
        if forward(path, args) {
            return nil, true, nil
        }
        return nil, false, err
    }

    return &Server{listener: listener, path: path}, false, nil
}

func forward(path string, args []string) bool {
    connection, err := net.DialTimeout("unix", path, timeout)
    if err != nil {
        return false
    }
    defer connection.Close()

    connection.SetDeadline(time.Now().Add(timeout))
    if args == nil {
        args = []string{}
    }
    if err := json.NewEncoder(connection).Encode(message{Args: args}); err != nil {
        return true
    }

    bufio.NewReader(connection).ReadString('\n')
    return true
}

func (s *Server) Serve(handle func(args []string)) {
    for {
        connection, err := s.listener.Accept()
        if err != nil {
            return
        }

        go s.receive(connection, handle)
    }
}

func (s *Server) receive(connection net.Conn, handle func(args []string)) {
    defer connection.Close()
    connection.SetDeadline(time.Now().Add(timeout))

    var received message
    if err := json.NewDecoder(io.LimitReader(connection, messageLimit)).Decode(&received); err != nil {
        return
    }

    handle(received.Args)
    io.WriteString(connection, "ok\n")
}

func (s *Server) Close() error {
    err := s.listener.Close()
    os.Remove(s.path)
    return err
}
