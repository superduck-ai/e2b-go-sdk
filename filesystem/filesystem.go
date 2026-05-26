package filesystem

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/envd"
	envdfs "github.com/e2b-dev/e2b-go-sdk/envd/filesystem"
	"github.com/e2b-dev/e2b-go-sdk/internal/shared"
)

const (
	defaultWatchTimeoutMs    = 60000
	keepalivePingIntervalSec = 50
	keepalivePingHeader      = "Keepalive-Ping-Interval"
)

type FileType string

const (
	FileTypeFile FileType = "file"
	FileTypeDir  FileType = "dir"
)

type WriteInfo struct {
	Name string   `json:"name"`
	Type FileType `json:"type,omitempty"`
	Path string   `json:"path"`
}

type EntryInfo struct {
	WriteInfo
	Size          int64      `json:"size"`
	Mode          int        `json:"mode"`
	Permissions   string     `json:"permissions"`
	Owner         string     `json:"owner"`
	Group         string     `json:"group"`
	ModifiedTime  *time.Time `json:"modifiedTime,omitempty"`
	SymlinkTarget string     `json:"symlinkTarget,omitempty"`
}

type WriteEntry struct {
	Path string
	Data io.Reader
}

type FilesystemRequestOpts struct {
	RequestTimeoutMs *int
	User             string
}

type FilesystemWriteOpts struct {
	FilesystemRequestOpts
	Gzip bool
}

type FilesystemReadOpts struct {
	FilesystemRequestOpts
	Gzip   bool
	Format string // Deprecated: Go Read always returns bytes; use ReadText for strings.
}

type FilesystemListOpts struct {
	FilesystemRequestOpts
	Depth int
}

type WatchOpts struct {
	FilesystemRequestOpts
	TimeoutMs *int
	OnExit    func(err error)
	Recursive bool
}

type connectionConfig struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
}

type Filesystem struct {
	connectionConfig *connectionConfig
	envdVersion      string
	httpClient       *http.Client
}

type streamEnvelope struct {
	payload json.RawMessage
	err     error
}

func NewFilesystem(cfg *struct {
	ApiKey           string
	AccessToken      string
	Domain           string
	ApiUrl           string
	SandboxUrl       string
	Debug            bool
	RequestTimeoutMs int
	Headers          map[string]string
}, envdVersion string) *Filesystem {
	var resolved *connectionConfig
	if cfg != nil {
		resolved = &connectionConfig{
			ApiKey:           cfg.ApiKey,
			AccessToken:      cfg.AccessToken,
			Domain:           cfg.Domain,
			ApiUrl:           cfg.ApiUrl,
			SandboxUrl:       cfg.SandboxUrl,
			Debug:            cfg.Debug,
			RequestTimeoutMs: cfg.RequestTimeoutMs,
			Headers:          cfg.Headers,
		}
	}
	return &Filesystem{
		connectionConfig: resolved,
		envdVersion:      envdVersion,
		httpClient:       &http.Client{},
	}
}

func (f *Filesystem) baseUrl() string {
	return f.connectionConfig.SandboxUrl
}

func (f *Filesystem) headers(user string) map[string]string {
	h := make(map[string]string)
	for k, v := range f.connectionConfig.Headers {
		h[k] = v
	}
	for k, v := range envd.AuthenticationHeader(f.envdVersion, user) {
		h[k] = v
	}
	return h
}

func (f *Filesystem) resolveUser(user string) string {
	if user == "" && versionGTE(f.envdVersion, envd.EnvdDefaultUser) {
		return ""
	}
	if user == "" {
		return "user"
	}
	return user
}

func (f *Filesystem) requestTimeout(timeoutMs *int) *int {
	if timeoutMs != nil {
		return timeoutMs
	}
	if f.connectionConfig.RequestTimeoutMs <= 0 {
		return nil
	}
	timeout := f.connectionConfig.RequestTimeoutMs
	return &timeout
}

func requestContext(ctx context.Context, timeoutMs *int) (context.Context, context.CancelFunc) {
	if timeoutMs == nil {
		return ctx, func() {}
	}
	if *timeoutMs == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
}

