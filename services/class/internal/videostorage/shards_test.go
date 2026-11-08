package videostorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memVideo struct{ *bytes.Reader }

func (f *memVideo) Close() error               { return nil }
func (f *memVideo) Stat() (os.FileInfo, error) { return nil, errors.New("unused") }

type manifestStore struct {
	data map[string][]byte
	fail bool
}

func (s *manifestStore) Put(_ context.Context, k string, r io.Reader) error {
	if s.fail {
		return errors.New("publish failed")
	}
	b, e := io.ReadAll(r)
	s.data[k] = b
	return e
}
func (s *manifestStore) Open(_ context.Context, k string) (File, error) {
	b, ok := s.data[k]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &memVideo{bytes.NewReader(b)}, nil
}
func (s *manifestStore) Delete(_ context.Context, k string) error { delete(s.data, k); return nil }

type memoryNode struct {
	data map[string][]byte
	down bool
}

func (n *memoryNode) putAsset(_ context.Context, k, name, local string) error {
	if n.down {
		return errors.New("offline")
	}
	b, e := os.ReadFile(local)
	n.data[k+"/"+name] = b
	return e
}
func (n *memoryNode) openAsset(_ context.Context, k, name string) (File, error) {
	if n.down {
		return nil, errors.New("offline")
	}
	b, ok := n.data[k+"/"+name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &memVideo{bytes.NewReader(b)}, nil
}
func (n *memoryNode) removeAssets(_ context.Context, k string) error {
	if n.down {
		return errors.New("offline")
	}
	for name := range n.data {
		if strings.HasPrefix(name, k+"/") {
			delete(n.data, name)
		}
	}
	return nil
}
func shardFixture(t *testing.T) (*Sharded, *manifestStore, []*memoryNode, string, string) {
	t.Helper()
	primary := &manifestStore{data: map[string][]byte{}}
	s := &Sharded{primary: primary, nodes: map[string]shardBackend{}, manifests: map[string]*shardManifest{}, readSlots: make(chan struct{}, 4)}
	nodes := []*memoryNode{}
	for i := 0; i < 4; i++ {
		id := fmt.Sprint(i)
		n := &memoryNode{data: map[string][]byte{}}
		nodes = append(nodes, n)
		s.nodes[id] = n
		s.order = append(s.order, id)
	}
	dir := t.TempDir()
	raw := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key\"\n"
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("segment%06d.ts", i)
		raw += name + "\n"
		if e := os.WriteFile(filepath.Join(dir, name), []byte("encrypted-"+name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	return s, primary, nodes, strings.Repeat("a", 32) + ".webm", dir
}
func TestShardsDistributeOnceAndReadAfterRestart(t *testing.T) {
	s, p, nodes, key, dir := shardFixture(t)
	ctx := context.Background()
	if e := s.PublishHLS(ctx, key, dir); e != nil {
		t.Fatal(e)
	}
	for _, n := range nodes {
		if len(n.data) != 2 {
			t.Fatalf("expected two unique segments per node, got %d", len(n.data))
		}
	}
	// IDs, not the order/count of configured nodes, determine reads.
	s.order = []string{"3", "2", "1", "0"}
	s.forget(key)
	r, e := s.OpenHLSAsset(ctx, key, "segment000000.ts")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "encrypted-segment000000.ts" {
		t.Fatal(string(b))
	}
	if len(p.data[key]) == 0 {
		t.Fatal("missing manifest")
	}
	for _, n := range nodes {
		for k := range n.data {
			n.data[k] = []byte("corrupt")
		}
	}
	if _, e = s.OpenHLSAsset(ctx, key, "segment000000.ts"); e == nil {
		t.Fatal("corruption accepted")
	}
	if _, e = s.OpenHLSAsset(ctx, key, "../../key"); !os.IsNotExist(e) {
		t.Fatal("path traversal accepted", e)
	}
}
func TestShardsOfflinePublishAndDeleteRetry(t *testing.T) {
	s, p, nodes, key, dir := shardFixture(t)
	ctx := context.Background()
	nodes[0].down = true
	if e := s.PublishHLS(ctx, key, dir); e != nil {
		t.Fatal(e)
	}
	if len(nodes[0].data) != 0 {
		t.Fatal("wrote offline node")
	}
	if e := s.Delete(ctx, key); e == nil {
		t.Fatal("expected retryable delete failure")
	}
	if len(p.data[key]) == 0 {
		t.Fatal("lost manifest before deletion acknowledged")
	}
	nodes[0].down = false
	if e := s.Delete(ctx, key); e != nil {
		t.Fatal(e)
	}
	if len(p.data) != 0 {
		t.Fatal("manifest retained")
	}
	for _, n := range nodes {
		n.down = true
	}
	if e := s.PublishHLS(ctx, key, dir); e == nil {
		t.Fatal("publish succeeded without storage")
	}
	if len(p.data) != 0 {
		t.Fatal("published incomplete video")
	}
}
func TestShardsLegacyAndFailedManifestCommit(t *testing.T) {
	s, p, _, key, dir := shardFixture(t)
	ctx := context.Background()
	p.data[key] = []byte("PK archive")
	if _, e := s.OpenHLSAsset(ctx, key, "index.m3u8"); !errors.Is(e, ErrLegacyBundle) {
		t.Fatal(e)
	}
	p.fail = true
	if e := s.PublishHLS(ctx, key, dir); e == nil {
		t.Fatal("commit failure ignored")
	}
	if string(p.data[key]) != "PK archive" {
		t.Fatal("legacy video replaced on failed commit")
	}
}

func TestShardReadersBoundMemoryAndReleaseOnClose(t *testing.T) {
	s, _, _, key, dir := shardFixture(t)
	ctx := context.Background()
	if err := s.PublishHLS(ctx, key, dir); err != nil {
		t.Fatal(err)
	}
	var readers []io.ReadCloser
	for i := 0; i < 4; i++ {
		r, err := s.OpenHLSAsset(ctx, key, "segment000000.ts")
		if err != nil {
			t.Fatal(err)
		}
		readers = append(readers, r)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.OpenHLSAsset(canceled, key, "segment000000.ts"); !errors.Is(err, context.Canceled) {
		t.Fatal("read limit did not respect cancellation", err)
	}
	for _, r := range readers {
		r.Close()
		r.Close()
	}
	r, err := s.OpenHLSAsset(ctx, key, "segment000000.ts")
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}

func TestShardProgressDoesNotCountFailedTransfers(t *testing.T) {
	s, p, nodes, key, dir := shardFixture(t)
	var completed []int
	ctx := WithPublishProgress(context.Background(), func(done, total int) {
		if total != 8 {
			t.Fatal(total)
		}
		completed = append(completed, done)
		// Simulate all storage disappearing after the first verified segment.
		for _, node := range nodes {
			node.down = true
		}
	})
	if err := s.PublishHLS(ctx, key, dir); err == nil {
		t.Fatal("publish succeeded after storage disappeared")
	}
	if len(completed) != 1 || completed[0] != 1 || len(p.data) != 0 {
		t.Fatal("failed transfers were counted or manifest published", completed)
	}
}
