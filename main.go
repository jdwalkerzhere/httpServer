package main

import (
	"net/http"
)

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func main() {
	serveMux := http.NewServeMux()
	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	prefixHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", prefixHandler)
	serveMux.HandleFunc("/healthz", healthz)
	server.ListenAndServe()
}
