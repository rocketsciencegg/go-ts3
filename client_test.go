package ts3

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	invalidCommand = "invalid"
)

type concurrentCommandResult struct {
	cmd   string
	lines []string
	err   error
}

// blockedSSHChannel models a channel close waiting behind a transport write.
type blockedSSHChannel struct {
	conn    net.Conn
	writing chan struct{}
	mu      sync.Mutex

	ssh.Channel
}

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

	_, err = c.ExecCmd(NewCmd(invalidCommand))
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

	_, err = c.ExecCmd(NewCmd(invalidCommand))
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
	setup := func(t *testing.T, serverOptions []serverOption, options ...func(*Client) error) (*Client, *server) {
		t.Helper()
		s := newServer(t, serverOptions...)
		c, err := NewClient(s.Addr, options...)
		if err != nil {
			require.NoError(t, s.Close())
		}
		require.NoError(t, err)
		return c, s
	}

	t.Run("tcp", func(t *testing.T) {
		c, s := setup(t, nil, Timeout(time.Second))
		defer func() { require.NoError(t, s.Close()) }()
		defer func() { require.NoError(t, c.Close()) }()
		requireConcurrentCommands(t, c)
	})

	t.Run("ssh", func(t *testing.T) {
		c, s := setup(t, []serverOption{useSSH()}, Timeout(time.Second), SSH(sshClientTestConfig))
		defer func() { require.NoError(t, s.Close()) }()
		defer func() { require.NoError(t, c.Close()) }()
		requireConcurrentCommands(t, c)
	})
}

func requireConcurrentCommands(t *testing.T, c *Client) {
	t.Helper()

	const (
		workers    = 8
		iterations = 25
	)
	var wg sync.WaitGroup
	successes := make(chan concurrentCommandResult, workers*iterations)
	failures := make(chan concurrentCommandResult, workers*iterations)
	start := make(chan struct{})

	for worker := 0; worker < workers; worker++ {
		wg.Go(func() {
			<-start

			for i := 0; i < iterations; i++ {
				cmd := []string{"version", "servergrouplist", "login", invalidCommand}[(worker+i)%4]
				lines, err := c.Exec(cmd)
				result := concurrentCommandResult{cmd: cmd, lines: lines, err: err}

				if cmd == invalidCommand {
					failures <- result
				} else {
					successes <- result
				}
			}
		})
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
	close(successes)
	close(failures)

	for result := range successes {
		requireConcurrentCommandSuccess(t, result)
	}

	for result := range failures {
		requireConcurrentCommandError(t, result)
	}
}

func requireConcurrentCommandSuccess(t *testing.T, result concurrentCommandResult) {
	t.Helper()
	require.NoError(t, result.err, "command: %s", result.cmd)

	var expected []string
	if line := commands[result.cmd]; line != "" {
		expected = []string{line}
	}
	require.Equal(t, expected, result.lines, "command: %s", result.cmd)
}

func requireConcurrentCommandError(t *testing.T, result concurrentCommandResult) {
	t.Helper()
	serverErr := new(Error)
	require.ErrorAs(t, result.err, &serverErr)
	require.Equal(t, 256, serverErr.ID)
	require.Empty(t, result.lines)
}

func TestClientWorkTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{
			timeout:     20 * time.Millisecond,
			work:        make(chan string),
			commandLock: make(chan struct{}, 1),
			done:        make(chan struct{}),
		}
		defer c.closeDone()
		start := time.Now()
		_, err := c.Exec("version")
		require.ErrorIs(t, err, ErrTimeout)
		require.Equal(t, start.Add(c.timeout), time.Now())
		require.True(t, c.IsConnected(), "an unsent command must not invalidate the connection")
	})
}

func TestClientQueuedCommand(t *testing.T) {
	setup := func() *Client {
		c := &Client{
			timeout:     20 * time.Millisecond,
			work:        make(chan string),
			commandLock: make(chan struct{}, 1),
			done:        make(chan struct{}),
		}
		c.commandLock <- struct{}{}
		return c
	}

	t.Run("timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c := setup()
			defer c.closeDone()

			start := time.Now()
			requireQueuedCommandError(t, c, ErrTimeout)
			require.Equal(t, start.Add(c.timeout), time.Now())
			require.True(t, c.IsConnected())
			<-c.commandLock
		})
	})

	t.Run("disconnect", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c := setup()
			c.closeDone()

			start := time.Now()
			requireQueuedCommandError(t, c, ErrNotConnected)
			require.Equal(t, start, time.Now())
			require.False(t, c.IsConnected())
			<-c.commandLock
		})
	})
}

func requireQueuedCommandError(t *testing.T, c *Client, expected error) {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		_, err := c.Exec("version")
		result <- err
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, expected)
	case cmd := <-c.work:
		require.FailNowf(t, "queued command sent before the preceding response", "command: %q", cmd)
	case <-time.After(time.Second):
		require.FailNow(t, "queued command did not return")
	}
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
	setup := func(trailer string) *Client {
		c := &Client{
			scanner:  bufio.NewScanner(strings.NewReader("late response\n" + trailer + "\n")),
			response: make(chan response),
			notify:   make(chan Notification),
			done:     make(chan struct{}),
		}
		c.closeDone()
		return c
	}

	t.Run("success-response", func(t *testing.T) {
		c := setup(errOK)
		requireLateResponseDiscarded(t, c)
	})

	t.Run("error-response", func(t *testing.T) {
		c := setup(errUnknownCmd)
		requireLateResponseDiscarded(t, c)
	})
}

func requireLateResponseDiscarded(t *testing.T, c *Client) {
	t.Helper()
	c.wg.Add(1)
	go c.messageHandler()

	select {
	case _, ok := <-c.notify:
		require.False(t, ok)
	case <-time.After(time.Second):
		require.FailNow(t, "message handler blocked delivering a response after disconnect")
	}
	c.wg.Wait()
}

func TestClientResponseTimeoutDisconnects(t *testing.T) {
	setup := func(t *testing.T, serverOptions []serverOption, options ...func(*Client) error) (*Client, *server) {
		t.Helper()
		s := newServer(t, serverOptions...)
		c, err := NewClient(s.Addr, options...)
		if err != nil {
			require.NoError(t, s.Close())
		}
		require.NoError(t, err)
		return c, s
	}

	t.Run("tcp", func(t *testing.T) {
		c, s := setup(t, nil, Timeout(200*time.Millisecond))
		defer func() { require.NoError(t, s.Close()) }()
		defer func() { require.ErrorIs(t, c.Close(), ErrNotConnected) }()
		requireResponseTimeoutDisconnect(t, c)
	})

	t.Run("ssh", func(t *testing.T) {
		c, s := setup(t, []serverOption{useSSH()}, Timeout(200*time.Millisecond), SSH(sshClientTestConfig))
		defer func() { require.NoError(t, s.Close()) }()
		defer func() { require.ErrorIs(t, c.Close(), ErrNotConnected) }()
		requireResponseTimeoutDisconnect(t, c)
	})
}

func requireResponseTimeoutDisconnect(t *testing.T, c *Client) {
	t.Helper()
	_, err := c.Exec(" ")
	require.ErrorIs(t, err, ErrTimeout)
	require.False(t, c.IsConnected())
	_, err = c.Exec("version")
	require.ErrorIs(t, err, ErrNotConnected)
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
