package ts3

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestClient(t *testing.T) {
	s := newServer(t)
	defer func() {
		require.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	require.NoError(t, err)

	defer func() {
		require.Error(t, c.Close())
	}()

	_, err = c.Exec("version")
	require.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("version"))
	require.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("invalid"))
	require.Error(t, err)

	_, err = c.ExecCmd(NewCmd("disconnect"))
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestClientSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		require.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second), SSH(sshClientTestConfig))
	require.NoError(t, err)

	defer func() {
		require.Error(t, c.Close())
	}()

	_, err = c.Exec("version")
	require.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("version"))
	require.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("invalid"))
	require.Error(t, err)

	_, err = c.ExecCmd(NewCmd("disconnect"))
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestClientNilOption(t *testing.T) {
	_, err := NewClient("", nil)
	if !assert.Error(t, err) {
		return
	}

	assert.Equal(t, ErrNilOption, err)
}

func TestClientOptionError(t *testing.T) {
	errBadOption := errors.New("bad option")
	_, err := NewClient("", func(_ *Client) error { return errBadOption })
	if !assert.Error(t, err) {
		return
	}

	assert.Equal(t, errBadOption, err)
}

func TestClientDisconnect(t *testing.T) {
	s := newServer(t)
	defer func() {
		require.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	require.NoError(t, err)

	require.NoError(t, c.Close())

	_, err = c.Exec("version")
	require.ErrorIs(t, err, ErrNotConnected)
}

func TestClientDisconnectSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second), SSH(sshClientTestConfig))
	if !assert.NoError(t, err) {
		return
	}

	assert.NoError(t, c.Close())

	_, err = c.Exec("version")
	assert.Error(t, err)
}

func TestClientWriteFail(t *testing.T) {
	s := newServer(t)
	defer func() {
		require.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	require.NoError(t, err)
	require.NoError(t, c.conn.(*legacyConnection).Conn.(*net.TCPConn).CloseWrite())

	_, err = c.Exec("version")
	opErr := new(net.OpError)
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &opErr), "lost transport error: %v", err)
}

func TestClientDialFail(t *testing.T) {
	c, err := NewClient("127.0.0.1", Timeout(time.Nanosecond))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientDialFailSSH(t *testing.T) {
	c, err := NewClient("127.0.0.1", Timeout(time.Nanosecond), SSH(sshClientTestConfig))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientTimeout(t *testing.T) {
	s := newServer(t)
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100))
	if !assert.NoError(t, err) {
		return
	}

	// Not receiving a response must cause a timeout
	_, err = c.Exec(" ")
	assert.Error(t, err)
}

func TestClientTimeoutSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100), SSH(sshClientTestConfig))
	if !assert.NoError(t, err) {
		return
	}

	// Not receiving a response must cause a timeout
	_, err = c.Exec(" ")
	assert.Error(t, err)
}

func TestClientDeadline(t *testing.T) {
	s := newServer(t)
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100))
	if !assert.NoError(t, err) {
		return
	}

	_, err = c.Exec("version")
	assert.NoError(t, err)

	// Inactivity must not cause a timeout
	time.Sleep(c.timeout * 2)

	_, err = c.Exec("version")
	assert.NoError(t, err)
}

func TestClientDeadlineSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100), SSH(sshClientTestConfig))
	if !assert.NoError(t, err) {
		return
	}

	_, err = c.Exec("version")
	assert.NoError(t, err)

	// Inactivity must not cause a timeout
	time.Sleep(c.timeout * 2)

	_, err = c.Exec("version")
	assert.NoError(t, err)
}

func TestClientNoHeader(t *testing.T) {
	s := newServer(t, noHeader())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientNoBanner(t *testing.T) {
	s := newServer(t, noBanner())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Millisecond*100))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientFailConn(t *testing.T) {
	s := newServer(t, failConn())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientFailConnSSH(t *testing.T) {
	s := newServer(t, failConn())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second), SSH(sshClientTestConfig))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestClientBadHeader(t *testing.T) {
	s := newServer(t, badHeader())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	if assert.Error(t, err) {
		return
	}

	// Should never get here
	assert.NoError(t, c.Close())
}

