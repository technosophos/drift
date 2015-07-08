/* Package pubsub provides publish/subscribe operations for HTTP/2.

*/
package pubsub

import (
	"errors"
	"net/http"
	"time"

	"github.com/Masterminds/cookoo"
)

const MediumDS = "drift.Medium"

const (
	// XHistorySince is an HTTP header for the client to send a request for history since TIMESTAMP.
	XHistorySince = "X-History-Since"
	// XHistoryLength is an HTTP header for the client to send a request for the last N records.
	XHistoryLength = "X-History-Length"
	// XHistoryEnabled is a flag for the server to notify the client whether history is enabled.
	XHistoryEnabled = "X-History-Enabled"
)

// Publish sends a new message to a topic.
//
// Params:
// 	- topic (string): The topic to send to.
// 	- message ([]byte): The message to send.
// 	- withHistory (bool): Turn on history. Default is true. This only takes
// 		effect when the channel is created.
//
// Datasources:
// 	- This uses the 'drift.Medium' datasource.
//
// Returns:
//
func Publish(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	hist := p.Get("withHistory", true).(bool)
	topic := p.Get("topic", "").(string)
	if len(topic) == 0 {
		return nil, errors.New("No topic supplied.")
	}

	medium, _ := getMedium(c)

	// Is there any reason to disallow empty messages?
	msg := p.Get("message", []byte{}).([]byte)
	c.Logf("info", "Msg: %s", msg)

	t := fetchOrCreateTopic(medium, topic, hist, DefaultMaxHistory)
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
	t := fetchOrCreateTopic(medium, topic, true, DefaultMaxHistory)
	t.Subscribe(sub)

	defer func() {
		t.Unsubscribe(sub)
		sub.Close()
	}()

	sub.Listen(clientGone)

	return nil, nil
}

// ReplayHistory sends back the history to a subscriber.
//
// This should be called before the client goes into active listening.
//
// Params:
// - topic (string): The topic to fetch.
//
// Returns:
// 	- int: The number of history messages sent to the client.
func ReplayHistory(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	req := c.Get("http.Request", nil).(*http.Request)
	res := c.Get("http.ResponseWriter", nil).(http.ResponseWriter)
	medium, _ := getMedium(c)
	name := p.Get("topic", "").(string)

	// This does not manage topics. If there is no topic set, we silently fail.
	if len(name) == 0 {
		c.Log("info", "No topic name given to ReplayHistory.")
		return 0, nil
	}
	top, ok := medium.Topic(name)
	if !ok {
		c.Logf("info", "No topic named %s exists yet. No history replayed.", name)
		return 0, nil
	}

	topic, ok := top.(HistoriedTopic)
	if !ok {
		c.Logf("info", "No history for topic %s.", name)
		res.Header().Add(XHistoryEnabled, "False")
		return 0, nil
	}
	res.Header().Add(XHistoryEnabled, "True")

	since := req.Header.Get(XHistorySince)
	max := req.Header.Get(XHistoryLength)

	maxLen := 0

	if len(max) > 0 {
		m, err := parseHistLen(max)
		if err != nil {
			maxLen = m
		}
	}
	if len(since) > 0 {
		ts, err := parseSince(since)
		if err != nil {
			c.Logf("warn", "Failed to parse X-History-Since field %s: %s", since, err)
			return 0, nil
		}
		toSend := topic.Since(ts)
		if maxLen > 0 && len(toSend) > maxLen {
			toSend := toSend[:maxLen]
			return len(toSend), nil
		}
	} else if maxLen > 0 {
		toSend := topic.Last(maxLen)
		return len(toSend), nil
	}

	return 0, nil
}

func parseSince(s string) (time.Time, error) {
	return time.Now(), nil
}

func parseHistLen(s string) (int, error) {
	return 10, nil
}

func fetchOrCreateTopic(m *Medium, name string, hist bool, l int) Topic {
	t, ok := m.Topic(name)
	if !ok {
		t = NewTopic(name)
		if hist && l > 0 {
			t = TrackHistory(t, l)
		}
		m.Add(t)
	}
	return t
}
