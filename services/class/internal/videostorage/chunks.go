package videostorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// ChunkStore stages a video on the storage server, without using the app disk.
type ChunkStore interface {
	CheckSpace(context.Context, int64) error
	WriteChunk(context.Context, string, int64, io.Reader) error
	FinishUpload(context.Context, string, int64) error
	AbortUpload(context.Context, string) error
}

func (s *SFTP) CheckSpace(ctx context.Context, size int64) error {
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	stat, err := c.StatVFS(s.cfg.Root)
	if err != nil {
		return fmt.Errorf("check video disk: %w", err)
	}
	if stat.Bavail*stat.Frsize < uint64(size*3+512*1024*1024) {
		return errors.New("video disk needs space for source, converted video and 512 MB reserve")
	}
	return nil
}

func (s *SFTP) WriteChunk(ctx context.Context, key string, offset int64, input io.Reader) error {
	name, err := s.filename(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	f, err := c.OpenFile(name+".upload", os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if offset < 0 || info.Size() < offset {
		return errors.New("missing previous upload chunk")
	}
	// The DB's received offset is authoritative, including after an interrupted request.
	if err = f.Truncate(offset); err != nil {
		return err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(f, input)
	if err != nil {
		_ = f.Truncate(offset)
		return err
	}
	return f.Close()
}

func (s *SFTP) FinishUpload(ctx context.Context, key string, size int64) error {
	name, err := s.filename(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	// Rename may already have succeeded before a DB/network failure. Retrying is safe.
	if info, err := c.Stat(name); err == nil && info.Size() == size {
		return nil
	}
	info, err := c.Stat(name + ".upload")
	if err != nil {
		return err
	}
	if info.Size() != size {
		return errors.New("incomplete uploaded video")
	}
	return c.Rename(name+".upload", name)
}

func (s *SFTP) AbortUpload(ctx context.Context, key string) error {
	name, err := s.filename(key)
	if err != nil {
		return err
	}
	c, close, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	err = c.Remove(name + ".upload")
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
