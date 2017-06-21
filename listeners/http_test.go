package listeners

import (
	"github.com/facebookgo/ensure"
	"github.com/millisecond/adaptlb/testutil"
	"net/http"
	"strconv"
	"testing"
)

func TestHTTPListener(t *testing.T) {
	port := 8000
	err := AddHTTPPort(port)
	ensure.Nil(t, err)

	url := "http://localhost:" + strconv.Itoa(port)

	resp, err := http.Get(url)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, resp.StatusCode, 200)
	ensure.DeepEqual(t, testutil.MustBody(t, resp), "OK")

	err = RemoveHTTPPort(port)
	ensure.Nil(t, err)

	// Shut it down and verify failure
	resp, err = http.Get(url)
	ensure.NotNil(t, err)
}
