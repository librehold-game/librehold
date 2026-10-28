package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"runtime/debug"

	"librehold/internal/game"
)

// Rejection counts one kind of refused order for one reason; refused orders are counted, never stored.
type Rejection struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// PackedLog is a world's action log after the world is gone: gzip JSON lines in seq order, with what a replay
// needs besides it. Its secret never goes out as JSON: only the admin export writes it, on purpose.
type PackedLog struct {
	WorldID    int64            `json:"worldId"`
	Seed       uint32           `json:"seed"`
	Secret     []byte           `json:"-"`
	Config     game.WorldConfig `json:"config"` // as the world ended; its create entry holds the starting one
	Build      string           `json:"build"`
	Rows       int              `json:"rows"`
	LastSeq    int64            `json:"lastSeq"`
	Data       []byte           `json:"-"`
	PackedAtMs int64            `json:"packedAt"`
}

// ActionLogStore keeps worlds' action logs (implemented by the world stores). Entries are written by
// WorldTx.LogAction, in the unit of work of the change they record.
type ActionLogStore interface {
	ActionLog(ctx context.Context, worldID int64) ([]game.LoggedAction, error) // in seq order
	CountRejection(ctx context.Context, worldID int64, kind, reason string) error
	Rejections(ctx context.Context, worldID int64) ([]Rejection, error) // by kind, then reason
	// SavePackedLog keeps the packed log and deletes its rows (seq ≤ p.LastSeq) at once; a world packed
	// before keeps its first packed log.
	SavePackedLog(ctx context.Context, p PackedLog) error
	PackedLog(ctx context.Context, worldID int64) (*PackedLog, error)
}

const (
	actorAdmin = "admin"
	actorWorld = "world"
)

// createEntry is the first entry of every world's log: what a replay starts from.
type createEntry struct {
	Config      game.WorldConfig `json:"config"`
	StartedAtMs int64            `json:"startedAt"`
}

type joinEntry struct {
	PlayerID game.PlayerID `json:"playerId"`
	Quadrant game.Quadrant `json:"quadrant"`
	Guest    bool          `json:"guest,omitempty"`
}

type resumeEntry struct {
	PausedTotalMs int64 `json:"pausedTotalMs"`
}

// logResume records a resumed world's paused total, which moves its clock origin (bots read local time).
func logResume(w WorldTx, wallMs int64, actor string) {
	w.SetNow(WorldNow(w.Row(), wallMs))
	w.LogAction(actor, false, "resume", resumeEntry{PausedTotalMs: w.Row().PausedTotalMs})
}

// newWorld is the init of every world the service creates: the create entry, then the bots.
func newWorld(w WorldTx) error {
	w.LogAction(actorWorld, false, "create", createEntry{Config: *w.Config(), StartedAtMs: w.Row().StartedAtMs})
	return game.NewWorld(w)
}

// rejectionReason is the code a refused order is counted under; other errors are not refusals.
func rejectionReason(err error) (string, bool) {
	var re *game.RuleError
	if errors.As(err, &re) {
		return re.Code, true
	}
	var be *game.BlockedError
	if errors.As(err, &be) && len(be.Blockers) > 0 {
		return be.Blockers[0].Kind(), true
	}
	return "", false
}

func (s *Service) countRejection(ctx context.Context, worldID int64, kind string, err error) {
	reason, ok := rejectionReason(err)
	if !ok {
		return
	}
	if err := s.worlds.CountRejection(ctx, worldID, kind, reason); err != nil {
		slog.Warn("action log: rejection", "world", worldID, "err", err)
	}
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range info.Settings {
			if kv.Key == "vcs.revision" && kv.Value != "" {
				return kv.Value
			}
		}
	}
	return "dev"
}

// PackActionLog packs the world's log rows into one gzip blob kept with its seed and settings, and deletes
// the rows. A world with no rows left is already packed.
func (s *Service) PackActionLog(ctx context.Context, worldID int64) error {
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return err
	}
	log, err := s.worlds.ActionLog(ctx, worldID)
	if err != nil || len(log) == 0 {
		return err
	}
	var buf bytes.Buffer
	if err := PackLines(&buf, log); err != nil {
		return err
	}
	return s.worlds.SavePackedLog(ctx, PackedLog{WorldID: worldID, Seed: row.Seed, Secret: row.Secret, Config: row.Config, Build: buildVersion(),
		Rows: len(log), LastSeq: log[len(log)-1].Seq, Data: buf.Bytes(), PackedAtMs: s.clock.NowMs()})
}

// PackLines writes the entries to out as gzip JSON lines, the packed log's format.
func PackLines(out io.Writer, log []game.LoggedAction) error {
	zw := gzip.NewWriter(out)
	enc := json.NewEncoder(zw)
	for _, e := range log {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return zw.Close()
}

// UnpackLog reads a packed log back.
func UnpackLog(data []byte) ([]game.LoggedAction, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	dec := json.NewDecoder(zr)
	var out []game.LoggedAction
	for dec.More() {
		var e game.LoggedAction
		if err := dec.Decode(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
