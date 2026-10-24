package ts3

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSHConnectionReadAfterWriteDeadline(t *testing.T) {
	tests := map[string]time.Duration{
		"bounded writes": time.Second,
		"zero timeout":   0,
	}
	for name, timeout := range tests {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, useSSH())
			defer func() {
				require.NoError(t, s.Close())
			}()

			c := &sshConnection{config: sshClientTestConfig}
			require.NoError(t, c.Connect(s.Addr, timeout))
			defer func() {
				require.NoError(t, c.Close())
			}()
			require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))

			reader := bufio.NewReader(c)
			for i := 0; i < 2; i++ {
				_, err := reader.ReadString('\r')
				require.NoError(t, err)
			}

			var ( //nolint:gofumpt // mp-go requires grouped declarations, including single items.
				serverConn net.Conn
			)
			s.mtx.Lock()
			for conn := range s.conns {
				serverConn = conn
			}
			s.mtx.Unlock()
			require.NotNil(t, serverConn)

			// Expire the command deadline, then read more than the SSH receive window
			// so reading must send window adjustments without another client command.
			require.NoError(t, c.SetWriteDeadline(time.Now().Add(-time.Second)))
			payload := bytes.Repeat([]byte("x"), 3<<20)
			written := make(chan error, 1)
			go func() {
				_, err := serverConn.Write(payload)
				written <- err
			}()

			received := make([]byte, len(payload))
			_, err := io.ReadFull(reader, received)
			require.NoError(t, err)
			require.True(t, bytes.Equal(payload, received), "SSH payload changed")
			select {
			case err := <-written:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				require.FailNow(t, "server blocked waiting for SSH window adjustment")
			}
		})
	}
}

var ( //nolint:gofumpt // mp-go requires grouped declarations, including single items.
	sshClientTestConfig = &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec
	}
)

type writeDeadlineConn struct {
	deadline    time.Time
	deadlineErr error
	writeErr    error
	written     int
	writes      int

	net.Conn
}

func (c *writeDeadlineConn) SetWriteDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.deadlineErr
}

func (c *writeDeadlineConn) Write(_ []byte) (int, error) {
	c.writes++
	return c.written, c.writeErr
}

func TestWriteTimeoutConn(t *testing.T) {
	errDeadline := errors.New("deadline failure")
	errWrite := errors.New("write failure")
	tests := map[string]struct {
		timeout     time.Duration
		deadlineErr error
		writeErr    error
		written     int
		wantWrites  int
		wantErr     error
		wantMessage string
	}{
		"success": {timeout: time.Second, written: 3, wantWrites: 1},
		"deadline failure": {
			timeout:     time.Second,
			deadlineErr: errDeadline,
			wantErr:     errDeadline,
			wantMessage: "set write deadline: deadline failure",
		},
		"partial write": {
			timeout:     time.Second,
			written:     2,
			writeErr:    errWrite,
			wantWrites:  1,
			wantErr:     errWrite,
			wantMessage: "write: write failure",
		},
		"zero timeout": {written: 3, wantWrites: 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conn := &writeDeadlineConn{
				deadline:    time.Now().Add(-time.Second),
				deadlineErr: tc.deadlineErr,
				writeErr:    tc.writeErr,
				written:     tc.written,
			}
			c := &writeTimeoutConn{timeout: tc.timeout, Conn: conn}
			for i := 0; i < 2; i++ {
				previous := conn.deadline
				before := time.Now()
				n, err := c.Write([]byte("abc"))
				require.ErrorIs(t, err, tc.wantErr)
				if tc.wantErr != nil {
					require.EqualError(t, err, tc.wantMessage)
				}
				require.Equal(t, tc.written, n)
				require.Equal(t, (i+1)*tc.wantWrites, conn.writes)
				if tc.timeout == 0 {
					require.True(t, conn.deadline.IsZero())
					continue
				}
				require.False(t, conn.deadline.Before(before.Add(c.timeout)))
				require.False(t, conn.deadline.After(time.Now().Add(c.timeout)))
				require.True(t, conn.deadline.After(previous))
			}
		})
	}
}

func TestWriteTimeoutConnBlockedWrite(t *testing.T) {
	conn, peer := net.Pipe()
	defer func() {
		require.NoError(t, conn.Close())
	}()
	defer func() {
		require.NoError(t, peer.Close())
	}()
	c := &writeTimeoutConn{timeout: 20 * time.Millisecond, Conn: conn}
	_, err := c.Write([]byte("blocked"))
	var ( //nolint:gofumpt // mp-go requires grouped declarations, including single items.
		netErr net.Error
	)
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout())
}

func TestConnection(t *testing.T) {
	testCases := map[string]struct {
		conn      Connection
		newServer func(*testing.T) *server
	}{
		"legacyConnection": {
			conn: new(legacyConnection),
			newServer: func(t *testing.T) *server {
				t.Helper()
				return newServer(t)
			},
		},
		"sshConnection": {
			conn: &sshConnection{config: sshClientTestConfig},
			newServer: func(t *testing.T) *server {
				t.Helper()
				return newServer(t, useSSH())
			},
		},
	}

	for description, tc := range testCases {
		t.Run(description, func(t *testing.T) {
			s := tc.newServer(t)
			require.NotNil(t, s)

			assert.NoError(t, tc.conn.Connect(s.Addr, time.Second))

			line, _, err := bufio.NewReader(tc.conn).ReadLine()
			assert.NoError(t, err)
			assert.Equal(t, "TS3", string(line))

			assert.NoError(t, s.Close())
			assert.NoError(t, tc.conn.Close())
		})
	}
}

func TestVerifyAddr(t *testing.T) {
	testCases := map[string]struct {
		addr     string
		expected string
	}{
		"hostname without port": {
			addr:     "localhost",
			expected: "localhost:1234",
		},
		"hostname with port": {
			addr:     "localhost:1337",
			expected: "localhost:1337",
		},
		"IPv4 without port": {
			addr:     "127.0.0.1",
			expected: "127.0.0.1:1234",
		},
		"IPv4 with port": {
			addr:     "127.0.0.1:1337",
			expected: "127.0.0.1:1337",
		},
		"IPv6 without port": {
			addr:     "[::1]",
			expected: "[::1]:1234",
		},
		"IPv6 with port": {
			addr:     "[::1]:1337",
			expected: "[::1]:1337",
		},
		"IPv6 without brackets": {
			addr: "::1",
		},
		"empty addr": {
			addr:     "",
			expected: ":1234",
		},
		"invalid addr": {
			addr: "invalid:ddd:",
		},
	}

	for description, tc := range testCases {
		t.Run(description, func(t *testing.T) {
			addr, err := verifyAddr(tc.addr, 1234)
			if tc.expected == "" {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, addr)
		})
	}
}
