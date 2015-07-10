package pubsub

import (
	"container/list"
	"sync"
	"time"
)

var DefaultMaxHistory = 1000

// historyTopic maintains the history for a channel.
type historyTopic struct {
	Topic
	buffer *list.List
	max    int
	mx     sync.Mutex
}

type entry struct {
	msg []byte
	ts  time.Time
}

// TrackHistory takes an existing topic and adds history tracking.
//
// The mechanism for history tracking is a doubly linked list no longer than
// maxLen.
func TrackHistory(t Topic, maxLen int) HistoriedTopic {
	return &historyTopic{
		Topic:  t,
		buffer: list.New(),
		max:    maxLen,
	}
}

// Since fetches an array of history entries.
//
// The entries will be in order, oldest to newest. And the list will not
// exceed the maximum number of histry items.
//
// If the history list grows beyond its max size, the history list is pruned,
// oldest to youngest.
func (h *historyTopic) Since(t time.Time) [][]byte {

	accumulator := [][]byte{}

	for v := h.buffer.Front(); v != nil; v = v.Next() {
		e, ok := v.Value.(*entry)
		if !ok {
			// Skip anything that's not an entry.
			continue
		}
		if e.ts.After(t) {
			accumulator = append(accumulator, e.msg)
		} else {
			return accumulator
		}
	}
	return accumulator
}

// Last fetches the last n items from the history, regardless of their time.
//
// Of course, it will return fewer than n if n is larger than the max length
// or if the total stored history is less than n.
func (h *historyTopic) Last(n int) [][]byte {
	acc := make([][]byte, 0, n)
	i := 0
	for v := h.buffer.Front(); v != nil; v = v.Next() {
		e, ok := v.Value.(*entry)
		if !ok {
			// Skip anything that's not an entry.
			continue
		}
		if i < n {
			acc = append(acc, e.msg)
		} else {
			return acc
		}
		i++
	}
	return acc
}

func (h *historyTopic) add(msg []byte) {
	h.mx.Lock()
	defer h.mx.Unlock()
	e := &entry{
		msg: msg,
		ts:  time.Now(),
	}

	h.buffer.PushBack(e)

	for h.buffer.Len() > h.max {
		h.buffer.Remove(h.buffer.Front())
	}
}

// Publish stores this msg as history and then forwards the publish request to the Topic.
func (h *historyTopic) Publish(msg []byte) error {
	h.add(msg)
	h.Topic.Publish(msg)
	return nil
}

func (h *historyTopic) Close() error {
	err := h.Topic.Close()
	// We don't want nil pointers during shutdown.
	h.buffer = list.New()
	return err
}
