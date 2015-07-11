package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/web"
	"github.com/bradfitz/http2"
	"github.com/technosophos/drift/httputil"
	"github.com/technosophos/drift/pubsub"
)

var hostport = "127.0.0.1:5500"
var baseurl = "https://127.0.0.1:5500"
var topicname = "test.topic"

func TestClient(t *testing.T) {
	// Lots of timing to simulate networkiness. Because that makes the
	// test nondeterministic, we use fairly large times.

	go standUpServer()
	time.Sleep(2 * time.Second)

	cli := New(baseurl)

	go func() {
		if err := cli.Publish(topicname, []byte("test")); err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)

		cli.Publish(topicname, []byte("Again"))
	}()

	println("Subscribing")
	si, err := cli.Subscribe(topicname)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		si.Cancel()
	}()

	if first := <-si.C; string(first) != "test" {
		t.Errorf("expected test, got %s", first)
	}
	if second := <-si.C; string(second) != "Again" {
		t.Errorf("expected Again, got %s", second)
	}

	time.Sleep(1 * time.Second)
	cli.Delete(topicname)
}

func standUpServer() error {
	srv := &http.Server{Addr: hostport}

	reg, router, cxt := cookoo.Cookoo()

	buildRegistry(reg, router, cxt)

	// Our main datasource is the Medium, which manages channels.
	m := pubsub.NewMedium()
	cxt.AddDatasource(pubsub.MediumDS, m)
	cxt.Put("routes", reg.Routes())

	http2.ConfigureServer(srv, &http2.Server{})

	srv.Handler = web.NewCookooHandler(reg, router, cxt)

	srv.ListenAndServeTLS("../server/server.crt", "../server/server.key")
	return nil
}

func buildRegistry(reg *cookoo.Registry, router *cookoo.Router, cxt cookoo.Context) {

	reg.AddRoute(cookoo.Route{
		Name: "PUT /v1/t/*",
		Help: "Create a new topic.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "topic",
				Fn:   pubsub.CreateTopic,
				Using: []cookoo.Param{
					{Name: "topic", From: "path:2"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "POST /v1/t/*",
		Help: "Publish a message to a channel.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "postBody",
				Fn:   httputil.BufferPost,
			},
			cookoo.Cmd{
				Name: "publish",
				Fn:   pubsub.Publish,
				Using: []cookoo.Param{
					{Name: "message", From: "cxt:postBody"},
					{Name: "topic", From: "path:2"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "GET /v1/t/*",
		Help: "Subscribe to a topic.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "history",
				Fn:   pubsub.ReplayHistory,
				Using: []cookoo.Param{
					{Name: "topic", From: "path:2"},
				},
			},
			cookoo.Cmd{
				Name: "subscribe",
				Fn:   pubsub.Subscribe,
				Using: []cookoo.Param{
					{Name: "topic", From: "path:2"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "HEAD /v1/t/*",
		Help: "Check whether a topic exists.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "has",
				Fn:   pubsub.TopicExists,
				Using: []cookoo.Param{
					{Name: "topic", From: "path:2"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "DELETE /v1/t/*",
		Help: "Delete a topic and close all subscriptions to the topic.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "delete",
				Fn:   pubsub.DeleteTopic,
				Using: []cookoo.Param{
					{Name: "topic", From: "path:2"},
				},
			},
		},
	})
}
