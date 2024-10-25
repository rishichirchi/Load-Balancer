package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Attempts int = iota
	Retry
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
	current uint64
}

var serverpool ServerPool

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
	backend.mux.Unlock()
}

func (backend *Backend) IsAlive() (alive bool){
	backend.mux.RLock()
	alive = backend.alive
	backend.mux.RUnlock()

	return
}

func GetAttemptsFromContext(r *http.Request) int{
	if attempts, ok := r.Context().Value(Attempts).(int); ok{
		return attempts
	}
	return 1
}

func GetRetryFromContext(r *http.Request) int{
	if retry, ok := r.Context().Value(Retry).(int); ok{
		return retry
	}
	return 0
}

func (s *ServerPool) HealthCheck(){
	for _, b := range s.backends{
		status := "up"
		alive := isBackendAlive(b.url)
		b.SetAlive(alive)

		if !alive{
			status = "down"
		}

		log.Printf("%s [%s]\n", b.url, status)
	}
}

func isBackendAlive(url *url.URL) bool{
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", url.Host, timeout)
	if err != nil{
		log.Println("Site unreachable, error: ", err)
		return false
	}
	defer conn.Close()

	return true
}

func healthCheck(){
	t := time.NewTicker(time.Minute * 2)

	for{
		select{
		case <- t.C: 
			log.Println("Starting health check...")
			serverpool.HealthCheck()
			log.Println("Health check completed")
		}
	}
}