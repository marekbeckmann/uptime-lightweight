package config

import (
	"context"
	"io/fs"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches configDir recursively and notifies on debounced updates
type Watcher struct {
	configDir string
	onChange  func(*RuntimeConfig)
	watcher   *fsnotify.Watcher
	mu        sync.Mutex
}

func NewWatcher(configDir string, onChange func(*RuntimeConfig)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		configDir: configDir,
		onChange:  onChange,
		watcher:   fw,
	}

	if err := w.addRecursive(configDir); err != nil {
		fw.Close()
		return nil, err
	}

	return w, nil
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return w.watcher.Add(path)
		}
		return nil
	})
}

// Start begins listening for file events and debounces reloads with 500ms
func (w *Watcher) Start(ctx context.Context) {
	var (
		debounceTimer *time.Timer
		debounceMu    sync.Mutex
	)

	for {
		select {
		case <-ctx.Done():
			w.watcher.Close()
			return
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] error: %v", err)
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Add new subdirectories if created
			if event.Has(fsnotify.Create) {
				w.addRecursive(event.Name)
			}

			// Debounce 500ms
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				w.reload()
			})
			debounceMu.Unlock()
		}
	}
}

func (w *Watcher) reload() {
	log.Printf("[watcher] config modification detected, re-evaluating %s", w.configDir)
	newCfg, err := Load(w.configDir)
	if err != nil {
		log.Printf("[watcher] syntax error loading config (ignoring): %v", err)
		return
	}

	errs := Validate(newCfg)
	if len(errs) > 0 {
		log.Printf("[watcher] validation failed (ignoring reload): %v", errs)
		return
	}

	log.Printf("[watcher] configuration successfully reloaded (%d endpoints, %d zones)", len(newCfg.Endpoints), len(newCfg.Zones))
	w.onChange(newCfg)
}
