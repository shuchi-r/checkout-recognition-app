package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"github.com/joho/godotenv"
)

type App struct {
	db            *pgxpool.Pool
	allowedOrigin string
	sessionSecret []byte
}

type registerRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type checkoutRequest struct {
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	ShippingAddress string `json:"shippingAddress"`
}

type loginRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func main() {
	godotenv.Load(".env")
	port := getenv("PORT", "8080")
	databaseURL := mustEnv("DATABASE_URL")
	allowedOrigin := getenv("ALLOWED_ORIGIN", "http://localhost:5173")
	sessionSecret := mustEnv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be at least 32 characters")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	app := &App{db: db, allowedOrigin: allowedOrigin, sessionSecret: []byte(sessionSecret)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("POST /api/register", app.register)
	mux.HandleFunc("POST /api/recognize", app.recognize)
	mux.HandleFunc("POST /api/login", app.login)
	mux.HandleFunc("GET /api/me", app.me)
	mux.HandleFunc("POST /api/logout", app.logout)
	mux.HandleFunc("POST /api/checkout", app.checkout)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.cors(app.logging(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if !validEmail(req.Email) || req.FirstName == "" || req.LastName == "" {
		errorJSON(w, http.StatusBadRequest, "Please provide a valid email, first name, and last name.")
		return
	}

	code, err := generateCode()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "Could not generate login code.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "Could not secure login code.")
		return
	}

	_, err = a.db.Exec(r.Context(), `
        INSERT INTO users (id, email, first_name, last_name, code_hash)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (email) DO UPDATE SET
            first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            code_hash = EXCLUDED.code_hash,
            updated_at = NOW()
    `, uuid.New(), req.Email, req.FirstName, req.LastName, string(hash))
	if err != nil {
		log.Printf("register: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Could not register user.")
		return
	}

	var id uuid.UUID
	if err := a.db.QueryRow(r.Context(), `SELECT id FROM users WHERE email = $1`, req.Email).Scan(&id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "Could not load registered user.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Registration successful.",
		"code":    code,
		"user":    userResponse{ID: id.String(), Email: req.Email, FirstName: req.FirstName, LastName: req.LastName},
	})
}

func (a *App) recognize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if !validEmail(email) {
		writeJSON(w, http.StatusOK, map[string]bool{"registered": false})
		return
	}

	var exists bool
	err := a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "Recognition check failed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"registered": exists})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Code = strings.TrimSpace(req.Code)

	if !validEmail(req.Email) || len(req.Code) != 6 || !allDigits(req.Code) {
		errorJSON(w, http.StatusBadRequest, "Enter the 6-digit code.")
		return
	}

	var id uuid.UUID
	var hash, firstName, lastName string
	err := a.db.QueryRow(r.Context(), `
        SELECT id, code_hash, first_name, last_name
        FROM users WHERE email = $1
    `, req.Email).Scan(&id, &hash, &firstName, &lastName)
	if err != nil {
		if err == pgx.ErrNoRows {
			errorJSON(w, http.StatusUnauthorized, "Invalid email or code.")
			return
		}
		errorJSON(w, http.StatusInternalServerError, "Login failed.")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Code)) != nil {
		errorJSON(w, http.StatusUnauthorized, "Incorrect code. Please try again.")
		return
	}

	setSessionCookie(w, r, a.signSession(id))
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Logged in.",
		"user":    userResponse{ID: id.String(), Email: req.Email, FirstName: firstName, LastName: lastName},
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	id, ok := a.readSession(r)
	if !ok {
		errorJSON(w, http.StatusUnauthorized, "Not logged in.")
		return
	}

	var email, firstName, lastName string
	err := a.db.QueryRow(r.Context(), `SELECT email, first_name, last_name FROM users WHERE id = $1`, id).Scan(&email, &firstName, &lastName)
	if err != nil {
		errorJSON(w, http.StatusUnauthorized, "Session is no longer valid.")
		return
	}
	writeJSON(w, http.StatusOK, userResponse{ID: id.String(), Email: email, FirstName: firstName, LastName: lastName})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: sameSite}
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out."})
}

func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	var req checkoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)
	req.ShippingAddress = strings.TrimSpace(req.ShippingAddress)

	if !validEmail(req.Email) || req.Phone == "" || req.ShippingAddress == "" {
		errorJSON(w, http.StatusBadRequest, "Please complete all checkout fields.")
		return
	}

	var userID any
	if id, ok := a.readSession(r); ok {
		userID = id
	}

	_, err := a.db.Exec(r.Context(), `
        INSERT INTO checkout_submissions (id, user_id, email, phone, shipping_address)
        VALUES ($1, $2, $3, $4, $5)
    `, uuid.New(), userID, req.Email, req.Phone, req.ShippingAddress)
	if err != nil {
		log.Printf("checkout: %v", err)
		errorJSON(w, http.StatusInternalServerError, "Could not save checkout data.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "Checkout data recorded successfully."})
}

func (a *App) signSession(id uuid.UUID) string {
	payload := id.String()
	mac := hmac.New(sha256.New, a.sessionSecret)
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature
}

func (a *App) readSession(r *http.Request) (uuid.UUID, bool) {
	c, err := r.Cookie("session")
	if err != nil || c.Value == "" {
		return uuid.Nil, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return uuid.Nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	mac := hmac.New(sha256.New, a.sessionSecret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, provided) {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(string(payload))
	return id, err == nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string) {
	secure := r.TLS != nil
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   7 * 24 * 60 * 60,
	})
}

func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == a.allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}

func generateCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func validEmail(email string) bool {
	if len(email) < 5 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && strings.Contains(email, "@")
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		errorJSON(w, http.StatusBadRequest, "Invalid request body.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func mustEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("missing environment variable %s", key)
	}
	return value
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
