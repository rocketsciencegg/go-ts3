package ts3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdsServer(t *testing.T) {
	s := newServer(t)
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second*2))
	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		assert.NoError(t, c.Close())
	}()

	testCmdsServer(t, c)
}

func TestCmdsServerSSH(t *testing.T) {
	s := newServer(t, useSSH())
	defer func() {
		assert.NoError(t, s.Close())
	}()

	c, err := NewClient(s.Addr, Timeout(time.Second*2), SSH(sshClientTestConfig))
	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		assert.NoError(t, c.Close())
	}()

	testCmdsServer(t, c)
}

func TestServerMethods_PrivilegeKeyAdd(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		s := newServer(t)
		defer func() {
			require.NoError(t, s.Close())
		}()

		testPrivilegeKeyAdd(t, s.Addr)
	})

	t.Run("ssh", func(t *testing.T) {
		s := newServer(t, useSSH())
		defer func() {
			require.NoError(t, s.Close())
		}()

		testPrivilegeKeyAdd(t, s.Addr, SSH(sshClientTestConfig))
	})
}

func testPrivilegeKeyAdd(t *testing.T, addr string, options ...func(*Client) error) {
	t.Helper()
	setup := func(t *testing.T) *Client {
		t.Helper()
		clientOptions := append([]func(*Client) error{Timeout(time.Second * 2)}, options...)
		c, err := NewClient(addr, clientOptions...)
		require.NoError(t, err)
		return c
	}

	t.Run("returns-created-token", func(t *testing.T) {
		c := setup(t)
		defer func() {
			require.NoError(t, c.Close())
		}()

		token, err := c.Server.PrivilegeKeyAdd(0, 17395, 0)
		require.NoError(t, err)
		require.Equal(t, "zTfamFVhiMEzhTl49KrOVYaMilHPgQEBQOJFh6qX", token)
	})

	t.Run("returns-not-connected", func(t *testing.T) {
		c := setup(t)
		err := c.Close()
		require.NoError(t, err)

		token, err := c.Server.PrivilegeKeyAdd(0, 17395, 0)
		require.ErrorIs(t, err, ErrNotConnected)
		require.Empty(t, token)
	})
}

