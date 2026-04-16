package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type application struct {
	db *sql.DB
}

type tenant struct {
	ID              string    `json:"id"`
	FullName        string    `json:"full_name"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	RoomNumber      string    `json:"room_number"`
	MonthlyRent     float64   `json:"monthly_rent"`
	SecurityDeposit float64   `json:"security_deposit"`
	CheckInDate     string    `json:"check_in_date"`
	CreatedAt       time.Time `json:"created_at"`
}

type createTenantRequest struct {
	FullName        string  `json:"full_name"`
	Phone           string  `json:"phone"`
	Email           string  `json:"email"`
	RoomNumber      string  `json:"room_number"`
	MonthlyRent     float64 `json:"monthly_rent"`
	SecurityDeposit float64 `json:"security_deposit"`
	CheckInDate     string  `json:"check_in_date"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := openDB(databaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := runMigrations(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	app := &application{db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/api/v1/tenants", app.tenantsHandler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func openDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS tenants (
		id UUID PRIMARY KEY,
		full_name TEXT NOT NULL,
		phone TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		room_number TEXT NOT NULL,
		monthly_rent NUMERIC(10,2) NOT NULL CHECK (monthly_rent >= 0),
		security_deposit NUMERIC(10,2) NOT NULL CHECK (security_deposit >= 0),
		check_in_date DATE NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`

	_, err := db.ExecContext(ctx, query)
	return err
}

func (app *application) healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *application) tenantsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.listTenantsHandler(w, r)
	case http.MethodPost:
		app.createTenantHandler(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	}
}

func (app *application) createTenantHandler(w http.ResponseWriter, r *http.Request) {
	var input createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	if err := validateCreateTenantInput(input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	checkInDate, err := time.Parse("2006-01-02", input.CheckInDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "check_in_date must be in YYYY-MM-DD format"})
		return
	}

	item := tenant{
		ID:              uuid.NewString(),
		FullName:        strings.TrimSpace(input.FullName),
		Phone:           strings.TrimSpace(input.Phone),
		Email:           strings.ToLower(strings.TrimSpace(input.Email)),
		RoomNumber:      strings.TrimSpace(input.RoomNumber),
		MonthlyRent:     input.MonthlyRent,
		SecurityDeposit: input.SecurityDeposit,
		CheckInDate:     checkInDate.Format("2006-01-02"),
	}

	query := `
	INSERT INTO tenants (id, full_name, phone, email, room_number, monthly_rent, security_deposit, check_in_date)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING created_at;
	`

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err = app.db.QueryRowContext(
		ctx,
		query,
		item.ID,
		item.FullName,
		item.Phone,
		item.Email,
		item.RoomNumber,
		item.MonthlyRent,
		item.SecurityDeposit,
		item.CheckInDate,
	).Scan(&item.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSON(w, http.StatusConflict, errorResponse{Error: "tenant email already exists"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not create tenant"})
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (app *application) listTenantsHandler(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT id, full_name, phone, email, room_number, monthly_rent, security_deposit, check_in_date, created_at
	FROM tenants
	ORDER BY created_at DESC;
	`

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := app.db.QueryContext(ctx, query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not fetch tenants"})
		return
	}
	defer rows.Close()

	tenants := make([]tenant, 0)
	for rows.Next() {
		var item tenant
		var checkInDate time.Time

		if err := rows.Scan(
			&item.ID,
			&item.FullName,
			&item.Phone,
			&item.Email,
			&item.RoomNumber,
			&item.MonthlyRent,
			&item.SecurityDeposit,
			&checkInDate,
			&item.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not read tenants"})
			return
		}

		item.CheckInDate = checkInDate.Format("2006-01-02")
		tenants = append(tenants, item)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not read tenants"})
		return
	}

	writeJSON(w, http.StatusOK, tenants)
}

func validateCreateTenantInput(input createTenantRequest) error {
	switch {
	case strings.TrimSpace(input.FullName) == "":
		return fmt.Errorf("full_name is required")
	case strings.TrimSpace(input.Phone) == "":
		return fmt.Errorf("phone is required")
	case strings.TrimSpace(input.Email) == "":
		return fmt.Errorf("email is required")
	case strings.TrimSpace(input.RoomNumber) == "":
		return fmt.Errorf("room_number is required")
	case input.MonthlyRent < 0:
		return fmt.Errorf("monthly_rent must be zero or greater")
	case input.SecurityDeposit < 0:
		return fmt.Errorf("security_deposit must be zero or greater")
	case strings.TrimSpace(input.CheckInDate) == "":
		return fmt.Errorf("check_in_date is required")
	default:
		return nil
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, `{"error":"could not encode response"}`, http.StatusInternalServerError)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
