package filesystem

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/e2b-dev/e2b-go-sdk/envd"
	envdfs "github.com/e2b-dev/e2b-go-sdk/envd/filesystem"
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
	Format string // "text", "bytes", "stream"
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

// ConnectionConfig holds the connection configuration passed from the parent package.
type ConnectionConfig struct {
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
	connectionConfig *ConnectionConfig
	envdVersion      string
	httpClient       *http.Client
}

func NewFilesystem(connectionConfig *ConnectionConfig, envdVersion string) *Filesystem {
	timeout := time.Duration(connectionConfig.RequestTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &Filesystem{
		connectionConfig: connectionConfig,
		envdVersion:      envdVersion,
		httpClient:       &http.Client{Timeout: timeout},
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
	if user == "" {
		user = "user"
	}
	for k, v := range envd.AuthenticationHeader(f.envdVersion, user) {
		h[k] = v
	}
	return h
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
			return envd.HandleRpcError(connectErr.Code, connectErr.Message)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
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
			return nil, envd.HandleRpcError(connectErr.Code, connectErr.Message)
		}
		return nil, fmt.Errorf("connect RPC error: %d %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (f *Filesystem) Read(ctx context.Context, path string, opts *FilesystemReadOpts) ([]byte, error) {
	user := "user"
	if opts != nil && opts.User != "" {
		user = opts.User
	}
	u := fmt.Sprintf("%s/files?path=%s&username=%s", f.baseUrl(), url.QueryEscape(path), url.QueryEscape(user))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range f.headers(user) {
		req.Header.Set(k, v)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, envd.HandleEnvdApiError(resp.StatusCode, body)
	}
	return body, nil
}

func (f *Filesystem) ReadText(ctx context.Context, path string, opts *FilesystemReadOpts) (string, error) {
	data, err := f.Read(ctx, path, opts)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *Filesystem) Write(ctx context.Context, path string, data io.Reader, opts *FilesystemWriteOpts) (*WriteInfo, error) {
	user := "user"
	if opts != nil && opts.User != "" {
		user = opts.User
	}
	u := fmt.Sprintf("%s/files?path=%s&username=%s", f.baseUrl(), url.QueryEscape(path), url.QueryEscape(user))

	var body io.Reader
	var contentType string

	if versionGTE(f.envdVersion, envd.EnvdOctetStreamUpload) {
		body = data
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
		writer.Close()
		body = &buf
		contentType = writer.FormDataContentType()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
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
		return nil, envd.HandleEnvdApiError(resp.StatusCode, respBody)
	}
	return &WriteInfo{Path: path}, nil
}

func (f *Filesystem) WriteFiles(ctx context.Context, files []WriteEntry, opts *FilesystemWriteOpts) ([]WriteInfo, error) {
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

func (f *Filesystem) List(ctx context.Context, path string, opts *FilesystemListOpts) ([]EntryInfo, error) {
	user := ""
	depth := int32(1)
	if opts != nil {
		user = opts.User
		if opts.Depth > 0 {
			depth = int32(opts.Depth)
		}
	}
	req := &envdfs.ListDirRequest{Path: path, Depth: depth}
	var resp envdfs.ListDirResponse
	if err := f.connectUnary(ctx, "/filesystem.Filesystem/ListDir", req, &resp, user); err != nil {
		return nil, err
	}
	entries := make([]EntryInfo, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entries = append(entries, convertEntryInfo(e))
	}
	return entries, nil
}

func (f *Filesystem) MakeDir(ctx context.Context, path string, opts *FilesystemRequestOpts) (bool, error) {
	user := ""
	if opts != nil {
		user = opts.User
	}
	req := &envdfs.MakeDirRequest{Path: path}
	var resp envdfs.MakeDirResponse
	err := f.connectUnary(ctx, "/filesystem.Filesystem/MakeDir", req, &resp, user)
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
	user := ""
	if opts != nil {
		user = opts.User
	}
	req := &envdfs.MoveRequest{Source: oldPath, Destination: newPath}
	var resp envdfs.MoveResponse
	if err := f.connectUnary(ctx, "/filesystem.Filesystem/Move", req, &resp, user); err != nil {
		return nil, err
	}
	info := convertEntryInfo(resp.Entry)
	return &info, nil
}

func (f *Filesystem) Remove(ctx context.Context, path string, opts *FilesystemRequestOpts) error {
	user := ""
	if opts != nil {
		user = opts.User
	}
	req := &envdfs.RemoveRequest{Path: path}
	return f.connectUnary(ctx, "/filesystem.Filesystem/Remove", req, nil, user)
}

func (f *Filesystem) Exists(ctx context.Context, path string, opts *FilesystemRequestOpts) (bool, error) {
	user := ""
	if opts != nil {
		user = opts.User
	}
	req := &envdfs.StatRequest{Path: path}
	var resp envdfs.StatResponse
	err := f.connectUnary(ctx, "/filesystem.Filesystem/Stat", req, &resp, user)
	if err != nil {
		if rpcErr, ok := err.(*envd.RpcError); ok {
			if rpcErr.Code == "not_found" || strings.Contains(rpcErr.Message, "not found") {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func (f *Filesystem) GetInfo(ctx context.Context, path string, opts *FilesystemRequestOpts) (*EntryInfo, error) {
	user := ""
	if opts != nil {
		user = opts.User
	}
	req := &envdfs.StatRequest{Path: path}
	var resp envdfs.StatResponse
	if err := f.connectUnary(ctx, "/filesystem.Filesystem/Stat", req, &resp, user); err != nil {
		return nil, err
	}
	info := convertEntryInfo(resp.Entry)
	return &info, nil
}

func (f *Filesystem) WatchDir(ctx context.Context, path string, onEvent func(FilesystemEvent), opts *WatchOpts) (*WatchHandle, error) {
	user := ""
	recursive := false
	var onExit func(err error)
	if opts != nil {
		user = opts.User
		recursive = opts.Recursive
		onExit = opts.OnExit
	}

	req := &envdfs.WatchDirRequest{Path: path, Recursive: recursive}
	body, err := f.connectServerStream(ctx, "/filesystem.Filesystem/WatchDir", req, user)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	handle := NewWatchHandle(func() {
		cancel()
		body.Close()
	}, onExit)

	go func() {
		defer body.Close()
		ch := make(chan json.RawMessage, 16)
		go readStreamEnvelopes(body, ch)
		for {
			select {
			case <-cancelCtx.Done():
				if onExit != nil {
					onExit(nil)
				}
				return
			case msg, ok := <-ch:
				if !ok {
					if onExit != nil {
						onExit(nil)
					}
					return
				}
				var resp envdfs.WatchDirResponse
				if json.Unmarshal(msg, &resp) != nil {
					continue
				}
				if resp.Event != nil && onEvent != nil {
					onEvent(convertFsEvent(resp.Event))
				}
			}
		}
	}()

	return handle, nil
}

// readStreamEnvelopes reads Connect protocol envelopes from a streaming response.
func readStreamEnvelopes(reader io.Reader, ch chan<- json.RawMessage) {
	defer close(ch)
	header := make([]byte, 5)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			return
		}
		flags := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		payload := make([]byte, length)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}
		if flags&0x02 != 0 {
			return
		}
		ch <- json.RawMessage(payload)
	}
}

func convertEntryInfo(e *envdfs.EntryInfo) EntryInfo {
	if e == nil {
		return EntryInfo{}
	}
	ft := FileTypeFile
	if e.Type == envdfs.FileTypeDirectory {
		ft = FileTypeDir
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
	return info
}

func convertFsEvent(e *envdfs.FilesystemEvent) FilesystemEvent {
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
	}
	return FilesystemEvent{Name: e.Name, Type: t}
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
