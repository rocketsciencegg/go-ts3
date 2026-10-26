package ts3

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	// connectionTestTimeout bounds network reads and server completion.
	connectionTestTimeout = 5 * time.Second
	// notificationTestPayloadSize exceeds the SSH receive window.
	notificationTestPayloadSize = 3 << 20
	// blockedWriteTestTimeout bounds a write with no reader.
	blockedWriteTestTimeout = 20 * time.Millisecond
)

var sshClientTestConfig = &ssh.ClientConfig{
	HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint: gosec
}

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

func TestSSHConnection_ReadAfterWriteDeadline(t *testing.T) {
	setup := func(t *testing.T, timeout time.Duration) (*sshConnection, net.Conn) {
		t.Helper()
		s := newServer(t, useSSH())
		t.Cleanup(func() {
			require.NoError(t, s.Close())
		})

		c := &sshConnection{config: sshClientTestConfig}
		err := c.Connect(s.Addr, timeout)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, c.Close())
		})

		err = c.SetReadDeadline(time.Now().Add(connectionTestTimeout))
		require.NoError(t, err)
		reader := bufio.NewReader(c)
		_, err = reader.ReadString('\r')
		require.NoError(t, err)
		_, err = reader.ReadString('\r')
		require.NoError(t, err)

		var serverConn net.Conn
		s.mtx.Lock()

		for conn := range s.conns {
			serverConn = conn
		}

		s.mtx.Unlock()
		require.NotNil(t, serverConn)
		return c, serverConn
	}

	t.Run("bounded-writes", func(t *testing.T) {
		c, serverConn := setup(t, time.Second)
		requireSSHReadAfterWriteDeadline(t, c, serverConn)
	})

	t.Run("zero-timeout", func(t *testing.T) {
		c, serverConn := setup(t, 0)
		requireSSHReadAfterWriteDeadline(t, c, serverConn)
	})
}

func requireSSHReadAfterWriteDeadline(t *testing.T, c *sshConnection, serverConn net.Conn) {
	t.Helper()
	// Force window adjustments after the command's write deadline has expired.
	err := c.SetWriteDeadline(time.Now().Add(-time.Second))
	require.NoError(t, err)
	payload := bytes.Repeat([]byte("x"), notificationTestPayloadSize)
	written := make(chan error, 1)

	go func() {
		_, err := serverConn.Write(payload)
		written <- err
	}()

	received := make([]byte, len(payload))
	_, err = io.ReadFull(c, received)
	require.NoError(t, err)
	require.True(t, bytes.Equal(payload, received), "SSH payload changed")

	select {
	case err := <-written:
		require.NoError(t, err)
	case <-time.After(connectionTestTimeout):
		require.FailNow(t, "server blocked waiting for SSH window adjustment")
	}
}

func TestWriteTimeoutConn_Write(t *testing.T) {
	setup := func(t *testing.T) (*writeTimeoutConn, *writeDeadlineConn) {
		t.Helper()
		conn := &writeDeadlineConn{
			deadline: time.Now().Add(-time.Second),
			written:  len("abc"),
		}
		return &writeTimeoutConn{timeout: time.Second, Conn: conn}, conn
	}

	t.Run("success", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, conn := setup(t)
			requireWriteTimeoutSuccess(t, c, conn, 1)
		})
	})

	t.Run("refreshes-each-write", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, conn := setup(t)
			requireWriteTimeoutSuccess(t, c, conn, 1)
			time.Sleep(time.Second)
			requireWriteTimeoutSuccess(t, c, conn, 2)
		})
	})

	t.Run("deadline-failure", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, conn := setup(t)
			errDeadline := errors.New("deadline failure")
			conn.deadlineErr = errDeadline
			n, err := c.Write([]byte("abc"))
			require.ErrorIs(t, err, errDeadline)
			require.EqualError(t, err, "set write deadline: deadline failure")
			require.Zero(t, n)
			require.Zero(t, conn.writes)
			require.Equal(t, time.Now().Add(c.timeout), conn.deadline)
		})
	})

	t.Run("partial-write", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, conn := setup(t)
			errWrite := errors.New("write failure")
			conn.writeErr = errWrite
			conn.written = len("ab")
			n, err := c.Write([]byte("abc"))
			require.ErrorIs(t, err, errWrite)
			require.EqualError(t, err, "write: write failure")
			require.Equal(t, len("ab"), n)
			require.Equal(t, 1, conn.writes)
			require.Equal(t, time.Now().Add(c.timeout), conn.deadline)
		})
	})

	t.Run("zero-timeout", func(t *testing.T) {
		c, conn := setup(t)
		c.timeout = 0
		requireUnlimitedWriteSuccess(t, c, conn, 1)
		requireUnlimitedWriteSuccess(t, c, conn, 2)
	})
}

func requireWriteTimeoutSuccess(t *testing.T, c *writeTimeoutConn, conn *writeDeadlineConn, writes int) {
	t.Helper()
	previous := conn.deadline
	n, err := c.Write([]byte("abc"))
	require.NoError(t, err)
	require.Equal(t, len("abc"), n)
	require.Equal(t, writes, conn.writes)
	require.Equal(t, time.Now().Add(c.timeout), conn.deadline)
	require.True(t, conn.deadline.After(previous))
}

func requireUnlimitedWriteSuccess(t *testing.T, c *writeTimeoutConn, conn *writeDeadlineConn, writes int) {
	t.Helper()
	n, err := c.Write([]byte("abc"))
	require.NoError(t, err)
	require.Equal(t, len("abc"), n)
	require.Equal(t, writes, conn.writes)
	require.True(t, conn.deadline.IsZero())
}

func TestWriteTimeoutConn_BlockedWrite(t *testing.T) {
	conn, peer := net.Pipe()
	defer func() {
		require.NoError(t, conn.Close())
	}()
	defer func() {
		require.NoError(t, peer.Close())
	}()
	c := &writeTimeoutConn{timeout: blockedWriteTestTimeout, Conn: conn}
	var netErr net.Error
	_, err := c.Write([]byte("blocked"))
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout())
}

func TestWriteTimeoutConn_NegativeTimeout(t *testing.T) {
	conn, peer := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		require.NoError(t, peer.Close())
	})
	c := &writeTimeoutConn{timeout: -time.Second, Conn: conn}
	n, err := c.Write([]byte("expired"))
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	require.Zero(t, n)
}

func TestSSHConnection_CloseAfterWriteDeadline(t *testing.T) {
	s := newServer(t, useSSH())
	t.Cleanup(func() {
		require.NoError(t, s.Close())
	})
	c := &sshConnection{config: sshClientTestConfig}
	err := c.Connect(s.Addr, time.Second)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := c.Conn.Close(); err != nil {
			require.ErrorIs(t, err, net.ErrClosed)
		}
	})
	reader := bufio.NewReader(c)
	err = c.SetReadDeadline(time.Now().Add(connectionTestTimeout))
	require.NoError(t, err)
	_, err = reader.ReadString('\r')
	require.NoError(t, err)
	_, err = reader.ReadString('\r')
	require.NoError(t, err)
	err = c.SetWriteDeadline(time.Now().Add(-time.Second))
	require.NoError(t, err)
	err = c.Close()
	require.NoError(t, err)
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
