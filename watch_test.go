package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A watched directory that is moved and then deleted before fsnotify handles
// the move causes fsnotify to report an error while holding its lock. If the
// error is not consumed, subsequent calls to watcher.Add block forever.
func TestWatchErrorsDoNotBlock(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	dirChan := make(chan *dir)
	fileChan := make(chan *file)
	delChan := make(chan string)

	w := newWatch(dirChan, fileChan, delChan)
	w.start()
	w.add(root)
	w.add(sub)

	// nobody is reading dirChan yet, so the watch loop and fsnotify stall
	os.WriteFile(filepath.Join(root, "x"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "y"), nil, 0o644)
	time.Sleep(100 * time.Millisecond)

	if err := os.Rename(sub, filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}

	quit := make(chan struct{})
	defer close(quit)
	defer w.stop()
	go func() {
		for {
			select {
			case <-dirChan:
			case <-fileChan:
			case <-delChan:
			case <-quit:
				return
			}
		}
	}()
	time.Sleep(200 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		w.add(root)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watch.add blocked: watcher deadlocked")
	}
}
