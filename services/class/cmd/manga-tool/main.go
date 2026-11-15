package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/config"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/manga"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() {
	input := flag.String("html", "", "HTML Mokuro input")
	inspect := flag.String("inspect-json", "", "Write parsed pages JSON")
	patch := flag.String("patch", "", "JSON array of {pageIndex,blockIndex,lines}")
	output := flag.String("output", "", "Write modified HTML to a new file")
	check := flag.Bool("check-storage", false, "Probe private image store and clean up the probe")
	flag.Parse()
	if *check {
		if e := checkStore(); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		fmt.Println("Private manga image storage: write/read/delete OK")
		return
	}
	if e := convert(*input, *inspect, *patch, *output); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func convert(input, inspect, patch, output string) error {
	source, e := os.ReadFile(input)
	if e != nil {
		return e
	}
	parsed, e := manga.ParseHTML(string(source), filepath.Base(input))
	if e != nil {
		return e
	}
	if inspect != "" {
		data, _ := json.Marshal(parsed)
		if e = os.WriteFile(inspect, data, 0600); e != nil {
			return e
		}
	}
	if output == "" {
		fmt.Printf("Inspected %d pages\n", len(parsed.Pages))
		return nil
	}
	inPath, _ := filepath.Abs(input)
	outPath, _ := filepath.Abs(output)
	if inPath == outPath {
		return fmt.Errorf("write to a new output file to preserve original")
	}
	pages := make([]manga.Page, len(parsed.Pages))
	for i, p := range parsed.Pages {
		pages[i] = p.Page
	}
	if patch != "" {
		var edits []struct {
			PageIndex  int      `json:"pageIndex"`
			BlockIndex int      `json:"blockIndex"`
			Lines      []string `json:"lines"`
		}
		raw, e := os.ReadFile(patch)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &edits); e != nil {
			return e
		}
		for _, edit := range edits {
			if edit.PageIndex < 0 || edit.PageIndex >= len(pages) || edit.BlockIndex < 0 || edit.BlockIndex >= len(pages[edit.PageIndex].Blocks) {
				return fmt.Errorf("invalid edit index")
			}
			pages[edit.PageIndex].Blocks[edit.BlockIndex].Lines = edit.Lines
		}
		for i := range pages {
			if e = manga.Validate(&pages[i]); e != nil {
				return e
			}
		}
	}
	updated, e := manga.ExportHTML(string(source), pages)
	if e != nil {
		return e
	}
	return os.WriteFile(output, updated, 0600)
}
func checkStore() error {
	store, e := videostorage.NewImages(config.Load().VideoStorage)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		return e
	}
	key := hex.EncodeToString(random[:]) + ".png"
	data := []byte("manga-storage-probe")
	if e = store.Put(ctx, key, bytes.NewReader(data)); e != nil {
		return e
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		store.Delete(cleanup, key)
	}()
	f, e := store.Open(ctx, key)
	if e != nil {
		return e
	}
	raw, e := io.ReadAll(io.LimitReader(f, 100))
	f.Close()
	if e != nil {
		return e
	}
	if !bytes.Equal(raw, data) {
		return fmt.Errorf("image read mismatch")
	}
	return store.Delete(ctx, key)
}
