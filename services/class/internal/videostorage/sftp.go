// Package videostorage stores private class videos outside the public asset store.
package videostorage

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type File interface {
	io.ReadSeekCloser
	Stat() (os.FileInfo, error)
}

type Store interface {
	Put(context.Context, string, io.Reader) error
	Open(context.Context, string) (File, error)
	Delete(context.Context, string) error
}

type Config struct {
	Host, User, Root, HostFingerprint      string
	Port                                   int
	Password, PasswordFile, PrivateKeyFile string
	MaxConnections                         int
}

type SFTP struct {
	cfg       Config
	sshConfig *ssh.ClientConfig
	slots     chan struct{}
}

// Refresh a bounded I/O deadline so a hung remote server cannot hold every
// connection slot forever; context cancellation still closes the socket sooner.
type idleConn struct{ net.Conn }

func (c *idleConn) Read(p []byte) (int, error) {
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	return c.Conn.Read(p)
}
func (c *idleConn) Write(p []byte) (int, error) {
	c.Conn.SetWriteDeadline(time.Now().Add(60 * time.Second))
	return c.Conn.Write(p)
}

func New(cfg Config) (*SFTP, error) {
	if cfg.Host == "" || cfg.User == "" || cfg.Port < 1 || cfg.Port > 65535 || !strings.HasPrefix(cfg.Root, "/") || cfg.Root == "/" {
		return nil, errors.New("video SFTP requires host, user, valid port and an absolute non-root directory")
	}
	if !strings.HasPrefix(cfg.HostFingerprint, "SHA256:") || len(cfg.HostFingerprint) != 50 {
		return nil, errors.New("VIDEO_SFTP_HOST_FINGERPRINT must contain the trusted SHA256 SSH fingerprint")
	}
	var auth []ssh.AuthMethod
	if cfg.PasswordFile != "" {
		data, err := os.ReadFile(cfg.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("read video SFTP password file: %w", err)
		}
		cfg.Password = strings.TrimRight(string(data), "\r\n")
	}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if cfg.PrivateKeyFile != "" {
		data, err := os.ReadFile(cfg.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read video SFTP key: %w", err)
		}
		key, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, errors.New("invalid video SFTP private key")
		}
		auth = append(auth, ssh.PublicKeys(key))
	}
	if len(auth) == 0 {
		return nil, errors.New("video SFTP requires a password file or private key")
	}
	if cfg.MaxConnections < 1 || cfg.MaxConnections > 32 {
		return nil, errors.New("VIDEO_SFTP_MAX_CONNECTIONS must be between 1 and 32")
	}
	return &SFTP{cfg: cfg, slots: make(chan struct{}, cfg.MaxConnections), sshConfig: &ssh.ClientConfig{
		User: cfg.User, Auth: auth, Timeout: 10 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if subtle.ConstantTimeCompare([]byte(ssh.FingerprintSHA256(key)), []byte(cfg.HostFingerprint)) != 1 {
				return errors.New("video SFTP host fingerprint mismatch")
			}
			return nil
		},
	}}, nil
}

var keyPattern = regexp.MustCompile(`^[a-f0-9]{32}\.(mp4|webm)$`)

func (s *SFTP) filename(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid video storage key")
	}
	return path.Join(s.cfg.Root, key), nil
}

// Each operation owns a bounded SSH connection. Cancellation closes the socket,
// including blocked SFTP reads/writes; streams are never loaded into RAM in full.
func (s *SFTP) connect(ctx context.Context) (*sftp.Client, func(), error) {
	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	release := func() { <-s.slots }
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)))
	if err != nil {
		release()
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	handshakeTimer := time.AfterFunc(10*time.Second, func() { conn.Close() })
	defer handshakeTimer.Stop()
	cc, chans, reqs, err := ssh.NewClientConn(&idleConn{conn}, net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)), s.sshConfig)
	if err != nil {
		stop()
		conn.Close()
		release()
		return nil, nil, err
	}
	sshClient := ssh.NewClient(cc, chans, reqs)
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		stop()
		sshClient.Close()
		release()
		return nil, nil, err
	}
	var once sync.Once
	close := func() { once.Do(func() { stop(); conn.Close(); client.Close(); sshClient.Close(); release() }) }
	return client, close, nil
}

func (s *SFTP) Put(ctx context.Context, key string, src io.Reader) error {
	name, err := s.filename(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	// Keep incomplete uploads separate from playable files.
	f, err := c.OpenFile(name+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, src)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		c.Remove(name + ".part")
		return errors.Join(copyErr, closeErr)
	}
	if err := c.Rename(name+".part", name); err != nil {
		c.Remove(name + ".part")
		return err
	}
	return nil
}

type remoteFile struct {
	*sftp.File
	closeConnection func()
}

func (f *remoteFile) Close() error { err := f.File.Close(); f.closeConnection(); return err }

func (s *SFTP) Open(ctx context.Context, key string) (File, error) {
	name, err := s.filename(key)
	if err != nil {
		return nil, err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	f, err := c.Open(name)
	if err != nil {
		close()
		return nil, err
	}
	return &remoteFile{f, close}, nil
}

func (s *SFTP) Delete(ctx context.Context, key string) error {
	name, err := s.filename(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	err = c.Remove(name)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
