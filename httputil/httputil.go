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
