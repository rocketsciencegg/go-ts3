package ts3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Adapted from Henner25's clientinfo fixture in PR #16.
const (
	clientInfoResponse = `cid=20 client_idle_time=28122 client_unique_identifier=P5H2hrN6+gpQI4n\/dXp3p17vtY0= client_nickname=Rabe85 client_version=3.0.0-alpha24\s[Build:\s8785]\s(UI:\s8785) client_platform=Windows client_input_muted=0 client_output_muted=0 client_outputonly_muted=0 client_input_hardware=1 client_output_hardware=1 client_default_channel=\/20 client_meta_data client_is_recording=0 client_version_sign=+\/BWvaeokGg4YkO1v3ouZB5vtIIgUZ5bM5cRfxBstfnHUdro2ja+5b+3sFUzEy8\/vvEISXVD6U95blTb638MCQ== client_security_hash client_login_name client_database_id=8 client_channel_group_id=8 client_servergroups=6,10 client_created=1503431624 client_lastconnected=1530383977 client_totalconnections=138 client_away=0 client_away_message client_type=0 client_flag_avatar=dd213abf2a94396ece544b22c4e56821 client_talk_power=75 client_talk_request=0 client_talk_request_msg client_description client_is_talker=0 client_month_bytes_uploaded=0 client_month_bytes_downloaded=0 client_total_bytes_uploaded=0 client_total_bytes_downloaded=3014720 client_is_priority_speaker=1 client_nickname_phonetic=rabeh client_needed_serverquery_view_power=75 client_default_token client_icon_id=0 client_is_channel_commander=1 client_country=DE client_channel_group_inherited_channel_id=20 client_badges=overwolf=0 client_base64HashClientUID=kdohhblmninnfhaecihcijemaigdnkdhgjllefed connection_filetransfer_bandwidth_sent=0 connection_filetransfer_bandwidth_received=0 connection_packets_sent_total=46880 connection_bytes_sent_total=6426774 connection_packets_received_total=14098 connection_bytes_received_total=1644574 connection_bandwidth_sent_last_second_total=81 connection_bandwidth_sent_last_minute_total=92 connection_bandwidth_received_last_second_total=83 connection_bandwidth_received_last_minute_total=97 connection_connected_time=2084247 connection_client_ip=83.123.45.6`
)

