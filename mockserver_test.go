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

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	cmdVersion    = "version"
	cmdClientList = "clientlist"
	statusOnline  = "online"
	testLine1     = "line1"
	cmdQuit       = "quit"
	banner        = `Welcome to the TeamSpeak 3 ServerQuery interface, type "help" for a list of commands and "help <command>" for information on a specific command.`

	errUnknownCmd = `error id=256 msg=command\snot\sfound`
	errOK         = `error id=0 msg=ok`

	// only used for testing.
	sshPrivateServerKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAaAAAABNlY2RzYS\n1zaGEyLW5pc3RwMjU2AAAACG5pc3RwMjU2AAAAQQRamQdnvjuFVMSN3wpq246IZxO9kS0y\n0f54xgj47XwyPUvhbpk27Ot6Z6CkqvLnj05pNQK6j7XJPkVoym16tiSLAAAAsOwJzensCc\n3pAAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFqZB2e+O4VUxI3f\nCmrbjohnE72RLTLR/njGCPjtfDI9S+FumTbs63pnoKSq8uePTmk1ArqPtck+RWjKbXq2JI\nsAAAAhAIVVOJZP3A2+tO26RnAXBAaD6aPpDfr1QgoeFz2Rd7E2AAAAFmZlcmRpbmFuZEBG\nZXJkaW5hbmQtUEMB\n-----END OPENSSH PRIVATE KEY-----"
)

