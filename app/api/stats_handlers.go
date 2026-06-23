package main

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type gameStats struct {
	TotalGames   int     `json:"total_games"`
	AverageScore float64 `json:"average_score"`
	TopScore     int     `json:"top_score"`
	UniqueWords  int     `json:"unique_words"`
	Cached       bool    `json:"cached_from_redis"`
}

func getStats(db *sql.DB, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		stats, ok := statsFromRedis(ctx, rdb)
		if ok {
			writeJSON(w, stats)
			return
		}
		writeJSON(w, statsFromDB(ctx, db))
	}
}

func statsFromRedis(ctx context.Context, rdb *redis.Client) (gameStats, bool) {
	games, err := rdb.Get(ctx, "stats:games").Int()
	if err != nil {
		return gameStats{}, false
	}
	points, _ := rdb.Get(ctx, "stats:points_sum").Int()
	top, _ := rdb.Get(ctx, "stats:top_score").Int()
	unique, _ := rdb.Get(ctx, "stats:unique_words").Int()

	avg := 0.0
	if games > 0 {
		avg = float64(points) / float64(games)
	}
	return gameStats{
		TotalGames:   games,
		AverageScore: avg,
		TopScore:     top,
		UniqueWords:  unique,
		Cached:       true,
	}, true
}

func statsFromDB(ctx context.Context, db *sql.DB) gameStats {
	var stats gameStats
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(score), 0), COALESCE(MAX(score), 0) FROM word_scores`).
		Scan(&stats.TotalGames, &stats.AverageScore, &stats.TopScore)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT LOWER(word)) FROM word_scores`).
		Scan(&stats.UniqueWords)
	return stats
}

func listTodayScores(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT id, word, score, length, length_points, uniqueness_points, player_name, created_at
			FROM word_scores
			WHERE created_at >= CURRENT_DATE
			ORDER BY score DESC, created_at DESC
			LIMIT 20
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		writeScoreRows(w, rows)
	}
}

func listPlayerScores(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if name == "" {
			http.Error(w, `{"error":"player name required"}`, http.StatusBadRequest)
			return
		}

		var best int
		_ = db.QueryRow(
			`SELECT COALESCE(MAX(score), 0) FROM word_scores WHERE LOWER(player_name) = LOWER($1)`,
			name,
		).Scan(&best)

		rows, err := db.Query(`
			SELECT id, word, score, length, length_points, uniqueness_points, player_name, created_at
			FROM word_scores
			WHERE LOWER(player_name) = LOWER($1)
			ORDER BY score DESC, created_at DESC
			LIMIT 10
		`, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		scores := scanScoreRows(rows)
		writeJSON(w, map[string]any{
			"player_name": name,
			"best_score":  best,
			"recent":      scores,
		})
	}
}

func writeScoreRows(w http.ResponseWriter, rows *sql.Rows) {
	writeJSON(w, scanScoreRows(rows))
}

func scanScoreRows(rows *sql.Rows) []WordScore {
	scores := []WordScore{}
	for rows.Next() {
		var s WordScore
		if err := rows.Scan(&s.ID, &s.Word, &s.Score, &s.Length, &s.LengthPoints, &s.UniquenessPoints, &s.PlayerName, &s.CreatedAt); err != nil {
			continue
		}
		s.Word = displayWord(s.Word)
		enrichCorpusMeta(&s)
		scores = append(scores, s)
	}
	return scores
}