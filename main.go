package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

//backend struct
type Backend struct {
	url *url.URL
	alive bool
	mux sync.RWMutex
	connections int
	reverseProxy *httputil.ReverseProxy
}

//serverpool struct
type ServerPool struct{
	backends []Backend
	mux sync.RWMutex
	current int
}

func (s *ServerPool) NextIndex() int{
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) GetNextPeer() *Backend{
	next := s.NextIndex();//get the next index
	l := len(s.backends) + next; //start from next and go around the circle

	for i := next; i < l; i++{
		idx := i%len(s.backends);//get the index

		if s.backends[idx].IsAlive(){//check if the backend is alive
			if i != next{//if the next index is not the same as the current index
				atomic.StoreUint64(&s.current, uint64(idx))//set the current index to the next index
			}

			return &s.backends[idx]
		}
	}

	return nil
}

func (backend *Backend) SetAlive(alive bool){
	backend.mux.Lock()
	backend.alive = alive
	backend.mux.UnLock()
}

func (backend *Backend) IsAlive() (alive bool){
	backend.mux.RLock()
	alive = backend.alive
	backend.mux.RUnlock()

	return
}