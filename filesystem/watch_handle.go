package filesystem

import "sync"

type FilesystemEventType string

const (
	FilesystemEventChmod  FilesystemEventType = "chmod"
	FilesystemEventCreate FilesystemEventType = "create"
	FilesystemEventRemove FilesystemEventType = "remove"
	FilesystemEventRename FilesystemEventType = "rename"
	FilesystemEventWrite  FilesystemEventType = "write"
)

type FilesystemEvent struct {
	Name string
	Type FilesystemEventType
}

type WatchHandle struct {
	stop     func()
	onExit   func(err error)
	stopOnce sync.Once
	exitOnce sync.Once
	stopped  chan struct{}
}

func newWatchHandle(stop func(), onExit func(err error)) *WatchHandle {
	return &WatchHandle{
		stop:    stop,
		onExit:  onExit,
		stopped: make(chan struct{}),
	}
}

func (w *WatchHandle) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopped)
		w.stop()
	})
}

func (w *WatchHandle) exit(err error) {
	w.exitOnce.Do(func() {
		if w.onExit != nil {
			w.onExit(err)
		}
	})
}

func (w *WatchHandle) stoppedByUser() bool {
	select {
	case <-w.stopped:
		return true
	default:
		return false
	}
}
