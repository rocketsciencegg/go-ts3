package ts3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCmdsBasic(t *testing.T) {
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

	testCmdsBasic(t, c)
}

func TestCmdsBasicSSH(t *testing.T) {
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

	testCmdsBasic(t, c)
}

func testCmdsBasic(t *testing.T, c *Client) {
	t.Helper()
	auth := func(t *testing.T) {
		t.Helper()
		if err := c.Login("user", "pass"); !assert.NoError(t, err) {
			return
		}

		if err := c.Logout(); !assert.NoError(t, err) {
			return
		}
	}

	version := func(t *testing.T) {
		t.Helper()
		v, err := c.Version()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "3.0.12.2", v.Version)
		assert.Equal(t, 1455547898, v.Build)
		assert.Equal(t, "FreeBSD", v.Platform)
	}

	useID := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.Use(1))
	}

	usePort := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, c.UsePort(1024))
	}

	whoami := func(t *testing.T) {
		t.Helper()
		info, err := c.Whoami()
		if !assert.NoError(t, err) {
			return
		}

		expected := &ConnectionInfo{
			ServerStatus:           statusOnline,
			ServerID:               18,
			ServerUniqueIdentifier: "gNITtWtKs9+Uh3L4LKv8/YHsn5c=",
			ServerPort:             9987,
			ClientID:               94,
			ClientChannelID:        432,
			ClientName:             "serveradmin from 127.0.0.1:49725",
			ClientDatabaseID:       1,
			ClientLoginName:        "serveradmin",
			ClientUniqueIdentifier: "serveradmin",
			ClientOriginServerID:   0,
		}

		assert.Equal(t, expected, info)
	}

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{"auth", auth},
		{cmdVersion, version},
		{"useid", useID},
		{"useport", usePort},
		{"whoami", whoami},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.f)
	}
}
