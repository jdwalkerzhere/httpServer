package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jdwalkerzhere/httpServer/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
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

func (c *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	c.fileServerHits.Swap(0)
	c.db.Reset(r.Context())
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

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type userFields struct {
		Email string `json:"email"`
	}
	defer r.Body.Close()

	fields := userFields{}
	err := json.NewDecoder(r.Body).Decode(&fields)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Request [%s] Malformed", r.Body)))
		return
	}
	timeNow := time.Now()
	userParams := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
		Email:     fields.Email,
	}
	dbUser, err := cfg.db.CreateUser(r.Context(), userParams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Could not create user"))
	}
	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		os.Exit(1)
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	serveMux := http.NewServeMux()
	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	apiConFig := apiConfig{db: dbQueries}
	prefixHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", apiConFig.middlewareMetricsInc(prefixHandler))
	serveMux.HandleFunc("GET /api/healthz", healthz)
	serveMux.HandleFunc("GET /admin/metrics", apiConFig.metrics)
	serveMux.HandleFunc("POST /admin/reset", apiConFig.reset)
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirp)
	serveMux.HandleFunc("POST /api/users", apiConFig.createUser)
	server.ListenAndServe()
}
