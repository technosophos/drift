/* Package main demos a Pub/Sub server.
 */
package main

import (
	"net/http"

	"github.com/Masterminds/cookoo"
	cfmt "github.com/Masterminds/cookoo/fmt"
	"github.com/Masterminds/cookoo/web"

	"github.com/technosophos/drift/httputil"
	"github.com/technosophos/drift/pubsub"

	"github.com/bradfitz/http2"
)

var helpTemplate = `<html>
<head>
<title>API Reference</title>
<body>
<h1>API Reference</h1>
<p>These are the API endpoints currently defined for this server.</p>
<dl>{{ range .Routes }}
	<dn>{{.Name}}</dn>
	<dd>{{.Description}}</dd>
{{end}}</dl>
</body>
</html>`

func main() {
	srv := &http.Server{
		Addr: ":5500",
	}

	reg, router, cxt := cookoo.Cookoo()

	buildRegistry(reg, router, cxt)

	// Our main datasource is the Medium, which manages channels.
	m := pubsub.NewMedium()
	cxt.AddDatasource(pubsub.MediumDS, m)
	cxt.Add("routes", reg.Routes())

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
		Help: "API Reference",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "help",
				Fn:   cfmt.Template,
				Using: []cookoo.Param{
					{Name: "template", DefaultValue: helpTemplate},
					{Name: "Routes", From: "cxt:routes"},
				},
			},
			cookoo.Cmd{
				Name: "_",
				Fn:   web.Flush,
				Using: []cookoo.Param{
					{Name: "content", From: "cxt:help"},
					{Name: "contentType", DefaultValue: "text/html"},
				},
			},
		},
	})

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
}
