package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/akuaruu/url-shortener/redirect-service/internal/domain"
)

// StartClickSyncWorker sekarang menerima *pgxpool.Pool
func StartClickSyncWorker(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Background worker berjalan, siklus sinkronisasi: %v\n", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker dihentikan dengan aman.")
			return
		case <-ticker.C:
			processClickQueue(ctx, db, rdb)
		}
	}
}

func processClickQueue(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) {
	results, err := rdb.LPopCount(ctx, "queue:click_events", 1000).Result()
	if err == redis.Nil || len(results) == 0 {
		return
	}
	if err != nil {
		log.Printf("Gagal menarik antrean klik: %v", err)
		return
	}

	clickCounts := make(map[string]int)
	var logs []domain.ClickEvent

	for _, res := range results {
		var event domain.ClickEvent
		if err := json.Unmarshal([]byte(res), &event); err == nil {
			clickCounts[event.ShortCode]++
			logs = append(logs, event)
		}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Gagal memulai transaksi DB: %v", err)
		requeueEvents(ctx, rdb, results) // kembalikan ke queue
		return
	}
	defer tx.Rollback(ctx)

	for shortCode, count := range clickCounts {
		_, err = tx.Exec(ctx,
			"UPDATE urls SET click_count = click_count + $1 WHERE short_code = $2",
			count, shortCode,
		)
		if err != nil {
			log.Printf("Gagal update click count untuk %s: %v", shortCode, err)
			requeueEvents(ctx, rdb, results) // kembalikan ke queue
			return
		}
	}

	for _, l := range logs {
		_, err = tx.Exec(ctx,
			"INSERT INTO click_logs (url_id, clicked_at, user_agent, ip_hash) VALUES ($1, $2, $3, $4)",
			l.URLID, l.ClickedAt, l.UserAgent, l.IPHash,
		)
		if err != nil {
			log.Printf("Gagal insert click_log untuk url_id %d: %v", l.URLID, err)
			requeueEvents(ctx, rdb, results)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Gagal commit transaksi: %v", err)
		requeueEvents(ctx, rdb, results)
		return
	}

	log.Printf("Sinkronisasi %d click event selesai.", len(logs))
}

// requeueEvents mengembalikan raw JSON strings ke ujung kanan queue
// sehingga tidak ada data analytics yang hilang saat DB error.
func requeueEvents(ctx context.Context, rdb *redis.Client, rawEvents []string) {
	args := make([]interface{}, len(rawEvents))
	for i, e := range rawEvents {
		args[i] = e
	}
	if err := rdb.RPush(ctx, "queue:click_events", args...).Err(); err != nil {
		log.Printf("CRITICAL: gagal requeue %d events: %v", len(rawEvents), err)
	}
}
