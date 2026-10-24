package ts3

import (
	"fmt"
	"time"
)

const (
	// maxClientInfoMilliseconds is the largest protocol duration that fits in time.Duration.
	maxClientInfoMilliseconds = (1<<63 - 1) / int64(time.Millisecond)
)

// DetailedOnlineClient contains the information returned by [ServerMethods.ClientInfo].
// It is independent of the optional extensions returned by [ServerMethods.ClientList].
type DetailedOnlineClient struct {
	ID          int    `ms:"-"`
	ChannelID   int    `ms:"cid"`
	DatabaseID  int    `ms:"client_database_id"`
	Nickname    string `ms:"client_nickname"`
	Type        int    `ms:"client_type"`
	Away        bool   `ms:"client_away"`
	AwayMessage string `ms:"client_away_message"`

	Created time.Time `ms:"client_created"`
	// LastConnected may report the last disconnect time on some servers.
	LastConnected time.Time `ms:"client_lastconnected"`
	// TalkRequest is zero when the client has not requested permission to talk.
	TalkRequest time.Time `ms:"-"`
	// These durations are converted from protocol milliseconds by [ServerMethods.ClientInfo].
	IdleTime                time.Duration `ms:"-"`
	ConnectionConnectedTime time.Duration `ms:"-"`

	Base64HashClientUID string `ms:"client_base64HashClientUID"`
	ConnectionClientIP  string `ms:"connection_client_ip"`
	Country             string `ms:"client_country"`
	DefaultChannel      string `ms:"client_default_channel"`
	DefaultToken        string `ms:"client_default_token"`
	Description         string `ms:"client_description"`
	LoginName           string `ms:"client_login_name"`
	NicknamePhonetic    string `ms:"client_nickname_phonetic"`
	SecurityHash        string `ms:"client_security_hash"`
	TalkRequestMsg      string `ms:"client_talk_request_msg"`
	UniqueIdentifier    string `ms:"client_unique_identifier"`
	FlagAvatar          string `ms:"client_flag_avatar"`
	Platform            string `ms:"client_platform"`
	Version             string `ms:"client_version"`
	VersionSign         string `ms:"client_version_sign"`

	ConnectionBytesReceivedTotal               uint64 `ms:"connection_bytes_received_total"`
	ConnectionBytesSentTotal                   uint64 `ms:"connection_bytes_sent_total"`
	ConnectionPacketsReceivedTotal             uint64 `ms:"connection_packets_received_total"`
	ConnectionPacketsSentTotal                 uint64 `ms:"connection_packets_sent_total"`
	MonthBytesDownloaded                       uint64 `ms:"client_month_bytes_downloaded"`
	MonthBytesUploaded                         uint64 `ms:"client_month_bytes_uploaded"`
	TotalBytesDownloaded                       uint64 `ms:"client_total_bytes_downloaded"`
	TotalBytesUploaded                         uint64 `ms:"client_total_bytes_uploaded"`
	ConnectionFiletransferBandwidthSent        uint64 `ms:"connection_filetransfer_bandwidth_sent"`
	ConnectionFiletransferBandwidthReceived    uint64 `ms:"connection_filetransfer_bandwidth_received"`
	ConnectionBandwidthSentLastSecondTotal     uint64 `ms:"connection_bandwidth_sent_last_second_total"`
	ConnectionBandwidthSentLastMinuteTotal     uint64 `ms:"connection_bandwidth_sent_last_minute_total"`
	ConnectionBandwidthReceivedLastSecondTotal uint64 `ms:"connection_bandwidth_received_last_second_total"`
	ConnectionBandwidthReceivedLastMinuteTotal uint64 `ms:"connection_bandwidth_received_last_minute_total"`
	ChannelGroupID                             int    `ms:"client_channel_group_id"`
	ChannelGroupInheritedChannelID             int    `ms:"client_channel_group_inherited_channel_id"`
	NeededServerQueryViewPower                 int    `ms:"client_needed_serverquery_view_power"`
	ServerGroups                               []int  `ms:"client_servergroups"`
	TalkPower                                  int    `ms:"client_talk_power"`
	TotalConnections                           int    `ms:"client_totalconnections"`
	IconID                                     uint32 `ms:"client_icon_id"`
	InputHardware                              bool   `ms:"client_input_hardware"`
	InputMuted                                 bool   `ms:"client_input_muted"`
	IsChannelCommander                         bool   `ms:"client_is_channel_commander"`
	IsPrioritySpeaker                          bool   `ms:"client_is_priority_speaker"`
	IsRecording                                bool   `ms:"client_is_recording"`
	OutputHardware                             bool   `ms:"client_output_hardware"`
	OutputMuted                                bool   `ms:"client_output_muted"`
	OutputOnlyMuted                            bool   `ms:"client_outputonly_muted"`
	// IsTalker indicates sufficient talk power, but some servers always report false.
	IsTalker bool `ms:"client_is_talker"`
}

// ClientInfo returns detailed information about a single online client.
// ID is taken from the request because the response does not include clid.
// IdleTime and ConnectionConnectedTime are decoded from milliseconds.
func (s *ServerMethods) ClientInfo(clientID int) (*DetailedOnlineClient, error) {
	response := struct {
		IdleMilliseconds      int64 `ms:"client_idle_time"`
		ConnectedMilliseconds int64 `ms:"connection_connected_time"`
		TalkRequestSeconds    int64 `ms:"client_talk_request"`

		DetailedOnlineClient `ms:",squash"`
	}{}
	if _, err := s.ExecCmd(NewCmd("clientinfo").WithArgs(NewArg("clid", clientID)).WithResponse(&response)); err != nil {
		return nil, fmt.Errorf("clientinfo: %w", err)
	}

	if response.TalkRequestSeconds < 0 {
		return nil, fmt.Errorf("clientinfo: client_talk_request timestamp out of range: %q", fmt.Sprint(response.TalkRequestSeconds))
	}

	if response.TalkRequestSeconds > 0 {
		response.TalkRequest = time.Unix(response.TalkRequestSeconds, 0)
	}

	for _, duration := range []struct {
		name  string
		value int64
	}{
		{"client_idle_time", response.IdleMilliseconds},
		{"connection_connected_time", response.ConnectedMilliseconds},
	} {
		if duration.value < 0 || duration.value > maxClientInfoMilliseconds {
			return nil, fmt.Errorf("clientinfo: %s milliseconds out of range: %q", duration.name, fmt.Sprint(duration.value))
		}
	}

	response.ID = clientID
	response.IdleTime = time.Duration(response.IdleMilliseconds) * time.Millisecond
	response.ConnectionConnectedTime = time.Duration(response.ConnectedMilliseconds) * time.Millisecond
	return &response.DetailedOnlineClient, nil
}
