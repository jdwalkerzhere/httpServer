package main

import (
	"net/http"
)

func main() {
	serveMux := http.ServeMux{}
	server := http.Server{
		Handler: &serveMux,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
