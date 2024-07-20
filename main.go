package main

import (
	"encoding/json"
	"golang.org/x/time/rate"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Message struct {
	Status string `json:"status"`
	Body   string `json:"body"`
}

func endpointHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	message := Message{
		Status: "Successful",
		Body:   "Hi! You've reached the API. How may I help you?",
	}
	err := json.NewEncoder(writer).Encode(&message)
	if err != nil {
		return
	}
}

func perClientRateLimiter(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	type client struct {
		LastSeen time.Time
		Limiter  *rate.Limiter
	}
	clients := make(map[string]*client)
	rmu := &sync.RWMutex{}
	go func() {
		for {
			time.Sleep(60 * time.Second)
			for ip, c := range clients {
				if time.Since(c.LastSeen) > time.Minute {
					rmu.Lock()
					delete(clients, ip)
					rmu.Unlock()
				}
			}
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if _, ok := clients[ip]; !ok {
			rmu.Lock()
			clients[ip] = &client{
				Limiter: rate.NewLimiter(5, 20),
			}
			rmu.Unlock()
		}
		rmu.RLock()
		defer rmu.RUnlock()
		c := clients[ip]
		c.LastSeen = time.Now()
		clients[ip] = c
		if !c.Limiter.Allow() {
			message := Message{
				Status: "Request Failed",
				Body:   "The API is at capacity, try again later.",
			}
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(&message)
			return
		}
		message := Message{
			Status: "Request OK",
			Body:   "OK",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&message)
		return
	})
}

func main() {
	http.Handle("/ping", perClientRateLimiter(endpointHandler))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("There was an error listening on port :8080", err)
	}
}
