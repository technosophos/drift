package httputil

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Masterminds/cookoo"
)

// BufferPost buffers the body of the POST request into the context.
//
// Params:
//
// Returns:
//	- []byte with the content of the request.
func BufferPost(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	req := c.Get("http.Request", nil).(*http.Request)
	var b bytes.Buffer
	_, err := io.Copy(&b, req.Body)
	c.Logf("info", "Received POST: %s", b.Bytes())
	return b.Bytes(), err
}

// Timestamp returns a UNIX timestamp.
//
// Params:
//
// Returns:
// 	- int64 timestamp as seconds since epoch.
//
func Timestamp(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	return fmt.Sprintf("%d", time.Now().Unix()), nil
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
