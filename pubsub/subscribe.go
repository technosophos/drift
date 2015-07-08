/* Package pubsub provides publish/subscribe operations for HTTP/2.

*/
package pubsub

import (
	"errors"
	"net/http"

	"github.com/Masterminds/cookoo"
)

const MediumDS = "drift.Medium"

// Publish sends a new message to a topic.
//
// Params:
// 	- topic (string): The topic to send to.
// 	- message ([]byte): The message to send.
//
// Datasources:
// 	- This uses the 'drift.Medium' datasource.
//
// Returns:
//
func Publish(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	topic := p.Get("topic", "").(string)
	if len(topic) == 0 {
		return nil, errors.New("No topic supplied.")
	}

	medium, _ := getMedium(c)

	// Is there any reason to disallow empty messages?
	msg := p.Get("message", []byte{}).([]byte)
	c.Logf("info", "Msg: %s", msg)

	t, ok := medium.Topic(topic)
	if !ok {
		// Create a new topic
		t = NewTopic(topic)
		medium.Add(t)
	}
	return nil, t.Publish(msg)

}

// Subscribe allows an request to subscribe to topic updates.
//
// Params:
// 	- topic (string): The topic to subscribe to.
// 	-
//
// Returns:
//
func Subscribe(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	medium, err := getMedium(c)
	if err != nil {
		return nil, &cookoo.FatalError{"No medium."}
	}
	topic := p.Get("topic", "").(string)
	if len(topic) == 0 {
		return nil, errors.New("No topic is set.")
	}

	rw := c.Get("http.ResponseWriter", nil).(http.ResponseWriter)
	clientGone := rw.(http.CloseNotifier).CloseNotify()

	sub := NewSubscription(rw)

	t, ok := medium.Topic(topic)
	if !ok {
		t = NewTopic(topic)
		medium.Add(t)
	}
	t.Subscribe(sub)

	defer func() {
		t.Unsubscribe(sub)
		sub.Close()
	}()

	sub.Listen(clientGone)

	return nil, nil
}