var commands = map[string]string{
	cmdVersion:                    "version=3.0.12.2 build=1455547898 platform=FreeBSD",
	"login":                       "",
	"logout":                      "",
	"use":                         "",
	"serverlist":                  `virtualserver_id=1 virtualserver_port=10677 virtualserver_status=online virtualserver_clientsonline=1 virtualserver_queryclientsonline=1 virtualserver_maxclients=35 virtualserver_uptime=12345025 virtualserver_name=Server\s#1 virtualserver_autostart=1 virtualserver_machine_id=1 virtualserver_unique_identifier=uniq1|virtualserver_id=2 virtualserver_port=10617 virtualserver_status=online virtualserver_clientsonline=3 virtualserver_queryclientsonline=2 virtualserver_maxclients=10 virtualserver_uptime=3165117 virtualserver_name=Server\s#2 virtualserver_autostart=1 virtualserver_machine_id=1 virtualserver_unique_identifier=uniq2`,
	"serverinfo":                  `virtualserver_antiflood_points_needed_command_block=150 virtualserver_antiflood_points_needed_ip_block=250 virtualserver_antiflood_points_tick_reduce=5 virtualserver_channel_temp_delete_delay_default=0 virtualserver_codec_encryption_mode=0 virtualserver_complain_autoban_count=5 virtualserver_complain_autoban_time=1200 virtualserver_complain_remove_time=3600 virtualserver_created=0 virtualserver_default_channel_admin_group=1 virtualserver_default_channel_group=4 virtualserver_default_server_group=5 virtualserver_download_quota=18446744073709551615 virtualserver_filebase=files virtualserver_flag_password=0 virtualserver_hostbanner_gfx_interval=0 virtualserver_hostbanner_gfx_url virtualserver_hostbanner_mode=0 virtualserver_hostbanner_url virtualserver_hostbutton_gfx_url virtualserver_hostbutton_tooltip=Multiplay\sGame\sServers virtualserver_hostbutton_url=http:\/\/www.multiplaygameservers.com virtualserver_hostmessage virtualserver_hostmessage_mode=0 virtualserver_icon_id=0 virtualserver_log_channel=0 virtualserver_log_client=0 virtualserver_log_filetransfer=0 virtualserver_log_permissions=1 virtualserver_log_query=0 virtualserver_log_server=0 virtualserver_max_download_total_bandwidth=18446744073709551615 virtualserver_max_upload_total_bandwidth=18446744073709551615 virtualserver_maxclients=32 virtualserver_min_android_version=0 virtualserver_min_client_version=0 virtualserver_min_clients_in_channel_before_forced_silence=100 virtualserver_min_ios_version=0 virtualserver_name=Test\sServer virtualserver_name_phonetic virtualserver_needed_identity_security_level=8 virtualserver_password virtualserver_priority_speaker_dimm_modificator=-18.0000 virtualserver_reserved_slots=0 virtualserver_status=template virtualserver_unique_identifier virtualserver_upload_quota=18446744073709551615 virtualserver_weblist_enabled=1 virtualserver_welcomemessage=Welcome\sto\sTeamSpeak,\scheck\s[URL]www.teamspeak.com[\/URL]\sfor\slatest\sinfos.`,
	"servercreate":                `sid=2 virtualserver_port=9988 token=eKnFZQ9EK7G7MhtuQB6+N2B1PNZZ6OZL3ycDp2OW`,
	"serveridgetbyport":           `server_id=1`,
	"servergrouplist":             `sgid=1 name=Guest\sServer\sQuery type=2 iconid=0 savedb=0 sortid=0 namemode=0 n_modifyp=0 n_member_addp=0 n_member_removep=0|sgid=2 name=Admin\sServer\sQuery type=2 iconid=500 savedb=1 sortid=0 namemode=0 n_modifyp=100 n_member_addp=100 n_member_removep=100`,
	"privilegekeylist":            `token=zTfamFVhiMEzhTl49KrOVYaMilHPDQEBQOJFh6qX token_type=0 token_id1=17395 token_id2=0 token_created=1499948005 token_description`,
	"privilegekeyadd":             `token=zTfamFVhiMEzhTl49KrOVYaMilHPgQEBQOJFh6qX`,
	"serverdelete":                "",
	"serverstop":                  "",
	"serverstart":                 "",
	"serveredit":                  "",
	"instanceinfo":                "serverinstance_database_version=26 serverinstance_filetransfer_port=30033 serverinstance_max_download_total_bandwidth=18446744073709551615 serverinstance_max_upload_total_bandwidth=18446744073709551615 serverinstance_guest_serverquery_group=1 serverinstance_serverquery_flood_commands=50 serverinstance_serverquery_flood_time=3 serverinstance_serverquery_ban_time=600 serverinstance_template_serveradmin_group=3 serverinstance_template_serverdefault_group=5 serverinstance_template_channeladmin_group=1 serverinstance_template_channeldefault_group=4 serverinstance_permissions_version=19 serverinstance_pending_connections_per_ip=0",
	"serverrequestconnectioninfo": "connection_filetransfer_bandwidth_sent=0 connection_filetransfer_bandwidth_received=0 connection_filetransfer_bytes_sent_total=617 connection_filetransfer_bytes_received_total=0 connection_packets_sent_total=926413 connection_bytes_sent_total=92911395 connection_packets_received_total=650335 connection_bytes_received_total=61940731 connection_bandwidth_sent_last_second_total=0 connection_bandwidth_sent_last_minute_total=0 connection_bandwidth_received_last_second_total=0 connection_bandwidth_received_last_minute_total=0 connection_connected_time=49408 connection_packetloss_total=0.0000 connection_ping=0.0000 connection_packets_sent_speech=320432180 connection_bytes_sent_speech=43805818511 connection_packets_received_speech=174885295 connection_bytes_received_speech=24127808273 connection_packets_sent_keepalive=55230363 connection_bytes_sent_keepalive=2264444883 connection_packets_received_keepalive=55149547 connection_bytes_received_keepalive=2316390993 connection_packets_sent_control=2376088 connection_bytes_sent_control=525691022 connection_packets_received_control=2376138 connection_bytes_received_control=227044870",
	"channellist":                 "cid=499 pid=0 channel_order=0 channel_name=Default\\sChannel total_clients=1 channel_needed_subscribe_power=0",
	cmdClientList:                 `clid=42087 cid=39 client_database_id=19 client_nickname=bdeb1337 client_type=0 client_away=0 client_away_message`,
	"clientlist -uid -away -voice -times -groups -info -icon -country -ip -badges": `clid=42087 cid=39 client_database_id=19 client_nickname=bdeb1337 client_type=0 client_away=1 client_away_message=afk client_flag_talking=0 client_input_muted=0 client_output_muted=0 client_input_hardware=1 client_output_hardware=1 client_talk_power=75 client_is_talker=0 client_is_priority_speaker=0 client_is_recording=0 client_is_channel_commander=0 client_unique_identifier=DZhdQU58qyooEK4Fr8Ly738hEmc= client_servergroups=6,8 client_channel_group_id=8 client_channel_group_inherited_channel_id=39 client_version=3.6.1\s[Build:\s1690193193] client_platform=OS\sX client_idle_time=1280228 client_created=1661793049 client_lastconnected=1691527133 client_icon_id=0 client_country=BE connection_client_ip=1.3.3.7 client_badges`,
	"clientdblist": "cldbid=7 client_unique_identifier=DZhdQU58qyooEK4Fr8Ly738hEmc= client_nickname=MuhChy client_created=1259147468 client_lastconnected=1259421233",
	"whoami":       "virtualserver_status=online virtualserver_id=18 virtualserver_unique_identifier=gNITtWtKs9+Uh3L4LKv8\\/YHsn5c= virtualserver_port=9987 client_id=94 client_channel_id=432 client_nickname=serveradmin\\sfrom\\s127.0.0.1:49725 client_database_id=1 client_login_name=serveradmin client_unique_identifier=serveradmin client_origin_server_id=0",
	cmdQuit:        "",
}

