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

type Publisher struct {
	Url    string
	Header http.Header
}

func NewPublisher(url string) *Publisher {
	return &Publisher{
		Url:    url,
		Header: map[string][]string{},
	}
}

func (p *Publisher) Publish(topic string, message []byte) error {

	if len(message) == 0 {
		return errors.New("Cannot send an empty message")
	}
	if len(topic) == 0 {
		return errors.New("Cannot publish to an empty topic.")
	}
	/* HTTP2 does not currently send the body! So we have to go to HTTP1
	t := &transport.Transport{InsecureTLSDial: true}
	*/
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	url := p.Url + path.Join(v1Path, topic)

	var body bytes.Buffer
	body.Write(message)

	req, _ := http.NewRequest("POST", url, &body)

	_, err := t.RoundTrip(req)
	return err
}

// Subscriber defines a client that subscribes to a topic on a PubSub.
type Subscriber struct {
	Url     string
	History *History
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

func (s *Subscriber) Subscribe(topic string) (chan []byte, error) {
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

	_, stream, err := t.Listen(req)
	if err != nil {
		return nil, err
	}

	return stream, nil
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
