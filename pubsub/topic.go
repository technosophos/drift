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

type Topic interface {
	Publish([]byte) error
	Subscribe(*Subscription)
	Unsubscribe(*Subscription)
	Name() string
}

type History interface {
	Last(int) [][]byte
	Since(time.Time) [][]byte
}

type HistoryTopic interface {
	History
	Topic
}

func NewTopic(name string) Topic {
	return &channeledTopic{
		name:        name,
		subscribers: make(map[uint64]*Subscription, 512), // Sane default space?
	}
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
		fmt.Printf("Sending msg to subscriber %d: %s\n", s.Id, msg)
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
	fmt.Printf("There are now %d subscribers", len(t.subscribers))
}

func (t *channeledTopic) Unsubscribe(s *Subscription) {
	t.mx.Lock()
	defer t.mx.Unlock()
	delete(t.subscribers, s.Id)
}

func (t *channeledTopic) Name() string {
	return t.name
}

type Subscription struct {
	Id     uint64
	Writer http.ResponseWriter
	Queue  chan []byte
}

func NewSubscription(r http.ResponseWriter) *Subscription {
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
			fmt.Printf("Forwarding message.\n")
			// Queue is always serial, and this should be the only writer to the
			// RequestWriter, so we don't explicitly sync right now.
			s.Writer.Write(msg)
			s.Writer.(http.Flusher).Flush()
		case <-until:
			fmt.Printf("Subscription ended.\n")
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
