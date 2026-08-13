# Haproxy SPOE Golang Agent Library [![Go Report Card](https://goreportcard.com/badge/github.com/AndreiSec/haproxy-spoe-go)](https://goreportcard.com/report/github.com/AndreiSec/haproxy-spoe-go) ![](https://github.com/AndreiSec/haproxy-spoe-go/workflows/Test/badge.svg)

Terms from [Haproxy SPOE specification](https://www.haproxy.org/download/1.9/doc/SPOE.txt)

```
* SPOE : Stream Processing Offload Engine.

    A SPOE is a filter talking to servers managed ba a SPOA to offload the
    stream processing. An engine is attached to a proxy. A proxy can have
    several engines. Each engine is linked to an agent and only one.

* SPOA : Stream Processing Offload Agent.

    A SPOA is a service that will receive info from a SPOE to offload the
    stream processing. An agent manages several servers. It uses a backend to
    reference all of them. By extension, these servers can also be called
    agents.

* SPOP : Stream Processing Offload Protocol, used by SPOEs to talk to SPOA
         servers.

    This protocol is used by engines to talk to agents. It is an in-house
    binary protocol described in this documentation.
```


This library implements SPOA for Golang applications

## Install

```
go get -u github.com/AndreiSec/haproxy-spoe-go
```

## Example

Example from Section 2.5 [SPOE specification](https://www.haproxy.org/download/1.9/doc/SPOE.txt) describes a simple IP reputation service.

> Here is a simple but complete example that sends client-ip address to a ip
  reputation service. This service can set the variable "ip_score" which is an
  integer between 0 and 100, indicating its reputation (100 means totally safe
  and 0 a blacklisted IP with no doubt).

Golang backend application for this example

```go
package main

import (
	"github.com/AndreiSec/haproxy-spoe-go/action"
	"github.com/AndreiSec/haproxy-spoe-go/agent"
	"github.com/AndreiSec/haproxy-spoe-go/request"
        "github.com/AndreiSec/haproxy-spoe-go/logger"
	"log"
	"math/rand"
	"net"
	"os"
)

func main() {

	log.Print("listen 3000")

	listener, err := net.Listen("tcp4", "127.0.0.1:3000")
	if err != nil {
		log.Printf("error create listener, %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	a := agent.New(handler, logger.NewDefaultLog())

	if err := a.Serve(listener); err != nil {
		log.Printf("error agent serve: %+v\n", err)
	}
}

func handler(req *request.Request) {

	log.Printf("handle request EngineID: '%s', StreamID: '%d', FrameID: '%d' with %d messages\n", req.EngineID, req.StreamID, req.FrameID, req.Messages.Len())

	messageName := "get-ip-reputation"

	mes, err := req.Messages.GetByName(messageName)
	if err != nil {
		log.Printf("message %s not found: %v", messageName, err)
		return
	}

	ipValue, ok := mes.KV.Get("ip")
	if !ok {
		log.Printf("var 'ip' not found in message")
		return
	}

	ip, ok := ipValue.(net.IP)
	if !ok {
		log.Printf("var 'ip' has wrong type. expect IP addr")
		return
	}

	ipScore := rand.Intn(100)

	log.Printf("IP: %s, send score '%d'", ip.String(), ipScore)

	req.Actions.SetVar(action.ScopeSession, "ip_score", ipScore)
}
```

## API

### Messages

Getting request data is possible through `Request.Messages`

#### Len() int

Returns count of messages in request

```
count := request.Messages.Len()
```

#### GetByName(name string) (*Message, error)

Returns message by name and error if not exists

```
mes, err := request.Messages.GetByName("get-ip-reputation")
```

#### GetByIndex(idx int) (*Message, error)

Returns message by index and error if not exists

```
mes, err := request.Messages.GetByIndex(0)
```

### Message

Represents one message, sent by Haproxy

Message has two fields
- Name string
- KV   *kv.KV

KV contains key-value data of message

### Actions

Actions is used for sending a response to Haproxy

#### SetVar(scope Scope, name string, value interface{})

Set variable with `name` to `value` in specific `scope` (see bellow)

```
request.Actions.SetVar(action.ScopeSession, "ip_score", 10)
```

#### UnsetVar(scope Scope, name string)

Unset variable with `name` in specific `scope`

```
request.Actions.UnsetVar(action.ScopeSession, "ip_score")
```

#### Actions Scopes
- ScopeProcess
- ScopeSession
- ScopeTransaction
- ScopeRequest
- ScopeResponse

### KV (key-value)

Contains message key-value data sent by Haproxy

#### Get(key string) (interface{}, bool)

Returns value by name. If key doesn't exist, last returned value will be set to False

```
ipValue, ok := message.KV.Get("ip")
```

## Value lifetimes

Frames are pooled and each frame reuses its read buffer, so values decoded from a
request are only valid while the handler runs. Binary (`[]byte`) and IP
(`net.IP`) values point straight into that buffer: copy them if you need to keep
them after the handler returns.

`string` values are copied by default, so they are safe to retain.

## Reducing allocations further

```go
frame.SetNoCopyStrings(true)
```

makes `string` values point into the frame's read buffer instead of being copied,
which removes the largest remaining allocation on the decode path. In exchange,
strings follow the same lifetime rule as binary and IP values: valid until the
handler returns, so anything stored, cached, or logged asynchronously has to be
copied first. Call it during startup, before serving.

For a Notify frame carrying a 4 KiB string argument, the decode path allocates
4118 B in 2 allocations by default, and 16 B in 1 allocation with
`SetNoCopyStrings(true)`.

`frame.MaxFrameLen` caps the frame length the reader accepts, and therefore the
largest buffer one read can allocate. It defaults to 16 MiB, far above HAProxy's
negotiated `max-frame-size`.