func TestConcurrency(t *testing.T) {
	for _, useSSHConnection := range []bool{false, true} {
		t.Run(fmt.Sprintf("ssh=%t", useSSHConnection), func(t *testing.T) {
			var (
				serverOptions []serverOption
				wg            sync.WaitGroup
			)
			options := []func(*Client) error{Timeout(time.Second)}
			if useSSHConnection {
				serverOptions = append(serverOptions, useSSH())
				options = append(options, SSH(sshClientTestConfig))
			}
			s := newServer(t, serverOptions...)
			defer func() { require.NoError(t, s.Close()) }()
			c, err := NewClient(s.Addr, options...)
			require.NoError(t, err)
			defer func() { require.NoError(t, c.Close()) }()

			const (
				workers    = 8
				iterations = 25
			)
			results := make(chan error, workers)
			start := make(chan struct{})
			for worker := 0; worker < workers; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					<-start
					for i := 0; i < iterations; i++ {
						cmd := []string{"version", "servergrouplist", "login", "invalid"}[(worker+i)%4]
						if err := execConcurrentCommand(c, cmd); err != nil {
							results <- err
							return
						}
					}
				}(worker)
			}
			close(start)
			finished := make(chan struct{})
			go func() {
				wg.Wait()
				close(finished)
			}()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "concurrent commands did not finish")
			}
			close(results)
			for err := range results {
				require.NoError(t, err)
			}
		})
	}
}

func execConcurrentCommand(c *Client, cmd string) error {
	lines, err := c.Exec(cmd)
	if cmd == "invalid" {
		serverErr := new(Error)
		if errors.As(err, &serverErr) && serverErr.ID == 256 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: unexpected server error: %w", cmd, err)
		}
		return fmt.Errorf("%s: expected server error", cmd)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	if expected := commands[cmd]; expected == "" {
		if len(lines) == 0 {
			return nil
		}
	} else if len(lines) == 1 && lines[0] == expected {
		return nil
	}
	return fmt.Errorf("%s: incorrect response %q", cmd, lines)
}

func TestClientWorkTimeout(t *testing.T) {
	c := &Client{
		timeout:     20 * time.Millisecond,
		work:        make(chan string),
		commandLock: make(chan struct{}, 1),
		done:        make(chan struct{}),
	}
	defer c.closeDone()
	result := make(chan error, 1)
	go func() {
		_, err := c.Exec("version")
		result <- err
	}()
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrTimeout)
		require.True(t, c.IsConnected(), "an unsent command must not invalidate the connection")
	case <-time.After(time.Second):
		require.FailNow(t, "command submission did not time out")
	}
}

func TestClientQueuedCommand(t *testing.T) {
	testCases := []struct {
		name      string
		connected bool
		err       error
	}{
		{name: "timeout", connected: true, err: ErrTimeout},
		{name: "disconnect", err: ErrNotConnected},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{
				timeout:     20 * time.Millisecond,
				work:        make(chan string),
				commandLock: make(chan struct{}, 1),
				done:        make(chan struct{}),
			}
			defer c.closeDone()
			c.commandLock <- struct{}{}
			if !tc.connected {
				c.closeDone()
			}

			result := make(chan error, 1)
			go func() {
				_, err := c.Exec("version")
				result <- err
			}()
			select {
			case err := <-result:
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, tc.connected, c.IsConnected())
			case cmd := <-c.work:
				require.FailNowf(t, "queued command sent before the preceding response", "command: %q", cmd)
			case <-time.After(time.Second):
				require.FailNow(t, "queued command did not return")
			}
			<-c.commandLock
		})
	}
}

// blockedSSHChannel models a channel close waiting behind a transport write.
type blockedSSHChannel struct {
	conn    net.Conn
	writing chan struct{}
	mu      sync.Mutex

	ssh.Channel
}

