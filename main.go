package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

//Backend
//backend interface
type Backend interface{
	SetAlive(bool)
	IsAlive() bool
	GetUrl() *url.URL
	GetActiveConnections() int
	Serve(http.ResponseWriter, *http.Request)
}

//backend struct
type backend struct {
	url *url.URL
	alive bool
	mux sync.RWMutex
	connections int
	reverseProxy *httputil.ReverseProxy
}