func (f *Filesystem) connectUnary(ctx context.Context, path string, reqBody interface{}, respBody interface{}, user string) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	u := f.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range f.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var connectErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &connectErr) == nil && connectErr.Code != "" {
			return wrapFilesystemError(envd.HandleRpcError(connectErr.Code, connectErr.Message))
		}
		return fmt.Errorf("connect RPC error: %d %s", resp.StatusCode, string(body))
	}
	if respBody != nil {
		return json.Unmarshal(body, respBody)
	}
	return nil
}

func (f *Filesystem) connectServerStream(ctx context.Context, path string, reqBody interface{}, user string) (io.ReadCloser, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	u := f.baseUrl() + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(envd.EncodeConnectEnvelope(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set(keepalivePingHeader, fmt.Sprintf("%d", keepalivePingIntervalSec))
	for k, v := range f.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var connectErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &connectErr) == nil && connectErr.Code != "" {
			return nil, wrapFilesystemError(envd.HandleRpcError(connectErr.Code, connectErr.Message))
		}
		return nil, fmt.Errorf("connect RPC error: %d %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (f *Filesystem) Read(ctx context.Context, path string, opts *FilesystemReadOpts) ([]byte, error) {
	body, _, err := f.readFile(ctx, path, opts)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (f *Filesystem) ReadText(ctx context.Context, path string, opts *FilesystemReadOpts) (string, error) {
	data, resp, err := f.readFile(ctx, path, opts)
	if err != nil {
		return "", err
	}
	if resp != nil && resp.Header.Get("Content-Length") == "0" {
		return "", nil
	}
	return string(data), nil
}

func (f *Filesystem) readFile(ctx context.Context, path string, opts *FilesystemReadOpts) ([]byte, *http.Response, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	user = f.resolveUser(user)
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)

	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()

	u := fmt.Sprintf("%s/files?path=%s", f.baseUrl(), url.QueryEscape(path))
	if user != "" {
		u += "&username=" + url.QueryEscape(user)
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range f.headers(user) {
		req.Header.Set(k, v)
	}
	if opts != nil && opts.Gzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, nil, wrapFilesystemError(envd.HandleEnvdApiError(resp.StatusCode, body))
	}
	return body, resp, nil
}

func (f *Filesystem) Write(ctx context.Context, path string, data io.Reader, opts *FilesystemWriteOpts) (*WriteInfo, error) {
	var requestTimeoutMs *int
	user := ""
	useGzip := false
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
		useGzip = opts.Gzip
	}
	user = f.resolveUser(user)
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)

	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()

	u := fmt.Sprintf("%s/files?path=%s", f.baseUrl(), url.QueryEscape(path))
	if user != "" {
		u += "&username=" + url.QueryEscape(user)
	}

	headers := f.headers(user)
	var body io.Reader
	var contentType string

	if versionGTE(f.envdVersion, envd.EnvdOctetStreamUpload) {
		if useGzip {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			if _, err := io.Copy(gz, data); err != nil {
				gz.Close()
				return nil, err
			}
			if err := gz.Close(); err != nil {
				return nil, err
			}
			body = &buf
			headers["Content-Encoding"] = "gzip"
		} else {
			body = data
		}
		contentType = "application/octet-stream"
	} else {
		// Use multipart form upload
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", path)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, data); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		body = &buf
		contentType = writer.FormDataContentType()
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, wrapFilesystemError(envd.HandleEnvdApiError(resp.StatusCode, respBody))
	}

	var infos []WriteInfo
	if len(respBody) > 0 && json.Unmarshal(respBody, &infos) == nil && len(infos) > 0 {
		return &infos[0], nil
	}

	return nil, fmt.Errorf("Expected to receive information about written file")
}

func (f *Filesystem) WriteFiles(ctx context.Context, files []WriteEntry, opts *FilesystemWriteOpts) ([]WriteInfo, error) {
	if len(files) == 0 {
		return []WriteInfo{}, nil
	}

	if versionGTE(f.envdVersion, envd.EnvdOctetStreamUpload) {
		results := make([]WriteInfo, 0, len(files))
		for _, file := range files {
			info, err := f.Write(ctx, file.Path, file.Data, opts)
			if err != nil {
				return results, err
			}
			results = append(results, *info)
		}
		return results, nil
	}

	return f.writeMultipartFiles(ctx, files, opts)
}

