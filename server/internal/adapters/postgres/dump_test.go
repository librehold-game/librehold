package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"librehold/internal/adapters/postgres"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

func jsonLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out = append(out, m)
	}
	return out
}

func TestExportWritesArchiveLeaderboardAndLogs(t *testing.T) {
	ctx := context.Background()
	pool := worldsDB(t)
	s := postgres.NewWorlds(pool)
	ended, err := s.Create(ctx, apptest.NewWorldRow(0), func(w app.WorldTx) error {
		w.LogAction("world", false, "create", map[string]any{})
		w.LogAction("k1", false, "observe", map[string]any{"points": 26})
		w.LogAction("k1", false, "build", map[string]any{"building": "wood"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	running, err := s.Create(ctx, apptest.NewWorldRow(0), func(w app.WorldTx) error {
		w.LogAction("world", false, "create", map[string]any{})
		w.LogAction("bot_1", true, "build", map[string]any{"building": "wood"})
		w.LogAction("k2", false, "observe", map[string]any{"points": 30})
		w.LogAction("k2", false, "send", map[string]any{"x": 1})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveArchive(ctx, app.ArchivedGame{WorldID: ended, Name: "Game #1", GameNumber: 1, Rows: []game.LeaderRow{
		{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1}, {Rank: 2, PlayerID: "bot_1", Name: "Bot", IsBot: true}}}); err != nil {
		t.Fatal(err)
	}
	log, _ := s.ActionLog(ctx, ended)
	if err := s.SavePackedLog(ctx, app.PackedLog{WorldID: ended, Seed: 7, Secret: []byte{1, 2, 3}, Build: "dev", Rows: len(log), LastSeq: log[len(log)-1].Seq,
		Data: gzipLines(t, log)}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := postgres.Export(ctx, pool, dir); err != nil {
		t.Fatal(err)
	}
	if games := jsonLines(t, filepath.Join(dir, "archive.jsonl")); len(games) != 1 || games[0]["name"] != "Game #1" || games[0]["gameNumber"] != 1.0 {
		t.Fatalf("archive %+v", games)
	}
	if rows := jsonLines(t, filepath.Join(dir, "leaderboard.jsonl")); len(rows) != 2 || rows[1]["isBot"] != true ||
		rows[0]["worldId"] != float64(ended) {
		t.Fatalf("leaderboard %+v", rows)
	}
	index := jsonLines(t, filepath.Join(dir, "logs", "index.jsonl"))
	secret := base64.StdEncoding.EncodeToString(apptest.NewWorldRow(0).Secret)
	if len(index) != 2 || index[0]["seed"] != float64(7) || index[0]["secret"] != "AQID" || index[0]["packed"] != true ||
		index[1]["secret"] != secret || index[1]["packed"] != false {
		t.Fatalf("index %+v", index)
	}
	for id, want := range map[int64]int{ended: 3, running: 4} {
		data, err := os.ReadFile(filepath.Join(dir, "logs", strconv.FormatInt(id, 10)+".jsonl.gz"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := app.UnpackLog(data)
		if err != nil || len(got) != want || got[0].Kind != "create" {
			t.Fatalf("log of %d: %+v %v", id, got, err)
		}
	}
	pairs := jsonLines(t, filepath.Join(dir, "training.jsonl"))
	if len(pairs) != 2 || pairs[0]["worldId"] != float64(ended) || pairs[0]["kind"] != "build" || pairs[1]["actor"] != "k2" ||
		pairs[1]["seen"].(map[string]any)["points"] != 30.0 {
		t.Fatalf("training %+v", pairs)
	}
}

func gzipLines(t *testing.T, log []game.LoggedAction) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := app.PackLines(&buf, log); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
