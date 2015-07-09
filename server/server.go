/* Package main demos a Pub/Sub server.
 */
package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/web"

	"github.com/technosophos/drift/httputil"
	"github.com/technosophos/drift/pubsub"

	"github.com/bradfitz/http2"
)

func main() {
	srv := &http.Server{
		Addr: ":5500",
	}

	reg, router, cxt := cookoo.Cookoo()

	buildRegistry(reg, router, cxt)

	// Our main datasource is the Medium, which manages channels.
	m := pubsub.NewMedium()
	cxt.AddDatasource(pubsub.MediumDS, m)

	http2.ConfigureServer(srv, &http2.Server{})

	srv.Handler = web.NewCookooHandler(reg, router, cxt)

	srv.ListenAndServeTLS("server.crt", "server.key")
}

func buildRegistry(reg *cookoo.Registry, router *cookoo.Router, cxt cookoo.Context) {
	reg.AddRoute(cookoo.Route{
		Name: "GET /ping",
		Help: "Ping the server, get a pong reponse.",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Fn: func(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
					w := c.Get("http.ResponseWriter", nil).(http.ResponseWriter)
					w.Write([]byte("pong"))
					return nil, nil
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "GET /v1/time",
		Help: "Print the current server time as a UNIX seconds-since-epoch",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "timestamp",
				Fn:   httputil.Timestamp,
			},
			cookoo.Cmd{
				Name: "_",
				Fn:   web.Flush,
				Using: []cookoo.Param{
					{Name: "content", From: "cxt:timestamp"},
					{Name: "contentType", DefaultValue: "text/plain"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "GET /",
		Help: "Main page",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "debug",
				Fn:   Debug,
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "PUT /v1/t/*",
		Help: "Create a new channel.",
		Does: cookoo.Tasks{},
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
		Help: "Subscribe to a channel.",
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
		Name: "DELETE /v1/t/*",
		Help: "Delete a channel.",
		Does: cookoo.Tasks{},
	})

	reg.Route("GET /tick", "Clock streamer demo").Does(ClockStreamer, "clock")
}

func simplePage(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got here.\n")
	fmt.Fprintf(w, "Test.")
}

// Debug displays debugging info.
func Debug(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	w := c.Get("http.ResponseWriter", nil).(http.ResponseWriter)
	r := c.Get("http.Request", nil).(*http.Request)
	reqInfoHandler(w, r)
	return nil, nil
}
func reqInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "Protocol: %s\n", r.Proto)
	fmt.Fprintf(w, "Host: %s\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr: %s\n", r.RemoteAddr)
	fmt.Fprintf(w, "RequestURI: %q\n", r.RequestURI)
	fmt.Fprintf(w, "URL: %#v\n", r.URL)
	fmt.Fprintf(w, "Body.ContentLength: %d (-1 means unknown)\n", r.ContentLength)
	fmt.Fprintf(w, "Close: %v (relevant for HTTP/1 only)\n", r.Close)
	fmt.Fprintf(w, "TLS: %#v\n", r.TLS)
	fmt.Fprintf(w, "\nHeaders:\n")
	r.Header.Write(w)
}

// ClockStreamer streams a clock, based on hte h2demo in http2 repo.
//
// Params:
//
// Returns:
//
func ClockStreamer(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	w := c.Get("http.ResponseWriter", nil).(http.ResponseWriter)
	//r := c.Get("http.Request", nil).(*http.Request)

	clientGone := w.(http.CloseNotifier).CloseNotify()
	w.Header().Set("Content-Type", "text/plain")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Flush buffer
	io.WriteString(w, strings.Repeat("# xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", 13))

	// Try this 50 times
	for i := 0; i < 50; i++ {
		fmt.Fprintf(w, "%v\n", time.Now())
		w.(http.Flusher).Flush()
		select {
		case <-ticker.C:
		case <-clientGone:
			c.Logf("info", "Client gone.")
			return nil, nil
		}
	}
	return nil, nil
}