func (f *Filesystem) writeMultipartFiles(ctx context.Context, files []WriteEntry, opts *FilesystemWriteOpts) ([]WriteInfo, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	user = f.resolveUser(user)
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)

	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()

	var queryPath string
	if len(files) == 1 {
		queryPath = files[0].Path
	}

	u := fmt.Sprintf("%s/files", f.baseUrl())
	query := url.Values{}
	if queryPath != "" {
		query.Set("path", queryPath)
	}
	if user != "" {
		query.Set("username", user)
	}
	if encoded := query.Encode(); encoded != "" {
		u += "?" + encoded
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, file := range files {
		part, err := writer.CreateFormFile("file", file.Path)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, file.Data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, u, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range f.headers(user) {
		req.Header.Set(k, v)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, wrapFilesystemError(envd.HandleEnvdApiError(resp.StatusCode, respBody))
	}

	var infos []WriteInfo
	if len(respBody) > 0 && json.Unmarshal(respBody, &infos) == nil && len(infos) > 0 {
		return infos, nil
	}

	return nil, fmt.Errorf("Expected to receive information about written file")
}

func (f *Filesystem) List(ctx context.Context, path string, opts *FilesystemListOpts) ([]EntryInfo, error) {
	var requestTimeoutMs *int
	user := ""
	depth := int32(1)
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
		if opts.Depth > 0 {
			depth = int32(opts.Depth)
		} else if opts.Depth < 0 {
			return nil, fmt.Errorf("depth should be at least one")
		}
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.ListDirRequest{Path: path, Depth: depth}
	var resp envdfs.ListDirResponse
	if err := f.connectUnary(reqCtx, "/filesystem.Filesystem/ListDir", req, &resp, user); err != nil {
		return nil, err
	}
	entries := make([]EntryInfo, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		info, ok := convertEntryInfo(e)
		if ok {
			entries = append(entries, info)
		}
	}
	return entries, nil
}

func (f *Filesystem) MakeDir(ctx context.Context, path string, opts *FilesystemRequestOpts) (bool, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.MakeDirRequest{Path: path}
	var resp envdfs.MakeDirResponse
	err := f.connectUnary(reqCtx, "/filesystem.Filesystem/MakeDir", req, &resp, user)
	if err != nil {
		if rpcErr, ok := err.(*envd.RpcError); ok {
			if rpcErr.Code == "already_exists" || strings.Contains(rpcErr.Message, "already exists") {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func (f *Filesystem) Rename(ctx context.Context, oldPath, newPath string, opts *FilesystemRequestOpts) (*EntryInfo, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.MoveRequest{Source: oldPath, Destination: newPath}
	var resp envdfs.MoveResponse
	if err := f.connectUnary(reqCtx, "/filesystem.Filesystem/Move", req, &resp, user); err != nil {
		return nil, err
	}
	if resp.Entry == nil {
		return nil, fmt.Errorf("Expected to receive information about moved object")
	}
	info, _ := convertEntryInfo(resp.Entry)
	return &info, nil
}

func (f *Filesystem) Remove(ctx context.Context, path string, opts *FilesystemRequestOpts) error {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.RemoveRequest{Path: path}
	return f.connectUnary(reqCtx, "/filesystem.Filesystem/Remove", req, nil, user)
}

func (f *Filesystem) Exists(ctx context.Context, path string, opts *FilesystemRequestOpts) (bool, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.StatRequest{Path: path}
	var resp envdfs.StatResponse
	err := f.connectUnary(reqCtx, "/filesystem.Filesystem/Stat", req, &resp, user)
	if err != nil {
		if isFilesystemNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *Filesystem) GetInfo(ctx context.Context, path string, opts *FilesystemRequestOpts) (*EntryInfo, error) {
	var requestTimeoutMs *int
	user := ""
	if opts != nil {
		user = opts.User
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	reqCtx, cancel := requestContext(ctx, requestTimeoutMs)
	defer cancel()
	req := &envdfs.StatRequest{Path: path}
	var resp envdfs.StatResponse
	if err := f.connectUnary(reqCtx, "/filesystem.Filesystem/Stat", req, &resp, user); err != nil {
		return nil, err
	}
	if resp.Entry == nil {
		return nil, fmt.Errorf("Expected to receive information about the file or directory")
	}
	info, _ := convertEntryInfo(resp.Entry)
	return &info, nil
}

func (f *Filesystem) WatchDir(ctx context.Context, path string, onEvent func(FilesystemEvent), opts *WatchOpts) (*WatchHandle, error) {
	var requestTimeoutMs *int
	var timeoutMs *int
	user := ""
	recursive := false
	var onExit func(err error)
	if opts != nil {
		user = opts.User
		recursive = opts.Recursive
		onExit = opts.OnExit
		timeoutMs = opts.TimeoutMs
		requestTimeoutMs = opts.RequestTimeoutMs
	}
	requestTimeoutMs = f.requestTimeout(requestTimeoutMs)
	if recursive && !versionGTE(f.envdVersion, envd.EnvdVersionRecursiveWatch) {
		return nil, fmt.Errorf("You need to update the template to use recursive watching. You can do this by running `e2b template build` in the directory with the template.")
	}

	req := &envdfs.WatchDirRequest{Path: path, Recursive: recursive}
	requestCtx, clearRequestTimeout, cancelRequestTimeout := requestTimeoutStreamContext(ctx, requestTimeoutMs)
	streamCtx, streamCancel := streamContext(requestCtx, timeoutMs, defaultWatchTimeoutMs)
	body, err := f.connectServerStream(streamCtx, "/filesystem.Filesystem/WatchDir", req, user)
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		return nil, err
	}

	ch := make(chan streamEnvelope, 16)
	go readStreamEnvelopes(body, ch)

	firstMsg, ok, err := waitForFirstEvent(ch, requestTimeoutMs)
	if err != nil {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, wrapFilesystemError(err)
	}
	if !ok {
		streamCancel()
		cancelRequestTimeout()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}
	clearRequestTimeout()

	var firstResp envdfs.WatchDirResponse
	if err := json.Unmarshal(firstMsg, &firstResp); err != nil {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("failed to parse watch start event: %w", err)
	}
	started := firstResp.Start != nil || (firstResp.Started != nil && *firstResp.Started)
	if !started {
		streamCancel()
		body.Close()
		return nil, fmt.Errorf("Expected start event")
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	handle := newWatchHandle(func() {
		cancel()
		streamCancel()
		body.Close()
	}, onExit)

	notifyExit := func(err error) {
		if handle.stoppedByUser() {
			handle.exit(nil)
			return
		}
		handle.exit(err)
	}

	go func() {
		defer cancelRequestTimeout()
		defer streamCancel()
		defer body.Close()
		for {
			select {
			case <-cancelCtx.Done():
				notifyExit(envd.HandleStreamContextError(streamCtx.Err()))
				return
			case msg, ok := <-ch:
				if !ok {
					notifyExit(envd.HandleStreamContextError(streamCtx.Err()))
					return
				}
				if msg.err != nil {
					notifyExit(wrapFilesystemError(msg.err))
					return
				}
				var resp envdfs.WatchDirResponse
				if json.Unmarshal(msg.payload, &resp) != nil {
					continue
				}
				eventResp := resp.Filesystem
				if eventResp == nil {
					eventResp = resp.Event
				}
				if eventResp != nil && onEvent != nil {
					if event, ok := convertFsEvent(eventResp); ok {
						onEvent(event)
					}
				}
			}
		}
	}()

	return handle, nil
}

// readStreamEnvelopes reads Connect protocol envelopes from a streaming response.
func readStreamEnvelopes(reader io.Reader, ch chan<- streamEnvelope) {
	defer close(ch)
	header := make([]byte, 5)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		flags := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		payload := make([]byte, length)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		if flags&0x02 != 0 {
			if err := envd.ParseConnectEndStreamError(payload); err != nil {
				ch <- streamEnvelope{err: err}
			}
			return
		}
		ch <- streamEnvelope{payload: json.RawMessage(payload)}
	}
}

func convertEntryInfo(e *envdfs.EntryInfo) (EntryInfo, bool) {
	if e == nil {
		return EntryInfo{}, false
	}
	var ft FileType
	switch e.Type {
	case envdfs.FileTypeFile:
		ft = FileTypeFile
	case envdfs.FileTypeDirectory:
		ft = FileTypeDir
	default:
		return EntryInfo{}, false
	}
	info := EntryInfo{
		WriteInfo: WriteInfo{
			Name: e.Name,
			Type: ft,
			Path: e.Path,
		},
		Size:          e.Size,
		Mode:          int(e.Mode),
		Permissions:   e.Permissions,
		Owner:         e.Owner,
		Group:         e.Group,
		SymlinkTarget: e.SymlinkTarget,
	}
	if e.ModifiedTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, e.ModifiedTime); err == nil {
			info.ModifiedTime = &t
		}
	}
	return info, true
}

func convertFsEvent(e *envdfs.FilesystemEvent) (FilesystemEvent, bool) {
	var t FilesystemEventType
	switch e.Type {
	case envdfs.EventTypeCreate:
		t = FilesystemEventCreate
	case envdfs.EventTypeWrite:
		t = FilesystemEventWrite
	case envdfs.EventTypeRemove:
		t = FilesystemEventRemove
	case envdfs.EventTypeRename:
		t = FilesystemEventRename
	case envdfs.EventTypeChmod:
		t = FilesystemEventChmod
	default:
		return FilesystemEvent{}, false
	}
	return FilesystemEvent{Name: e.Name, Type: t}, true
}

// versionGTE returns true if version >= minVersion (semver comparison).
func versionGTE(version, minVersion string) bool {
	if version == "" {
		return true
	}
	parseSemver := func(v string) (int, int, int) {
		var major, minor, patch int
		fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
		return major, minor, patch
	}
	maj1, min1, pat1 := parseSemver(version)
	maj2, min2, pat2 := parseSemver(minVersion)
	if maj1 != maj2 {
		return maj1 > maj2
	}
	if min1 != min2 {
		return min1 > min2
	}
	return pat1 >= pat2
}

func waitForFirstEvent(ch <-chan streamEnvelope, timeoutMs *int) (json.RawMessage, bool, error) {
	if timeoutMs == nil || *timeoutMs == 0 {
		msg, ok := <-ch
		if !ok {
			return nil, false, nil
		}
		if msg.err != nil {
			return nil, false, wrapFilesystemError(msg.err)
		}
		return msg.payload, true, nil
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			return nil, false, nil
		}
		if msg.err != nil {
			return nil, false, wrapFilesystemError(msg.err)
		}
		return msg.payload, true, nil
	case <-time.After(time.Duration(*timeoutMs) * time.Millisecond):
		return nil, false, envd.HandleRequestTimeoutError()
	}
}

func streamContext(ctx context.Context, timeoutMs *int, defaultTimeoutMs int) (context.Context, context.CancelFunc) {
	if timeoutMs == nil {
		return context.WithTimeout(ctx, time.Duration(defaultTimeoutMs)*time.Millisecond)
	}
	if *timeoutMs == 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
}

func requestTimeoutStreamContext(ctx context.Context, timeoutMs *int) (context.Context, func(), context.CancelFunc) {
	if timeoutMs == nil || *timeoutMs == 0 {
		return ctx, func() {}, func() {}
	}

	requestCtx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(time.Duration(*timeoutMs)*time.Millisecond, cancel)

	return requestCtx, func() {
			timer.Stop()
		}, func() {
			timer.Stop()
			cancel()
		}
}

func wrapFilesystemError(err error) error {
	if err == nil {
		return nil
	}
	if !isFilesystemNotFoundError(err) {
		return err
	}
	return &shared.FileNotFoundError{
		NotFoundError: shared.NotFoundError{
			SandboxError: shared.SandboxError{
				Message: filesystemErrorMessage(err),
			},
		},
	}
}

func isFilesystemNotFoundError(err error) bool {
	var fileErr *shared.FileNotFoundError
	if errors.As(err, &fileErr) {
		return true
	}

	var rpcErr *envd.RpcError
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == "not_found" || strings.Contains(strings.ToLower(rpcErr.Message), "not found")
	}

	var apiErr *envd.EnvdApiError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}

	var notFoundErr *envd.NotFoundError
	if errors.As(err, &notFoundErr) {
		return true
	}

	return false
}

func filesystemErrorMessage(err error) string {
	var fileErr *shared.FileNotFoundError
	if errors.As(err, &fileErr) && fileErr.NotFoundError.SandboxError.Message != "" {
		return fileErr.NotFoundError.SandboxError.Message
	}

	var rpcErr *envd.RpcError
	if errors.As(err, &rpcErr) && rpcErr.Message != "" {
		return rpcErr.Message
	}

	var apiErr *envd.EnvdApiError
	if errors.As(err, &apiErr) && apiErr.Body != "" {
		return apiErr.Body
	}

	var notFoundErr *envd.NotFoundError
	if errors.As(err, &notFoundErr) && notFoundErr.Message != "" {
		return notFoundErr.Message
	}

	return err.Error()
}
