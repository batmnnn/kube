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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type WordScore struct {
	ID                   int       `json:"id"`
	Word                 string    `json:"word"`
	Score                int       `json:"score"`
	Length               int       `json:"length"`
	LengthPoints         int       `json:"length_points"`
	UniquenessPoints     int       `json:"uniqueness_points"`
	CorpusFrequency      int64     `json:"corpus_frequency,omitempty"`
	RarityTier           string    `json:"rarity_tier,omitempty"`
	PlayerName           string    `json:"player_name"`
	CreatedAt            time.Time `json:"created_at"`
}

type submitScoreRequest struct {
	Word       string `json:"word"`
	PlayerName string `json:"player_name"`
}

var (
	scoresSubmitted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubelab_word_scores_submitted_total",
		Help: "Total word scores submitted",
	})
	scoresIndexed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubelab_word_scores_indexed_total",
		Help: "Total word scores indexed by worker",
	})
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kubelab_http_requests_total",
		Help: "Total HTTP requests by route and status",
	}, []string{"method", "path", "status"})
)

func init() {
	prometheus.MustRegister(scoresSubmitted, scoresIndexed, httpRequests)
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
		r.Get("/scores", listScores(db))
		r.Get("/scores/today", listTodayScores(db))
		r.Get("/stats", getStats(db, rdb))
		r.Get("/hint", getHint())
		r.Get("/players/{name}/scores", listPlayerScores(db))
		r.Post("/scores", submitScore(db, rdb))
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
		CREATE TABLE IF NOT EXISTS word_scores (
			id SERIAL PRIMARY KEY,
			word TEXT NOT NULL,
			score INTEGER NOT NULL CHECK (score BETWEEN 1 AND 1000),
			length INTEGER NOT NULL CHECK (length > 0),
			length_points INTEGER NOT NULL,
			uniqueness_points INTEGER NOT NULL,
			player_name TEXT NOT NULL DEFAULT 'Player',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_word_scores_score ON word_scores (score DESC, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_word_scores_word ON word_scores (LOWER(word));
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

func listScores(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT id, word, score, length, length_points, uniqueness_points, player_name, created_at
			FROM word_scores
			ORDER BY score DESC, created_at DESC
			LIMIT 50
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		scores := []WordScore{}
		for rows.Next() {
			var s WordScore
			if err := rows.Scan(&s.ID, &s.Word, &s.Score, &s.Length, &s.LengthPoints, &s.UniquenessPoints, &s.PlayerName, &s.CreatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.Word = displayWord(s.Word)
			enrichCorpusMeta(&s)
			scores = append(scores, s)
		}
		writeJSON(w, scores)
	}
}

func submitScore(db *sql.DB, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req submitScoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
			return
		}

		word, err := normalizeWord(req.Word)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
			return
		}

		player := req.PlayerName
		if player == "" {
			player = "Player"
		}
		if len(player) > 40 {
			player = player[:40]
		}

		breakdown := calculateScore(word)

		var s WordScore
		err = db.QueryRow(
			`INSERT INTO word_scores (word, score, length, length_points, uniqueness_points, player_name, status)
			 VALUES ($1, $2, $3, $4, $5, $6, 'pending')
			 RETURNING id, word, score, length, length_points, uniqueness_points, player_name, created_at`,
			word, breakdown.Score, breakdown.Length, breakdown.LengthPoints, breakdown.UniquenessPoints, player,
		).Scan(&s.ID, &s.Word, &s.Score, &s.Length, &s.LengthPoints, &s.UniquenessPoints, &s.PlayerName, &s.CreatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := rdb.LPush(r.Context(), "score_queue", s.ID).Err(); err != nil {
			slog.Warn("failed to enqueue score", "score_id", s.ID, "error", err)
		}

		scoresSubmitted.Inc()
		s.Word = displayWord(s.Word)
		s.CorpusFrequency = breakdown.CorpusFrequency
		s.RarityTier = breakdown.RarityTier
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, s)
	}
}

func enrichCorpusMeta(s *WordScore) {
	_, freq, tier := corpusRarityPoints(strings.ToLower(s.Word))
	s.CorpusFrequency = freq
	s.RarityTier = tier
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
