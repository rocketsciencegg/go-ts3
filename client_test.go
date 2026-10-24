package ts3

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	s := newServer(t)
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		assert.Error(t, c.Close())
	}()

	_, err = c.Exec("version")
	assert.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("version"))
	assert.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("invalid"))
	assert.Error(t, err)

	_, err = c.ExecCmd(NewCmd("disconnect"))
	assert.Error(t, err)
}

func TestClientSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second), SSH(sshClientTestConfig))
	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		assert.Error(t, c.Close())
	}()

	_, err = c.Exec("version")
	assert.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("version"))
	assert.NoError(t, err)

	_, err = c.ExecCmd(NewCmd("invalid"))
	assert.Error(t, err)

	_, err = c.ExecCmd(NewCmd("disconnect"))
	assert.Error(t, err)
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
	_, err := NewClient("", func(c *Client) error { return errBadOption })
	if !assert.Error(t, err) {
		return
	}

	assert.Equal(t, errBadOption, err)
}

func TestClientDisconnect(t *testing.T) {
	s := newServer(t)
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	if !assert.NoError(t, err) {
		return
	}

	assert.NoError(t, c.Close())

	_, err = c.Exec("version")
	assert.Error(t, err)
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
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second))
	if !assert.NoError(t, err) {
		return
	}
	assert.NoError(t, c.conn.(*legacyConnection).Conn.(*net.TCPConn).CloseWrite())

	_, err = c.Exec("version")
	assert.Error(t, err)
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
			var serverOptions []serverOption
			options := []func(*Client) error{Timeout(time.Second)}
			if useSSHConnection {
				serverOptions = append(serverOptions, useSSH())
				options = append(options, SSH(sshClientTestConfig))
			}
			s := newServer(t, serverOptions...)
			defer func() { assert.NoError(t, s.Close()) }()
			c, err := NewClient(s.Addr, options...)
			require.NoError(t, err)
			defer func() { assert.NoError(t, c.Close()) }()

			const workers, iterations = 8, 25
			results := make(chan error, workers)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for worker := 0; worker < workers; worker++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					<-start
					for i := 0; i < iterations; i++ {
						cmd := []string{"version", "servergrouplist", "login", "invalid"}[(worker+i)%4]
						lines, err := c.Exec(cmd)
						if cmd == "invalid" {
							var serverErr *Error
							if !errors.As(err, &serverErr) || serverErr.ID != 256 {
								results <- fmt.Errorf("%s: unexpected error %v", cmd, err)
								return
							}
						} else if err != nil {
							results <- fmt.Errorf("%s: %w", cmd, err)
							return
						} else if expected := commands[cmd]; expected == "" {
							if len(lines) != 0 {
								results <- fmt.Errorf("%s: unexpected response %q", cmd, lines)
								return
							}
						} else if len(lines) != 1 || lines[0] != expected {
							results <- fmt.Errorf("%s: incorrect response %q", cmd, lines)
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
				t.Fatal("concurrent commands did not finish")
			}
			close(results)
			for err := range results {
				assert.NoError(t, err)
			}
		})
	}
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
		assert.ErrorIs(t, err, ErrTimeout)
		assert.True(t, c.IsConnected(), "an unsent command must not invalidate the connection")
	case <-time.After(time.Second):
		t.Error("command submission did not time out")
	}
}

func TestClientQueuedCommand(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(fmt.Sprintf("disconnect=%t", disconnect), func(t *testing.T) {
			c := &Client{
				timeout:     20 * time.Millisecond,
				work:        make(chan string),
				commandLock: make(chan struct{}, 1),
				done:        make(chan struct{}),
			}
			defer c.closeDone()
			c.commandLock <- struct{}{}
			if disconnect {
				c.closeDone()
			}

			result := make(chan error, 1)
			go func() {
				_, err := c.Exec("version")
				result <- err
			}()
			select {
			case err := <-result:
				if disconnect {
					assert.ErrorIs(t, err, ErrNotConnected)
				} else {
					assert.ErrorIs(t, err, ErrTimeout)
					assert.True(t, c.IsConnected())
				}
			case cmd := <-c.work:
				t.Errorf("queued command was sent before the preceding response: %q", cmd)
			case <-time.After(time.Second):
				t.Error("queued command did not return")
			}
			<-c.commandLock
		})
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
				assert.False(t, ok)
			case <-time.After(time.Second):
				t.Fatal("message handler blocked delivering a response after disconnect")
			}
			c.wg.Wait()
		})
	}
}

func TestClientResponseTimeoutDisconnects(t *testing.T) {
	for _, useSSHConnection := range []bool{false, true} {
		t.Run(fmt.Sprintf("ssh=%t", useSSHConnection), func(t *testing.T) {
			var serverOptions []serverOption
			options := []func(*Client) error{Timeout(50 * time.Millisecond)}
			if useSSHConnection {
				serverOptions = append(serverOptions, useSSH())
				options = append(options, SSH(sshClientTestConfig))
			}
			s := newServer(t, serverOptions...)
			defer func() { assert.NoError(t, s.Close()) }()
			c, err := NewClient(s.Addr, options...)
			require.NoError(t, err)
			defer func() { assert.ErrorIs(t, c.Close(), ErrNotConnected) }()

			_, err = c.Exec(" ")
			assert.ErrorIs(t, err, ErrTimeout)
			assert.False(t, c.IsConnected())
			_, err = c.Exec("version")
			assert.ErrorIs(t, err, ErrNotConnected)
		})
	}
}

func TestClientConcurrentDisconnect(t *testing.T) {
	s := newServer(t)
	defer func() { assert.NoError(t, s.Close()) }()
	c, err := NewClient(s.Addr, Timeout(time.Second))
	require.NoError(t, err)
	defer func() { assert.ErrorIs(t, c.Close(), ErrNotConnected) }()

	const workers = 8
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
			assert.Error(t, err)
			assert.NotErrorIs(t, err, ErrTimeout)
		case <-time.After(2 * time.Second):
			t.Fatal("command blocked after disconnect")
		}
	}
	assert.False(t, c.IsConnected())
}