// newLockListener creates a new listener on the local IP.
func newLocalListener() (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			return nil, fmt.Errorf("local listener: listen: %w", err)
		}
	}
	return l, nil
}

// server is a mock TeamSpeak 3 server.
type server struct {
	Addr     string
	Listener net.Listener

	wg        sync.WaitGroup
	noHeader  bool
	noBanner  bool
	failConn  bool
	badHeader bool
	useSSH    bool
	responses map[string]string

	// Below here is protected by mtx.
	mtx    sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
	err    error
}

// sconn represents a server connection.
type sconn struct {
	net.Conn
}

type serverOption func(s *server)

func useSSH() serverOption {
	return func(s *server) {
		s.useSSH = true
	}
}

func noHeader() serverOption {
	return func(s *server) {
		s.noHeader = true
	}
}

func noBanner() serverOption {
	return func(s *server) {
		s.noBanner = true
	}
}

func failConn() serverOption {
	return func(s *server) {
		s.failConn = true
	}
}

func badHeader() serverOption {
	return func(s *server) {
		s.badHeader = true
	}
}

// newServer returns a running server. It fails the test immediately if an error occurred.
func newServer(t *testing.T, options ...serverOption) *server {
	t.Helper()
	l, err := newLocalListener()
	require.NoError(t, err)

	s := &server{
		Listener: l,
		conns:    make(map[net.Conn]struct{}),
	}
	for _, f := range options {
		f(s)
	}
	s.Addr = s.Listener.Addr().String()
	s.Start()

	return s
}

func (s *server) handleError(err error) bool {
	if err == nil {
		return false
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if !s.closed {
		s.err = err
	}

	return true
}

// Start starts the server.
func (s *server) Start() {
	s.wg.Add(1)
	go s.serve()
}

// singleClose ensures that a connection is only closed once
// to avoid spurious errors.
type singleClose struct {
	net.Conn

	once sync.Once
	err  error
}

func (c *singleClose) close() {
	c.err = c.Conn.Close()
}

func (c *singleClose) Close() error {
	c.once.Do(c.close)
	return c.err
}

// server processes incoming requests until signaled to stop with Close.
func (s *server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.Listener.Accept()
		if s.handleError(err) {
			return
		}

		if s.useSSH {
			conn, err = newSSHServerShell(&singleClose{Conn: conn})
			if s.handleError(err) {
				return
			}
		}
		s.wg.Add(1)
		go s.handle(conn)
	}
}

// writeResponse writes the given msg followed by an error (ok) response.
// If msg is empty the only the error (ok) rsponse is sent.
func (s *server) writeResponse(c *sconn, msg string) error {
	if msg != "" {
		if err := s.write(c.Conn, msg); err != nil {
			return err
		}
	}

	return s.write(c.Conn, errOK)
}

