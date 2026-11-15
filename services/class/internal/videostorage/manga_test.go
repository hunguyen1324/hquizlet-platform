package videostorage

import "testing"

func TestMangaStorageKeysStaySeparateFromVideoKeys(t *testing.T) {
	images := &SFTP{cfg: Config{Root: "/private-videos"}, images: true}
	videos := &SFTP{cfg: Config{Root: "/private-videos"}}
	key := "0123456789abcdef0123456789abcdef"
	if _, e := images.filename(key + ".jpg"); e != nil {
		t.Fatal(e)
	}
	if _, e := videos.filename(key + ".jpg"); e == nil {
		t.Fatal("video store accepted image")
	}
	if _, e := images.filename(key + ".mp4"); e == nil {
		t.Fatal("image store accepted video")
	}
	if _, e := images.filename("../" + key + ".jpg"); e == nil {
		t.Fatal("path traversal accepted")
	}
}
