package storage

import (
	"context"
	"errors"
	"io"
)

// ErrObjectNotFound reports that the object store has no copy of a key.
var ErrObjectNotFound = errors.New("audio object not found")

// ObjectRangeStore reads a stored object by byte range. It is the
// source of truth for streaming: no object is ever loaded into memory in one
// piece, so process memory does not grow with the size of an audio file.
type ObjectRangeStore interface {
	// Stat returns the size of the object in bytes.
	Stat(ctx context.Context, key string) (int64, error)
	// Get streams [start, start+length). A negative length means "to the end".
	Get(ctx context.Context, key string, start, length int64) (io.ReadCloser, error)
}

// audioRangeMaxWindow bounds a single Range request while streaming: reads
// ramp up to this window, so a full play costs a handful of round trips.
const audioRangeMaxWindow = 4 << 20

// rangeReader exposes one stored object as an io.ReadSeeker backed by Range
// requests. The first window after a seek covers exactly what the caller
// asked for, so the `Range: bytes=0-0` warm-up transfers a single byte instead
// of the whole file; consecutive reads then double the window up to
// audioRangeMaxWindow.
type rangeReader struct {
	store ObjectRangeStore
	key   string
	ctx   context.Context

	size   int64
	pos    int64
	window int64 // byte length of the most recently opened window
	fresh  bool  // the next Read sizes its window to the caller's buffer
	body   io.ReadCloser
	left   int64 // bytes left in the open window
	empty  int   // consecutive windows that delivered no data
	closed bool
}

// newRangeReader stats the object once and returns a seekable reader over it.
func newRangeReader(ctx context.Context, store ObjectRangeStore, key string) (io.ReadSeekCloser, error) {
	size, err := store.Stat(ctx, key)
	if err != nil {
		return nil, err
	}
	return &rangeReader{store: store, key: key, ctx: ctx, size: size, fresh: true}, nil
}

func (r *rangeReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.pos + offset
	case io.SeekEnd:
		abs = r.size + offset
	default:
		return 0, errors.New("storage: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("storage: negative position")
	}
	r.drop()
	r.pos = abs
	r.fresh = true
	return abs, nil
}

func (r *rangeReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("storage: read on closed audio")
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if r.pos >= r.size {
			return 0, io.EOF
		}
		if r.body == nil {
			if err := r.openWindow(int64(len(p))); err != nil {
				return 0, err
			}
		}
		n, err := r.body.Read(p)
		if n > 0 {
			r.empty = 0
			r.pos += int64(n)
			r.left -= int64(n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			r.drop()
			return n, err
		}
		if r.left <= 0 || errors.Is(err, io.EOF) {
			if n == 0 {
				// The store closed the window before delivering anything:
				// reopen, but never spin on a broken object.
				r.empty++
				if r.empty > 2 {
					r.drop()
					return 0, io.ErrUnexpectedEOF
				}
			}
			r.drop()
		}
		if n > 0 {
			return n, nil
		}
	}
}

func (r *rangeReader) Close() error {
	r.drop()
	r.closed = true
	return nil
}

func (r *rangeReader) drop() {
	if r.body != nil {
		r.body.Close()
		r.body = nil
	}
	r.left = 0
}

func (r *rangeReader) openWindow(want int64) error {
	if want < 1 {
		want = 1
	}
	w := want
	if !r.fresh {
		w = r.window * 2
		if w < want {
			w = want
		}
	}
	if w > audioRangeMaxWindow {
		w = audioRangeMaxWindow
	}
	end := r.pos + w - 1
	if end >= r.size {
		end = r.size - 1
	}
	length := end - r.pos + 1
	body, err := r.store.Get(r.ctx, r.key, r.pos, length)
	if err != nil {
		return err
	}
	r.body = body
	r.left = length
	r.window = length
	r.fresh = false
	return nil
}

// readSeekCloser adapts an in-memory reader for callers that must close what
// they open; used when no disk cache is available.
type readSeekCloser struct{ io.ReadSeeker }

func (readSeekCloser) Close() error { return nil }
