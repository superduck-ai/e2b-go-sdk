package filesystem

type FilesystemEventType int

const (
	FilesystemEventChmod FilesystemEventType = iota
	FilesystemEventCreate
	FilesystemEventRemove
	FilesystemEventRename
	FilesystemEventWrite
)

type FilesystemEvent struct {
	Name string
	Type FilesystemEventType
}

type WatchHandle struct {
	stop   func()
	onExit func(err error)
}

func NewWatchHandle(stop func(), onExit func(err error)) *WatchHandle {
	return &WatchHandle{stop: stop, onExit: onExit}
}

func (w *WatchHandle) Stop() {
	w.stop()
}
