// +build !go1.13

//
// Copyright (c) 2020 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), as published by the Free Software Foundation,
// or under the Apache License, Version 2.0 <LICENSE-APACHE2 or
// http://www.apache.org/licenses/LICENSE-2.0>.
//
// You may not use this file except in compliance with those terms.
//

package client

import "net/http"

// make a copy of the default transport at package initialization time
var defaultTransportCopy = copyTransport(http.DefaultTransport.(*http.Transport))

// manually copy all public fields available from go1.10+
// TODO: remove this once go1.13 is the minimum supported version
func copyTransport(in *http.Transport) *http.Transport {
	if in == nil {
		return nil
	}
	return &http.Transport{
		Proxy:                  in.Proxy,
		DialContext:            in.DialContext,
		Dial:                   in.Dial,
		DialTLS:                in.DialTLS,
		TLSClientConfig:        in.TLSClientConfig,
		TLSHandshakeTimeout:    in.TLSHandshakeTimeout,
		DisableKeepAlives:      in.DisableKeepAlives,
		DisableCompression:     in.DisableCompression,
		MaxIdleConns:           in.MaxIdleConns,
		MaxIdleConnsPerHost:    in.MaxIdleConnsPerHost,
		IdleConnTimeout:        in.IdleConnTimeout,
		ResponseHeaderTimeout:  in.ResponseHeaderTimeout,
		ExpectContinueTimeout:  in.ExpectContinueTimeout,
		TLSNextProto:           in.TLSNextProto,
		ProxyConnectHeader:     in.ProxyConnectHeader,
		MaxResponseHeaderBytes: in.MaxResponseHeaderBytes,
	}
}

func defaultTransportClone() *http.Transport {
	return copyTransport(defaultTransportCopy)
}
