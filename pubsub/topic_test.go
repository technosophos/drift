package pubsub

import (
	"bytes"
	"net/http"
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
	if rw.writer.String() != "hi" {
		t.Errorf("Expected bytes 'hi', got '%s'", rw.writer.Bytes())
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
		mw := s.Writer.(*mockResponseWriter).writer
		if mw.String() != "hithere" {
			t.Errorf("Expected Subscription %d to have 'hithere'. Got '%s'", s.Id, mw.String())
		}

		topic.Unsubscribe(s)
	}

}

type mockResponseWriter struct {
	headers http.Header
	writer  bytes.Buffer
}

func (r *mockResponseWriter) Header() http.Header {
	return r.headers
}
func (r *mockResponseWriter) Write(d []byte) (int, error) {
	return r.writer.Write(d)
}
func (r *mockResponseWriter) WriteHeader(c int) {
}

func (r *mockResponseWriter) Flush() {}
