package pubsub

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/cookoo"
)

// ResponseWriterFlusher handles both HTTP response writing and flushing.
//
// We use this simply to declare which interfaces we require support for.
type ResponseWriterFlusher interface {
	http.ResponseWriter
	http.Flusher
}

// Topic is the main channel for sending messages to subscribers.
//
// A publisher is anything that sends a message to a Topic. All
// attached subscribers will receive that message.
type Topic interface {
	// Publish sends a message to all subscribers.
	Publish([]byte) error
	// Subscribe attaches a subscription to this topic.
	Subscribe(*Subscription)
	// Unsubscribe detaches a subscription from the topic.
	Unsubscribe(*Subscription)
	// Name returns the topic name.
	Name() string
	// Subscribers returns a list of subscriptions attached to this topic.
	Subscribers() []*Subscription
}

// History provides access too the last N messages on a particular Topic.
type History interface {
	// Last provides access to up to N messages.
	Last(int) [][]byte
	// Since provides access to all messages in history since the given time.
	Since(time.Time) [][]byte
}

// HistoriedTopic is a topic that has an attached history.
type HistoriedTopic interface {
	History
	Topic
}

// NewTopic creates a new Topic with no history capabilities.
func NewTopic(name string) Topic {
	ct := &channeledTopic{
		name:        name,
		subscribers: make(map[uint64]*Subscription, 512), // Sane default space?
	}
	return ct
}

// NewHistoriedTopic creates a new HistoriedTopic.
//
// This topic will retain `length` history items for the topic.
func NewHistoriedTopic(name string, length int) HistoriedTopic {
	return TrackHistory(NewTopic(name), length)
}

type channeledTopic struct {
	name        string
	subscribers map[uint64]*Subscription
	mx          sync.Mutex
}

func (t *channeledTopic) Publish(msg []byte) error {
	t.mx.Lock()
	defer func() {
		t.mx.Unlock()
		if err := recover(); err != nil {
			fmt.Printf("Recovered from failed publish. Some messages probably didn't get through. %s\n", err)
		}
	}()
	for _, s := range t.subscribers {
		if s.Queue == nil {
			fmt.Printf("Channel appears to be closed. Skipping.\n")
			continue
		}
		//fmt.Printf("Sending msg to subscriber %d: %s\n", s.Id, msg)
		s.Queue <- msg
	}
	fmt.Printf("Message sent.\n")
	return nil
}

func (t *channeledTopic) Subscribe(s *Subscription) {
	t.mx.Lock()
	defer t.mx.Unlock()
	//t.subscribers = append(t.subscribers, s)
	if _, ok := t.subscribers[s.Id]; ok {
		fmt.Printf("Surprisingly got the same ID as an existing subscriber.")
	}
	t.subscribers[s.Id] = s
	//fmt.Printf("There are now %d subscribers", len(t.subscribers))
}

func (t *channeledTopic) Unsubscribe(s *Subscription) {
	t.mx.Lock()
	defer t.mx.Unlock()
	delete(t.subscribers, s.Id)
}

func (t *channeledTopic) Name() string {
	return t.name
}

func (t *channeledTopic) Subscribers() []*Subscription {
	c := len(t.subscribers)
	s := make([]*Subscription, 0, c)
	for _, v := range t.subscribers {
		s = append(s, v)
	}
	return s
}

// Subscription describes a subscriber.
//
// A subscription attaches to ONLY ONE Topic.
type Subscription struct {
	Id     uint64
	Writer ResponseWriterFlusher
	Queue  chan []byte
}

// NewSubscription creates a new subscription.
//
//
func NewSubscription(r ResponseWriterFlusher) *Subscription {
	// Queue depth should be revisited.
	q := make(chan []byte, 10)
	return &Subscription{
		Writer: r,
		Queue:  q,
		Id:     newSubId(),
	}
}

func (s *Subscription) Listen(until <-chan bool) {
	// s.Queue <- []byte("SUBSCRIBED")
	for {
		//for msg := range s.Queue {
		select {
		case msg := <-s.Queue:
			//fmt.Printf("Forwarding message.\n")
			// Queue is always serial, and this should be the only writer to the
			// RequestWriter, so we don't explicitly sync right now.
			s.Writer.Write(msg)
			s.Writer.Flush()
		case <-until:
			//fmt.Printf("Subscription ended.\n")
			return
		default:
		}
	}
}

func (s *Subscription) Close() {
	close(s.Queue)
}

func getMedium(c cookoo.Context) (*Medium, error) {
	ds, ok := c.HasDatasource(MediumDS)
	if !ok {
		return nil, errors.New("Cannot find a Medium")
	}
	return ds.(*Medium), nil
}

func NewMedium() *Medium {
	return &Medium{
		topics: make(map[string]Topic, 256), // Premature optimization...
	}
}

// Medium handles channeling messages to topics.
type Medium struct {
	topics map[string]Topic
	mx     sync.Mutex
}

func (m *Medium) Topic(name string) (Topic, bool) {
	t, ok := m.topics[name]
	return t, ok
}

func (m *Medium) Add(t Topic) {
	m.topics[t.Name()] = t
}

var lastSubId uint64 = 0

func newSubId() uint64 {
	z := atomic.AddUint64(&lastSubId, uint64(1))
	// FIXME: And when we hit max? Rollover?
	if z == math.MaxUint64 {
		atomic.StoreUint64(&lastSubId, uint64(0))
	}
	return z
}
