package ts3

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// MaxParseTokenSize is the maximum buffer size used to parse the
	// server responses.
	// It's relatively large to enable us to deal with the typical responses
	// to commands such as serversnapshotcreate.
	MaxParseTokenSize = 10 << 20

	// DefaultConnectHeader send by server on connect.
	DefaultConnectHeader = "TS3"

	// startBufSize is the initial size of allocation for the parse buffer.
	startBufSize = 4096

	// responseErrTimeout is the timeout used for sending response errors.
	responseErrTimeout = time.Millisecond * 100
)

var (
	// respTrailerRe is the regexp which matches a server response to a command.
	respTrailerRe = regexp.MustCompile(`^error id=(\d+) msg=([^ ]+)(.*)`)

	// keepAliveData is data which will be ignored by the server used to ensure
	// the connection is kept alive.
	keepAliveData = []byte(" \n")

	// DefaultTimeout is the default read / write / dial timeout for Clients.
	DefaultTimeout = 10 * time.Second

	// DefaultKeepAlive is the default interval in which keepalive data is sent.
	DefaultKeepAlive = 200 * time.Second

	// DefaultNotifyBufSize is the default notification buffer size.
	DefaultNotifyBufSize = 5
)

// Connection is a connection to a TeamSpeak 3 server.
// It's a wrapper around net.Conn with a Connect method.
type Connection interface {
	net.Conn

	// Connect connects to the server on addr with a timeout.
	Connect(addr string, timeout time.Duration) error
}

type response struct {
	err   error
	lines []string
}

// Client is a TeamSpeak 3 ServerQuery client.
type Client struct {
	conn          Connection
	timeout       time.Duration
	keepAlive     time.Duration
	scanner       *bufio.Scanner
	buf           []byte
	maxBufSize    int
	notifyBufSize int
	work          chan string
	response      chan response
	notify        chan Notification
	closing       chan struct{} // closing is closed to indicate we're closing our connection.
	done          chan struct{} // done is closed once we're seen a fatal error.
	doneOnce      sync.Once
	connectHeader string
	wg            sync.WaitGroup

	Server *ServerMethods
}

// Timeout sets read / write / dial timeout for a TeamSpeak 3 Client.
func Timeout(timeout time.Duration) func(*Client) error {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

// KeepAlive sets the keepAlive interval.
func KeepAlive(keepAlive time.Duration) func(*Client) error {
	return func(c *Client) error {
		c.keepAlive = keepAlive
		return nil
	}
}

// NotificationBuffer sets the notification buffer size.
func NotificationBuffer(size int) func(*Client) error {
	return func(c *Client) error {
		c.notifyBufSize = size
		return nil
	}
}

// Buffer sets the initial buffer used to parse responses from
// the server and the maximum size of buffer that may be allocated.
// The maximum parsable token size is the larger of max and cap(buf).
// If max <= cap(buf), scanning will use this buffer only and do no
// allocation.
//
// By default, parsing uses an internal buffer and sets the maximum
// token size to MaxParseTokenSize.
func Buffer(buf []byte, max int) func(*Client) error {
	return func(c *Client) error {
		c.buf = buf
		c.maxBufSize = max
		return nil
	}
}

// ConnectHeader sets the header expected on connect.
//
// Default is "TS3" which is sent by server query. For client query
// use "TS3 Client".
func ConnectHeader(connectHeader string) func(*Client) error {
	return func(c *Client) error {
		c.connectHeader = connectHeader
		return nil
	}
}

// SSH tells the client to use SSH instead of insecure legacy TCP.
// A valid login has to be provided with ssh.ClientConfig.
//
// Example config (missing host-key validation):
//
//	&ssh.ClientConfig{
//		User: "serveradmin",
//		Auth: []ssh.AuthMethod{
//			ssh.Password("password"),
//		},
//	}
func SSH(config *ssh.ClientConfig) func(*Client) error {
	return func(c *Client) error {
		c.conn = &sshConnection{config: config}
		return nil
	}
}

// NewClient returns a new TeamSpeak 3 client connected to addr.
// Use with SSH where possible for improved security.
func NewClient(addr string, options ...func(c *Client) error) (*Client, error) {
	c := &Client{
		conn:          new(legacyConnection),
		timeout:       DefaultTimeout,
		keepAlive:     DefaultKeepAlive,
		buf:           make([]byte, startBufSize),
		maxBufSize:    MaxParseTokenSize,
		notifyBufSize: DefaultNotifyBufSize,
		work:          make(chan string),
		response:      make(chan response),
		closing:       make(chan struct{}),
		done:          make(chan struct{}),
		connectHeader: DefaultConnectHeader,
	}
	for _, f := range options {
		if f == nil {
			return nil, ErrNilOption
		}
		if err := f(c); err != nil {
			return nil, err
		}
	}

	c.notify = make(chan Notification, c.notifyBufSize)

	// Wire up command groups
	c.Server = &ServerMethods{Client: c}

	if err := c.conn.Connect(addr, c.timeout); err != nil {
		return nil, fmt.Errorf("client: connect: %w", err)
	}

	c.scanner = bufio.NewScanner(bufio.NewReader(c.conn))
	c.scanner.Buffer(c.buf, c.maxBufSize)
	c.scanner.Split(ScanLines)

	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("client: set deadline: %w", err)
	}

	// Read the connection header
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("client: header: %w", c.scanErr())
	}

	if l := c.scanner.Text(); l != c.connectHeader {
		return nil, fmt.Errorf("client: invalid connection header %q", l)
	}

	// Slurp the banner
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("client: banner: %w", c.scanErr())
	}

	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("client: set read deadline: %w", err)
	}

	// Start handlers
	c.wg.Add(2)
	go c.messageHandler()
	go c.workHandler()

	return c, nil
}