func (c *blockedSSHChannel) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.writing)
	return c.conn.Write(p) //nolint: wrapcheck
}

func (c *blockedSSHChannel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return nil
}

func TestClientQueuedSSHTimeoutAbortsTransport(t *testing.T) {
	conn, peer := net.Pipe()
	defer func() { require.NoError(t, peer.Close()) }()
	channel := &blockedSSHChannel{conn: conn, writing: make(chan struct{})}
	c := &Client{
		conn:        &sshConnection{Conn: conn, channel: channel},
		timeout:     600 * time.Millisecond,
		keepAlive:   time.Hour,
		work:        make(chan string),
		commandLock: make(chan struct{}, 1),
		response:    make(chan response),
		done:        make(chan struct{}),
	}
	c.wg.Add(1)
	go c.workHandler()
	defer func() {
		c.closeDone()
		require.NoError(t, conn.Close())
		c.wg.Wait()
	}()

	c.commandLock <- struct{}{}
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		_, err := c.Exec("version")
		result <- err
	}()
	<-started
	time.Sleep(c.timeout * 2 / 3)
	<-c.commandLock
	select {
	case <-channel.writing:
	case <-time.After(time.Second):
		require.FailNow(t, "queued command did not reach the SSH transport")
	}
	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrTimeout)
		require.False(t, c.IsConnected())
	case <-time.After(c.timeout / 2):
		require.FailNow(t, "SSH shutdown exceeded the remaining command budget")
	}
}

func TestClientLateResponseAfterDisconnect(t *testing.T) {
	for _, trailer := range []string{errOK, errUnknownCmd} {
		t.Run(trailer, func(t *testing.T) {
			c := &Client{
				scanner:  bufio.NewScanner(strings.NewReader("late response\n" + trailer + "\n")),
				response: make(chan response),
				notify:   make(chan Notification),
				done:     make(chan struct{}),
			}
			c.closeDone()
			c.wg.Add(1)
			go c.messageHandler()
			select {
			case _, ok := <-c.notify:
				require.False(t, ok)
			case <-time.After(time.Second):
				require.FailNow(t, "message handler blocked delivering a response after disconnect")
			}
			c.wg.Wait()
		})
	}
}

func TestClientResponseTimeoutDisconnects(t *testing.T) {
	for _, useSSHConnection := range []bool{false, true} {
		t.Run(fmt.Sprintf("ssh=%t", useSSHConnection), func(t *testing.T) {
			var (
				serverOptions []serverOption
				options       = []func(*Client) error{Timeout(200 * time.Millisecond)}
			)
			if useSSHConnection {
				serverOptions = append(serverOptions, useSSH())
				options = append(options, SSH(sshClientTestConfig))
			}
			s := newServer(t, serverOptions...)
			defer func() { require.NoError(t, s.Close()) }()
			c, err := NewClient(s.Addr, options...)
			require.NoError(t, err)
			defer func() { require.ErrorIs(t, c.Close(), ErrNotConnected) }()

			_, err = c.Exec(" ")
			require.ErrorIs(t, err, ErrTimeout)
			require.False(t, c.IsConnected())
			_, err = c.Exec("version")
			require.ErrorIs(t, err, ErrNotConnected)
		})
	}
}

func TestClientConcurrentDisconnect(t *testing.T) {
	s := newServer(t)
	defer func() { require.NoError(t, s.Close()) }()
	c, err := NewClient(s.Addr, Timeout(time.Second))
	require.NoError(t, err)
	defer func() { require.ErrorIs(t, c.Close(), ErrNotConnected) }()

	const (
		workers = 8
	)
	results := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			<-start
			_, err := c.Exec("disconnect")
			results <- err
		}()
	}
	close(start)
	for i := 0; i < workers; i++ {
		select {
		case err := <-results:
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrTimeout)
		case <-time.After(2 * time.Second):
			require.FailNow(t, "command blocked after disconnect")
		}
	}
	require.False(t, c.IsConnected())
}
