package httputil

import (
	"github.com/Masterminds/cookoo"
	"strconv"
	"testing"
)

func TestTimestamp(t *testing.T) {
	reg, router, cxt := cookoo.Cookoo()

	reg.Route("test", "Test route").
		Does(Timestamp, "res")

	err := router.HandleRequest("test", cxt, true)
	if err != nil {
		t.Error(err)
	}

	ts := cxt.Get("res", "").(string)

	if len(ts) == 0 {
		t.Errorf("Expected timestamp, not empty string.")
	}

	tsInt, err := strconv.Atoi(ts)
	if err != nil {
		t.Error(err)
	}

	if tsInt <= 5 {
		t.Error("Dude, you're stuck in the '70s.")
	}
}
