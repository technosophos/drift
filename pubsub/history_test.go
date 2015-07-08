package pubsub

import (
	"bytes"
	"testing"
)

func TestHistory(t *testing.T) {
	topic := NewHistoriedTopic("test", 5)

	for _, s := range []string{"a", "b", "c", "d", "e", "f"} {
		topic.Publish([]byte(s))
	}

	short := topic.Last(1)
	if len(short) != 1 {
		t.Errorf("Expected 1 in list, got %d", len(short))
	}
	if string(short[0]) != "f" {
		t.Errorf("Expected 'f', got '%s'", short[0])
	}

	long := topic.Last(6)
	if len(long) != 5 {
		t.Errorf("Expected 5 in list, got %d", len(long))
	}

	str := string(bytes.Join(long, []byte("")))
	if str != "fedcb" {
		t.Errorf("Expected fedbc, got %s", str)
	}

}
