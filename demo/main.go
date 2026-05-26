package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	e2b "github.com/e2b-dev/e2b-go-sdk"
	"github.com/e2b-dev/e2b-go-sdk/filesystem"
	"github.com/e2b-dev/e2b-go-sdk/volume"
)

//go:embed dist
var distFS embed.FS

var (
	db          *sql.DB
	subscribers = struct {
		sync.RWMutex
		list []chan []byte
	}{}
)

type LifecycleEvent struct {
	Version            string                 `json:"version"`
	ID                 string                 `json:"id"`
	Type               string                 `json:"type"`
	EventData          map[string]interface{} `json:"eventData"`
	SandboxBuildID     string                 `json:"sandboxBuildId,omitempty"`
	SandboxExecutionID string                 `json:"sandboxExecutionId,omitempty"`
	SandboxID          string                 `json:"sandboxId,omitempty"`
	SandboxTeamID      string                 `json:"sandboxTeamId,omitempty"`
	SandboxTemplateID  string                 `json:"sandboxTemplateId,omitempty"`
	Timestamp          string                 `json:"timestamp"`
}

func emit(evt LifecycleEvent) {
	if evt.Version == "" {
		evt.Version = "v2"
	}
	if evt.ID == "" {
		evt.ID = uuid.New().String()
	}
	if evt.Timestamp == "" {
		evt.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, _ := json.Marshal(evt)
	msg := []byte("data: " + string(data) + "\n\n")

	subscribers.RLock()
	defer subscribers.RUnlock()
	for _, ch := range subscribers.list {
		select {
		case ch <- msg:
		default:
		}
	}
}

func jsonResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres:postgres@10.21.48.115:5432/postgres?sslmode=disable"
	}
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "13001"
	}

	mux := http.NewServeMux()

	// Serve frontend from embedded dist/ directory
	distSubFS, _ := fs.Sub(distFS, "dist")
	fileServer := http.FileServer(http.FS(distSubFS))
	mux.Handle("GET /", fileServer)

	mux.HandleFunc("GET /api/sandboxes", handleListSandboxes)
	mux.HandleFunc("GET /api/events", handleSSE)
	mux.HandleFunc("GET /api/env", handleEnv)
	mux.HandleFunc("POST /api/sandbox", handleCreateSandbox)
	mux.HandleFunc("POST /api/sandbox/{id}/run", handleRunCommand)
	mux.HandleFunc("GET /api/sandbox/{id}/files", handleListFiles)
	mux.HandleFunc("GET /api/sandbox/{id}/files/info", handleFileInfo)
	mux.HandleFunc("GET /api/sandbox/{id}/files/read", handleReadFile)
	mux.HandleFunc("POST /api/sandbox/{id}/files/write", handleWriteFile)
	mux.HandleFunc("GET /api/sandbox/{id}/host", handleGetHost)
	mux.HandleFunc("POST /api/sandbox/{id}/snapshot", handleCreateSnapshot)
	mux.HandleFunc("GET /api/snapshots", handleListSnapshots)
	mux.HandleFunc("DELETE /api/snapshots/{id}", handleDeleteSnapshot)
	mux.HandleFunc("POST /api/sandbox/{id}/pause", handlePauseSandbox)
	mux.HandleFunc("POST /api/sandbox/{id}/resume", handleResumeSandbox)
	mux.HandleFunc("GET /api/volumes", handleListVolumes)
	mux.HandleFunc("POST /api/volumes", handleCreateVolume)
	mux.HandleFunc("GET /api/volumes/{id}", handleGetVolume)
	mux.HandleFunc("DELETE /api/volumes/{id}", handleDeleteVolume)
	mux.HandleFunc("GET /api/api-keys", handleAPIKeys)
	mux.HandleFunc("GET /api/access-tokens", handleAccessTokens)
	mux.HandleFunc("GET /api/db-tables", handleDBTables)
	mux.HandleFunc("GET /api/db-table/{schema}/{name}", handleDBTable)
	mux.HandleFunc("GET /api/templates", handleTemplates)
	mux.HandleFunc("DELETE /api/sandbox/{id}", handleKillSandbox)

	log.Printf("E2B demo running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleListSandboxes(w http.ResponseWriter, r *http.Request) {
	paginator := e2b.List(nil)
	var items []map[string]interface{}
	for paginator.HasNext {
		page, err := paginator.NextItems()
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		for _, s := range page {
			items = append(items, map[string]interface{}{
				"sandboxId":  s.SandboxID,
				"templateId": s.TemplateID,
				"name":       s.Name,
				"state":      s.State,
				"startedAt":  safeTime(s.StartedAt),
				"endAt":      safeTime(s.EndAt),
				"cpuCount":   s.CpuCount,
				"memoryMB":   s.MemoryMB,
				"metadata":   s.Metadata,
			})
		}
	}
	jsonResponse(w, 200, map[string]interface{}{"sandboxes": items})
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 64)
	subscribers.Lock()
	subscribers.list = append(subscribers.list, ch)
	subscribers.Unlock()

	defer func() {
		subscribers.Lock()
		for i, c := range subscribers.list {
			if c == ch {
				subscribers.list = append(subscribers.list[:i], subscribers.list[i+1:]...)
				break
			}
		}
		subscribers.Unlock()
		close(ch)
	}()

	w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			w.Write(msg)
			flusher.Flush()
		case <-ticker.C:
			w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func handleEnv(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("E2B_API_KEY")
	masked := ""
	if apiKey != "" {
		masked = "***" + apiKey[max(0, len(apiKey)-4):]
	}
	jsonResponse(w, 200, map[string]string{
		"E2B_DOMAIN":  os.Getenv("E2B_DOMAIN"),
		"E2B_API_URL": os.Getenv("E2B_API_URL"),
		"E2B_API_KEY": masked,
	})
}

