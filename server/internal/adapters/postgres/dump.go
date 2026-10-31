package postgres

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/app"
)

// logIndexEntry describes one exported log: what a replay needs besides the entries.
type logIndexEntry struct {
	WorldID int64           `json:"worldId"`
	Seed    uint32          `json:"seed"`
	Secret  []byte          `json:"secret,omitempty"`
	Config  json.RawMessage `json:"config"`
	Build   string          `json:"build,omitempty"`
	Rows    int             `json:"rows"`
	Packed  bool            `json:"packed"`
}

// Export writes the archive, its leaderboards and every action log, packed or still running, to dir for offline
// work: archive.jsonl, leaderboard.jsonl, training.jsonl (human decisions, app.TrainingPairs), logs/index.jsonl
// and logs/<world>.jsonl.gz.
func Export(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return err
	}
	if err := exportLines(ctx, pool, filepath.Join(dir, "archive.jsonl"), `select json_build_object('worldId', world_id,
		'name', name, 'gameNumber', game_number, 'startedAt', started_at_ms, 'endedAt', ended_at_ms, 'winnerName', winner_name,
		'winnerIsBot', winner_is_bot, 'summary', summary) from instant_archive order by world_id`); err != nil {
		return err
	}
	if err := exportLines(ctx, pool, filepath.Join(dir, "leaderboard.jsonl"), `select json_build_object('worldId', world_id,
		'rank', rank, 'playerId', player_id, 'name', name, 'isBot', is_bot, 'accountId', account_id, 'points', points,
		'villages', villages, 'conquered', conquered, 'killed', killed) from instant_leaderboard order by world_id, rank`); err != nil {
		return err
	}
	var index []logIndexEntry
	rows, err := pool.Query(ctx, `select world_id, seed, secret, config, build, rows, data from action_log_packed order by world_id`)
	if err != nil {
		return err
	}
	var packed [][]byte
	for rows.Next() {
		var e logIndexEntry
		var seed int64
		var data []byte
		if err := rows.Scan(&e.WorldID, &seed, &e.Secret, &e.Config, &e.Build, &e.Rows, &data); err != nil {
			rows.Close()
			return err
		}
		e.Seed, e.Packed = uint32(seed), true
		index, packed = append(index, e), append(packed, data)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var pairs []app.TrainingPair
	for i, e := range index {
		if err := os.WriteFile(logPath(dir, e.WorldID), packed[i], 0o644); err != nil {
			return err
		}
		log, err := app.UnpackLog(packed[i])
		if err != nil {
			return err
		}
		pairs = append(pairs, app.TrainingPairs(e.WorldID, log)...)
	}
	live, err := pool.Query(ctx, `select w.id, w.seed, w.secret, w.config from worlds w
		where exists (select 1 from action_log l where l.world_id = w.id) order by w.id`)
	if err != nil {
		return err
	}
	running, err := pgx.CollectRows(live, func(r pgx.CollectableRow) (logIndexEntry, error) {
		var e logIndexEntry
		var seed int64
		err := r.Scan(&e.WorldID, &seed, &e.Secret, &e.Config)
		e.Seed = uint32(seed)
		return e, err
	})
	if err != nil {
		return err
	}
	worlds := NewWorlds(pool)
	for _, e := range running {
		log, err := worlds.ActionLog(ctx, e.WorldID)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := app.PackLines(&buf, log); err != nil {
			return err
		}
		if err := os.WriteFile(logPath(dir, e.WorldID), buf.Bytes(), 0o644); err != nil {
			return err
		}
		e.Rows = len(log)
		index = append(index, e)
		pairs = append(pairs, app.TrainingPairs(e.WorldID, log)...)
	}
	if err := writeJSONLines(filepath.Join(dir, "training.jsonl"), pairs); err != nil {
		return err
	}
	return writeJSONLines(filepath.Join(dir, "logs", "index.jsonl"), index)
}

func writeJSONLines[T any](path string, items []T) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func logPath(dir string, worldID int64) string {
	return filepath.Join(dir, "logs", strconv.FormatInt(worldID, 10)+".jsonl.gz")
}

// exportLines writes one JSON line per row of a query selecting a single JSON column.
func exportLines(ctx context.Context, pool *pgxpool.Pool, path, query string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var line []byte
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}