func testCmdsServer(t *testing.T, c *Client) {
	t.Helper()
	list := func(t *testing.T) {
		t.Helper()
		servers, err := c.Server.List()
		if !assert.NoError(t, err) {
			return
		}
		expected := []*Server{
			{
				ID:                 1,
				Port:               10677,
				Status:             statusOnline,
				ClientsOnline:      1,
				QueryClientsOnline: 1,
				MaxClients:         35,
				Uptime:             12345025,
				Name:               "Server #1",
				AutoStart:          true,
				MachineID:          "1",
				UniqueIdentifier:   "uniq1",
			},
			{
				ID:                 2,
				Port:               10617,
				Status:             statusOnline,
				ClientsOnline:      3,
				QueryClientsOnline: 2,
				MaxClients:         10,
				Uptime:             3165117,
				Name:               "Server #2",
				AutoStart:          true,
				MachineID:          "1",
				UniqueIdentifier:   "uniq2",
			},
		}
		assert.Equal(t, expected, servers)
	}

	idgetbyport := func(t *testing.T) {
		t.Helper()
		id, err := c.Server.IDGetByPort(1234)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, 1, id)
	}

	info := func(t *testing.T) {
		t.Helper()
		s, err := c.Server.Info()
		if !assert.NoError(t, err) {
			return
		}
		expected := &Server{
			Status:                                 "template",
			MaxClients:                             32,
			Name:                                   "Test Server",
			AntiFloodPointsNeededCommandBlock:      150,
			AntiFloodPointsNeededIPBlock:           250,
			AntiFloodPointsTickReduce:              5,
			ComplainAutoBanCount:                   5,
			ComplainAutoBanTime:                    1200,
			ComplainRemoveTime:                     3600,
			DefaultChannelAdminGroup:               1,
			DefaultChannelGroup:                    4,
			DefaultServerGroup:                     5,
			MinClientsInChannelBeforeForcedSilence: 100,
			NeededIdentitySecurityLevel:            8,
			LogPermissions:                         true,
			PrioritySpeakerDimmModificator:         -18,
			MaxDownloadTotalBandwidth:              18446744073709551615,
			MaxUploadTotalBandwidth:                18446744073709551615,
			FileBase:                               "files",
			HostButtonToolTip:                      "Multiplay Game Servers",
			HostButtonURL:                          "http://www.multiplaygameservers.com",
			WelcomeMessage:                         "Welcome to TeamSpeak, check [URL]www.teamspeak.com[/URL] for latest infos.",
			VirtualServerDownloadQuota:             18446744073709551615,
			VirtualServerUploadQuota:               18446744073709551615,
		}
		assert.Equal(t, expected, s)
	}

	create := func(t *testing.T) {
		t.Helper()
		s, err := c.Server.Create("my server")
		if !assert.NoError(t, err) {
			return
		}
		expected := &CreatedServer{ //nolint:gosec // Public mock-server response, not a real credential.
			ID:    2,
			Port:  9988,
			Token: "eKnFZQ9EK7G7MhtuQB6+N2B1PNZZ6OZL3ycDp2OW",
		}
		assert.Equal(t, expected, s)
	}

	edit := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.Server.Edit(NewArg("virtualserver_maxclients", 10)))
	}

	del := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.Server.Delete(1))
	}

	start := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.Server.Start(1))
	}

	stop := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.Server.Stop(1))
	}

	grouplist := func(t *testing.T) {
		t.Helper()
		groups, err := c.Server.GroupList()
		if !assert.NoError(t, err) {
			return
		}

		expected := []*Group{
			{
				ID:   1,
				Name: "Guest Server Query",
				Type: 2,
			},
			{
				ID:                2,
				Name:              "Admin Server Query",
				Type:              2,
				IconID:            500,
				Saved:             true,
				ModifyPower:       100,
				MemberAddPower:    100,
				MemberRemovePower: 100,
			},
		}
		assert.Equal(t, expected, groups)
	}

	privilegekeylist := func(t *testing.T) {
		t.Helper()
		keys, err := c.Server.PrivilegeKeyList()
		if !assert.NoError(t, err) {
			return
		}

		expected := []*PrivilegeKey{
			{ //nolint:gosec // Public mock-server response, not a real credential.
				Token:   "zTfamFVhiMEzhTl49KrOVYaMilHPDQEBQOJFh6qX",
				ID1:     17395,
				Created: 1499948005,
			},
		}
		assert.Equal(t, expected, keys)
	}

	serverrequestconnectioninfo := func(t *testing.T) {
		t.Helper()
		ci, err := c.Server.ServerConnectionInfo()
		if !assert.NoError(t, err) {
			return
		}
		expected := &ServerConnectionInfo{
			FileTransferBandwidthSent:     0,
			FileTransferBandwidthReceived: 0,
			FileTransferTotalSent:         617,
			FileTransferTotalReceived:     0,
			PacketsSentTotal:              926413,
			PacketsReceivedTotal:          650335,
			BytesSentTotal:                92911395,
			BytesReceivedTotal:            61940731,
			BandwidthSentLastSecond:       0,
			BandwidthReceivedLastSecond:   0,
			BandwidthSentLastMinute:       0,
			BandwidthReceivedLastMinute:   0,
			ConnectedTime:                 49408,
			PacketLossTotalAvg:            0.0,
			PingTotalAvg:                  0.0,
			PacketsSentSpeech:             320432180,
			PacketsReceivedSpeech:         174885295,
			BytesSentSpeech:               43805818511,
			BytesReceivedSpeech:           24127808273,
			PacketsSentKeepalive:          55230363,
			PacketsReceivedKeepalive:      55149547,
			BytesSentKeepalive:            2264444883,
			BytesReceivedKeepalive:        2316390993,
			PacketsSentControl:            2376088,
			PacketsReceivedControl:        2376138,
			BytesSentControl:              525691022,
			BytesReceivedControl:          227044870,
		}
		assert.Equal(t, expected, ci)
	}

	instanceinfo := func(t *testing.T) {
		t.Helper()
		ii, err := c.Server.InstanceInfo()
		if !assert.NoError(t, err) {
			return
		}
		expected := &Instance{
			DatabaseVersion:             26,
			FileTransferPort:            30033,
			MaxTotalDownloadBandwidth:   18446744073709551615,
			MaxTotalUploadBandwidth:     18446744073709551615,
			GuestServerQueryGroup:       1,
			ServerQueryFloodCommands:    50,
			ServerQueryFloodTime:        3,
			ServerQueryBanTime:          600,
			TemplateServerAdminGroup:    3,
			TemplateServerDefaultGroup:  5,
			TemplateChannelAdminGroup:   1,
			TemplateChannelDefaultGroup: 4,
			PermissionsVersion:          19,
			PendingConnectionsPerIP:     0,
		}
		assert.Equal(t, expected, ii)
	}

	channellist := func(t *testing.T) {
		t.Helper()
		channels, err := c.Server.ChannelList()
		if !assert.NoError(t, err) {
			return
		}

		expected := []*Channel{
			{
				ID:                   499,
				ParentID:             0,
				ChannelOrder:         0,
				ChannelName:          "Default Channel",
				TotalClients:         1,
				NeededSubscribePower: 0,
			},
		}

		assert.Equal(t, expected, channels)
	}

	clientlist := func(t *testing.T) {
		t.Helper()
		clients, err := c.Server.ClientList()
		if !assert.NoError(t, err) {
			return
		}

		expected := []*OnlineClient{
			{
				ID:         42087,
				ChannelID:  39,
				DatabaseID: 19,
				Nickname:   onlineClientTestNickname,
				Type:       0,
			},
		}

		assert.Equal(t, expected, clients)
	}

	clientlistextended := func(t *testing.T) {
		t.Helper()
		clientz, err := c.Server.ClientList(ClientListFull)
		if !assert.NoError(t, err) {
			return
		}

		// helper variables & functions for pointers
		falseP := false
		trueP := true
		stringptr := func(s string) *string {
			return &s
		}
		intptr := func(i int) *int {
			return &i
		}

		expected := []*OnlineClient{
			{
				ID:          42087,
				ChannelID:   39,
				DatabaseID:  19,
				Nickname:    onlineClientTestNickname,
				Type:        0,
				Away:        true,
				AwayMessage: "afk",
				OnlineClientExt: &OnlineClientExt{
					UniqueIdentifier: stringptr("DZhdQU58qyooEK4Fr8Ly738hEmc="),
					OnlineClientVoice: &OnlineClientVoice{
						FlagTalking:        &falseP,
						InputMuted:         &falseP,
						OutputMuted:        &falseP,
						InputHardware:      &trueP,
						OutputHardware:     &trueP,
						TalkPower:          intptr(75),
						IsTalker:           &falseP,
						IsPrioritySpeaker:  &falseP,
						IsRecording:        &falseP,
						IsChannelCommander: &falseP,
					},
					OnlineClientTimes: &OnlineClientTimes{
						IdleTime:      intptr(1280228),
						Created:       intptr(1661793049),
						LastConnected: intptr(1691527133),
					},
					OnlineClientGroups: &OnlineClientGroups{
						ChannelGroupID:                 intptr(8),
						ChannelGroupInheritedChannelID: intptr(39),
						ServerGroups:                   &[]int{6, 8},
					},
					OnlineClientInfo: &OnlineClientInfo{
						Version:  stringptr("3.6.1 [Build: 1690193193]"),
						Platform: stringptr("OS X"),
					},
					IconID:  intptr(0),
					Country: stringptr("BE"),
					IP:      stringptr("1.3.3.7"),
					Badges:  stringptr(""),
				},
			},
		}

		assert.Equal(t, expected, clientz)
	}

	clientdblist := func(t *testing.T) {
		t.Helper()
		clients, err := c.Server.ClientDBList()
		if !assert.NoError(t, err) {
			return
		}

		expected := []*DBClient{
			{
				ID:               7,
				UniqueIdentifier: "DZhdQU58qyooEK4Fr8Ly738hEmc=",
				Nickname:         "MuhChy",
				Created:          time.Unix(1259147468, 0),
				LastConnected:    time.Unix(1259421233, 0),
			},
		}

		assert.Equal(t, expected, clients)
	}

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{"list", list},
		{"idgetbyport", idgetbyport},
		{"info", info},
		{"create", create},
		{"edit", edit},
		{"del", del},
		{"start", start},
		{"stop", stop},
		{"grouplist", grouplist},
		{"privilegekeylist", privilegekeylist},
		{"serverrequestconnectioninfo", serverrequestconnectioninfo},
		{"instanceinfo", instanceinfo},
		{"channellist", channellist},
		{cmdClientList, clientlist},
		{"clientlistextended", clientlistextended},
		{"clientdblist", clientdblist},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.f)
	}
}

