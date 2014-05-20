package gce

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "projects/coreos-cloud/global/images/coreos-alpha-317-0-0-v20140515")
	}))
	defer ts.Close()
	url, err := getLatestImg(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	expect := "https://www.googleapis.com/compute/v1/projects/coreos-cloud/global/images/coreos-alpha-317-0-0-v20140515"
	if url != expect {
		t.Fatalf("got:\t%s\n expected:\t%s", url, expect)
	}
}
