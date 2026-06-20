package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
)

// Worker demonstrates a background Deployment that consumes a Redis queue.
// In production you'd use a proper message broker; Redis LPOP/BRPOP is fine for learning.

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

	slog.Info("worker started", "queue", "order_queue")
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
	// BRPOP blocks until an item arrives — efficient for queue workers
	result, err := rdb.BRPop(ctx, 5*time.Second, "order_queue").Result()
	if err == redis.Nil {
		return // timeout, loop again
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		slog.Error("redis error", "error", err)
		time.Sleep(time.Second)
		return
	}

	orderID, err := strconv.Atoi(result[1])
	if err != nil {
		slog.Error("invalid order id in queue", "value", result[1])
		return
	}

	_, err = db.ExecContext(ctx,
		`UPDATE orders SET status = 'processed', updated_at = NOW() WHERE id = $1 AND status = 'pending'`,
		orderID,
	)
	if err != nil {
		slog.Error("failed to process order", "order_id", orderID, "error", err)
		// Re-enqueue on failure so we don't lose the job
		_ = rdb.LPush(ctx, "order_queue", orderID).Err()
		time.Sleep(time.Second)
		return
	}

	slog.Info("order processed", "order_id", orderID)
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
