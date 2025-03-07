package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

type Chirp struct {
	Body string `json:"body"`
}

func cleanChirp(c Chirp, p map[string]bool) string {
	cleanedString := []string{}
	for _, word := range strings.Split(c.Body, " ") {
		_, ok := p[strings.ToLower(word)]
		if ok {
			cleanedString = append(cleanedString, "****")
			continue
		}
		cleanedString = append(cleanedString, word)
	}
	return strings.Join(cleanedString, " ")
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	const maxChirpLength = 140
	profane := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}

	type httpError struct {
		Message string `json:"error"`
	}
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	chirp := Chirp{}
	err := decoder.Decode(&chirp)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Something went wrong"})
		return
	}

	if len(chirp.Body) > maxChirpLength {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Chirp is too long"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"cleaned_body": cleanChirp(chirp, profane)})
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
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirp)
	server.ListenAndServe()
}
