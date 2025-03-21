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
	"github.com/jdwalkerzhere/httpServer/internal/auth"
	"github.com/jdwalkerzhere/httpServer/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	authSecret     string
}

func (c *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (c *apiConfig) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	out := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", c.fileServerHits.Load())
	w.Write([]byte(out))
}

func (c *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	c.fileServerHits.Swap(0)
	c.db.Reset(r.Context())
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type ChirpRequest struct {
	Body string `json:"body"`
}

type httpError struct {
	Message string `json:"error"`
}

func (cfg *apiConfig) handlerChirp(w http.ResponseWriter, r *http.Request) {
	const maxChirpLength = 140
	profane := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}

	defer r.Body.Close()

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(httpError{"User Not Logged In"})
		return
	}

	id, err := auth.ValidateJWT(bearerToken, cfg.authSecret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(httpError{"Invalid JWT Token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	chirpRequest := ChirpRequest{}
	err = json.NewDecoder(r.Body).Decode(&chirpRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Something went wrong"})
		return
	}
	if len(chirpRequest.Body) > maxChirpLength {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Chirp is too long"})
		return
	}

	timeNow := time.Now()
	chirpParams := database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
		Body:      cleanChirp(Chirp{Body: chirpRequest.Body}, profane),
		UserID:    id,
	}
	chirp, err := cfg.db.CreateChirp(r.Context(), chirpParams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(httpError{"Error Saving Chirp"})
		return
	}
	w.WriteHeader(http.StatusCreated)
	chirpResponse := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	json.NewEncoder(w).Encode(chirpResponse)

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

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	Password  string    `json:"-"`
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type userFields struct {
		Password string `json:"password"`
		Email    string `json:"email"`
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
	hashedPassword, err := auth.HashPassword(fields.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(httpError{"Error Hashing Password"})
		return
	}
	userParams := database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      timeNow,
		UpdatedAt:      timeNow,
		Email:          fields.Email,
		HashedPassword: hashedPassword,
	}
	dbUser, err := cfg.db.CreateUser(r.Context(), userParams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Could not create user"))
		return
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

func (cfg *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	uuidChirp, err := uuid.Parse(chirpID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Malformed Chirp UUID"})
		return
	}
	dbChirp, err := cfg.db.GetChirp(r.Context(), uuidChirp)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(httpError{fmt.Sprintf("No Chirp by [%s] id found", chirpID)})
		return
	}
	respChirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respChirp)
}

func (cfg *apiConfig) getAllChirps(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(httpError{"Something went wrong"})
		return
	}
	respChirps := []Chirp{}
	for _, dbChirp := range dbChirps {
		chirp := Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
		respChirps = append(respChirps, chirp)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respChirps)
}

func (cfg *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		ExpiresIn int    `json:"expires_in_seconds"`
	}
	defer r.Body.Close()

	loginReq := loginRequest{}
	err := json.NewDecoder(r.Body).Decode(&loginReq)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(httpError{"Malformed Request"})
		return
	}
	user, err := cfg.db.GetUserByEmail(r.Context(), loginReq.Email)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(httpError{"User not found by Email"})
		return
	}
	if auth.CheckPasswordHash(loginReq.Password, user.HashedPassword) != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(httpError{"Incorrect Password"})
		return
	}
	if loginReq.ExpiresIn == 0 || loginReq.ExpiresIn > 3600 {
		loginReq.ExpiresIn = 3600
	}

	token, err := auth.MakeJWT(user.ID, cfg.authSecret, time.Duration(loginReq.ExpiresIn)*time.Second)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(httpError{"Error generating Auth Token"})
		return
	}

	userResp := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Token:     token,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userResp)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		os.Exit(1)
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	authSecret := os.Getenv("AUTH_SECRET")

	serveMux := http.NewServeMux()
	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	cfg := apiConfig{db: dbQueries, authSecret: authSecret}
	prefixHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(prefixHandler))
	serveMux.HandleFunc("GET /api/healthz", healthz)
	serveMux.HandleFunc("GET /admin/metrics", cfg.metrics)
	serveMux.HandleFunc("POST /admin/reset", cfg.reset)
	serveMux.HandleFunc("POST /api/chirps", cfg.handlerChirp)
	serveMux.HandleFunc("POST /api/users", cfg.createUser)
	serveMux.HandleFunc("GET /api/chirps", cfg.getAllChirps)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", cfg.getChirp)
	serveMux.HandleFunc("POST /api/login", cfg.login)
	server.ListenAndServe()
}