func handleCreateSandbox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TemplateID          string `json:"templateId"`
		AllowInternetAccess *bool  `json:"allowInternetAccess,omitempty"`
		Network             *struct {
			AllowOut           []string `json:"allowOut"`
			DenyOut            []string `json:"denyOut"`
			AllowPublicTraffic *bool    `json:"allowPublicTraffic,omitempty"`
		} `json:"network,omitempty"`
		VolumeMounts map[string]string `json:"volumeMounts,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// allow empty body
	}
	if body.TemplateID == "" {
		jsonError(w, 400, "templateId is required")
		return
	}

	opts := &e2b.SandboxOpts{}
	if body.AllowInternetAccess != nil {
		opts.AllowInternetAccess = body.AllowInternetAccess
	}
	if body.Network != nil {
		net := &e2b.SandboxNetworkOpts{}
		if len(body.Network.AllowOut) > 0 {
			net.AllowOut = body.Network.AllowOut
		}
		if len(body.Network.DenyOut) > 0 {
			net.DenyOut = body.Network.DenyOut
		}
		if body.Network.AllowPublicTraffic != nil {
			net.AllowPublicTraffic = *body.Network.AllowPublicTraffic
		}
		opts.Network = net
	}
	if len(body.VolumeMounts) > 0 {
		opts.VolumeMounts = make(map[string]any, len(body.VolumeMounts))
		for mountPath, volumeName := range body.VolumeMounts {
			opts.VolumeMounts[mountPath] = volumeName
		}
	}
	if len(body.Metadata) > 0 {
		opts.Metadata = body.Metadata
	}

	ctx := r.Context()
	sb, err := e2b.Create(ctx, body.TemplateID, opts)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			EventData: map[string]interface{}{"message": err.Error(), "phase": "create"},
		})
		jsonError(w, 500, err.Error())
		return
	}

	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	emit(LifecycleEvent{
		Type:               "sandbox.lifecycle.created",
		SandboxID:          sb.SandboxID,
		SandboxExecutionID: sb.SandboxID,
		EventData: map[string]interface{}{
			"sandbox_metadata": map[string]interface{}{},
			"execution":        map[string]interface{}{"started_at": startedAt},
		},
	})
	jsonResponse(w, 200, map[string]interface{}{
		"sandboxId": sb.SandboxID,
		"startedAt": startedAt,
	})
}

func handleRunCommand(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Command string `json:"command"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	cmdId := uuid.New().String()
	t0 := time.Now()
	emit(LifecycleEvent{
		Type:      "sandbox.command.started",
		SandboxID: id,
		EventData: map[string]interface{}{"commandId": cmdId, "command": body.Command},
	})

	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.command.completed",
			SandboxID: id,
			EventData: map[string]interface{}{
				"commandId":   cmdId,
				"command":     body.Command,
				"exitCode":    -1,
				"duration_ms": time.Since(t0).Milliseconds(),
				"error":       err.Error(),
			},
		})
		jsonResponse(w, 200, map[string]interface{}{
			"stdout":   "",
			"stderr":   err.Error(),
			"exitCode": -1,
			"error":    err.Error(),
		})
		return
	}

	result, err := sb.Commands.Run(ctx, body.Command, nil)
	if err != nil {
		errMsg := err.Error()
		stdout, stderr := "", errMsg
		exitCode := -1
		if result != nil {
			stdout = result.Stdout
			stderr = result.Stderr
			exitCode = result.ExitCode
		}
		emit(LifecycleEvent{
			Type:      "sandbox.command.completed",
			SandboxID: id,
			EventData: map[string]interface{}{
				"commandId":   cmdId,
				"command":     body.Command,
				"exitCode":    exitCode,
				"duration_ms": time.Since(t0).Milliseconds(),
				"error":       errMsg,
			},
		})
		jsonResponse(w, 200, map[string]interface{}{
			"stdout":   stdout,
			"stderr":   stderr,
			"exitCode": exitCode,
			"error":    errMsg,
		})
		return
	}

	emit(LifecycleEvent{
		Type:      "sandbox.command.completed",
		SandboxID: id,
		EventData: map[string]interface{}{
			"commandId":   cmdId,
			"command":     body.Command,
			"exitCode":    result.ExitCode,
			"duration_ms": time.Since(t0).Milliseconds(),
		},
	})
	jsonResponse(w, 200, map[string]interface{}{
		"stdout":   result.Stdout,
		"stderr":   result.Stderr,
		"exitCode": result.ExitCode,
	})
}

func handleListFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	entries, err := sb.Files.List(ctx, path, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	var result []map[string]interface{}
	for _, e := range entries {
		result = append(result, map[string]interface{}{
			"name":        e.Name,
			"path":        e.Path,
			"type":        e.Type,
			"size":        e.Size,
			"permissions": e.Permissions,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"entries": result})
}

func handleFileInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonError(w, 400, "path is required")
		return
	}
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	info, err := sb.Files.GetInfo(ctx, path, nil)
	if err != nil {
		status := 500
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			status = 404
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"info": info})
}

func handleReadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	content, err := sb.Files.ReadText(ctx, path, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"content": content, "path": path})
}

func handleWriteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Files   []struct {
			Path string `json:"path"`
			Data string `json:"data"`
		} `json:"files"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	if len(body.Files) > 0 {
		var entries []filesystem.WriteEntry
		for _, f := range body.Files {
			if strings.TrimSpace(f.Path) == "" {
				continue
			}
			entries = append(entries, filesystem.WriteEntry{
				Path: f.Path,
				Data: strings.NewReader(f.Data),
			})
		}
		if len(entries) == 0 {
			jsonError(w, 400, "files array must contain entries with non-empty path")
			return
		}
		_, err = sb.Files.WriteFiles(ctx, entries, nil)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]interface{}{"ok": true, "mode": "multi", "count": len(entries)})
		return
	}

	wPath := strings.TrimSpace(body.Path)
	if wPath == "" {
		jsonError(w, 400, "path is required")
		return
	}
	_, err = sb.Files.Write(ctx, wPath, strings.NewReader(body.Content), nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "mode": "single", "path": wPath})
}

func handleGetHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	portStr := r.URL.Query().Get("port")
	port := 3000
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		jsonError(w, 502, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"host": sb.GetHost(port), "port": port})
}

func handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			SandboxID: id,
			EventData: map[string]interface{}{"message": err.Error(), "phase": "snapshot"},
		})
		jsonError(w, 502, err.Error())
		return
	}
	snap, err := sb.CreateSnapshot(ctx, nil)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			SandboxID: id,
			EventData: map[string]interface{}{"message": err.Error(), "phase": "snapshot"},
		})
		jsonError(w, 502, err.Error())
		return
	}
	emit(LifecycleEvent{
		Type:      "sandbox.lifecycle.checkpointed",
		SandboxID: id,
		EventData: map[string]interface{}{"snapshotId": snap.SnapshotID},
	})
	jsonResponse(w, 200, map[string]interface{}{"snapshotId": snap.SnapshotID})
}

func handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	sandboxId := r.URL.Query().Get("sandboxId")
	opts := &e2b.SnapshotListOpts{}
	if sandboxId != "" {
		opts.SandboxID = sandboxId
	}
	paginator := e2b.ListSnapshots(opts)
	var snapshots []map[string]interface{}
	for paginator.HasNext {
		page, err := paginator.NextItems()
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				jsonResponse(w, 200, map[string]interface{}{"snapshots": []interface{}{}, "listUnavailableReason": "list_snapshots_404"})
				return
			}
			jsonError(w, 502, err.Error())
			return
		}
		for _, s := range page {
			snapshots = append(snapshots, map[string]interface{}{
				"snapshotId": s.SnapshotID,
			})
		}
	}
	jsonResponse(w, 200, map[string]interface{}{"snapshots": snapshots})
}

func handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	deleted, err := e2b.DeleteSnapshot(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "deleted": deleted})
}

func handlePauseSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	paused, err := e2b.Pause(ctx, id, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already paused") {
			jsonResponse(w, 200, map[string]interface{}{"ok": true, "alreadyPaused": true})
			return
		}
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			SandboxID: id,
			EventData: map[string]interface{}{"message": err.Error(), "phase": "pause"},
		})
		jsonError(w, 500, err.Error())
		return
	}
	_ = paused
	emit(LifecycleEvent{
		Type:      "sandbox.lifecycle.paused",
		SandboxID: id,
		EventData: map[string]interface{}{"sandbox_metadata": map[string]interface{}{}},
	})
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "sdkPaused": paused})
}

func handleResumeSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	sb, err := e2b.Connect(ctx, id, nil)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			SandboxID: id,
			EventData: map[string]interface{}{"message": err.Error(), "phase": "resume"},
		})
		jsonError(w, 500, err.Error())
		return
	}
	emit(LifecycleEvent{
		Type:      "sandbox.lifecycle.resumed",
		SandboxID: id,
		EventData: map[string]interface{}{"sandbox_metadata": map[string]interface{}{}},
	})
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "sandboxId": sb.SandboxID})
}

func handleListVolumes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	volumes, err := volume.List(ctx, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	var result []map[string]interface{}
	for _, v := range volumes {
		result = append(result, map[string]interface{}{
			"volumeId": v.VolumeID,
			"name":     v.Name,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"volumes": result})
}

func handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		jsonError(w, 400, "name is required")
		return
	}
	ctx := r.Context()
	v, err := volume.Create(ctx, name, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"volumeId": v.VolumeID, "name": v.Name})
}

func handleGetVolume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	info, err := volume.GetInfo(ctx, id, nil)
	if err != nil {
		status := 500
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			status = 404
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"volumeId": info.VolumeID, "name": info.Name})
}

func handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	ok, err := volume.Destroy(ctx, id, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": ok})
}

func handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	teamId := r.URL.Query().Get("teamId")
	var rows *sql.Rows
	var err error
	query := `SELECT k.id, k.name, k.created_at, k.updated_at, k.last_used, k.team_id, k.api_key_prefix, k.api_key_mask_prefix, k.api_key_mask_suffix, k.api_key_length, t.name AS team_name
		FROM team_api_keys k
		LEFT JOIN teams t ON t.id = k.team_id`
	if teamId != "" {
		query += ` WHERE k.team_id = $1 ORDER BY k.created_at DESC`
		rows, err = db.QueryContext(r.Context(), query, teamId)
	} else {
		query += ` ORDER BY k.created_at DESC`
		rows, err = db.QueryContext(r.Context(), query)
	}
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var keys []map[string]interface{}
	for rows.Next() {
		var id, name, teamID, prefix, maskPrefix, maskSuffix string
		var teamName sql.NullString
		var createdAt, updatedAt time.Time
		var lastUsed sql.NullTime
		var keyLength int
		err := rows.Scan(&id, &name, &createdAt, &updatedAt, &lastUsed, &teamID, &prefix, &maskPrefix, &maskSuffix, &keyLength, &teamName)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		var lastUsedVal interface{}
		if lastUsed.Valid {
			lastUsedVal = lastUsed.Time
		}
		keys = append(keys, map[string]interface{}{
			"id":        id,
			"name":      name,
			"createdAt": createdAt,
			"updatedAt": updatedAt,
			"lastUsed":  lastUsedVal,
			"teamId":    teamID,
			"teamName":  teamName.String,
			"maskedKey": prefix + maskPrefix + "..." + maskSuffix,
			"keyLength": keyLength,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"keys": keys, "count": len(keys)})
}

func handleAccessTokens(w http.ResponseWriter, r *http.Request) {
	query := `SELECT t.id, t.name, t.created_at, t.user_id, t.access_token_prefix, t.access_token_mask_prefix, t.access_token_mask_suffix, t.access_token_length, u.email AS user_email
		FROM access_tokens t
		LEFT JOIN auth.users u ON u.id = t.user_id
		ORDER BY t.created_at DESC`
	rows, err := db.QueryContext(r.Context(), query)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var tokens []map[string]interface{}
	for rows.Next() {
		var id, name, userID, prefix, maskPrefix, maskSuffix string
		var userEmail sql.NullString
		var createdAt time.Time
		var tokenLength int
		err := rows.Scan(&id, &name, &createdAt, &userID, &prefix, &maskPrefix, &maskSuffix, &tokenLength, &userEmail)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		tokens = append(tokens, map[string]interface{}{
			"id":          id,
			"name":        name,
			"createdAt":   createdAt,
			"userId":      userID,
			"userEmail":   userEmail.String,
			"maskedToken": prefix + maskPrefix + "..." + maskSuffix,
			"tokenLength": tokenLength,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"tokens": tokens, "count": len(tokens)})
}

func handleDBTables(w http.ResponseWriter, r *http.Request) {
	query := `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY table_schema, table_name`
	rows, err := db.QueryContext(r.Context(), query)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var tables []map[string]string
	for rows.Next() {
		var schema, name string
		rows.Scan(&schema, &name)
		tables = append(tables, map[string]string{"table_schema": schema, "table_name": name})
	}
	jsonResponse(w, 200, map[string]interface{}{"tables": tables})
}

func handleDBTable(w http.ResponseWriter, r *http.Request) {
	schema := r.PathValue("schema")
	name := r.PathValue("name")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 100
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if limit > 500 {
		limit = 500
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	ctx := r.Context()

	// Get columns
	colRows, err := db.QueryContext(ctx, `SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, schema, name)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer colRows.Close()

	var columns []map[string]string
	for colRows.Next() {
		var colName, dataType, nullable string
		colRows.Scan(&colName, &dataType, &nullable)
		columns = append(columns, map[string]string{"column_name": colName, "data_type": dataType, "is_nullable": nullable})
	}
	if len(columns) == 0 {
		jsonError(w, 404, "table not found")
		return
	}

	// Count
	var total int
	db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM "%s"."%s"`, schema, name)).Scan(&total)

	// Data
	dataRows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT * FROM "%s"."%s" LIMIT %d OFFSET %d`, schema, name, limit, offset))
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer dataRows.Close()

	cols, _ := dataRows.Columns()
	var rows []map[string]interface{}
	for dataRows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		dataRows.Scan(valuePtrs...)
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		rows = append(rows, row)
	}

	jsonResponse(w, 200, map[string]interface{}{
		"schema":  schema,
		"table":   name,
		"columns": columns,
		"rows":    rows,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func handleTemplates(w http.ResponseWriter, r *http.Request) {
	teamId := r.URL.Query().Get("teamId")
	templateId := r.URL.Query().Get("templateId")
	ctx := r.Context()

	baseQuery := `SELECT e.id, e.created_at, e.updated_at, e.public, e.build_count,
		e.spawn_count, e.last_spawned_at, e.team_id, e.source,
		b.id AS build_id, b.status, b.dockerfile, b.start_cmd,
		b.vcpu, b.ram_mb, b.free_disk_size_mb, b.total_disk_size_mb,
		b.kernel_version, b.firecracker_version, b.envd_version,
		b.finished_at AS build_finished_at, b.status_group
	FROM envs e
	LEFT JOIN env_builds b ON b.env_id = e.id
		AND b.id = (SELECT eb.id FROM env_builds eb WHERE eb.env_id = e.id ORDER BY eb.created_at DESC LIMIT 1)`

	var rows *sql.Rows
	var err error
	if templateId != "" {
		rows, err = db.QueryContext(ctx, baseQuery+` WHERE e.id = $1`, templateId)
	} else if teamId != "" {
		rows, err = db.QueryContext(ctx, baseQuery+` WHERE e.team_id = $1::uuid ORDER BY e.updated_at DESC`, teamId)
	} else {
		rows, err = db.QueryContext(ctx, baseQuery+` ORDER BY e.updated_at DESC`)
	}
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var templates []map[string]interface{}
	var envIds []string
	for rows.Next() {
		var id, teamID string
		var createdAt, updatedAt time.Time
		var public bool
		var buildCount int
		var spawnCount int64
		var lastSpawnedAt sql.NullTime
		var source sql.NullString
		var buildId, status, dockerfile, startCmd sql.NullString
		var vcpu, ramMB, freeDisk, totalDisk sql.NullInt64
		var kernelVersion, firecrackerVersion, envdVersion sql.NullString
		var buildFinishedAt sql.NullTime
		var statusGroup sql.NullString

		err := rows.Scan(&id, &createdAt, &updatedAt, &public, &buildCount,
			&spawnCount, &lastSpawnedAt, &teamID, &source,
			&buildId, &status, &dockerfile, &startCmd,
			&vcpu, &ramMB, &freeDisk, &totalDisk,
			&kernelVersion, &firecrackerVersion, &envdVersion,
			&buildFinishedAt, &statusGroup)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}

		envIds = append(envIds, id)
		t := map[string]interface{}{
			"id":            id,
			"createdAt":     createdAt,
			"updatedAt":     updatedAt,
			"public":        public,
			"buildCount":    buildCount,
			"spawnCount":    spawnCount,
			"lastSpawnedAt": nullTimeVal(lastSpawnedAt),
			"teamId":        teamID,
			"source":        source.String,
		}
		if buildId.Valid {
			t["latestBuild"] = map[string]interface{}{
				"buildId":            buildId.String,
				"status":             status.String,
				"statusGroup":        statusGroup.String,
				"dockerfile":         dockerfile.String,
				"startCmd":           startCmd.String,
				"vcpu":               vcpu.Int64,
				"ramMB":              ramMB.Int64,
				"freeDiskSizeMB":     freeDisk.Int64,
				"totalDiskSizeMB":    totalDisk.Int64,
				"kernelVersion":      kernelVersion.String,
				"firecrackerVersion": firecrackerVersion.String,
				"envdVersion":        envdVersion.String,
				"finishedAt":         nullTimeVal(buildFinishedAt),
			}
		} else {
			t["latestBuild"] = nil
		}
		templates = append(templates, t)
	}

	// Fetch aliases
	if len(envIds) > 0 {
		placeholders := make([]string, len(envIds))
		args := make([]interface{}, len(envIds))
		for i, eid := range envIds {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = eid
		}
		aliasQuery := fmt.Sprintf(`SELECT env_id, alias, namespace FROM env_aliases WHERE env_id IN (%s)`, strings.Join(placeholders, ","))
		aliasRows, err := db.QueryContext(ctx, aliasQuery, args...)
		if err == nil {
			defer aliasRows.Close()
			aliasMap := map[string][]map[string]string{}
			for aliasRows.Next() {
				var envId, alias, namespace string
				aliasRows.Scan(&envId, &alias, &namespace)
				aliasMap[envId] = append(aliasMap[envId], map[string]string{"alias": alias, "namespace": namespace})
			}
			for _, t := range templates {
				id := t["id"].(string)
				if aliases, ok := aliasMap[id]; ok {
					t["aliases"] = aliases
				} else {
					t["aliases"] = []map[string]string{}
				}
			}
		}
	}

	jsonResponse(w, 200, map[string]interface{}{"templates": templates, "count": len(templates)})
}

func handleKillSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	t0 := time.Now()
	_, err := e2b.Kill(ctx, id, nil)
	if err != nil {
		emit(LifecycleEvent{
			Type:      "sandbox.error",
			SandboxID: id,
			EventData: map[string]interface{}{"message": err.Error(), "phase": "kill"},
		})
		jsonError(w, 500, err.Error())
		return
	}
	emit(LifecycleEvent{
		Type:      "sandbox.lifecycle.killed",
		SandboxID: id,
		EventData: map[string]interface{}{
			"sandbox_metadata": map[string]interface{}{},
			"execution":        map[string]interface{}{"execution_time": time.Since(t0).Milliseconds()},
		},
	})
	jsonResponse(w, 200, map[string]interface{}{"ok": true})
}

func safeTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

func nullTimeVal(t sql.NullTime) interface{} {
	if !t.Valid {
		return nil
	}
	return t.Time
}