// fatalError returns false if err is nil otherwise it ensures
// that done is closed and returns true.
func (c *Client) fatalError(err error) bool {
	if err == nil {
		return false
	}

	c.closeDone()
	return true
}

// closeDone safely closes c.done.
func (c *Client) closeDone() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

// messageHandler scans incoming lines and handles them accordingly.
// - Notifications are sent to c.notify.
// - ExecCmd responses are sent to c.response.
// If a fatal error occurs it stops processing and exits.
func (c *Client) messageHandler() {
	defer func() {
		close(c.notify)
		c.wg.Done()
	}()

	buf := make([]string, 0, 10)
	for {
		if c.scanner.Scan() {
			line := c.scanner.Text()
			if line == "error id=0 msg=ok" {
				var resp response
				// Avoid creating a new buf if there was no data in the response.
				if len(buf) > 0 {
					resp.lines = buf
					buf = make([]string, 0, 10)
				}
				c.response <- resp
			} else if matches := respTrailerRe.FindStringSubmatch(line); len(matches) == 4 {
				c.response <- response{err: NewError(matches)}
				// Avoid creating a new buf if there was no data in the response.
				if len(buf) > 0 {
					buf = make([]string, 0, 10)
				}
			} else if strings.Index(line, "notify") == 0 {
				if n, err := decodeNotification(line); err == nil {
					// non-blocking write
					select {
					case c.notify <- n:
					default:
					}
				}
			} else {
				// Partial response.
				buf = append(buf, line)
			}
		} else {
			if err := c.scanErr(); c.fatalError(err) {
				c.responseErr(err)
			} else {
				// Ensure that done is closed as scanner has seen an io.EOF.
				c.closeDone()
			}
			return
		}
	}
}

// responseErr sends err to c.response with a timeout to ensure it
// doesn't block forever when multiple errors occur during the
// processing of a single ExecCmd call.
func (c *Client) responseErr(err error) {
	t := time.NewTimer(responseErrTimeout)
	defer t.Stop()

	select {
	case c.response <- response{err: err}:
	case <-t.C:
	}
}

// workHandler handles commands and keepAlive messages.
func (c *Client) workHandler() {
	defer c.wg.Done()

	for {
		select {
		case w := <-c.work:
			if err := c.write([]byte(w)); c.fatalError(err) {
				// Command send failed, inform the caller.
				c.responseErr(err)
				return
			}
		case <-time.After(c.keepAlive):
			// Send a keep alive to prevent the connection from timing out.
			if err := c.write(keepAliveData); c.fatalError(err) {
				// We don't send to c.response as no ExecCmd is expecting a
				// response and the next caller will get an error.
				return
			}
		case <-c.done:
			return
		}
	}
}

// write writes data to the clients connection with the configured timeout
// returning any error.
func (c *Client) write(data []byte) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// Exec executes cmd on the server and returns the response.
func (c *Client) Exec(cmd string) ([]string, error) {
	return c.ExecCmd(NewCmd(cmd))
}

// ExecCmd executes cmd on the server and returns the response.
func (c *Client) ExecCmd(cmd *Cmd) ([]string, error) {
	select {
	case c.work <- cmd.String():
	case <-c.done:
		return nil, ErrNotConnected
	}

	var resp response
	select {
	case resp = <-c.response:
		if resp.err != nil {
			return nil, resp.err
		}
	case <-time.After(c.timeout):
		return nil, ErrTimeout
	}

	if cmd.response != nil {
		if err := DecodeResponse(resp.lines, cmd.response); err != nil {
			return nil, err
		}
	}

	return resp.lines, nil
}

// IsConnected returns true if the client is connected,
// false otherwise.
func (c *Client) IsConnected() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// Close closes the connection to the server.
func (c *Client) Close() error {
	defer c.wg.Wait()

	// Signal we're expecting EOF.
	close(c.closing)
	_, err := c.Exec("quit")
	err2 := c.conn.Close()

	if err != nil {
		return err
	} else if err2 != nil && !strings.HasSuffix(err2.Error(), "connection reset by peer") {
		return fmt.Errorf("client: close: %w", err2)
	}

	return nil
}

// scanError returns nil if c is closing else if the scanner returns a
// non-nil error it is returned, otherwise returns `io.ErrUnexpectedEOF`.
// Callers must have seen c.scanner.Scan() return false.
func (c *Client) scanErr() error {
	select {
	case <-c.closing:
		// We know we're closing the connection so ignore any errors
		// an return nil. This prevents spurious errors being returned
		// to the caller.
		return nil
	default:
		if err := c.scanner.Err(); err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		// As caller has seen c.scanner.Scan() return false
		// this must have been triggered by an unexpected EOF.
		return io.ErrUnexpectedEOF
	}
}
