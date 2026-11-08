package videostorage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// ErrLegacyBundle lets the HTTP layer continue reading existing ZIP videos.
var ErrLegacyBundle = errors.New("legacy HLS bundle")
var segmentPattern = regexp.MustCompile(`^segment[0-9]{6}\.ts$`)

type HLSAssets interface {
	PublishHLS(context.Context, string, string) error
	OpenHLSAsset(context.Context, string, string) (io.ReadCloser, error)
}

type NodeConfig struct {
	ID     string
	Config Config
}

type shardBackend interface {
	putAsset(context.Context, string, string, string) error
	openAsset(context.Context, string, string) (File, error)
	removeAssets(context.Context, string) error
}
type shardAsset struct {
	Node   string `json:"node"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type shardManifest struct {
	Format   string                `json:"format"`
	Playlist string                `json:"playlist"`
	Assets   map[string]shardAsset `json:"assets"`
}

const shardFormat = "hquizlet-hls-shards-v1"

// Sharded keeps tiny manifests on the original store. Each encrypted segment
// exists on exactly one node. Node IDs are persisted: never reuse an ID for a
// different server or remove a node before migrating its segments.
type Sharded struct {
	*SFTP
	primary   Store
	nodes     map[string]shardBackend
	order     []string
	mu        sync.Mutex
	manifests map[string]*shardManifest
	readSlots chan struct{}
}

func NewSharded(primary *SFTP, configs []NodeConfig) (*Sharded, error) {
	s := &Sharded{SFTP: primary, primary: primary, nodes: make(map[string]shardBackend), manifests: make(map[string]*shardManifest), readSlots: make(chan struct{}, 4)}
	if len(configs) < 1 || len(configs) > 16 {
		return nil, errors.New("HLS shards require 1 to 16 nodes")
	}
	for _, cfg := range configs {
		if cfg.Config.HostKeyAlgorithm == "" {
			cfg.Config.HostKeyAlgorithm = "ssh-ed25519"
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`).MatchString(cfg.ID) || s.nodes[cfg.ID] != nil {
			return nil, errors.New("invalid or duplicate HLS node ID")
		}
		node, err := New(cfg.Config)
		if err != nil {
			return nil, fmt.Errorf("HLS node %s: %w", cfg.ID, err)
		}
		s.nodes[cfg.ID] = node
		s.order = append(s.order, cfg.ID)
	}
	sort.Strings(s.order)
	return s, nil
}

func (s *Sharded) forget(key string) { s.mu.Lock(); delete(s.manifests, key); s.mu.Unlock() }

