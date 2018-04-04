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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heketi/tests"
	"github.com/urfave/negroni"
)

func TestNewHTTPThrottler(t *testing.T) {

	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)

}

func TestReachedMaxRequest(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt.reachedMaxRequest() != false)
	nt.ServingCount = 11
	tests.Assert(t, nt.reachedMaxRequest() != true)

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

	called := false
	mw := func(rw http.ResponseWriter, r *http.Request) {
		called = true
	}
	n.UseHandlerFunc(mw)

	ts := httptest.NewServer(n)
	r, err := http.Get(ts.URL)
	tests.Assert(t, err == nil)
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, called == true)

}

func TestServeHTTPDelete(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)

	called := false
	mw := func(rw http.ResponseWriter, r *http.Request) {
		called = true
	}
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	r, err := http.NewRequest("DELETE", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusOK)
	tests.Assert(t, called == true)

}

func TestServeHTTPTrottleTomanyReq(t *testing.T) {
	nt := NewHTTPThrottler(9)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)

	called := false
	mw := func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("X-Request-ID", "12345")
		called = true
	}
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()
	go func(n int) {
		for i := 0; i < n; i++ {
			r, _ := http.NewRequest("DELETE", ts.URL, nil)
			client.Do(r)
		}

	}(10)
	r, err := http.NewRequest("DELETE", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusTooManyRequests)
	//should not get called
	tests.Assert(t, called == false)

}

func TestServeHTTPTrottle(t *testing.T) {
	nt := NewHTTPThrottler(10)
	tests.Assert(t, nt != nil)
	n := negroni.New(nt)

	called := false
	mw := func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodDelete:
			rw.Header().Set("X-Request-ID", "12345")
			rw.Header().Set("X-Pending", "True")
			rw.WriteHeader(http.StatusAccepted)
		case http.MethodGet:
			rw.Header().Del("X-Pending")
			rw.WriteHeader(http.StatusOK)

		}

		called = true
	}
	n.UseHandlerFunc(mw)
	client := http.Client{}
	ts := httptest.NewServer(n)
	defer ts.Close()
	go func(n int) {
		for i := 0; i < n; i++ {
			r, _ := http.NewRequest("POST", ts.URL, nil)
			client.Do(r)
			http.Get(ts.URL + "/volume/12345")
		}

	}(20)
	r, err := http.NewRequest("POST", ts.URL, nil)
	tests.Assert(t, err == nil)
	resp, err := client.Do(r)
	tests.Assert(t, err == nil)
	tests.Assert(t, resp.StatusCode == http.StatusAccepted, resp.StatusCode)
	tests.Assert(t, called == true)

}
