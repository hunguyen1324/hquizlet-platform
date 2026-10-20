package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultAudioCacheDir is where local audio copies live when no directory is
// configured. The cache is disposable: content-addressed files are rebuilt
// from the object store or from the database on demand.
var DefaultAudioCacheDir = filepath.Join(os.TempDir(), "hquizlet-quiz-audio")

// DefaultAudioCacheBytes caps the local copy so a replica's disk use stays
// bounded no matter how many lessons are played.
const DefaultAudioCacheBytes int64 = 512 << 20

// partSuffix marks a file that is still being written; it is never served and
// is swept once it is old enough to be a crash leftover.
const partSuffix = ".part"

// diskCache keeps one local copy per audio id so a request can serve ranges
// from an os.File (the OS page cache supplies the RAM) instead of Postgres.
type diskCache struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex // serialises store + eviction
}

func newDiskCache(dir string, maxBytes int64) (*diskCache, error) {
	if dir == "" || maxBytes <= 0 {
		return nil, errors.New("storage: audio cache disabled")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &diskCache{dir: dir, maxBytes: maxBytes}, nil
}

// validAudioID keeps identifiers out of the file path: ids are sha256 hex.
func validAudioID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (c *diskCache) path(id string) (string, error) {
	if c == nil || !validAudioID(id) {
		return "", errors.New("storage: audio not cacheable")
	}
	return filepath.Join(c.dir, id), nil
}

// open returns a seekable handle on the cached copy, or an error on a miss.
func (c *diskCache) open(id string) (io.ReadSeekCloser, error) {
	path, err := c.path(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || fi.Size() == 0 {
		f.Close()
		os.Remove(path)
		return nil, errors.New("storage: empty audio cache entry")
	}
	return f, nil
}

// store materialises data under id atomically (write to a part file, then
// rename) so a concurrent reader never sees a half-written audio, and returns
// a reader on the stored copy.
func (c *diskCache) store(id string, data []byte) (io.ReadSeekCloser, error) {
	path, err := c.path(id)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(c.dir, id+"-*"+partSuffix)
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	c.trim(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

type cacheEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// trim enforces the byte cap after a write by dropping the oldest copies, and
// sweeps abandoned part files. The file just written is always kept; a copy
// that another request is still serving is left alone and retried next time.
func (c *diskCache) trim(keep string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	now := time.Now()
	var files []cacheEntry
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.HasSuffix(e.Name(), partSuffix) {
			if now.Sub(info.ModTime()) > 10*time.Minute {
				os.Remove(filepath.Join(c.dir, e.Name()))
			}
			continue
		}
		files = append(files, cacheEntry{path: filepath.Join(c.dir, e.Name()), size: info.Size(), modTime: info.ModTime()})
		total += info.Size()
	}
	if total <= c.maxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for _, f := range files {
		if total <= c.maxBytes {
			return
		}
		if f.path == keep {
			continue
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
