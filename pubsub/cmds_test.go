package pubsub

import (
	"net/http"
	"os"
	"testing"

	"github.com/Masterminds/cookoo"
)

func TestReplayHistory(t *testing.T) {
	reg, router, cxt := cookoo.Cookoo()
	cxt.AddLogger("out", os.Stdout)

	medium := NewMedium()
	cxt.AddDatasource(MediumDS, medium)

	topic := NewHistoriedTopic("test", 5)
	medium.Add(topic)

	topic.Publish([]byte("first"))
	topic.Publish([]byte("second"))

	req, _ := http.NewRequest("GET", "https://localhost/v1/t/test", nil)
	req.Header.Add(XHistoryLength, "4")
	res := &mockResponseWriter{}

	cxt.Put("http.Request", req)
	cxt.Put("http.ResponseWriter", res)

	reg.Route("test", "Test route").
		Does(ReplayHistory, "res").Using("topic").WithDefault("test")

	err := router.HandleRequest("test", cxt, true)
	if err != nil {
		t.Error(err)
	}

	last := res.String()
	if last != "firstsecond" {
		t.Errorf("Expected 'firstsecond', got '%s'", last)
	}

}
