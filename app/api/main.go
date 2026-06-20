package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Order struct {
	ID        int       `json:"id"`
	Product   string    `json:"product"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type createOrderRequest struct {
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
}

var (
	ordersCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubelab_orders_created_total",
		Help: "Total number of orders created",
	})
	ordersProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubelab_orders_processed_total",
		Help: "Total number of orders processed by worker",
	})
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kubelab_http_requests_total",
		Help: "Total HTTP requests by route and status",
	}, []string{"method", "path", "status"})
)

func init() {
	prometheus.MustRegister(ordersCreated, ordersProcessed, httpRequests)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	db, err := connectDB()
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb := connectRedis()
	defer rdb.Close()

	if err := migrate(db); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, `{"status":"not ready","reason":"database"}`, http.StatusServiceUnavailable)
			return
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			http.Error(w, `{"status":"not ready","reason":"redis"}`, http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api", func(r chi.Router) {
		r.Get("/orders", listOrders(db))
		r.Post("/orders", createOrder(db, rdb))
		r.Get("/orders/{id}", getOrder(db))
		r.Patch("/orders/{id}/status", updateOrderStatus(db))
	})

	port := envOr("PORT", "8080")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func connectDB() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		envOr("DB_HOST", "postgres"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "kubelab"),
		envOr("DB_PASSWORD", "kubelab"),
		envOr("DB_NAME", "kubelab"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db, nil
		}
		slog.Info("waiting for database", "attempt", i+1)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("database not reachable after retries")
}

func connectRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", envOr("REDIS_HOST", "redis"), envOr("REDIS_PORT", "6379")),
		Password: envOr("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			product TEXT NOT NULL,
			quantity INTEGER NOT NULL CHECK (quantity > 0),
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		httpRequests.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(ww.Status())).Inc()
	})
}

func listOrders(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, product, quantity, status, created_at, updated_at FROM orders ORDER BY id DESC LIMIT 50`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		orders := []Order{}
		for rows.Next() {
			var o Order
			if err := rows.Scan(&o.ID, &o.Product, &o.Quantity, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			orders = append(orders, o)
		}
		writeJSON(w, orders)
	}
}

func createOrder(db *sql.DB, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Product == "" || req.Quantity < 1 {
			http.Error(w, `{"error":"invalid request: product and quantity required"}`, http.StatusBadRequest)
			return
		}

		var o Order
		err := db.QueryRow(
			`INSERT INTO orders (product, quantity, status) VALUES ($1, $2, 'pending') RETURNING id, product, quantity, status, created_at, updated_at`,
			req.Product, req.Quantity,
		).Scan(&o.ID, &o.Product, &o.Quantity, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23514" {
				http.Error(w, `{"error":"quantity must be positive"}`, http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Push order ID to Redis queue for the worker — demonstrates async processing
		if err := rdb.LPush(r.Context(), "order_queue", o.ID).Err(); err != nil {
			slog.Warn("failed to enqueue order", "order_id", o.ID, "error", err)
		}

		ordersCreated.Inc()
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, o)
	}
}

func getOrder(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		var o Order
		err = db.QueryRow(
			`SELECT id, product, quantity, status, created_at, updated_at FROM orders WHERE id = $1`, id,
		).Scan(&o.ID, &o.Product, &o.Quantity, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, o)
	}
}

func updateOrderStatus(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Status == "" {
			http.Error(w, `{"error":"status required"}`, http.StatusBadRequest)
			return
		}

		var o Order
		err = db.QueryRow(
			`UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2 RETURNING id, product, quantity, status, created_at, updated_at`,
			body.Status, id,
		).Scan(&o.ID, &o.Product, &o.Quantity, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if body.Status == "processed" {
			ordersProcessed.Inc()
		}
		writeJSON(w, o)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
