# Drift: An HTTP/2 Pub/Sub service

Drift is a topic-based PubSub service based on HTTP/2. It uses HTTP/2's
ability to stream data as a simple mechanism for managing subscriptions.

In a nutshell, one or more _publishers_ send data to _topics_. One or
more _subscribers_ can listen on that topic. Every time a publisher
sends a message, all of the subscribers will receive it.

Features:

- Uses the HTTP/2 standard with no additions.
- JSON? Thrift? ProtoBuf? Use whatever.
- Service metadata is confinted to HTTP headers. The payload is all
  yours.
- Configurable history lets clients quickly catch up on what they missed.
- Extensible architecture makes it easy for you to add your own flair.
- And more in the works...

The current implementation streams Data Frames. Once the Go libraries
mature, we may instead opt to use full pushes (though the overhead for
that may be higher than we want).

**This library is not stable. The interfaces may change before the 0.1
release**

**Currently, the library ONLY supports HTTPS.**

## Installation

```
$ brew install glide
$ git clone $THIS_REPO
$ glide init
```

From there, you can build the server (`go build server/server.go`) or
the example client (`go build client/client.go`).

## Simple Client Example

To use Drift as a client, import the client library:

```go
import "github.com/technosophos/drift/client"
```

Here is a simple publisher:

```go
c := client.New("https://localhost:5500")
c.Publish("example", []byte("Hello World"))
```

The above sends the "Hello World" message over the `example` topic.

A subscriber looks like this:

```go
s := client.New("https://localhost:5500")
subscription, err := s.Subscribe("example")
if err != nil {
  fmt.Printf("Failed subscription: %s", err)
  return
}

// Now listen on a stream.
for msg := range subscription.Stream {
  fmt.Printf("Received: %s\n", msg)
}

// When you're done...
subscription.Cancel()
```

A more advanced API is provided for configuring history and adding
arbitrary HTTP headers.

## About the Server

The server lives in `server/server.go`. The basic server provides
convenient features for getting running quickly.

But the server was also designed as a composable system. You can easily
take the parts here and add your own. Take a look at the registry in
`server.server.go` to see how this is done.

## API

`GET /`

Prints a the runtime API documentation.

`DELETE /v1/t/TOPIC`

Destroy a topic named `TOPIC`.

This will destroy the history and cancel subscriptions for all
subscribed clients.

`GET /v1/t/TOPIC`

Subscribe to a topic named `TOPIC`. The client is expected to hold open
a connection for the duration of its subscription.

This method **does not support HTTP/1 at all!** You must use HTTP/2.


`POST /v1/t/TOPIC`

Post a new message into the topic named `TOPIC`.

The body of the post message is pushed wholesale into the queue.

This method accepts HTTP/1.1 POST content in addition to HTTP/2 POST.
Only one data frame of HTTP/2 POST data is accepted. Streamed POST is
currently not supported (though it will be).

`PUT /v1/t/TOPIC`

Create a new topic named `TOPIC`.

The body of this message is a well-defined JSON data structure that
describes the topic.

`GET /v1/time`

Get the current time. This returns a plain text value with nothing but a
timestamp.

```
$ curl -k https://localhost:5500/v1/time
1436464998
```

The purpose of this callback is to give client libries a timestamp to
use as the base time for calculating dates. This can reduce problems
with clock skew.
