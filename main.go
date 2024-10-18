package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

//Backend
//backend struct
type Backend struct {
	url *url.URL
	alive bool
	mux sync.RWMutex
	connections int
	reverseProxy *httputil.ReverseProxy
}

//Serverpool
//serverpool struct
type ServerPool struct{
	backends []Backend
	mux sync.RWMutex
	current int
}
