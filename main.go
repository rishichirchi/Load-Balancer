package main

import (
	"net/http"
	"net/url"
)

//Backend

type Backend interface{
	SetAlive(bool)
	IsAlive() bool
	GetUrl() *url.URL
	GetActiveConnections() int
	Serve(http.ResponseWriter, *http.Request)
}