// write writes msg to w.
func (s *server) write(w io.Writer, msg string) error {
	if _, err := w.Write([]byte(msg + "\n\r")); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// handle handles a client connection.
func (s *server) handle(conn net.Conn) {
	s.mtx.Lock()
	s.conns[conn] = struct{}{}
	s.mtx.Unlock()

	defer func() {
		s.closeConn(conn)
		s.wg.Done()
	}()

	if s.failConn {
		return
	}

	sc := bufio.NewScanner(bufio.NewReader(conn))
	sc.Split(bufio.ScanLines)

	if !s.noHeader {
		if s.badHeader {
			if s.handleError(s.write(conn, "bad")) {
				return
			}
		} else {
			if s.handleError(s.write(conn, DefaultConnectHeader)) {
				return
			}
		}

		if !s.noBanner {
			if s.handleError(s.write(conn, banner)) {
				return
			}
		}
	}

	c := &sconn{Conn: conn}
	for sc.Scan() {
		l := sc.Text()
		parts := strings.Split(l, " ")
		cmd := strings.TrimSpace(parts[0])
		// Support server commands with specific optional parameters,
		// they can be bypassed from the usual parameter trimming here.
		if cmd == cmdClientList || cmd == "clientinfo" {
			cmd = l
		}
		resp, ok := commands[cmd]
		if response, found := s.responses[cmd]; found {
			resp, ok = response, true
		}
		var err error
		switch {
		case ok:
			// Request has response, send it.
			err = s.writeResponse(c, resp)
		case cmd == "disconnect":
			return
		case cmd != "":
			err = s.write(c, errUnknownCmd)
		}

		if s.handleError(err) {
			return
		}

		if cmd == cmdQuit {
			return
		}
	}

	s.handleError(sc.Err())
}

// closeConn closes a client connection and removes it from our map of connections.
func (s *server) closeConn(conn net.Conn) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) && !s.closed {
		s.err = err
	}
	delete(s.conns, conn)
}

// Close cleanly shuts down the server.
func (s *server) Close() error {
	s.mtx.Lock()
	s.closed = true
	err := s.Listener.Close()
	for c := range s.conns {
		if err2 := c.Close(); err2 != nil && err == nil {
			err = err2
		}
	}
	s.mtx.Unlock()
	s.wg.Wait()

	if err != nil {
		return err
	}

	return s.err
}

// sshServerShell provides an ssh server shell session.
type sshServerShell struct {
	net.Conn
	cond *sync.Cond

	// Everything below is protected by mtx.
	mtx        sync.RWMutex
	sshChannel ssh.Channel
	closed     bool
}

// newSSHServerShell creates a new sshServerShell from a net.Conn.
func newSSHServerShell(conn net.Conn) (*sshServerShell, error) {
	private, err := ssh.ParsePrivateKey([]byte(sshPrivateServerKey))
	if err != nil {
		return nil, fmt.Errorf("mock ssh shell: parse private key: %w", err)
	}

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(private)

	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return nil, fmt.Errorf("mock ssh shell: new server conn: %w", err)
	}
	go ssh.DiscardRequests(reqs)

	m := new(sync.Mutex)
	m.Lock()

	c := &sshServerShell{
		Conn: conn,
		cond: sync.NewCond(m),
	}

	go func() {
		newChan := <-chans
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, ssh.UnknownChannelType.String())
		}

		sChan, reqs, _ := newChan.Accept()
		go func(in <-chan *ssh.Request) {
			for req := range in {
				_ = req.Reply(req.Type == "shell", nil)
			}
		}(reqs)

		c.mtx.Lock()
		c.sshChannel = sChan
		c.mtx.Unlock()
		c.cond.Broadcast()
	}()

	return c, nil
}

func (c *sshServerShell) channel() (ssh.Channel, bool) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	return c.sshChannel, c.closed
}

func (c *sshServerShell) waitChannel() (ssh.Channel, error) {
	for {
		ch, closed := c.channel()
		if closed {
			return nil, net.ErrClosed
		}
		if ch != nil {
			return c.sshChannel, nil
		}
		c.cond.Wait()
	}
}

// Read reads from the ssh channel.
func (c *sshServerShell) Read(b []byte) (int, error) {
	ch, err := c.waitChannel()
	if err != nil {
		return 0, err
	}

	return ch.Read(b) //nolint: wrapcheck
}

// Write writes to the ssh channel.
func (c *sshServerShell) Write(b []byte) (int, error) {
	ch, err := c.waitChannel()
	if err != nil {
		return 0, err
	}

	return ch.Write(b) //nolint: wrapcheck
}

// Close closes the ssh channel and connection.
func (c *sshServerShell) Close() error {
	c.mtx.Lock()
	c.closed = true
	c.mtx.Unlock()
	c.cond.Broadcast()

	return c.Conn.Close() //nolint: wrapcheck
}
