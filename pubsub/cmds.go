/* Package pubsub provides publish/subscribe operations for HTTP/2.

*/
package pubsub

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Masterminds/cookoo"
)

const MediumDS = "drift.Medium"

const (
	// XHistorySince is an HTTP header for the client to send a request for history since TIMESTAMP.
	XHistorySince = "x-history-since"
	// XHistoryLength is an HTTP header for the client to send a request for the last N records.
	XHistoryLength = "x-history-length"
	// XHistoryEnabled is a flag for the server to notify the client whether history is enabled.
	XHistoryEnabled = "x-history-enabled"
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

	rw := c.Get("http.ResponseWriter", nil).(ResponseWriterFlusher)
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

// CreateTopic creates a new topic.
//
// Params:
// 	- topic (string)
// 	- history (bool): whether or not to track history
// 	- historyLength (int): How much history to track. Default is DefaultMaxHistory.
//
// Returns:
// 	Topic the new topic.
func CreateTopic(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	name := p.Get("topic", "").(string)
	if len(name) == 0 {
		return nil, &cookoo.FatalError{"Topic name required."}
	}

	hist := p.Get("history", true).(bool)
	histLen := p.Get("historyLength", DefaultMaxHistory).(int)

	m, err := getMedium(c)
	if err != nil {
		return nil, &cookoo.FatalError{"No medium."}
	}

	t := fetchOrCreateTopic(m, name, hist, histLen)

	return t, nil

}

// DeleteTopic deletes a topic and its history.
//
// Params:
// 	- name (string)
//
// Returns:
//
func DeleteTopic(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	name := p.Get("topic", "").(string)
	if len(name) == 0 {
		return nil, &cookoo.FatalError{"Topic name required."}
	}

	m, err := getMedium(c)
	if err != nil {
		return nil, &cookoo.FatalError{"No medium."}
	}

	err = m.Delete(name)
	if err != nil {
		c.Logf("warn", "Failed to delete topic: %s", err)
	}

	return nil, nil
}

// TopicExists tests whether a topic exists, and sends an HTTP 200 if yes, 404 if no.
//
// Params:
// 	- topic (string): The topic to look up.
// Returns:
//
func TopicExists(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	res := c.Get("http.ResponseWriter", nil).(ResponseWriterFlusher)
	name := p.Get("topic", "").(string)
	if len(name) == 0 {
		res.WriteHeader(404)
		return nil, nil
	}

	medium, err := getMedium(c)
	if err != nil {
		res.WriteHeader(404)
		return nil, nil
	}

	if _, ok := medium.Topic(name); ok {
		res.WriteHeader(200)
		return nil, nil
	}
	res.WriteHeader(404)
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
	res := c.Get("http.ResponseWriter", nil).(ResponseWriterFlusher)
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

	// maxLen can be used either on its own or paired with X-History-Since.
	maxLen := 0
	if len(max) > 0 {
		m, err := parseHistLen(max)
		if err != nil {
			c.Logf("info", "failed to parse X-History-Length %s", max)
		} else {
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

		// If maxLen is also set, we trim the list by sending the newest.
		ls := len(toSend)
		if maxLen > 0 && ls > maxLen {
			offset := ls - maxLen - 1
			toSend = toSend[offset:]
		}
		return sendHistory(c, res, toSend)
	} else if maxLen > 0 {
		toSend := topic.Last(maxLen)
		return sendHistory(c, res, toSend)
	}

	return 0, nil
}

// sendHistory sends the accumulated history to the writer.
func sendHistory(c cookoo.Context, writer ResponseWriterFlusher, data [][]byte) (int, error) {
	c.Logf("info", "Sending history.")
	var i int
	var d []byte
	for i, d = range data {
		_, err := writer.Write(d)
		if err != nil {
			c.Logf("warn", "Failed to write history message: %s", err)
			return i + 1, nil
		}
		writer.Flush()
	}
	return i + 1, nil
}

// parseSince parses the X-History-Since value.
func parseSince(s string) (time.Time, error) {
	tint, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return time.Unix(0, 0), fmt.Errorf("Could not parse as time: %s", s)
	}
	return time.Unix(tint, 0), nil
}

// parseHistLen parses the X-History-Length value.
func parseHistLen(s string) (int, error) {
	return strconv.Atoi(s)
}

// fetchOrCreateTopic gets a topic if it exists, and creates one if it doesn't.
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
