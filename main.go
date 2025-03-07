package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (c *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (c *apiConfig) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(200)
	out := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", c.fileServerHits.Load())
	w.Write([]byte(out))
}

func (c *apiConfig) reset(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	c.fileServerHits.Swap(0)
}

func main() {
	serveMux := http.NewServeMux()
	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	metrics := apiConfig{}
	prefixHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", metrics.middlewareMetricsInc(prefixHandler))
	serveMux.HandleFunc("GET /api/healthz", healthz)
	serveMux.HandleFunc("GET /admin/metrics", metrics.metrics)
	serveMux.HandleFunc("POST /admin/reset", metrics.reset)
	server.ListenAndServe()
}
