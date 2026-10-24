# TeamSpeak 3 [![Go Report Card](https://goreportcard.com/badge/github.com/multiplay/go-ts3)](https://goreportcard.com/report/github.com/multiplay/go-ts3) [![License](https://img.shields.io/badge/license-BSD-blue.svg)](https://github.com/multiplay/go-ts3/blob/master/LICENSE) [![GoDoc](https://godoc.org/github.com/multiplay/go-ts3?status.svg)](https://godoc.org/github.com/multiplay/go-ts3)

go-ts3 is a [Go](http://golang.org/) client for the [TeamSpeak 3 ServerQuery Protocol](http://media.teamspeak.com/ts3_literature/TeamSpeak%203%20Server%20Query%20Manual.pdf).

Features
--------
* [ServerQuery](http://media.teamspeak.com/ts3_literature/TeamSpeak%203%20Server%20Query%20Manual.pdf) Support.

Installation
------------
```sh
go get -u github.com/multiplay/go-ts3
```

Examples
--------

Using go-ts3 is simple just create a client, login and then send commands e.g.
```go
package main

import (
	"log"

        "github.com/multiplay/go-ts3"
)

func main() {
        c, err := ts3.NewClient("192.168.1.102:10011")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err := c.Login(user, pass); err != nil {
		log.Fatal(err)
	}

	if v, err := c.Version(); err != nil {
		log.Fatal(err)
	} else {
		log.Println("server is running:", v)
	}
}
```

Documentation
-------------
- [GoDoc API Reference](http://godoc.org/github.com/multiplay/go-ts3).

### Optional client information

`c.Server.ClientList()` requests only the basic client list. Select additional
information explicitly rather than requesting every extension on every call:

```go
clients, err := c.Server.ClientList(ts3.ClientAway, ts3.ClientVoice, ts3.ClientCountry)
if err != nil {
	log.Fatal(err)
}
for _, client := range clients {
	log.Println(client.Nickname, client.Away, client.AwayMessage)
	if client.OnlineClientExt == nil {
		continue
	}
	if client.Country != nil {
		log.Println("country:", *client.Country)
	}
	if client.OnlineClientVoice == nil {
		continue
	}
	if client.InputMuted != nil && client.OutputMuted != nil {
		log.Println("muted:", *client.InputMuted, *client.OutputMuted)
	}
}
```

`ClientList(ts3.ClientListFull)` requests all supported extensions. Country and
voice fields are pointers: `nil` means the field was not returned, while a
non-nil pointer can contain an empty country or a `false` mute status. Check the
embedded extension pointers before accessing their fields. `Away` and
`AwayMessage` retain their non-pointer types for compatibility.

License
-------
go-ts3 is available under the [BSD 2-Clause License](https://opensource.org/licenses/BSD-2-Clause).
```
