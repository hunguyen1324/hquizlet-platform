package videostorage

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Exercise the actual SSH handshake and SFTP protocol against an in-memory
// remote filesystem; this catches chroot paths and host pinning regressions.
func testSFTPServer(t *testing.T) Config {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) { return nil, nil }}
	cfg.AddHostKey(signer)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	handlers := sftp.InMemHandler()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					if ch.ChannelType() != "session" {
						ch.Reject(ssh.UnknownChannelType, "unsupported")
						continue
					}
					channel, requests, err := ch.Accept()
					if err != nil {
						return
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							var payload struct{ Name string }
							ssh.Unmarshal(req.Payload, &payload)
							if req.Type == "subsystem" && payload.Name == "sftp" {
								req.Reply(true, nil)
								srv := sftp.NewRequestServer(channel, handlers)
								srv.Serve()
								srv.Close()
								return
							}
							req.Reply(false, nil)
						}
					}()
				}
			}()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	p, _ := strconv.Atoi(port)
	return Config{Host: "127.0.0.1", Port: p, User: "test", Root: "/private-videos", HostFingerprint: ssh.FingerprintSHA256(signer.PublicKey()), Password: "test", MaxConnections: 2}
}

func TestSFTPRoundTripAndHostPinning(t *testing.T) {
	cfg := testSFTPServer(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, close, err := store.connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Mkdir("/private-videos"); err != nil {
		close()
		t.Fatal(err)
	}
	close()
	key := strings.Repeat("a", 32) + ".mp4"
	if err := store.Put(ctx, key, strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	f, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "3456789" {
		t.Fatalf("read %q: %v", data, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete absent file must be idempotent: %v", err)
	}
	if _, err := store.Open(ctx, key); err == nil {
		t.Fatal("deleted video remains readable")
	}
	if err := store.Put(ctx, "../index.js", strings.NewReader("bad")); err == nil {
		t.Fatal("accepted traversal")
	}
	bad := cfg
	bad.HostFingerprint = "SHA256:" + strings.Repeat("A", 43)
	wrong, err := New(bad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrong.Open(ctx, key); err == nil {
		t.Fatal("accepted wrong host key")
	}
}

func TestSFTPCancellationReleasesConnectionSlot(t *testing.T) {
	cfg := testSFTPServer(t)
	cfg.MaxConnections = 1
	store, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, close, err := store.connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer waitCancel()
	if _, _, err := store.connect(waitCtx); err == nil {
		t.Fatal("connection limit not enforced")
	}
	cancel()
	close()
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	defer retryCancel()
	_, close, err = store.connect(retryCtx)
	if err != nil {
		t.Fatal(err)
	}
	close()
}
