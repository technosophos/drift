package httputil

import (
	"bytes"
	"io"
	"net/http"

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
