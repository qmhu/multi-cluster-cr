package configwatcher

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/radovskyb/watcher"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"qmhu/multi-cluster-cr/pkg/config"
)

// FileWatcher watches a set of files, convert to kubeconfig and delivering events to a channel.
type FileWatcher struct {
	watcher   *watcher.Watcher
	logger    logr.Logger
	eventChan chan Event
	errorChan chan error
	closeChan chan struct{}
	configMap map[string]*config.NamedConfig
	mu        sync.Mutex
}

func NewFileWatcher(fileOrDir string) (*FileWatcher, error) {
	watcher := watcher.New()

	// add a kubeconfig file or directory that contains some kubeconfig files
	if err := watcher.Add(fileOrDir); err != nil {
		return nil, err
	}

	fileWatcher := &FileWatcher{
		watcher:   watcher,
		logger:    log.Log.WithName("file-watcher"),
		eventChan: make(chan Event),
		errorChan: make(chan error),
		closeChan: make(chan struct{}),
		configMap: make(map[string]*config.NamedConfig), // filename+configname->kubeconfig
	}

	go fileWatcher.watch(fileOrDir)

	return fileWatcher, nil
}

func (w *FileWatcher) Events() <-chan Event {
	return w.eventChan
}

func (w *FileWatcher) Errors() <-chan error {
	return w.errorChan
}

func (w *FileWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// close watcher first
	w.watcher.Wait()
	w.watcher.Close()

	// notify watch() to close
	w.closeChan <- struct{}{}
}

func (w *FileWatcher) watch(fileOrDir string) {
	defer close(w.eventChan)
	defer close(w.errorChan)
	defer w.Stop()

	go func() {
		for {
			select {
			case event := <-w.watcher.Event:
				// not interested in directory events
				if event.IsDir() {
					continue
				}

				// handle file update
				if event.Op == watcher.Create || event.Op == watcher.Write ||
					event.Op == watcher.Chmod || event.Op == watcher.Move ||
					event.Op == watcher.Rename {
					w.onFileUpdate(event.Path)
				}

				// handle file delete
				if event.Op == watcher.Remove {
					w.onFileDelete(event.Path)
				}
			case err := <-w.watcher.Error:
				// delivery internal watcher error to errorChan
				w.errorChan <- err
			case <-w.watcher.Closed:
				// if internal watcher closed, then close filewatcher too
				return
			case <-w.closeChan:
				// when closeChan closed, return
				return
			}
		}
	}()

	go func() {
		w.watcher.Wait()

		// trigger existing file to generate kubeconfig when filewatcher starts
		for path, watchFile := range w.watcher.WatchedFiles() {
			if !watchFile.IsDir() {
				w.watcher.Event <- watcher.Event{Op: watcher.Create, Path: path, FileInfo: watchFile}
			}
		}
	}()

	// start internal watcher here
	w.logger.Info("start watching", "filepath", fileOrDir)
	if err := w.watcher.Start(time.Millisecond * 100); err != nil {
		w.errorChan <- err
	}
}

func (w *FileWatcher) onFileUpdate(filename string) {
	configs, err := config.LoadConfigsFromConfigFile(filename)
	if err != nil {
		w.errorChan <- err
		return
	}

	if configs == nil || len(configs) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, config := range configs {
		if savedConfig, exist := w.configMap[buildKey(filename, config.Name)]; exist {
			if reflect.DeepEqual(savedConfig, config) {
				continue
			} else {
				w.eventChan <- Event{
					Type:   Updated,
					Config: config,
				}
			}
		} else {
			w.eventChan <- Event{
				Type:   Added,
				Config: config,
			}
		}

		w.configMap[buildKey(filename, config.Name)] = config
	}
}

func (w *FileWatcher) onFileDelete(filename string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for key, config := range w.configMap {
		if strings.HasPrefix(key, buildKeyPrefix(filename)) {
			w.eventChan <- Event{
				Type:   Deleted,
				Config: config,
			}
		}

		delete(w.configMap, buildKey(filename, config.Name))
	}
}

// buildKey return a unique key string for store a Config in configMap
func buildKey(filename string, name string) string {
	return buildKeyPrefix(filename) + name
}

// buildKeyPrefix return a unique key prefix string
func buildKeyPrefix(filename string) string {
	return filename + "|"
}