func TestServerMethods_ClientList(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		testClientListOptions(t, false)
	})

	t.Run("ssh", func(t *testing.T) {
		testClientListOptions(t, true)
	})
}

func testClientListOptions(t *testing.T, useSSHTransport bool) {
	t.Helper()
	setup := func(t *testing.T) (*server, *Client, *OnlineClient) {
		t.Helper()
		var (
			serverOptions []serverOption
			clientOptions = []func(*Client) error{Timeout(time.Second * 2)}
		)

		if useSSHTransport {
			serverOptions = append(serverOptions, useSSH())
			clientOptions = append(clientOptions, SSH(sshClientTestConfig))
		}

		s := newServer(t, serverOptions...)
		c, err := NewClient(s.Addr, clientOptions...)
		if err != nil {
			require.NoError(t, s.Close())
		}
		require.NoError(t, err)

		return s, c, &OnlineClient{
			ID:         42087,
			ChannelID:  39,
			DatabaseID: 19,
			Nickname:   onlineClientTestNickname,
			Type:       0,
		}
	}

	t.Run("away", func(t *testing.T) {
		s, c, expected := setup(t)
		defer requireClientListClosed(t, s, c)
		expected.Away = true
		expected.AwayMessage = "back soon"
		requireClientListSuccess(t, c, expected, ClientAway)
	})

	t.Run("voice", func(t *testing.T) {
		s, c, expected := setup(t)
		defer requireClientListClosed(t, s, c)
		expected.OnlineClientExt = &OnlineClientExt{
			OnlineClientVoice: &OnlineClientVoice{
				InputMuted:  new(true),
				OutputMuted: new(false),
			},
		}
		requireClientListSuccess(t, c, expected, ClientVoice)
	})

	t.Run("empty-country", func(t *testing.T) {
		s, c, expected := setup(t)
		defer requireClientListClosed(t, s, c)
		expected.OnlineClientExt = &OnlineClientExt{Country: new("")}
		requireClientListSuccess(t, c, expected, ClientCountry)
	})

	t.Run("away-voice-country", func(t *testing.T) {
		s, c, expected := setup(t)
		defer requireClientListClosed(t, s, c)
		expected.Away = true
		expected.AwayMessage = "back soon"
		expected.OnlineClientExt = &OnlineClientExt{
			OnlineClientVoice: &OnlineClientVoice{
				InputMuted:  new(false),
				OutputMuted: new(true),
			},
			Country: new("BE"),
		}
		requireClientListSuccess(t, c, expected, ClientAway, ClientVoice, ClientCountry)
	})

	t.Run("unsupported-option", func(t *testing.T) {
		s, c, _ := setup(t)
		defer requireClientListClosed(t, s, c)
		var commandError *Error
		clients, err := c.Server.ClientList(ClientAway, ClientVoice, ClientCountry, "-unsupported")
		require.ErrorAs(t, err, &commandError)
		require.Nil(t, clients)
		require.Equal(t, &Error{ID: 256, Msg: "command not found"}, commandError)
	})
}

func requireClientListSuccess(t *testing.T, c *Client, expected *OnlineClient, options ...string) {
	t.Helper()
	clients, err := c.Server.ClientList(options...)
	require.NoError(t, err)
	require.Equal(t, []*OnlineClient{expected}, clients)
}

func requireClientListClosed(t *testing.T, s *server, c *Client) {
	t.Helper()
	defer func() {
		require.NoError(t, s.Close())
	}()
	require.NoError(t, c.Close())
}
