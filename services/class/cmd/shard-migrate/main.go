// shard-migrate atomically replaces a legacy ZIP with a distributed manifest.
// Keep its local backup until production playback has been verified.
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/config"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: shard-migrate OBJECT_KEY BACKUP_DIRECTORY")
	}
	key, backup := os.Args[1], os.Args[2]
	if key != "--check" && !regexp.MustCompile(`^[a-f0-9]{32}\.(mp4|webm)$`).MatchString(key) {
		log.Fatal("invalid key")
	}
	cfg := config.Load()
	primary, err := videostorage.New(cfg.VideoStorage)
	if err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(os.Getenv("VIDEO_HLS_NODES_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	var nodes []videostorage.NodeConfig
	if err = json.Unmarshal(raw, &nodes); err != nil {
		log.Fatal(err)
	}
	shards, err := videostorage.NewSharded(primary, nodes)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if key == "--check" {
		for _, node := range nodes {
			if node.Config.HostKeyAlgorithm == "" {
				node.Config.HostKeyAlgorithm = "ssh-ed25519"
			}
			store, err := videostorage.New(node.Config)
			if err != nil {
				log.Fatal(err)
			}
			if err := store.CheckSpace(ctx, 0); err != nil {
				log.Fatalf("node %s: %v", node.ID, err)
			}
			fmt.Println("Node ready", node.ID)
		}
		return
	}
	if r, e := shards.OpenHLSAsset(ctx, key, "index.m3u8"); e == nil {
		r.Close()
		fmt.Println("Already distributed", key)
		return
	} else if e != videostorage.ErrLegacyBundle {
		log.Fatal(e)
	}
	if err = os.MkdirAll(backup, 0700); err != nil {
		log.Fatal(err)
	}
	src, err := primary.Open(ctx, key)
	if err != nil {
		log.Fatal(err)
	}
	archive := filepath.Join(backup, key+".zip")
	out, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		src.Close()
		log.Fatal(err)
	}
	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	src.Close()
	if copyErr != nil || closeErr != nil {
		log.Fatal("backup copy failed", copyErr, closeErr)
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		log.Fatal(err)
	}
	defer z.Close()
	dir, err := os.MkdirTemp(backup, "unpack-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	valid := regexp.MustCompile(`^(index\.m3u8|segment[0-9]{6}\.ts)$`)
	var total uint64
	for _, entry := range z.File {
		if !valid.MatchString(entry.Name) || entry.Method != zip.Store || entry.UncompressedSize64 > 32*1024*1024 {
			log.Fatal("invalid archive entry")
		}
		total += entry.UncompressedSize64
		if total > 3*uint64(cfg.VideoMaxBytes) {
			log.Fatal("archive exceeds configured size")
		}
		r, err := entry.Open()
		if err != nil {
			log.Fatal(err)
		}
		f, err := os.OpenFile(filepath.Join(dir, entry.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			r.Close()
			log.Fatal(err)
		}
		_, err = io.Copy(f, r)
		r.Close()
		closeErr = f.Close()
		if err != nil || closeErr != nil {
			log.Fatal("extract failed", err, closeErr)
		}
	}
	if err = shards.PublishHLS(ctx, key, dir); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Distributed", key, "backup", archive)
}
