// Package client provides a client library for Drift.
package client

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/technosophos/drift/transport"
)

const v1Path = "/v1/t/"

// Client provides consumer functions for Drift.
//
// Client contains the simple methods for working with subscriptions
// and publishing. The Subscriber and Publisher objects can be used for
// more detailed work.
type Client struct {
	Url string
	// Does not verify cert against authorities.
	InsecureTLSDial bool
}

// New creates and initializes a new client.
func New(url string) *Client {
	return &Client{
		Url: url,
	}
}

// Create creates a new topic on the pubsub server.
func (c *Client) Create(topic string) error {
	url := c.Url + path.Join(v1Path, topic)
	_, err := c.basicRoundTrip("PUT", url)
	return err
}

// Delete removes an existing topic from the pubsub server.
func (c *Client) Delete(topic string) error {
	url := c.Url + path.Join(v1Path, topic)
	_, err := c.basicRoundTrip("DELETE", url)
	return err
}

// Checks whether the server already has the topic.
func (c *Client) Exists(topic string) bool {
	url := c.Url + path.Join(v1Path, topic)
	_, err := c.basicRoundTrip("HEAD", url)
	return err == nil
}

func (c *Client) Publish(topic string, msg []byte) error {
	p := NewPublisher(c.Url)
	_, err := p.Publish(topic, msg)
	return err
}

func (c *Client) Subscribe(topic string) (*Subscription, error) {
	s := NewSubscriber(c.Url)
	s.History.Len = 100
	return s.Subscribe(topic)
}

func (c *Client) basicRoundTrip(verb, url string) (*http.Response, error) {
	t := &transport.Transport{InsecureTLSDial: c.InsecureTLSDial}

	req, err := http.NewRequest(verb, url, nil)
	if err != nil {
		return nil, err
	}

	return t.RoundTrip(req)
}

// Publisher is responsible for publishing messages to the service.
type Publisher struct {
	Url    string
	Header http.Header
}

// NewPublisher creates a new Publisher.
func NewPublisher(url string) *Publisher {
	return &Publisher{
		Url:    url,
		Header: map[string][]string{},
	}
}

// Publish sends the service a message for a particular topic.
func (p *Publisher) Publish(topic string, message []byte) (*http.Response, error) {

	if len(message) == 0 {
		return nil, errors.New("Cannot send an empty message")
	}
	if len(topic) == 0 {
		return nil, errors.New("Cannot publish to an empty topic.")
	}
	/* HTTP2 does not currently send the body! So we have to go to HTTP1
	t := &transport.Transport{InsecureTLSDial: true}
	*/
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	url := p.Url + path.Join(v1Path, topic)

	var body bytes.Buffer
	body.Write(message)

	req, _ := http.NewRequest("POST", url, &body)

	return t.RoundTrip(req)
}

// Subscription represents an existing subscription that a subscriber
// has subscribed to.
type Subscription struct {
	C        chan []byte
	listener transport.Listener
}

func (s *Subscription) Cancel() {
	// Signal the transport that the clientStream should be removed.
	s.listener.Cancel()
}

// Subscriber defines a client that subscribes to a topic on a PubSub.
type Subscriber struct {
	Url     string
	History History
	Header  http.Header
}

func NewSubscriber(url string) *Subscriber {
	return &Subscriber{
		Url:    url,
		Header: map[string][]string{},
	}
}

// History describes how much history a subscriber should ask for.
//
// Be default, Subscribers do not ask for any history.
type History struct {
	Since time.Time
	Len   int
}

func (s *Subscriber) Subscribe(topic string) (*Subscription, error) {
	if len(topic) == 0 {
		return nil, errors.New("Cannot subscribe to an empty channel.")
	}

	url := s.Url + path.Join(v1Path, topic)
	fmt.Printf("URL: %s\n", url)

	t := &transport.Transport{InsecureTLSDial: true}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setHeaders(req)

	_, listener, err := t.Listen(req)
	if err != nil {
		return nil, err
	}

	stream, err := listener.Stream()
	if err != nil {
		return nil, err
	}

	return &Subscription{C: stream, listener: listener}, nil
}

func (s *Subscriber) setHeaders(req *http.Request) {
	req.Header = s.Header
	if s.History.Len > 0 {
		req.Header.Add("X-History-Length", fmt.Sprintf("%d", s.History.Len))
	}
	if s.History.Since.After(time.Unix(0, 0)) {
		req.Header.Add("X-History-Since", fmt.Sprintf("%d", s.History.Since.Unix()))
	}
}
