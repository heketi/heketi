//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package middleware

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
	"github.com/urfave/negroni"
)

func TestNewHTTPThrottler(t *testing.T) {

	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)

}

func TestReachedMaxRequest(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt.reachedMaxRequest() == false)
	nt.servingCount = 10
	tests.Assert(t, nt.reachedMaxRequest() == true)

}

func TestIsSuccess(t *testing.T) {
	s := isSuccess(200)
	tests.Assert(t, s == true)
	s = isSuccess(400)
	tests.Assert(t, s == false)

}

func TestServeHTTP(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)

	n := negroni.New(nt)
	n.UseHandlerFunc(mw)

	ts := httptest.NewServer(n)
	defer ts.Close()
	r, err := http.Get(ts.URL)

	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)

}

func TestServeHTTPDelete(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)

	n := negroni.New(nt)
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()
	r, err := http.NewRequest("DELETE", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusAccepted)

}

func TestServeHTTPTrottleTomanyReq(t *testing.T) {
	nt := NewHTTPThrottler(9)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()

	func(n int) {
		for i := 0; i < n; i++ {
			_, r := createReq("DELETE", ts.URL)
			client.Do(r)
		}

	}(10)

	r, err := http.NewRequest("DELETE", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusTooManyRequests)

}

func TestServeHTTPTrottle(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)

	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()

	func(n int) {
		for i := 0; i < n; i++ {
			uid, r := createReq("POST", ts.URL)
			client.Do(r)
			http.Get(ts.URL + "/volume/" + uid)
		}

	}(20)

	r, err := http.NewRequest("POST", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusAccepted)

}

func TestServeHTTPTrottleQueue(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)

	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()
	var url string
	func(n int) {
		for i := 0; i < n; i++ {
			uid, r := createReq("POST", ts.URL)
			client.Do(r)
			url = ts.URL + "/volume/" + uid
		}

	}(10)

	http.Get(url)
	_, r := createReq("POST", ts.URL)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusAccepted)
	_, r = createReq("POST", ts.URL)
	resp, err = client.Do(r)

	tests.Assert(t, resp != nil)
	tests.Assert(t, resp.StatusCode == http.StatusTooManyRequests)

}
func TestThrottlingCleanup(t *testing.T) {

	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)
	tt := time.Now()
	throttleNow = func() time.Time { return tt }
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()
	defer nt.Stop()
	go nt.Cleanup(time.Second * 2)
	func(n int) {
		for i := 0; i < n; i++ {
			_, r := createReq("POST", ts.URL)
			client.Do(r)
		}

	}(10)
	time.Sleep(2 * time.Second)
	_, r := createReq("POST", ts.URL)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusAccepted, resp.StatusCode)

}

var createReq = func(method, url string) (string, *http.Request) {
	uid := utils.GenUUID()
	values := map[string]string{"RequestId": uid}
	jsonValue, _ := json.Marshal(values)
	r, _ := http.NewRequest(method, url, bytes.NewBuffer(jsonValue))
	return uid, r
}

var mw = func(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost, http.MethodDelete:
		body, _ := ioutil.ReadAll(r.Body)
		data := make(map[string]string)
		json.Unmarshal(body, &data)
		rw.Header().Set("X-Request-ID", data["RequestId"])
		rw.Header().Set("X-Pending", "true")
		rw.WriteHeader(http.StatusAccepted)
	case http.MethodGet:
		rw.WriteHeader(http.StatusOK)

	}

}
