package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
)

// Worker indexes submitted scores into Redis for fast leaderboard reads.

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	db := mustConnectDB()
	defer db.Close()

	rdb := connectRedis()
	defer rdb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("worker shutting down")
		cancel()
	}()

	slog.Info("worker started", "queue", "score_queue")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			processNext(ctx, db, rdb)
		}
	}
}

func processNext(ctx context.Context, db *sql.DB, rdb *redis.Client) {
	result, err := rdb.BRPop(ctx, 5*time.Second, "score_queue").Result()
	if err == redis.Nil {
		return
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		slog.Error("redis error", "error", err)
		time.Sleep(time.Second)
		return
	}

	scoreID, err := strconv.Atoi(result[1])
	if err != nil {
		slog.Error("invalid score id in queue", "value", result[1])
		return
	}

	var word string
	var score int
	err = db.QueryRowContext(ctx,
		`SELECT word, score FROM word_scores WHERE id = $1 AND status = 'pending'`,
		scoreID,
	).Scan(&word, &score)
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		slog.Error("failed to load score", "score_id", scoreID, "error", err)
		return
	}

	_, err = db.ExecContext(ctx,
		`UPDATE word_scores SET status = 'indexed' WHERE id = $1`,
		scoreID,
	)
	if err != nil {
		slog.Error("failed to mark score indexed", "score_id", scoreID, "error", err)
		_ = rdb.LPush(ctx, "score_queue", scoreID).Err()
		time.Sleep(time.Second)
		return
	}

	// Sorted set + live stats cache (served by GET /api/stats).
	pipe := rdb.Pipeline()
	pipe.ZAdd(ctx, "leaderboard", redis.Z{Score: float64(score), Member: fmt.Sprintf("%d", scoreID)})
	pipe.Incr(ctx, "stats:games")
	pipe.IncrBy(ctx, "stats:points_sum", int64(score))
	pipe.SAdd(ctx, "stats:words", strings.ToLower(word))

	curTop, _ := rdb.Get(ctx, "stats:top_score").Int()
	if score > curTop {
		pipe.Set(ctx, "stats:top_score", score, 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("failed to update redis cache", "score_id", scoreID, "error", err)
	}

	// Keep unique word count in sync (approximate via SCARD).
	if n, err := rdb.SCard(ctx, "stats:words").Result(); err == nil {
		_ = rdb.Set(ctx, "stats:unique_words", n, 0).Err()
	}

	slog.Info("score indexed", "score_id", scoreID, "word", word, "score", score)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustConnectDB() *sql.DB {
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
		slog.Error("database open failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i := 0; i < 60; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		time.Sleep(time.Second)
	}
	slog.Error("database not reachable")
	os.Exit(1)
	return nil
}

func connectRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", envOr("REDIS_HOST", "redis"), envOr("REDIS_PORT", "6379")),
	})
}