func TestClientInfo(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		want          *DetailedOnlineClient
		wantErr       string
		wantServerErr bool
	}{
		{
			name:     "full response",
			response: clientInfoResponse,
			want: &DetailedOnlineClient{
				ID:                                     8,
				ChannelID:                              20,
				DatabaseID:                             8,
				Nickname:                               "Rabe85",
				IdleTime:                               28122 * time.Millisecond,
				ConnectionConnectedTime:                2084247 * time.Millisecond,
				UniqueIdentifier:                       "P5H2hrN6+gpQI4n/dXp3p17vtY0=",
				Version:                                "3.0.0-alpha24 [Build: 8785] (UI: 8785)",
				Platform:                               "Windows",
				InputHardware:                          true,
				OutputHardware:                         true,
				DefaultChannel:                         "/20",
				VersionSign:                            "+/BWvaeokGg4YkO1v3ouZB5vtIIgUZ5bM5cRfxBstfnHUdro2ja+5b+3sFUzEy8/vvEISXVD6U95blTb638MCQ==",
				ChannelGroupID:                         8,
				ServerGroups:                           []int{6, 10},
				Created:                                time.Unix(1503431624, 0),
				LastConnected:                          time.Unix(1530383977, 0),
				TotalConnections:                       138,
				FlagAvatar:                             "dd213abf2a94396ece544b22c4e56821",
				TalkPower:                              75,
				TotalBytesDownloaded:                   3014720,
				IsPrioritySpeaker:                      true,
				NicknamePhonetic:                       "rabeh",
				NeededServerQueryViewPower:             75,
				IsChannelCommander:                     true,
				Country:                                "DE",
				ChannelGroupInheritedChannelID:         20,
				Base64HashClientUID:                    "kdohhblmninnfhaecihcijemaigdnkdhgjllefed",
				ConnectionPacketsSentTotal:             46880,
				ConnectionBytesSentTotal:               6426774,
				ConnectionPacketsReceivedTotal:         14098,
				ConnectionBytesReceivedTotal:           1644574,
				ConnectionBandwidthSentLastSecondTotal: 81,
				ConnectionBandwidthSentLastMinuteTotal: 92,
				ConnectionBandwidthReceivedLastSecondTotal: 83,
				ConnectionBandwidthReceivedLastMinuteTotal: 97,
				ConnectionClientIP:                         "83.123.45.6",
			},
		},
		{
			name:     "nonzero counters and flags",
			response: `client_type=1 client_away=1 client_away_message=Away client_default_token=token client_description=Description client_login_name=admin client_security_hash=hash client_talk_request_msg=Please client_input_muted=1 client_output_muted=1 client_outputonly_muted=1 client_is_recording=1 client_is_talker=1 client_talk_request=1700000000 client_month_bytes_downloaded=11 client_month_bytes_uploaded=12 client_total_bytes_downloaded=13 client_total_bytes_uploaded=14 connection_filetransfer_bandwidth_sent=15 connection_filetransfer_bandwidth_received=16 client_icon_id=-1294967296`,
			want: &DetailedOnlineClient{
				ID: 8, Type: 1, Away: true, AwayMessage: "Away", DefaultToken: "token",
				Description: "Description", LoginName: "admin", SecurityHash: "hash", TalkRequestMsg: "Please",
				InputMuted: true, OutputMuted: true, OutputOnlyMuted: true, IsRecording: true,
				IsTalker: true, TalkRequest: time.Unix(1700000000, 0), IconID: 3000000000,
				MonthBytesDownloaded: 11, MonthBytesUploaded: 12, TotalBytesDownloaded: 13, TotalBytesUploaded: 14,
				ConnectionFiletransferBandwidthSent: 15, ConnectionFiletransferBandwidthReceived: 16,
			},
		},
		{
			name:     "unsigned icon and no talk request",
			response: `client_icon_id=3000000000 client_talk_request=0`,
			want:     &DetailedOnlineClient{ID: 8, IconID: 3000000000},
		},
		{
			name:     "single group and comma text",
			response: `client_servergroups=6 client_nickname=Hello,\sworld client_description=a,b client_idle_time=0 connection_connected_time=0 client_month_bytes_uploaded=18446744073709551615`,
			want: &DetailedOnlineClient{
				ID: 8, ServerGroups: []int{6}, Nickname: "Hello, world", Description: "a,b",
				MonthBytesUploaded: 1<<64 - 1,
			},
		},
		{
			name:     "largest duration",
			response: `client_idle_time=9223372036854 connection_connected_time=9223372036854`,
			want: &DetailedOnlineClient{
				ID: 8, IdleTime: 9223372036854 * time.Millisecond,
				ConnectionConnectedTime: 9223372036854 * time.Millisecond,
			},
		},
		{name: "missing optional fields", response: `client_nickname=Query`, want: &DetailedOnlineClient{ID: 8, Nickname: "Query"}},
		{name: "invalid groups", response: `client_servergroups=6,no`, wantErr: "decode server group"},
		{name: "invalid timestamp", response: `client_created=invalid`, wantErr: "invalid time"},
		{name: "invalid last connected", response: `client_lastconnected=invalid`, wantErr: "invalid time"},
		{name: "invalid talk request", response: `client_talk_request=invalid`, wantErr: "client_talk_request"},
		{name: "negative talk request", response: `client_talk_request=-1`, wantErr: "client_talk_request timestamp out of range"},
		{name: "invalid duration", response: `client_idle_time=invalid`, wantErr: "client_idle_time"},
		{name: "idle overflow", response: `client_idle_time=9223372036855`, wantErr: "client_idle_time milliseconds out of range"},
		{name: "connected overflow", response: `connection_connected_time=9223372036855`, wantErr: "connection_connected_time milliseconds out of range"},
		{name: "negative duration", response: `client_idle_time=-1`, wantErr: "client_idle_time milliseconds out of range"},
		{name: "negative connected duration", response: `connection_connected_time=-1`, wantErr: "connection_connected_time milliseconds out of range"},
		{name: "server error", wantErr: "command not found", wantServerErr: true},
	}
	for _, transport := range []string{"tcp", "ssh"} {
		t.Run(transport, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					options := []serverOption{func(s *server) {
						s.responses = make(map[string]string)
						if tc.response != "" {
							s.responses["clientinfo clid=8"] = tc.response
						}
					}}
					if transport == "ssh" {
						options = append(options, useSSH())
					}
					s := newServer(t, options...)
					defer func() { require.NoError(t, s.Close()) }()
					clientOptions := []func(*Client) error{Timeout(2 * time.Second)}
					if transport == "ssh" {
						clientOptions = append(clientOptions, SSH(sshClientTestConfig))
					}
					c, err := NewClient(s.Addr, clientOptions...)
					require.NoError(t, err)
					defer func() { require.NoError(t, c.Close()) }()

					got, err := c.Server.ClientInfo(8)
					if tc.wantErr != "" {
						require.ErrorContains(t, err, tc.wantErr)
						require.ErrorContains(t, err, "clientinfo:")
						require.Nil(t, got)
						if tc.wantServerErr {
							var (
								serverErr *Error
							)
							require.ErrorAs(t, err, &serverErr)
							require.Equal(t, 256, serverErr.ID)
						}
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.want, got)
				})
			}
		})
	}
}

func TestClientInfoNotConnected(t *testing.T) {
	done := make(chan struct{})
	close(done)
	s := &ServerMethods{Client: &Client{done: done}}

	got, err := s.ClientInfo(8)
	require.ErrorIs(t, err, ErrNotConnected)
	require.ErrorContains(t, err, "clientinfo:")
	require.Nil(t, got)
}