func (s *Sharded) manifest(ctx context.Context, key string) (*shardManifest, error) {
	if !keyPattern.MatchString(key) {
		return nil, errors.New("invalid video key")
	}
	s.mu.Lock()
	m := s.manifests[key]
	s.mu.Unlock()
	if m != nil {
		return m, nil
	}
	f, err := s.primary.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	header := make([]byte, 2)
	if _, err = io.ReadFull(f, header); err != nil {
		return nil, err
	}
	if string(header) == "PK" {
		return nil, ErrLegacyBundle
	}
	raw, err := io.ReadAll(io.LimitReader(io.MultiReader(bytes.NewReader(header), f), 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 1024*1024 {
		return nil, errors.New("HLS manifest too large")
	}
	m = &shardManifest{}
	if err = json.Unmarshal(raw, m); err != nil {
		return nil, err
	}
	if m.Format != shardFormat || len(m.Assets) == 0 || !strings.HasPrefix(m.Playlist, "#EXTM3U") {
		return nil, errors.New("invalid HLS shard manifest")
	}
	for name, a := range m.Assets {
		if !segmentPattern.MatchString(name) || a.Size <= 0 || a.Size > 32*1024*1024 || len(a.SHA256) != 64 || s.nodes[a.Node] == nil {
			return nil, errors.New("invalid or unavailable HLS node mapping")
		}
	}
	s.mu.Lock()
	if len(s.manifests) >= 128 {
		s.manifests = make(map[string]*shardManifest)
	}
	s.manifests[key] = m
	s.mu.Unlock()
	return m, nil
}

func (s *Sharded) OpenHLSAsset(ctx context.Context, key, asset string) (io.ReadCloser, error) {
	m, err := s.manifest(ctx, key)
	if err != nil {
		return nil, err
	}
	if asset == "index.m3u8" {
		return io.NopCloser(strings.NewReader(m.Playlist)), nil
	}
	a, ok := m.Assets[asset]
	if !ok {
		return nil, os.ErrNotExist
	}
	select {
	case s.readSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	keepSlot := false
	defer func() {
		if !keepSlot {
			<-s.readSlots
		}
	}()
	f, err := s.nodes[a.Node].openAsset(ctx, key, asset)
	if err != nil {
		return nil, err
	}
	// Validate before writing a response: segments are small, bounded by the
	// publisher. This also detects corrupt files rather than sending bad video.
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, a.Size+1))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if int64(len(raw)) != a.Size || hex.EncodeToString(sum[:]) != a.SHA256 {
		return nil, errors.New("HLS segment checksum mismatch")
	}
	keepSlot = true
	return &shardReader{Reader: bytes.NewReader(raw), release: func() { <-s.readSlots }}, nil
}

type shardReader struct {
	*bytes.Reader
	once    sync.Once
	release func()
}

func (r *shardReader) Close() error { r.once.Do(r.release); return nil }

func (s *Sharded) PublishHLS(ctx context.Context, key, dir string) error {
	if !keyPattern.MatchString(key) {
		return errors.New("invalid video key")
	}
	playlist, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		return err
	}
	if len(playlist) > 512*1024 || !strings.HasPrefix(string(playlist), "#EXTM3U") {
		return errors.New("invalid HLS playlist")
	}
	names := []string{}
	for _, line := range strings.Split(string(playlist), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !segmentPattern.MatchString(line) {
			return errors.New("invalid HLS segment name")
		}
		names = append(names, line)
	}
	if len(names) == 0 || len(names) > 10000 {
		return errors.New("invalid HLS segment count")
	}
	m := &shardManifest{Format: shardFormat, Playlist: string(playlist), Assets: make(map[string]shardAsset)}
	seed := sha256.Sum256([]byte(key))
	offset := int(seed[0]) % len(s.order)
	for i, name := range names {
		local := filepath.Join(dir, name)
		data, err := os.ReadFile(local)
		if err != nil {
			return err
		}
		if len(data) == 0 || len(data) > 32*1024*1024 {
			return errors.New("HLS segment must be between 1 byte and 32 MiB")
		}
		sum := sha256.Sum256(data)
		var last error
		var chosen string
		// Skip unavailable/full nodes at publish time; persisted mappings ensure
		// subsequent reads never depend on a modulo over the current node count.
		for attempt := 0; attempt < len(s.order); attempt++ {
			id := s.order[(offset+i+attempt)%len(s.order)]
			if last = s.nodes[id].putAsset(ctx, key, name, local); last == nil {
				chosen = id
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		if chosen == "" {
			return fmt.Errorf("store HLS segment %s: %w", name, last)
		}
		m.Assets[name] = shardAsset{Node: chosen, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(raw) > 1024*1024 {
		return errors.New("HLS manifest too large")
	}
	if err = s.primary.Put(ctx, key, bytes.NewReader(raw)); err != nil {
		return err
	}
	s.forget(key)
	return nil
}

func (s *Sharded) Delete(ctx context.Context, key string) error {
	if !keyPattern.MatchString(key) {
		return errors.New("invalid video key")
	}
	s.forget(key)
	var errs []error
	for _, id := range s.order {
		if err := s.nodes[id].removeAssets(ctx, key); err != nil {
			errs = append(errs, fmt.Errorf("node %s: %w", id, err))
		}
	}
	// Keep the manifest until all nodes have acknowledged deletion, so the
	// existing durable outbox can retry partial failures.
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return s.primary.Delete(ctx, key)
}

func (s *SFTP) assetDir(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid video key")
	}
	return path.Join(s.cfg.Root, key+".hls"), nil
}
func (s *SFTP) putAsset(ctx context.Context, key, name, local string) error {
	dir, err := s.assetDir(key)
	if err != nil {
		return err
	}
	if !segmentPattern.MatchString(name) {
		return errors.New("invalid HLS asset")
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	stat, err := c.StatVFS(s.cfg.Root)
	if err != nil {
		return err
	}
	in, err := os.Open(local)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if stat.Bavail*stat.Frsize < uint64(info.Size())+1024*1024*1024 {
		return errors.New("HLS node has less than 1 GiB reserve")
	}
	if err = c.MkdirAll(dir); err != nil {
		return err
	}
	dst := path.Join(dir, name)
	out, err := c.OpenFile(dst+".part", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	err = errors.Join(copyErr, out.Close())
	if err != nil {
		_ = c.Remove(dst + ".part")
		return err
	}
	if err = c.PosixRename(dst+".part", dst); err != nil {
		_ = c.Remove(dst + ".part")
		return err
	}
	// Read back to verify transfer before publishing a manifest.
	check, err := c.Open(dst)
	if err != nil {
		return err
	}
	defer check.Close()
	remoteHash := sha256.New()
	_, err = io.Copy(remoteHash, check)
	if err != nil {
		return err
	}
	if _, err = in.Seek(0, io.SeekStart); err != nil {
		return err
	}
	localHash := sha256.New()
	if _, err = io.Copy(localHash, in); err != nil {
		return err
	}
	if !bytes.Equal(remoteHash.Sum(nil), localHash.Sum(nil)) {
		return errors.New("HLS transfer checksum mismatch")
	}
	return nil
}
func (s *SFTP) openAsset(ctx context.Context, key, name string) (File, error) {
	dir, err := s.assetDir(key)
	if err != nil {
		return nil, err
	}
	if !segmentPattern.MatchString(name) {
		return nil, errors.New("invalid HLS asset")
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	f, err := c.Open(path.Join(dir, name))
	if err != nil {
		close()
		return nil, err
	}
	return &remoteFile{f, close}, nil
}
func (s *SFTP) removeAssets(ctx context.Context, key string) error {
	dir, err := s.assetDir(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	entries, err := c.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!segmentPattern.MatchString(name) && !segmentPattern.MatchString(strings.TrimSuffix(name, ".part"))) {
			return errors.New("unexpected file in HLS directory")
		}
		if err = c.Remove(path.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err = c.RemoveDirectory(dir)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
