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
	http.CloseNotifier
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
	// Close and destroy the topic.
	Close() error
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
	mx          sync.RWMutex
	closed      bool
}

func (t *channeledTopic) Close() error {
	t.mx.Lock()
	t.closed = true
	for _, s := range t.subscribers {
		s.Close()
	}
	t.subscribers = map[uint64]*Subscription{}
	t.mx.Unlock()
	return nil
}

func (t *channeledTopic) Publish(msg []byte) error {
	if t.closed {
		return errors.New("Topic is being deleted.")
	}
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
	//fmt.Printf("Message sent.\n")
	return nil
}

func (t *channeledTopic) Subscribe(s *Subscription) {
	if t.closed {
		return
	}
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
	if t.closed {
		return
	}
	t.mx.Lock()
	defer t.mx.Unlock()
	delete(t.subscribers, s.Id)
}

func (t *channeledTopic) Name() string {
	return t.name
}

func (t *channeledTopic) Subscribers() []*Subscription {
	t.mx.RLock()
	defer t.mx.RUnlock()
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

// Listen copies messages fromt the Queue into the Writer.
//
// It listens on the Queue unless the `stop` channel receives a message.
func (s *Subscription) Listen(stop <-chan bool) {
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
		case <-stop:
			//fmt.Printf("Subscription ended.\n")
			return
		default:
		}
	}
}

// Close closes things and cleans up.
func (s *Subscription) Close() {
	close(s.Queue)
}

// getMedium fetches the Medium from the Datasources list.
func getMedium(c cookoo.Context) (*Medium, error) {
	ds, ok := c.HasDatasource(MediumDS)
	if !ok {
		return nil, errors.New("Cannot find a Medium")
	}
	return ds.(*Medium), nil
}

// NewMedium creates and initializes a Medium.
func NewMedium() *Medium {
	return &Medium{
		topics: make(map[string]Topic, 256), // Premature optimization...
	}
}

// Medium handles channeling messages to topics.
//
// You should always create one with NewMedium or else you will not be able
// to add new topics.
type Medium struct {
	topics map[string]Topic
	mx     sync.RWMutex
}

// Topic gets a Topic by name.
//
// If no topic is found, the ok flag will return false.
func (m *Medium) Topic(name string) (Topic, bool) {
	m.mx.RLock()
	defer m.mx.RUnlock()
	t, ok := m.topics[name]
	return t, ok
}

// Add a new Topic to the Medium.
func (m *Medium) Add(t Topic) {
	m.mx.Lock()
	m.topics[t.Name()] = t
	m.mx.Unlock()
}

// Delete closes a topic and removes it.
func (m *Medium) Delete(name string) error {
	t, ok := m.topics[name]
	if !ok {
		return fmt.Errorf("Cannot delete. No topic named %s.", name)
	}
	t.Close()
	m.mx.Lock()
	delete(m.topics, name)
	m.mx.Unlock()
	return nil
}

var lastSubId uint64 = 0

// newSubId returns an atomically incremented ID.
//
// This can probably be done better.
func newSubId() uint64 {
	z := atomic.AddUint64(&lastSubId, uint64(1))
	// FIXME: And when we hit max? Rollover?
	if z == math.MaxUint64 {
		atomic.StoreUint64(&lastSubId, uint64(0))
	}
	return z
}
