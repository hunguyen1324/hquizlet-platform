package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// QuizSnapshots stores immutable question content, shared by every session of
// the same quiz version. Rows contain correct answers, so they are only read
// through the service (never serialized to a client).
type QuizSnapshots interface {
	Get(ctx context.Context, id string) (model.QuizSnapshot, error)

	// Put inserts the snapshot if no row with that id exists. Identical content
	// hashes to the same id, so concurrent sessions of the same version race to
	// insert the same value and the loser is a no-op.
	Put(ctx context.Context, snap model.QuizSnapshot) error

	// DeleteUnreferenced drops snapshots created before the cutoff that no
	// session points at anymore.
	DeleteUnreferenced(ctx context.Context, before time.Time) (int64, error)
}

type QuizSnapshotRepository struct{ db *sql.DB }

func NewQuizSnapshotRepository(db *sql.DB) *QuizSnapshotRepository {
	return &QuizSnapshotRepository{db: db}
}

func (r *QuizSnapshotRepository) Put(ctx context.Context, snap model.QuizSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO protected_quiz_snapshots(id,study_set_id,part,total,payload,created_at)
         VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO NOTHING`,
		snap.ID, snap.StudySetID, snap.Part, len(snap.Items), data, snap.CreatedAt)
	return err
}

func (r *QuizSnapshotRepository) Get(ctx context.Context, id string) (model.QuizSnapshot, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT payload FROM protected_quiz_snapshots WHERE id=$1`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return model.QuizSnapshot{}, ErrNotFound
	}
	if err != nil {
		return model.QuizSnapshot{}, err
	}
	var snap model.QuizSnapshot
	if err = json.Unmarshal(data, &snap); err != nil {
		return model.QuizSnapshot{}, err
	}
	snap.ID = id
	return snap, nil
}

func (r *QuizSnapshotRepository) DeleteUnreferenced(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM protected_quiz_snapshots s
         WHERE s.created_at < $1
           AND NOT EXISTS (SELECT 1 FROM protected_quiz_sessions p WHERE p.snapshot_id = s.id)`,
		before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
