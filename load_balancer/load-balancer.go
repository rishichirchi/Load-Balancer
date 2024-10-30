package main

import (
	// "context"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
	reverseProxy *httputil.ReverseProxy
}

//serverpool struct
type ServerPool struct{
	backends []*Backend
	current uint64
}

var serverpool ServerPool

func (s *ServerPool) NextIndex() int{
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.backends = append(s.backends, backend)
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

			log.Printf("Selected backend: %s\n", s.backends[idx].url)

			return s.backends[idx]
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

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool){
	for _, b := range s.backends{
		if b.url.String() == backendUrl.String(){
			b.SetAlive(alive)
			break
		}
	}
}

func lb(w http.ResponseWriter, r *http.Request){
	attempts := GetAttemptsFromContext(r)
	if attempts > 3{
		log.Printf("%s(%s) Max retries reached\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverpool.GetNextPeer()
	
	if peer != nil{
		peer.reverseProxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

func main(){
	var serverlist string
	var port int

	flag.StringVar(&serverlist, "backends", "", "Load balanced backends, use commas to separate")
	flag.IntVar(&port, "port", 3030, "Port to serve on")

	flag.Parse()
	

	if len(serverlist) == 0{
		log.Fatal("Please provide one or more backends to load balance")

	}

	tokens := strings.Split(serverlist, ",")

	for _, token := range tokens{
		serverUrl, err := url.Parse(token)

		if err != nil{
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)

		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error){
			log.Printf("[%s] %s\n", serverUrl, e.Error())
			retries := GetRetryFromContext(request)

			if retries < 3{
				select{
					case <- time.After(10*time.Millisecond):
						ctx := context.WithValue(request.Context(), Retry, retries+1)
						proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			serverpool.MarkBackendStatus(serverUrl, false)

			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)

			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			lb(writer, request.WithContext(ctx))
		}

		serverpool.AddBackend(&Backend{
			url: serverUrl,
			reverseProxy: proxy,
			alive: true,
		})

		log.Printf("Configured server: %s\n", serverUrl)
	}

	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}

	go healthCheck()

	log.Printf("Load balancer started at :%d\n", port)

	if err := server.ListenAndServe(); err != nil{
		log.Fatal(err)
	}
}