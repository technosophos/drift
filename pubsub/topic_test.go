package pubsub

import (
	"bytes"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestSubscription(t *testing.T) {

	// canaries:
	var _ http.ResponseWriter = &mockResponseWriter{}
	var _ http.Flusher = &mockResponseWriter{}

	rw := &mockResponseWriter{}
	sub := NewSubscription(rw)

	rw2 := &mockResponseWriter{}
	sub2 := NewSubscription(rw2)

	if sub.Id == sub2.Id {
		t.Error("Two subscriptions have the same ID!!!!")
	}

	// Make sure the Queue is buffered.
	sub.Queue <- []byte("hi")
	out := <-sub.Queue

	if string(out) != "hi" {
		t.Error("Expected out to be 'hi'")
	}

	// Make sure that listen works.
	until := make(chan bool)
	go sub.Listen(until)
	sub.Queue <- []byte("hi")

	time.Sleep(2 * time.Millisecond)
	until <- true

	sub.Close()
	if rw.String() != "hi" {
		t.Errorf("Expected bytes 'hi', got '%s'", rw.String())
	}

}

func TestTopic(t *testing.T) {
	topic := NewTopic("test")

	if topic.Name() != "test" {
		t.Errorf("Expected name 'test', got '%s'", topic.Name())
	}

	subs := make([]*Subscription, 50)

	// Subscribe 50 times.
	for i := 0; i < 50; i++ {
		rw := &mockResponseWriter{}
		sub := NewSubscription(rw)
		subs[i] = sub
		done := make(chan bool)
		topic.Subscribe(sub)
		go sub.Listen(done)
	}

	topic.Publish([]byte("hi"))
	topic.Publish([]byte("there"))

	if len(topic.Subscribers()) != 50 {
		t.Errorf("Expected 50 subscribers, got %d.", len(topic.Subscribers()))
	}

	time.Sleep(5 * time.Millisecond)

	for _, s := range topic.Subscribers() {
		mw := s.Writer.(*mockResponseWriter).String()
		if mw != "hithere" {
			t.Errorf("Expected Subscription %d to have 'hithere'. Got '%s'", s.Id, mw)
		}

		topic.Unsubscribe(s)
	}

}

func BenchmarkTopic1Client(b *testing.B) {
	benchmarkTopic(1, b.N)
}

func BenchmarkTopic5Clients(b *testing.B) {
	benchmarkTopic(5, b.N)
}

// Create 50 subscribers and send 100 messages.
func BenchmarkTopic50Clients(b *testing.B) {
	benchmarkTopic(50, b.N)
}

/*
func BenchmarkTopic1Message(b *testing.B) {
	benchmarkTopic(b.N, 1)
}
func BenchmarkTopic5Message(b *testing.B) {
	benchmarkTopic(b.N, 1)
}
*/

func benchmarkTopic(scount, mcount int) {
	topic := NewTopic("test")
	subs := make([]*Subscription, scount)

	// Subscribe 50 times.
	for i := 0; i < scount; i++ {
		rw := &mockResponseWriter{}
		sub := NewSubscription(rw)
		subs[i] = sub
		done := make(chan bool)
		topic.Subscribe(sub)
		go sub.Listen(done)
	}

	for i := 0; i < mcount; i++ {
		topic.Publish([]byte("hi"))
	}

	//for _, s := range topic.Subscribers() {
	//	topic.Unsubscribe(s)
	//}

}

type mockResponseWriter struct {
	headers http.Header
	writer  bytes.Buffer
	mx      sync.Mutex
}

func (r *mockResponseWriter) Header() http.Header {
	if len(r.headers) == 0 {
		r.headers = make(map[string][]string, 1)
	}
	return r.headers
}
func (r *mockResponseWriter) Write(d []byte) (int, error) {
	r.mx.Lock()
	defer r.mx.Unlock()
	return r.writer.Write(d)
}
func (r *mockResponseWriter) Buf() []byte {
	r.mx.Lock()
	defer r.mx.Unlock()
	return r.writer.Bytes()
}
func (r *mockResponseWriter) String() string {
	r.mx.Lock()
	defer r.mx.Unlock()
	return r.writer.String()
}
func (r *mockResponseWriter) WriteHeader(c int) {
}

func (r *mockResponseWriter) Flush() {}

// For benchmarking.
type nilResponseWriter struct {
	headers http.Header
	writer  bytes.Buffer
	mx      sync.Mutex
}

func (r *nilResponseWriter) Header() http.Header {
	if len(r.headers) == 0 {
		r.headers = make(map[string][]string, 1)
	}
	return r.headers
}
func (r *nilResponseWriter) Write(d []byte) (int, error) {
	return len(d), nil
}
func (r *nilResponseWriter) WriteHeader(c int) {}

func (r *nilResponseWriter) Flush() {}
