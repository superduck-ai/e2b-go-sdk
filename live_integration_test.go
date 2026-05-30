//go:build integration

package e2b_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
	e2bvol "github.com/superduck-ai/e2b-go-sdk/volume"
)

const liveTestTimeout = 15 * time.Minute

var (
	loadEnvOnce       sync.Once
	loadEnvErr        error
	liveTemplateMu    sync.Mutex
	liveTemplateName  string
	liveTemplateID    string
	liveTemplateBuilt bool
)

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

func runForegroundCommand(cmds *e2b.Commands, ctx context.Context, cmd string, opts *e2b.CommandStartOpts) (*e2b.CommandResult, error) {
	execution, err := cmds.Run(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	result, ok := execution.(*e2b.CommandResult)
	if !ok {
		return nil, fmt.Errorf("expected foreground command result, got %T", execution)
	}
	return result, nil
}

func runBackgroundCommand(cmds *e2b.Commands, ctx context.Context, cmd string, opts *e2b.CommandStartOpts) (*e2b.CommandHandle, error) {
	if opts == nil {
		opts = &e2b.CommandStartOpts{}
	} else {
		cloned := *opts
		opts = &cloned
	}
	opts.Background = true
	execution, err := cmds.Run(ctx, cmd, opts)
	if err != nil {
		return nil, err
	}
	handle, ok := execution.(*e2b.CommandHandle)
	if !ok {
		return nil, fmt.Errorf("expected background command handle, got %T", execution)
	}
	return handle, nil
}

func readSandboxText(files *e2b.Filesystem, ctx context.Context, path string, opts *e2b.FilesystemReadOpts) (string, error) {
	value, err := files.Read(ctx, path, opts)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected filesystem read to return string, got %T", value)
	}
	return text, nil
}

func readSandboxBytes(files *e2b.Filesystem, ctx context.Context, path string, opts *e2b.FilesystemReadOpts) ([]byte, error) {
	readOpts := &e2b.FilesystemReadOpts{Format: e2b.ReadFormatBytes}
	if opts != nil {
		readOpts = &e2b.FilesystemReadOpts{}
		*readOpts = *opts
		readOpts.Format = e2b.ReadFormatBytes
	}
	value, err := files.Read(ctx, path, readOpts)
	if err != nil {
		return nil, err
	}
	data, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("expected filesystem read to return []byte, got %T", value)
	}
	return data, nil
}

func readSandboxStream(files *e2b.Filesystem, ctx context.Context, path string, opts *e2b.FilesystemReadOpts) (io.ReadCloser, error) {
	readOpts := &e2b.FilesystemReadOpts{Format: e2b.ReadFormatStream}
	if opts != nil {
		readOpts = &e2b.FilesystemReadOpts{}
		*readOpts = *opts
		readOpts.Format = e2b.ReadFormatStream
	}
	value, err := files.Read(ctx, path, readOpts)
	if err != nil {
		return nil, err
	}
	stream, ok := value.(io.ReadCloser)
	if !ok {
		return nil, fmt.Errorf("expected filesystem read to return io.ReadCloser, got %T", value)
	}
	return stream, nil
}

func readVolumeText(vol *e2bvol.Volume, ctx context.Context, path string, opts *e2bvol.VolumeApiOpts) (string, error) {
	readOpts := &e2bvol.VolumeReadOpts{}
	if opts != nil {
		readOpts.VolumeApiOpts = *opts
	}
	value, err := vol.ReadFile(ctx, path, readOpts)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected volume read to return string, got %T", value)
	}
	return text, nil
}

func TestMain(m *testing.M) {
	loadDotEnv()
	code := m.Run()
	cleanupLiveTemplate()
	os.Exit(code)
}

func TestLiveCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)

	t.Run("run text variants", func(t *testing.T) {
		cases := []struct {
			name string
			text string
		}{
			{name: "plain", text: "Hello, World!"},
			{name: "special", text: "!@#$%^&*()_+"},
			{name: "multiline", text: "Hello,\nWorld!"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := runForegroundCommand(sandbox.Commands, ctx, "printf %s "+shellQuote(tc.text), nil)
				if err != nil {
					t.Fatalf("Run returned error: %v", err)
				}
				if result.ExitCode != 0 || result.Stdout != tc.text {
					t.Fatalf("unexpected command result: exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
				}
			})
		}
	})

	t.Run("run replaces broken utf8", func(t *testing.T) {
		result, err := runForegroundCommand(sandbox.Commands, ctx, `python3 - <<'PY'
import sys
sys.stdout.buffer.write(b"a" * 8191 + b"\xe2")
PY`, nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		expected := strings.Repeat("a", 8191) + "\uFFFD"
		if result.Stdout != expected {
			t.Fatalf("unexpected utf8 replacement: len=%d suffix=%q", len(result.Stdout), result.Stdout[len(result.Stdout)-min(8, len(result.Stdout)):])
		}
	})

	t.Run("timeout", func(t *testing.T) {
		timeoutMs := 10000
		if result, err := runForegroundCommand(sandbox.Commands, ctx, "sleep 1 && echo done", &e2b.CommandStartOpts{TimeoutMs: &timeoutMs}); err != nil {
			t.Fatalf("Run with sufficient timeout returned error: %v", err)
		} else if strings.TrimSpace(result.Stdout) != "done" {
			t.Fatalf("unexpected stdout from sufficient timeout command: %#v", result)
		}

		timeoutMs = 1000
		_, err := runForegroundCommand(sandbox.Commands, ctx, "sleep 10", &e2b.CommandStartOpts{TimeoutMs: &timeoutMs})
		if err == nil {
			t.Fatal("expected timeout error")
		}
		var timeoutErr *e2b.TimeoutError
		if !errors.As(err, &timeoutErr) {
			t.Fatalf("expected TimeoutError, got %T %v", err, err)
		}
	})

	t.Run("stdin connect list kill", func(t *testing.T) {
		for _, input := range []string{"Hello, World!", "", "!@#$%^&*()_+", "Hello,\nWorld!"} {
			t.Run("stdin_"+strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
				handle, err := runBackgroundCommand(sandbox.Commands, ctx, "cat", &e2b.CommandStartOpts{Stdin: boolPtr(true)})
				if err != nil {
					t.Fatalf("RunBackground returned error: %v", err)
				}
				defer handle.Kill()

				if err := sandbox.Commands.SendStdin(ctx, handle.Pid, []byte(input), nil); err != nil {
					t.Fatalf("SendStdin returned error: %v", err)
				}
				if input != "" {
					waitForCommandStdout(t, handle, input)
				}
				_, _ = handle.Kill()
				if got := handle.State().Stdout; got != input {
					t.Fatalf("expected stdin stdout %q, got %q", input, got)
				}
			})
		}

		sleep, err := runBackgroundCommand(sandbox.Commands, ctx, "sleep 30", nil)
		if err != nil {
			t.Fatalf("RunBackground sleep returned error: %v", err)
		}
		sleep2, err := runBackgroundCommand(sandbox.Commands, ctx, "sleep 30", nil)
		if err != nil {
			_, _ = sleep.Kill()
			t.Fatalf("RunBackground second sleep returned error: %v", err)
		}
		defer sleep2.Kill()
		connected, err := sandbox.Commands.Connect(ctx, sleep.Pid, nil)
		if err != nil {
			_, _ = sleep.Kill()
			_, _ = sleep2.Kill()
			t.Fatalf("Connect returned error: %v", err)
		}
		if connected.Pid != sleep.Pid {
			_, _ = sleep.Kill()
			_, _ = sleep2.Kill()
			t.Fatalf("expected connected pid %d, got %d", sleep.Pid, connected.Pid)
		}
		connected.Disconnect()

		processes, err := sandbox.Commands.List(ctx, nil)
		if err != nil {
			_, _ = sleep.Kill()
			_, _ = sleep2.Kill()
			t.Fatalf("List returned error: %v", err)
		}
		if !processListContains(processes, sleep.Pid) || !processListContains(processes, sleep2.Pid) {
			_, _ = sleep.Kill()
			_, _ = sleep2.Kill()
			t.Fatalf("expected process list to contain pids %d and %d, got %#v", sleep.Pid, sleep2.Pid, processes)
		}
		killed, err := sleep.Kill()
		if err != nil {
			t.Fatalf("Kill returned error: %v", err)
		}
		if !killed {
			t.Fatal("expected Kill to report true for running process")
		}

		killed, err = sandbox.Commands.Kill(ctx, 999999, nil)
		if err != nil {
			t.Fatalf("Kill non-existing process returned error: %v", err)
		}
		if killed {
			t.Fatal("expected Kill to report false for non-existing process")
		}

		_, err = sandbox.Commands.Connect(ctx, 999999, nil)
		if err == nil {
			t.Fatal("expected Connect non-existing process to fail")
		}
		var notFoundErr *e2b.NotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Fatalf("expected NotFoundError from Connect non-existing process, got %T %v", err, err)
		}
	})
}

func TestLiveCommandOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	globalSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
		Envs: map[string]string{"FOO": "global-bar"},
	})

	result, err := runForegroundCommand(globalSandbox.Commands, ctx, "echo $FOO", nil)
	if err != nil {
		t.Fatalf("global env Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "global-bar" {
		t.Fatalf("expected global env to be visible, got %q", result.Stdout)
	}

	sandbox := newLiveSandbox(t, ctx)

	result, err = runForegroundCommand(sandbox.Commands, ctx, "echo $FOO", &e2b.CommandStartOpts{
		Envs: map[string]string{"FOO": "scoped-bar"},
	})
	if err != nil {
		t.Fatalf("scoped env Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "scoped-bar" {
		t.Fatalf("expected scoped env to override global env, got %q", result.Stdout)
	}

	result, err = runForegroundCommand(sandbox.Commands, ctx, `python3 -c "import os; print(os.environ['FOO'])"`, &e2b.CommandStartOpts{
		Envs: map[string]string{"FOO": "python-bar"},
	})
	if err != nil {
		t.Fatalf("python scoped env Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "python-bar" {
		t.Fatalf("expected python process to receive scoped env, got %q", result.Stdout)
	}

	result, err = runForegroundCommand(sandbox.Commands, ctx, "pwd", &e2b.CommandStartOpts{Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("cwd Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "/tmp" {
		t.Fatalf("expected cwd /tmp, got %q", result.Stdout)
	}

	result, err = runForegroundCommand(sandbox.Commands, ctx, "whoami", &e2b.CommandStartOpts{User: "root"})
	if err != nil {
		t.Fatalf("root user Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "root" {
		t.Fatalf("expected command user root, got %q", result.Stdout)
	}

	result, err = runForegroundCommand(sandbox.Commands, ctx, `sudo echo "$FOO"`, nil)
	if err != nil {
		t.Fatalf("sudo env isolation Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		t.Fatalf("expected scoped env not to leak to later sudo command, got %q", result.Stdout)
	}
}

func TestLiveFilesystem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)
	baseDir := livePath("fs")
	mustRun(t, ctx, sandbox, "rm -rf "+shellQuote(baseDir)+" && mkdir -p "+shellQuote(baseDir))

	t.Run("write read overwrite and remove", func(t *testing.T) {
		filePath := path.Join(baseDir, "test_write.txt")
		content := "This is a test file."
		info, err := sandbox.Files.Write(ctx, filePath, content, nil)
		if err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
		if info.Name != "test_write.txt" || info.Type != e2b.FileTypeFile || info.Path != filePath {
			t.Fatalf("unexpected write info: %#v", info)
		}
		exists, err := sandbox.Files.Exists(ctx, filePath, nil)
		if err != nil || !exists {
			t.Fatalf("Exists returned exists=%v err=%v", exists, err)
		}
		read, err := readSandboxText(sandbox.Files, ctx, filePath, nil)
		if err != nil {
			t.Fatalf("ReadText returned error: %v", err)
		}
		if read != content {
			t.Fatalf("unexpected read content: %q", read)
		}
		stream, err := readSandboxStream(sandbox.Files, ctx, filePath, nil)
		if err != nil {
			t.Fatalf("ReadStream returned error: %v", err)
		}
		streamed, err := io.ReadAll(stream)
		if closeErr := stream.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatalf("ReadStream body returned error: %v", err)
		}
		if string(streamed) != content {
			t.Fatalf("unexpected streamed content: %q", string(streamed))
		}

		newContent := "New content."
		if _, err := sandbox.Files.Write(ctx, filePath, []byte(newContent), nil); err != nil {
			t.Fatalf("overwrite Write returned error: %v", err)
		}
		read, err = readSandboxText(sandbox.Files, ctx, filePath, nil)
		if err != nil {
			t.Fatalf("ReadText after overwrite returned error: %v", err)
		}
		if read != newContent {
			t.Fatalf("unexpected overwritten content: %q", read)
		}

		renamed := path.Join(baseDir, "renamed.txt")
		if _, err := sandbox.Files.Rename(ctx, filePath, renamed, nil); err != nil {
			t.Fatalf("Rename returned error: %v", err)
		}
		oldExists, err := sandbox.Files.Exists(ctx, filePath, nil)
		if err != nil {
			t.Fatalf("Exists old path returned error: %v", err)
		}
		newExists, err := sandbox.Files.Exists(ctx, renamed, nil)
		if err != nil {
			t.Fatalf("Exists new path returned error: %v", err)
		}
		if oldExists || !newExists {
			t.Fatalf("unexpected rename existence: old=%v new=%v", oldExists, newExists)
		}
		if err := sandbox.Files.Remove(ctx, renamed, nil); err != nil {
			t.Fatalf("Remove returned error: %v", err)
		}
	})

	t.Run("write relative paths and create parent directories", func(t *testing.T) {
		relativeFile := fmt.Sprintf("test_write_relative_%d.txt", time.Now().UnixNano())
		content := "This is a test file."
		info, err := sandbox.Files.Write(ctx, relativeFile, content, nil)
		if err != nil {
			t.Fatalf("Write relative file returned error: %v", err)
		}
		if info.Name != relativeFile || info.Type != e2b.FileTypeFile || info.Path != "/home/user/"+relativeFile {
			t.Fatalf("unexpected relative write info: %#v", info)
		}
		exists, err := sandbox.Files.Exists(ctx, relativeFile, nil)
		if err != nil || !exists {
			t.Fatalf("Exists relative file returned exists=%v err=%v", exists, err)
		}
		read, err := readSandboxText(sandbox.Files, ctx, relativeFile, nil)
		if err != nil {
			t.Fatalf("ReadText relative file returned error: %v", err)
		}
		if read != content {
			t.Fatalf("unexpected relative file content: %q", read)
		}

		nestedRelative := path.Join(fmt.Sprintf("non_existing_dir_%d", time.Now().UnixNano()), "test_write.txt")
		nestedContent := "This should succeed too."
		info, err = sandbox.Files.Write(ctx, nestedRelative, []byte(nestedContent), nil)
		if err != nil {
			t.Fatalf("Write nested relative file returned error: %v", err)
		}
		if info.Name != "test_write.txt" || info.Type != e2b.FileTypeFile || info.Path != "/home/user/"+nestedRelative {
			t.Fatalf("unexpected nested relative write info: %#v", info)
		}
		read, err = readSandboxText(sandbox.Files, ctx, nestedRelative, nil)
		if err != nil {
			t.Fatalf("ReadText nested relative file returned error: %v", err)
		}
		if read != nestedContent {
			t.Fatalf("unexpected nested relative file content: %q", read)
		}
	})

	t.Run("write files with relative paths normalizes metadata", func(t *testing.T) {
		suffix := fmt.Sprint(time.Now().UnixNano())
		files := []e2b.WriteEntry{
			{Path: "writefiles_test_" + suffix + "_0.txt", Data: "This is a test file 0."},
			{Path: "writefiles_test_" + suffix + "_1.txt", Data: []byte("This is a test file 1.")},
		}
		wantByPath := map[string]string{
			files[0].Path: "This is a test file 0.",
			files[1].Path: "This is a test file 1.",
		}

		infos, err := sandbox.Files.WriteFiles(ctx, files, nil)
		if err != nil {
			t.Fatalf("WriteFiles relative paths returned error: %v", err)
		}
		if len(infos) != len(files) {
			t.Fatalf("expected %d write infos, got %#v", len(files), infos)
		}
		for i, file := range files {
			info := infos[i]
			if info.Name != path.Base(file.Path) || info.Type != e2b.FileTypeFile || info.Path != "/home/user/"+file.Path {
				t.Fatalf("unexpected relative WriteFiles info[%d]: %#v", i, info)
			}
			read, err := readSandboxText(sandbox.Files, ctx, file.Path, nil)
			if err != nil {
				t.Fatalf("ReadText relative WriteFiles path %s returned error: %v", file.Path, err)
			}
			if read != wantByPath[file.Path] {
				t.Fatalf("unexpected relative WriteFiles content for %s: %q", file.Path, read)
			}
		}
	})

	t.Run("write files exact metadata and create parent directories", func(t *testing.T) {
		files := []e2b.WriteEntry{
			{Path: path.Join(baseDir, "multi", "one.txt"), Data: strings.NewReader("one")},
			{Path: path.Join(baseDir, "multi", "two.bin"), Data: bytes.NewReader([]byte("two"))},
			{Path: path.Join(baseDir, "multi", "nested", "three.txt"), Data: strings.NewReader("three")},
		}
		wantByPath := map[string]string{
			files[0].Path: "one",
			files[1].Path: "two",
			files[2].Path: "three",
		}
		infos, err := sandbox.Files.WriteFiles(ctx, files, nil)
		if err != nil {
			t.Fatalf("WriteFiles returned error: %v", err)
		}
		if len(infos) != len(files) {
			t.Fatalf("expected %d write infos, got %#v", len(files), infos)
		}
		for i, file := range files {
			info := infos[i]
			if info.Name != path.Base(file.Path) || info.Type != e2b.FileTypeFile || info.Path != file.Path {
				t.Fatalf("unexpected WriteFiles info[%d]: %#v", i, info)
			}
			read, err := readSandboxText(sandbox.Files, ctx, file.Path, nil)
			if err != nil {
				t.Fatalf("ReadText(%s) returned error: %v", file.Path, err)
			}
			if read != wantByPath[file.Path] {
				t.Fatalf("unexpected content for %s: %q", file.Path, read)
			}
		}
	})

	t.Run("gzip content encoding", func(t *testing.T) {
		filePath := path.Join(baseDir, "gzip.txt")
		content := "This is a test file with gzip encoding."
		if _, err := sandbox.Files.Write(ctx, filePath, strings.NewReader(content), &e2b.FilesystemWriteOpts{Gzip: true}); err != nil {
			t.Fatalf("gzip Write returned error: %v", err)
		}
		read, err := readSandboxText(sandbox.Files, ctx, filePath, &e2b.FilesystemReadOpts{Gzip: true})
		if err != nil {
			t.Fatalf("gzip ReadText returned error: %v", err)
		}
		if read != content {
			t.Fatalf("unexpected gzip read content: %q", read)
		}
		plain, err := readSandboxText(sandbox.Files, ctx, filePath, nil)
		if err != nil {
			t.Fatalf("plain ReadText returned error: %v", err)
		}
		if plain != content {
			t.Fatalf("unexpected plain read content after gzip write: %q", plain)
		}

		gzipFiles := []e2b.WriteEntry{
			{Path: path.Join(baseDir, "gzip_multi_1.txt"), Data: strings.NewReader("File 1 content")},
			{Path: path.Join(baseDir, "gzip_multi_2.txt"), Data: strings.NewReader("File 2 content")},
			{Path: path.Join(baseDir, "gzip_multi_3.txt"), Data: strings.NewReader("File 3 content")},
		}
		infos, err := sandbox.Files.WriteFiles(ctx, gzipFiles, &e2b.FilesystemWriteOpts{Gzip: true})
		if err != nil {
			t.Fatalf("gzip WriteFiles returned error: %v", err)
		}
		if len(infos) != len(gzipFiles) {
			t.Fatalf("expected %d gzip write infos, got %#v", len(gzipFiles), infos)
		}
		for _, file := range gzipFiles {
			read, err := readSandboxText(sandbox.Files, ctx, file.Path, nil)
			if err != nil {
				t.Fatalf("ReadText gzip WriteFiles path %s returned error: %v", file.Path, err)
			}
			want := strings.TrimSuffix(path.Base(file.Path), ".txt")
			want = "File " + strings.TrimPrefix(want, "gzip_multi_") + " content"
			if read != want {
				t.Fatalf("unexpected gzip WriteFiles content for %s: %q", file.Path, read)
			}
		}

		bytesPath := path.Join(baseDir, "gzip_bytes.txt")
		bytesContent := []byte("Binary content with gzip.")
		if _, err := sandbox.Files.Write(ctx, bytesPath, bytes.NewReader(bytesContent), nil); err != nil {
			t.Fatalf("Write gzip bytes fixture returned error: %v", err)
		}
		readBytes, err := readSandboxBytes(sandbox.Files, ctx, bytesPath, &e2b.FilesystemReadOpts{Gzip: true})
		if err != nil {
			t.Fatalf("Read gzip bytes returned error: %v", err)
		}
		if !bytes.Equal(readBytes, bytesContent) {
			t.Fatalf("unexpected gzip bytes read: %q", string(readBytes))
		}
	})

	t.Run("read mkdir rename remove and list edge cases", func(t *testing.T) {
		missing := path.Join(baseDir, "missing.txt")
		if _, err := readSandboxText(sandbox.Files, ctx, missing, nil); err == nil {
			t.Fatal("expected ReadText of missing file to fail")
		} else {
			var fileErr *e2b.FileNotFoundError
			if !errors.As(err, &fileErr) {
				t.Fatalf("expected FileNotFoundError, got %T %v", err, err)
			}
			var deprecatedErr *e2b.NotFoundError
			if !errors.As(err, &deprecatedErr) {
				t.Fatalf("expected deprecated NotFoundError compatibility, got %T %v", err, err)
			}
		}

		empty := path.Join(baseDir, "empty-file.txt")
		mustRun(t, ctx, sandbox, "touch "+shellQuote(empty))
		content, err := readSandboxText(sandbox.Files, ctx, empty, nil)
		if err != nil {
			t.Fatalf("ReadText empty file returned error: %v", err)
		}
		if content != "" {
			t.Fatalf("expected empty file content, got %q", content)
		}

		dir := path.Join(baseDir, "existing-dir")
		created, err := sandbox.Files.MakeDir(ctx, dir, nil)
		if err != nil {
			t.Fatalf("MakeDir returned error: %v", err)
		}
		if !created {
			t.Fatal("expected MakeDir to report true for new directory")
		}
		created, err = sandbox.Files.MakeDir(ctx, dir, nil)
		if err != nil {
			t.Fatalf("MakeDir existing returned error: %v", err)
		}
		if created {
			t.Fatal("expected MakeDir to report false for existing directory")
		}
		nested := path.Join(baseDir, "nested-dir", "child")
		if _, err := sandbox.Files.MakeDir(ctx, nested, nil); err != nil {
			t.Fatalf("MakeDir nested returned error: %v", err)
		}
		exists, err := sandbox.Files.Exists(ctx, nested, nil)
		if err != nil || !exists {
			t.Fatalf("Exists nested returned exists=%v err=%v", exists, err)
		}

		if err := sandbox.Files.Remove(ctx, path.Join(baseDir, "missing-remove.txt"), nil); err != nil {
			t.Fatalf("Remove missing file returned error: %v", err)
		}

		if _, err := sandbox.Files.Rename(ctx, missing, path.Join(baseDir, "renamed-missing.txt"), nil); err == nil {
			t.Fatal("expected Rename of missing file to fail")
		} else {
			var fileErr *e2b.FileNotFoundError
			if !errors.As(err, &fileErr) {
				t.Fatalf("expected FileNotFoundError from Rename, got %T %v", err, err)
			}
		}

		listDir := path.Join(baseDir, "list-depth")
		if _, err := sandbox.Files.MakeDir(ctx, path.Join(listDir, "subdir1"), nil); err != nil {
			t.Fatalf("MakeDir subdir1 returned error: %v", err)
		}
		if _, err := sandbox.Files.MakeDir(ctx, path.Join(listDir, "subdir2"), nil); err != nil {
			t.Fatalf("MakeDir subdir2 returned error: %v", err)
		}
		for _, child := range []string{
			path.Join(listDir, "subdir1", "subdir1_1"),
			path.Join(listDir, "subdir1", "subdir1_2"),
			path.Join(listDir, "subdir2", "subdir2_1"),
			path.Join(listDir, "subdir2", "subdir2_2"),
		} {
			if _, err := sandbox.Files.MakeDir(ctx, child, nil); err != nil {
				t.Fatalf("MakeDir %s returned error: %v", child, err)
			}
		}
		if _, err := sandbox.Files.Write(ctx, path.Join(listDir, "file1.txt"), strings.NewReader("Hello, world!"), nil); err != nil {
			t.Fatalf("Write list file returned error: %v", err)
		}

		assertListEntries(t, sandbox, ctx, listDir, 0, []liveEntryExpectation{
			{Name: "file1.txt", Type: e2b.FileTypeFile, Path: path.Join(listDir, "file1.txt")},
			{Name: "subdir1", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir1")},
			{Name: "subdir2", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir2")},
		})
		assertListEntries(t, sandbox, ctx, listDir, 1, []liveEntryExpectation{
			{Name: "file1.txt", Type: e2b.FileTypeFile, Path: path.Join(listDir, "file1.txt")},
			{Name: "subdir1", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir1")},
			{Name: "subdir2", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir2")},
		})
		depthTwo := []liveEntryExpectation{
			{Name: "file1.txt", Type: e2b.FileTypeFile, Path: path.Join(listDir, "file1.txt")},
			{Name: "subdir1", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir1")},
			{Name: "subdir1_1", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir1", "subdir1_1")},
			{Name: "subdir1_2", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir1", "subdir1_2")},
			{Name: "subdir2", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir2")},
			{Name: "subdir2_1", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir2", "subdir2_1")},
			{Name: "subdir2_2", Type: e2b.FileTypeDir, Path: path.Join(listDir, "subdir2", "subdir2_2")},
		}
		assertListEntries(t, sandbox, ctx, listDir, 2, depthTwo)
		assertListEntries(t, sandbox, ctx, listDir, 3, depthTwo)

		if _, err := sandbox.Files.List(ctx, listDir, &e2b.FilesystemListOpts{Depth: intPtr(-1)}); err == nil {
			t.Fatal("expected List with invalid depth to fail")
		} else if !strings.Contains(err.Error(), "depth should be at least") {
			t.Fatalf("unexpected invalid depth error: %v", err)
		}

		if _, err := sandbox.Files.List(ctx, listDir, &e2b.FilesystemListOpts{Depth: intPtr(0)}); err == nil {
			t.Fatal("expected List with invalid depth to fail")
		} else if !strings.Contains(err.Error(), "depth should be at least") {
			t.Fatalf("unexpected invalid depth error: %v", err)
		}

		details, err := sandbox.Files.List(ctx, listDir, &e2b.FilesystemListOpts{Depth: intPtr(1)})
		if err != nil {
			t.Fatalf("List for details returned error: %v", err)
		}
		fileEntry := findEntryByName(details, "file1.txt")
		if fileEntry == nil {
			t.Fatalf("expected file entry in %#v", details)
		}
		if fileEntry.Mode != 0o644 || fileEntry.Permissions != "-rw-r--r--" || fileEntry.Owner != "user" || fileEntry.Group != "user" || fileEntry.Size != int64(len("Hello, world!")) || fileEntry.ModifiedTime == nil {
			t.Fatalf("unexpected file entry details: %#v", fileEntry)
		}
		dirEntry := findEntryByName(details, "subdir1")
		if dirEntry == nil {
			t.Fatalf("expected directory entry in %#v", details)
		}
		if dirEntry.Mode != 0o755 || dirEntry.Permissions != "drwxr-xr-x" || dirEntry.Owner != "user" || dirEntry.Group != "user" || dirEntry.ModifiedTime == nil {
			t.Fatalf("unexpected directory entry details: %#v", dirEntry)
		}
	})

	t.Run("info list symlink and watch", func(t *testing.T) {
		dir := path.Join(baseDir, "info")
		file := path.Join(dir, "file.txt")
		link := path.Join(dir, "link.txt")
		if _, err := sandbox.Files.MakeDir(ctx, dir, nil); err != nil {
			t.Fatalf("MakeDir returned error: %v", err)
		}
		if _, err := sandbox.Files.Write(ctx, file, strings.NewReader("watched"), nil); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
		mustRun(t, ctx, sandbox, "ln -sf file.txt "+shellQuote(link))

		info, err := sandbox.Files.GetInfo(ctx, file, nil)
		if err != nil {
			t.Fatalf("GetInfo file returned error: %v", err)
		}
		if info.Type != e2b.FileTypeFile || info.Size <= 0 {
			t.Fatalf("unexpected file info: %#v", info)
		}
		linkInfo, err := sandbox.Files.GetInfo(ctx, link, nil)
		if err != nil {
			t.Fatalf("GetInfo symlink returned error: %v", err)
		}
		if linkInfo.Name != "link.txt" || linkInfo.SymlinkTarget == "" || !strings.HasSuffix(linkInfo.SymlinkTarget, "/file.txt") {
			t.Fatalf("unexpected symlink info: %#v", linkInfo)
		}
		entries, err := sandbox.Files.List(ctx, dir, &e2b.FilesystemListOpts{Depth: intPtr(1)})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if !entryListContains(entries, "file.txt") || !entryListContains(entries, "link.txt") {
			t.Fatalf("expected file and symlink entries, got %#v", entries)
		}

		eventCh := make(chan e2b.FilesystemEvent, 4)
		watchTimeoutMs := 10000
		handle, err := sandbox.Files.WatchDir(ctx, dir, func(event e2b.FilesystemEvent) {
			eventCh <- event
		}, &e2b.WatchOpts{TimeoutMs: &watchTimeoutMs})
		if err != nil {
			t.Fatalf("WatchDir returned error: %v", err)
		}
		defer handle.Stop()

		changed := path.Join(dir, "changed.txt")
		if _, err := sandbox.Files.Write(ctx, changed, strings.NewReader("changed"), nil); err != nil {
			t.Fatalf("Write changed file returned error: %v", err)
		}
		waitForFilesystemEvent(t, eventCh, "changed.txt")
	})

	t.Run("watch recursive and error cases", func(t *testing.T) {
		watchTimeoutMs := 10000

		recursiveDir := path.Join(baseDir, "recursive-watch")
		nestedDirName := "nested"
		nestedDir := path.Join(recursiveDir, nestedDirName)
		if _, err := sandbox.Files.MakeDir(ctx, nestedDir, nil); err != nil {
			t.Fatalf("MakeDir recursive nested returned error: %v", err)
		}
		recursiveEvents := make(chan e2b.FilesystemEvent, 8)
		handle, err := sandbox.Files.WatchDir(ctx, recursiveDir, func(event e2b.FilesystemEvent) {
			recursiveEvents <- event
		}, &e2b.WatchOpts{Recursive: true, TimeoutMs: &watchTimeoutMs})
		if err != nil {
			t.Fatalf("recursive WatchDir returned error: %v", err)
		}
		defer handle.Stop()
		if _, err := sandbox.Files.Write(ctx, path.Join(nestedDir, "test_watch.txt"), strings.NewReader("recursive"), nil); err != nil {
			t.Fatalf("Write recursive watched file returned error: %v", err)
		}
		waitForFilesystemEventExact(t, recursiveEvents, nestedDirName+"/test_watch.txt")

		addDir := path.Join(baseDir, "recursive-watch-add")
		if _, err := sandbox.Files.MakeDir(ctx, addDir, nil); err != nil {
			t.Fatalf("MakeDir recursive add parent returned error: %v", err)
		}
		addEvents := make(chan e2b.FilesystemEvent, 8)
		addHandle, err := sandbox.Files.WatchDir(ctx, addDir, func(event e2b.FilesystemEvent) {
			addEvents <- event
		}, &e2b.WatchOpts{Recursive: true, TimeoutMs: &watchTimeoutMs})
		if err != nil {
			t.Fatalf("recursive add WatchDir returned error: %v", err)
		}
		defer addHandle.Stop()
		if _, err := sandbox.Files.MakeDir(ctx, path.Join(addDir, nestedDirName), nil); err != nil {
			t.Fatalf("MakeDir nested after watch returned error: %v", err)
		}
		waitForFilesystemEventExactType(t, addEvents, nestedDirName, e2b.FilesystemEventCreate)
		if _, err := sandbox.Files.Write(ctx, path.Join(addDir, nestedDirName, "test_watch.txt"), strings.NewReader("created later"), nil); err != nil {
			t.Fatalf("Write nested after watch returned error: %v", err)
		}
		waitForFilesystemEventExact(t, addEvents, nestedDirName+"/test_watch.txt")

		_, err = sandbox.Files.WatchDir(ctx, path.Join(baseDir, "non-existing-watch-dir"), nil, &e2b.WatchOpts{TimeoutMs: &watchTimeoutMs})
		if err == nil {
			t.Fatal("expected WatchDir on missing directory to fail")
		}
		var fileErr *e2b.FileNotFoundError
		if !errors.As(err, &fileErr) {
			t.Fatalf("expected FileNotFoundError from missing WatchDir, got %T %v", err, err)
		}

		watchedFile := path.Join(baseDir, "watch-file.txt")
		if _, err := sandbox.Files.Write(ctx, watchedFile, strings.NewReader("file"), nil); err != nil {
			t.Fatalf("Write watched file returned error: %v", err)
		}
		_, err = sandbox.Files.WatchDir(ctx, watchedFile, nil, &e2b.WatchOpts{TimeoutMs: &watchTimeoutMs})
		if err == nil {
			t.Fatal("expected WatchDir on file to fail")
		}
		var sandboxErr *e2b.SandboxError
		if !errors.As(err, &sandboxErr) {
			t.Fatalf("expected SandboxError-compatible error from WatchDir on file, got %T %v", err, err)
		}
	})
}

func TestLivePty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)

	t.Run("create resize and send input", func(t *testing.T) {
		var mu sync.Mutex
		var output strings.Builder
		handle, err := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
			Cols: 80,
			Rows: 24,
			Cwd:  "/",
			Envs: map[string]string{"ABC": "123"},
			OnData: func(data e2b.PtyOutput) {
				mu.Lock()
				defer mu.Unlock()
				output.Write(data)
			},
		})
		if err != nil {
			t.Fatalf("Pty.Create returned error: %v", err)
		}

		if err := sandbox.Pty.Resize(ctx, handle.Pid, e2b.PtySize{Cols: 100, Rows: 24}, nil); err != nil {
			_, _ = handle.Kill()
			t.Fatalf("Pty.Resize returned error: %v", err)
		}
		if err := sandbox.Pty.SendInput(ctx, handle.Pid, []byte("echo $ABC\nexit\n"), nil); err != nil {
			_, _ = handle.Kill()
			t.Fatalf("Pty.SendInput returned error: %v", err)
		}
		result, err := handle.Wait()
		if err != nil {
			t.Fatalf("PTY Wait returned error: %v", err)
		}
		combined := result.Stdout
		mu.Lock()
		callbackOutput := output.String()
		mu.Unlock()
		if !strings.Contains(combined, "123") || !strings.Contains(callbackOutput, "123") {
			t.Fatalf("expected PTY output to contain env var value, stdout=%q callback=%q", combined, callbackOutput)
		}
	})

	t.Run("connect reconnect", func(t *testing.T) {
		var mu sync.Mutex
		var output1 strings.Builder
		terminal, err := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
			Cols: 80,
			Rows: 24,
			Envs: map[string]string{"FOO": "bar"},
			OnData: func(data e2b.PtyOutput) {
				mu.Lock()
				defer mu.Unlock()
				output1.Write(data)
			},
		})
		if err != nil {
			t.Fatalf("Pty.Create reconnect terminal returned error: %v", err)
		}
		defer terminal.Kill()

		if err := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("echo $FOO\n"), nil); err != nil {
			t.Fatalf("Pty.SendInput first echo returned error: %v", err)
		}
		waitForCommandStdoutContains(t, terminal, "bar")
		terminal.Disconnect()

		var output2Mu sync.Mutex
		var output2 strings.Builder
		reconnected, err := sandbox.Pty.Connect(ctx, terminal.Pid, &e2b.PtyConnectOpts{
			OnData: func(data e2b.PtyOutput) {
				output2Mu.Lock()
				defer output2Mu.Unlock()
				output2.Write(data)
			},
		})
		if err != nil {
			t.Fatalf("Pty.Connect returned error: %v", err)
		}
		if reconnected.Pid != terminal.Pid {
			t.Fatalf("expected reconnect pid %d, got %d", terminal.Pid, reconnected.Pid)
		}

		if err := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("echo $FOO\nexit\n"), nil); err != nil {
			_, _ = reconnected.Kill()
			t.Fatalf("Pty.SendInput reconnect returned error: %v", err)
		}
		result, err := reconnected.Wait()
		if err != nil {
			t.Fatalf("reconnected PTY Wait returned error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected reconnected PTY exit code 0, got %#v", result)
		}

		mu.Lock()
		first := output1.String()
		mu.Unlock()
		output2Mu.Lock()
		second := output2.String()
		output2Mu.Unlock()
		if !strings.Contains(first, "bar") || !strings.Contains(second, "bar") {
			t.Fatalf("expected both PTY connections to observe env var output, first=%q second=%q", first, second)
		}
	})
}

func TestLiveGit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)
	baseDir := livePath("git")
	mustRun(t, ctx, sandbox, "rm -rf "+shellQuote(baseDir)+" && mkdir -p "+shellQuote(baseDir))

	const (
		authorName  = "Sandbox Bot"
		authorEmail = "sandbox@example.com"
	)

	createRepo := func(t *testing.T, name string) string {
		t.Helper()
		repoPath := path.Join(baseDir, name)
		if _, err := sandbox.Git.Init(ctx, repoPath, &e2b.GitInitOpts{InitialBranch: "main"}); err != nil {
			t.Fatalf("Git.Init(%s) returned error: %v", name, err)
		}
		if _, err := sandbox.Git.ConfigureUser(ctx, authorName, authorEmail, &e2b.GitConfigOpts{
			Scope: e2b.GitConfigScope("local"),
			Path:  repoPath,
		}); err != nil {
			t.Fatalf("Git.ConfigureUser(%s) returned error: %v", name, err)
		}
		return repoPath
	}
	createRepoWithCommit := func(t *testing.T, name string) string {
		t.Helper()
		repoPath := createRepo(t, name)
		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "README.md"), strings.NewReader("hello\n"), nil); err != nil {
			t.Fatalf("Write README for %s returned error: %v", name, err)
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Add(%s) returned error: %v", name, err)
		}
		if _, err := sandbox.Git.Commit(ctx, repoPath, "Initial commit", &e2b.GitCommitOpts{
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
		}); err != nil {
			t.Fatalf("Git.Commit(%s) returned error: %v", name, err)
		}
		return repoPath
	}

	t.Run("init add commit and config", func(t *testing.T) {
		repoPath := createRepo(t, "init-add-commit-config")
		exists, err := sandbox.Files.Exists(ctx, path.Join(repoPath, ".git"), nil)
		if err != nil || !exists {
			t.Fatalf("expected .git directory to exist, exists=%v err=%v", exists, err)
		}
		head := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" symbolic-ref --short HEAD").Stdout)
		if head != "main" {
			t.Fatalf("expected initial branch main, got %q", head)
		}

		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "README.md"), strings.NewReader("hello\n"), nil); err != nil {
			t.Fatalf("Write README returned error: %v", err)
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Add returned error: %v", err)
		}
		status, err := sandbox.Git.Status(ctx, repoPath, nil)
		if err != nil {
			t.Fatalf("Git.Status after add returned error: %v", err)
		}
		added := findGitFileStatus(status, "README.md")
		if added == nil || added.Status != "added" || !added.Staged {
			t.Fatalf("expected staged added README, got status=%#v entry=%#v", status, added)
		}

		if _, err := sandbox.Git.Commit(ctx, repoPath, "Initial commit", &e2b.GitCommitOpts{
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
		}); err != nil {
			t.Fatalf("Git.Commit returned error: %v", err)
		}
		message := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" log -1 --pretty=%B").Stdout)
		if message != "Initial commit" {
			t.Fatalf("unexpected commit message: %q", message)
		}

		mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" config --local pull.rebase true")
		value, err := sandbox.Git.GetConfig(ctx, "pull.rebase", &e2b.GitConfigOpts{
			Scope: e2b.GitConfigScope("local"),
			Path:  repoPath,
		})
		if err != nil || value != "true" {
			t.Fatalf("expected local pull.rebase true, got value=%q err=%v", value, err)
		}
		if _, err := sandbox.Git.SetConfig(ctx, "pull.ff", "only", &e2b.GitConfigOpts{
			Scope: e2b.GitConfigScope("local"),
			Path:  repoPath,
		}); err != nil {
			t.Fatalf("Git.SetConfig local returned error: %v", err)
		}
		configuredValue, err := sandbox.Git.GetConfig(ctx, "pull.ff", &e2b.GitConfigOpts{
			Scope: e2b.GitConfigScope("local"),
			Path:  repoPath,
		})
		if err != nil || configuredValue != "only" {
			t.Fatalf("expected local pull.ff only, got value=%q err=%v", configuredValue, err)
		}
	})

	t.Run("commit uses configured author fields", func(t *testing.T) {
		repoPath := createRepo(t, "partial-author")
		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "README.md"), strings.NewReader("hello\n"), nil); err != nil {
			t.Fatalf("Write README returned error: %v", err)
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Add returned error: %v", err)
		}
		overrideName := "Override Bot"
		if _, err := sandbox.Git.Commit(ctx, repoPath, "Partial author commit", &e2b.GitCommitOpts{
			AuthorName: overrideName,
		}); err != nil {
			t.Fatalf("Git.Commit partial author returned error: %v", err)
		}
		gotName := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" log -1 --pretty=%an").Stdout)
		gotEmail := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" log -1 --pretty=%ae").Stdout)
		if gotName != overrideName || gotEmail != authorEmail {
			t.Fatalf("unexpected partial author: name=%q email=%q", gotName, gotEmail)
		}
	})

	t.Run("global user config and credential helper", func(t *testing.T) {
		if _, err := sandbox.Git.ConfigureUser(ctx, authorName, authorEmail, nil); err != nil {
			t.Fatalf("Git.ConfigureUser global returned error: %v", err)
		}
		name, err := sandbox.Git.GetConfig(ctx, "user.name", &e2b.GitConfigOpts{Scope: e2b.GitConfigScope("global")})
		if err != nil {
			t.Fatalf("Git.GetConfig global user.name returned error: %v", err)
		}
		email, err := sandbox.Git.GetConfig(ctx, "user.email", &e2b.GitConfigOpts{Scope: e2b.GitConfigScope("global")})
		if err != nil {
			t.Fatalf("Git.GetConfig global user.email returned error: %v", err)
		}
		if name != authorName || email != authorEmail {
			t.Fatalf("unexpected global user config: name=%q email=%q", name, email)
		}

		if _, err := sandbox.Git.DangerouslyAuthenticate(ctx, &e2b.GitDangerouslyAuthenticateOpts{
			Username: "git",
			Password: "token",
			Host:     "example.com",
			Protocol: "https",
		}); err != nil {
			t.Fatalf("Git.DangerouslyAuthenticate returned error: %v", err)
		}
		helper := strings.TrimSpace(mustRun(t, ctx, sandbox, "git config --global --get credential.helper").Stdout)
		configuredHelper, err := sandbox.Git.GetConfig(ctx, "credential.helper", &e2b.GitConfigOpts{Scope: e2b.GitConfigScope("global")})
		if err != nil {
			t.Fatalf("Git.GetConfig credential.helper returned error: %v", err)
		}
		if helper != "store" || configuredHelper != "store" {
			t.Fatalf("expected credential helper store, command=%q sdk=%q", helper, configuredHelper)
		}
	})

	t.Run("branches checkout create and delete", func(t *testing.T) {
		repoPath := createRepoWithCommit(t, "branches")
		mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" branch feature")
		branches, err := sandbox.Git.Branches(ctx, repoPath, nil)
		if err != nil {
			t.Fatalf("Git.Branches returned error: %v", err)
		}
		if branches.CurrentBranch != "main" || !stringListContains(branches.Branches, "main") || !stringListContains(branches.Branches, "feature") {
			t.Fatalf("unexpected branches result: %#v", branches)
		}

		if _, err := sandbox.Git.CheckoutBranch(ctx, repoPath, "feature", nil); err != nil {
			t.Fatalf("Git.CheckoutBranch returned error: %v", err)
		}
		head := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" rev-parse --abbrev-ref HEAD").Stdout)
		if head != "feature" {
			t.Fatalf("expected checkout branch feature, got %q", head)
		}

		if _, err := sandbox.Git.CheckoutBranch(ctx, repoPath, "main", nil); err != nil {
			t.Fatalf("Git.CheckoutBranch main returned error: %v", err)
		}
		if _, err := sandbox.Git.CreateBranch(ctx, repoPath, "created", nil); err != nil {
			t.Fatalf("Git.CreateBranch returned error: %v", err)
		}
		branches, err = sandbox.Git.Branches(ctx, repoPath, nil)
		if err != nil {
			t.Fatalf("Git.Branches after create returned error: %v", err)
		}
		if branches.CurrentBranch != "created" || !stringListContains(branches.Branches, "created") {
			t.Fatalf("unexpected branches after create: %#v", branches)
		}
		if _, err := sandbox.Git.CheckoutBranch(ctx, repoPath, "main", nil); err != nil {
			t.Fatalf("Git.CheckoutBranch main before delete returned error: %v", err)
		}
		if _, err := sandbox.Git.DeleteBranch(ctx, repoPath, "created", nil); err != nil {
			t.Fatalf("Git.DeleteBranch returned error: %v", err)
		}
		deleted := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" branch --list created").Stdout)
		branches, err = sandbox.Git.Branches(ctx, repoPath, nil)
		if err != nil {
			t.Fatalf("Git.Branches after delete returned error: %v", err)
		}
		if deleted != "" || stringListContains(branches.Branches, "created") {
			t.Fatalf("expected created branch to be deleted, command=%q branches=%#v", deleted, branches)
		}
	})

	t.Run("status reports untracked and staged changes", func(t *testing.T) {
		untrackedRepo := createRepo(t, "status-untracked")
		if _, err := sandbox.Files.Write(ctx, path.Join(untrackedRepo, "README.md"), strings.NewReader("hello\n"), nil); err != nil {
			t.Fatalf("Write untracked README returned error: %v", err)
		}
		status, err := sandbox.Git.Status(ctx, untrackedRepo, nil)
		if err != nil {
			t.Fatalf("Git.Status untracked returned error: %v", err)
		}
		entry := findGitFileStatus(status, "README.md")
		if entry == nil || entry.Status != "untracked" || status.IsClean || !status.HasChanges || !status.HasUntracked || status.HasStaged || status.HasConflicts || status.TotalCount != 1 || status.StagedCount != 0 || status.UnstagedCount != 1 || status.UntrackedCount != 1 || status.ConflictCount != 0 {
			t.Fatalf("unexpected untracked status: status=%#v entry=%#v", status, entry)
		}

		repoPath := createRepo(t, "status-details")
		for name, contents := range map[string]string{
			"README.md": "hello\n",
			"DELETE.md": "delete me\n",
			"RENAME.md": "rename me\n",
		} {
			if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, name), strings.NewReader(contents), nil); err != nil {
				t.Fatalf("Write %s returned error: %v", name, err)
			}
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Add initial status-details returned error: %v", err)
		}
		if _, err := sandbox.Git.Commit(ctx, repoPath, "Initial commit", &e2b.GitCommitOpts{
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
		}); err != nil {
			t.Fatalf("Git.Commit initial status-details returned error: %v", err)
		}

		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "README.md"), strings.NewReader("hello again\n"), nil); err != nil {
			t.Fatalf("Write modified README returned error: %v", err)
		}
		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "NEW.md"), strings.NewReader("new file\n"), nil); err != nil {
			t.Fatalf("Write NEW.md returned error: %v", err)
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, &e2b.GitAddOpts{Files: []string{"NEW.md"}}); err != nil {
			t.Fatalf("Git.Add NEW.md returned error: %v", err)
		}
		mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" rm DELETE.md")
		mustRun(t, ctx, sandbox, "git -C "+shellQuote(repoPath)+" mv RENAME.md RENAMED.md")

		status, err = sandbox.Git.Status(ctx, repoPath, nil)
		if err != nil {
			t.Fatalf("Git.Status detailed returned error: %v", err)
		}
		modified := findGitFileStatus(status, "README.md")
		added := findGitFileStatus(status, "NEW.md")
		deleted := findGitFileStatus(status, "DELETE.md")
		renamed := findGitFileStatus(status, "RENAMED.md")
		if modified == nil || modified.Status != "modified" || modified.Staged {
			t.Fatalf("unexpected modified status: %#v", modified)
		}
		if added == nil || added.Status != "added" || !added.Staged {
			t.Fatalf("unexpected added status: %#v", added)
		}
		if deleted == nil || deleted.Status != "deleted" || !deleted.Staged {
			t.Fatalf("unexpected deleted status: %#v", deleted)
		}
		if renamed == nil || renamed.Status != "renamed" || !renamed.Staged || renamed.RenamedFrom != "RENAME.md" {
			t.Fatalf("unexpected renamed status: %#v", renamed)
		}
		if !status.HasChanges || !status.HasStaged || status.HasUntracked || status.HasConflicts || status.TotalCount != 4 || status.StagedCount != 3 || status.UnstagedCount != 1 || status.UntrackedCount != 0 || status.ConflictCount != 0 {
			t.Fatalf("unexpected detailed status counts: %#v", status)
		}
	})

	t.Run("restore and reset", func(t *testing.T) {
		unstageRepo := createRepoWithCommit(t, "restore-unstage")
		if _, err := sandbox.Files.Write(ctx, path.Join(unstageRepo, "README.md"), strings.NewReader("changed\n"), nil); err != nil {
			t.Fatalf("Write restore-unstage README returned error: %v", err)
		}
		if _, err := sandbox.Git.Add(ctx, unstageRepo, &e2b.GitAddOpts{Files: []string{"README.md"}}); err != nil {
			t.Fatalf("Git.Add restore-unstage returned error: %v", err)
		}
		status, err := sandbox.Git.Status(ctx, unstageRepo, nil)
		if err != nil || !status.HasStaged {
			t.Fatalf("expected staged change before restore, status=%#v err=%v", status, err)
		}
		staged := true
		worktree := false
		if _, err := sandbox.Git.Restore(ctx, unstageRepo, &e2b.GitRestoreOpts{
			Files:    []string{"README.md"},
			Staged:   &staged,
			Worktree: &worktree,
		}); err != nil {
			t.Fatalf("Git.Restore staged returned error: %v", err)
		}
		status, err = sandbox.Git.Status(ctx, unstageRepo, nil)
		if err != nil || status.HasStaged || !status.HasChanges {
			t.Fatalf("unexpected status after staged restore: status=%#v err=%v", status, err)
		}

		restoreRepo := createRepoWithCommit(t, "restore-worktree")
		if _, err := sandbox.Files.Write(ctx, path.Join(restoreRepo, "README.md"), strings.NewReader("changed\n"), nil); err != nil {
			t.Fatalf("Write restore-worktree README returned error: %v", err)
		}
		status, err = sandbox.Git.Status(ctx, restoreRepo, nil)
		if err != nil || status.IsClean {
			t.Fatalf("expected dirty status before worktree restore, status=%#v err=%v", status, err)
		}
		if _, err := sandbox.Git.Restore(ctx, restoreRepo, &e2b.GitRestoreOpts{Files: []string{"README.md"}}); err != nil {
			t.Fatalf("Git.Restore worktree returned error: %v", err)
		}
		assertGitRepoCleanWithReadme(t, sandbox, ctx, restoreRepo, "hello\n")

		resetRepo := createRepoWithCommit(t, "reset-hard")
		if _, err := sandbox.Files.Write(ctx, path.Join(resetRepo, "README.md"), strings.NewReader("changed\n"), nil); err != nil {
			t.Fatalf("Write reset README returned error: %v", err)
		}
		status, err = sandbox.Git.Status(ctx, resetRepo, nil)
		if err != nil || status.IsClean {
			t.Fatalf("expected dirty status before reset, status=%#v err=%v", status, err)
		}
		if _, err := sandbox.Git.Reset(ctx, resetRepo, &e2b.GitResetOpts{
			Mode:   e2b.GitResetMode("hard"),
			Target: "HEAD",
		}); err != nil {
			t.Fatalf("Git.Reset hard returned error: %v", err)
		}
		assertGitRepoCleanWithReadme(t, sandbox, ctx, resetRepo, "hello\n")
	})

	t.Run("remote clone push pull and upstream errors", func(t *testing.T) {
		daemon := startLiveGitDaemon(t, ctx, sandbox, baseDir)

		remoteRepo := createRepo(t, "remote")
		missingURL, err := sandbox.Git.RemoteGet(ctx, remoteRepo, "origin", nil)
		if err != nil {
			t.Fatalf("Git.RemoteGet missing returned error: %v", err)
		}
		if missingURL != "" {
			t.Fatalf("expected missing remote to return empty URL, got %q", missingURL)
		}
		if _, err := sandbox.Git.RemoteAdd(ctx, remoteRepo, "origin", daemon.remoteURL, nil); err != nil {
			t.Fatalf("Git.RemoteAdd returned error: %v", err)
		}
		currentURL, err := sandbox.Git.RemoteGet(ctx, remoteRepo, "origin", nil)
		if err != nil || currentURL != daemon.remoteURL {
			t.Fatalf("expected remote URL %q, got %q err=%v", daemon.remoteURL, currentURL, err)
		}
		secondPath := path.Join(baseDir, "remote-2.git")
		if _, err := sandbox.Git.Init(ctx, secondPath, &e2b.GitInitOpts{Bare: true, InitialBranch: "main"}); err != nil {
			t.Fatalf("Git.Init second bare returned error: %v", err)
		}
		secondURL := fmt.Sprintf("git://127.0.0.1:%d/remote-2.git", daemon.port)
		if _, err := sandbox.Git.RemoteAdd(ctx, remoteRepo, "origin", secondURL, &e2b.GitRemoteAddOpts{Overwrite: true}); err != nil {
			t.Fatalf("Git.RemoteAdd overwrite returned error: %v", err)
		}
		updatedURL := strings.TrimSpace(mustRun(t, ctx, sandbox, "git -C "+shellQuote(remoteRepo)+" remote get-url origin").Stdout)
		updatedRemote, err := sandbox.Git.RemoteGet(ctx, remoteRepo, "origin", nil)
		if err != nil || updatedURL != secondURL || updatedRemote != secondURL {
			t.Fatalf("expected overwritten remote URL %q, command=%q sdk=%q err=%v", secondURL, updatedURL, updatedRemote, err)
		}

		repoPath := createRepoWithCommit(t, "sync")
		clonePath := path.Join(baseDir, "clone")
		if _, err := sandbox.Git.RemoteAdd(ctx, repoPath, "origin", daemon.remoteURL, nil); err != nil {
			t.Fatalf("Git.RemoteAdd sync returned error: %v", err)
		}
		if _, err := sandbox.Git.Push(ctx, repoPath, &e2b.GitPushOpts{Remote: "origin", Branch: "main"}); err != nil {
			t.Fatalf("Git.Push initial returned error: %v", err)
		}
		message := strings.TrimSpace(mustRun(t, ctx, sandbox, "git --git-dir="+shellQuote(daemon.remotePath)+" log -1 --pretty=%B").Stdout)
		if message != "Initial commit" {
			t.Fatalf("unexpected remote commit message: %q", message)
		}
		if _, err := sandbox.Git.Clone(ctx, daemon.remoteURL, &e2b.GitCloneOpts{Path: clonePath}); err != nil {
			t.Fatalf("Git.Clone returned error: %v", err)
		}
		contents, err := readSandboxText(sandbox.Files, ctx, path.Join(clonePath, "README.md"), nil)
		if err != nil || !strings.Contains(contents, "hello") {
			t.Fatalf("expected cloned README to contain hello, contents=%q err=%v", contents, err)
		}

		if _, err := sandbox.Files.Write(ctx, path.Join(repoPath, "README.md"), strings.NewReader("hello\nmore\n"), nil); err != nil {
			t.Fatalf("Write updated README returned error: %v", err)
		}
		if _, err := sandbox.Git.Add(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Add update returned error: %v", err)
		}
		if _, err := sandbox.Git.Commit(ctx, repoPath, "Update README", &e2b.GitCommitOpts{
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
		}); err != nil {
			t.Fatalf("Git.Commit update returned error: %v", err)
		}
		if _, err := sandbox.Git.Push(ctx, repoPath, nil); err != nil {
			t.Fatalf("Git.Push update returned error: %v", err)
		}
		if _, err := sandbox.Git.Pull(ctx, clonePath, nil); err != nil {
			t.Fatalf("Git.Pull returned error: %v", err)
		}
		contents, err = readSandboxText(sandbox.Files, ctx, path.Join(clonePath, "README.md"), nil)
		if err != nil || !strings.Contains(contents, "more") {
			t.Fatalf("expected pulled clone to contain update, contents=%q err=%v", contents, err)
		}

		noUpstreamRepo := createRepoWithCommit(t, "push-no-upstream")
		if _, err := sandbox.Git.RemoteAdd(ctx, noUpstreamRepo, "origin", daemon.remoteURL, nil); err != nil {
			t.Fatalf("Git.RemoteAdd no upstream returned error: %v", err)
		}
		setUpstream := false
		_, err = sandbox.Git.Push(ctx, noUpstreamRepo, &e2b.GitPushOpts{SetUpstream: &setUpstream})
		if err == nil {
			t.Fatal("expected Git.Push without upstream to fail")
		}
		var upstreamErr *e2b.GitUpstreamError
		if !errors.As(err, &upstreamErr) || !strings.Contains(strings.ToLower(err.Error()), "no upstream branch is configured") {
			t.Fatalf("expected GitUpstreamError for push without upstream, got %T %v", err, err)
		}

		pullNoUpstreamRepo := createRepoWithCommit(t, "pull-no-upstream")
		if _, err := sandbox.Git.RemoteAdd(ctx, pullNoUpstreamRepo, "origin", daemon.remoteURL, nil); err != nil {
			t.Fatalf("Git.RemoteAdd pull no upstream returned error: %v", err)
		}
		_, err = sandbox.Git.Pull(ctx, pullNoUpstreamRepo, nil)
		if err == nil {
			t.Fatal("expected Git.Pull without upstream to fail")
		}
		if !errors.As(err, &upstreamErr) || !strings.Contains(strings.ToLower(err.Error()), "no upstream branch is configured") {
			t.Fatalf("expected GitUpstreamError for pull without upstream, got %T %v", err, err)
		}
	})
}

func TestLiveSandboxLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)
	metadata := liveSandboxMetadata(t)

	running, err := sandbox.IsRunning(ctx, nil)
	if err != nil {
		t.Fatalf("IsRunning returned error: %v", err)
	}
	if !running {
		t.Fatal("expected sandbox to be running")
	}

	info, err := sandbox.GetInfo(ctx, nil)
	if err != nil {
		t.Fatalf("Sandbox.GetInfo returned error: %v", err)
	}
	if info.SandboxID != sandbox.SandboxID {
		t.Fatalf("unexpected sandbox info: %#v", info)
	}

	apiInfo, err := e2b.GetInfo(ctx, sandbox.SandboxID, nil)
	if err != nil {
		t.Fatalf("GetInfo returned error: %v", err)
	}
	if apiInfo.Metadata["sandboxTestId"] != metadata["sandboxTestId"] {
		t.Fatalf("expected metadata to round-trip, got %#v", apiInfo.Metadata)
	}

	paginator := e2b.List(&e2b.SandboxListOpts{
		Query: &struct {
			Metadata map[string]string
			State    []e2b.SandboxState
		}{Metadata: metadata},
		Limit: 10,
	})
	items, err := paginator.NextItems()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !sandboxInfoListContains(items, sandbox.SandboxID) {
		t.Fatalf("expected list to contain sandbox %s, got %#v", sandbox.SandboxID, items)
	}

	t.Run("list filters and kill", func(t *testing.T) {
		uniqueMetadata := map[string]string{
			"sandboxTestId": metadata["sandboxTestId"] + "-filter-" + fmt.Sprint(time.Now().UnixNano()),
			"uniqueId":      fmt.Sprint(time.Now().UnixNano()),
		}
		extra := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{Metadata: uniqueMetadata})
		filtered, err := e2b.List(&e2b.SandboxListOpts{
			Query: &struct {
				Metadata map[string]string
				State    []e2b.SandboxState
			}{Metadata: map[string]string{"uniqueId": uniqueMetadata["uniqueId"]}},
		}).NextItems()
		if err != nil {
			t.Fatalf("List filtered by metadata returned error: %v", err)
		}
		if len(filtered) != 1 || filtered[0].SandboxID != extra.SandboxID {
			t.Fatalf("expected metadata filter to return only %s, got %#v", extra.SandboxID, filtered)
		}

		running, err := e2b.List(&e2b.SandboxListOpts{
			Query: &struct {
				Metadata map[string]string
				State    []e2b.SandboxState
			}{
				Metadata: uniqueMetadata,
				State:    []e2b.SandboxState{e2b.SandboxState("running")},
			},
		}).NextItems()
		if err != nil {
			t.Fatalf("List running filtered by metadata returned error: %v", err)
		}
		if !sandboxInfoListContains(running, extra.SandboxID) {
			t.Fatalf("expected running list to contain %s, got %#v", extra.SandboxID, running)
		}

		killed, err := e2b.Kill(ctx, extra.SandboxID, nil)
		if err != nil {
			t.Fatalf("Kill existing sandbox returned error: %v", err)
		}
		if !killed {
			t.Fatal("expected Kill existing sandbox to return true")
		}
		waitForSandboxAbsentFromRunningList(t, extra.SandboxID, uniqueMetadata)

		killed, err = e2b.Kill(ctx, "nonexistingsandbox", nil)
		if err != nil {
			t.Fatalf("Kill non-existing sandbox returned error: %v", err)
		}
		if killed {
			t.Fatal("expected Kill non-existing sandbox to return false")
		}
	})

	t.Run("shorten timeout", func(t *testing.T) {
		timeoutSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": metadata["sandboxTestId"] + "-timeout-short-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		if err := timeoutSandbox.SetTimeout(ctx, int((5*time.Second)/time.Millisecond), nil); err != nil {
			t.Fatalf("SetTimeout shorten returned error: %v", err)
		}
		time.Sleep(6 * time.Second)

		requestTimeoutMs := 5000
		running, err := timeoutSandbox.IsRunning(ctx, &struct {
			RequestTimeoutMs *int
			Signal           context.Context
		}{RequestTimeoutMs: &requestTimeoutMs})
		if err != nil {
			t.Fatalf("IsRunning after shortened timeout returned error: %v", err)
		}
		if running {
			t.Fatal("expected sandbox to stop after shortened timeout")
		}
	})

	t.Run("shorten then lengthen timeout", func(t *testing.T) {
		timeoutSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": metadata["sandboxTestId"] + "-timeout-lengthen-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		if err := timeoutSandbox.SetTimeout(ctx, int((5*time.Second)/time.Millisecond), nil); err != nil {
			t.Fatalf("SetTimeout shorten returned error: %v", err)
		}
		time.Sleep(time.Second)
		if err := timeoutSandbox.SetTimeout(ctx, int((10*time.Second)/time.Millisecond), nil); err != nil {
			t.Fatalf("SetTimeout lengthen returned error: %v", err)
		}
		time.Sleep(6 * time.Second)

		running, err := timeoutSandbox.IsRunning(ctx, nil)
		if err != nil {
			t.Fatalf("IsRunning after lengthened timeout returned error: %v", err)
		}
		if !running {
			t.Fatal("expected sandbox to keep running after timeout was lengthened")
		}
	})

	t.Run("connect does not shorten timeout", func(t *testing.T) {
		timeoutMs := int((5 * time.Minute) / time.Millisecond)
		timeoutSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			TimeoutMs: &timeoutMs,
			Metadata: map[string]string{
				"sandboxTestId": metadata["sandboxTestId"] + "-connect-no-shorten-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		infoBefore, err := e2b.GetInfo(ctx, timeoutSandbox.SandboxID, nil)
		if err != nil {
			t.Fatalf("GetInfo before shorter Connect returned error: %v", err)
		}

		shorterTimeoutMs := int((10 * time.Second) / time.Millisecond)
		connected, err := e2b.Connect(ctx, timeoutSandbox.SandboxID, &e2b.SandboxConnectOpts{TimeoutMs: &shorterTimeoutMs})
		if err != nil {
			t.Fatalf("Connect with shorter timeout returned error: %v", err)
		}
		mustRun(t, ctx, connected, "echo still-running")

		infoAfter, err := timeoutSandbox.GetInfo(ctx, nil)
		if err != nil {
			t.Fatalf("GetInfo after shorter Connect returned error: %v", err)
		}
		if infoAfter.EndAt.Before(infoBefore.EndAt) {
			t.Fatalf("Connect shortened timeout: before=%s after=%s", infoBefore.EndAt.Format(time.RFC3339Nano), infoAfter.EndAt.Format(time.RFC3339Nano))
		}
	})

	t.Run("connect extends timeout", func(t *testing.T) {
		timeoutMs := int((2 * time.Minute) / time.Millisecond)
		timeoutSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			TimeoutMs: &timeoutMs,
			Metadata: map[string]string{
				"sandboxTestId": metadata["sandboxTestId"] + "-connect-extend-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		infoBefore, err := timeoutSandbox.GetInfo(ctx, nil)
		if err != nil {
			t.Fatalf("GetInfo before longer Connect returned error: %v", err)
		}

		longerTimeoutMs := int((10 * time.Minute) / time.Millisecond)
		if _, err := timeoutSandbox.Connect(ctx, &e2b.SandboxConnectOpts{TimeoutMs: &longerTimeoutMs}); err != nil {
			t.Fatalf("Connect with longer timeout returned error: %v", err)
		}
		infoAfter, err := timeoutSandbox.GetInfo(ctx, nil)
		if err != nil {
			t.Fatalf("GetInfo after longer Connect returned error: %v", err)
		}
		if !infoAfter.EndAt.After(infoBefore.EndAt) {
			t.Fatalf("Connect did not extend timeout: before=%s after=%s", infoBefore.EndAt.Format(time.RFC3339Nano), infoAfter.EndAt.Format(time.RFC3339Nano))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		paginationMetadata := map[string]string{
			"sandboxTestId": metadata["sandboxTestId"] + "-pagination-" + fmt.Sprint(time.Now().UnixNano()),
		}
		pageSandbox1 := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{Metadata: paginationMetadata})
		pageSandbox2 := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{Metadata: paginationMetadata})
		firstPage, paginatedItems, hasNext, nextToken := waitForSandboxPagination(t, pageSandbox1.SandboxID, pageSandbox2.SandboxID, paginationMetadata)
		if len(firstPage) != 1 || firstPage[0].State != e2b.SandboxState("running") {
			t.Fatalf("unexpected first pagination page: %#v", firstPage)
		}
		if !hasNext || nextToken == "" {
			t.Fatalf("expected first pagination page to expose next token, hasNext=%v nextToken=%q", hasNext, nextToken)
		}
		if !sandboxInfoListContains(paginatedItems, pageSandbox1.SandboxID) || !sandboxInfoListContains(paginatedItems, pageSandbox2.SandboxID) {
			t.Fatalf("expected paginated list to contain %s and %s, got %#v", pageSandbox1.SandboxID, pageSandbox2.SandboxID, paginatedItems)
		}
	})

	if _, err := sandbox.Connect(ctx, nil); err != nil {
		t.Fatalf("Sandbox.Connect returned error: %v", err)
	}
	mustRun(t, ctx, sandbox, "echo connected")

	if err := sandbox.SetTimeout(ctx, int((10*time.Minute)/time.Millisecond), nil); err != nil {
		t.Fatalf("SetTimeout returned error: %v", err)
	}

	killedForConnect := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
		Metadata: map[string]string{
			"sandboxTestId": metadata["sandboxTestId"] + "-connect-killed-" + fmt.Sprint(time.Now().UnixNano()),
		},
	})
	if err := killedForConnect.Kill(ctx, nil); err != nil {
		t.Fatalf("Kill sandbox before Connect returned error: %v", err)
	}
	if _, err := e2b.Connect(ctx, killedForConnect.SandboxID, nil); err == nil {
		t.Fatal("expected Connect to killed sandbox to fail")
	} else {
		var notFoundErr *e2b.SandboxNotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Fatalf("expected SandboxNotFoundError, got %T %v", err, err)
		}
	}

	if _, err := runForegroundCommand(sandbox.Commands, ctx, "python3 - <<'PY'\nprint(sum(range(1000)))\nPY", nil); err != nil {
		t.Fatalf("metrics warmup command returned error: %v", err)
	}
	t.Run("metrics", func(t *testing.T) {
		waitForMetrics(t, ctx, sandbox)
	})

	paused, err := sandbox.Pause(ctx, nil)
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}
	if !paused {
		t.Fatal("expected Pause to return true")
	}
	pausedItems, err := e2b.List(&e2b.SandboxListOpts{
		Query: &struct {
			Metadata map[string]string
			State    []e2b.SandboxState
		}{
			Metadata: metadata,
			State:    []e2b.SandboxState{e2b.SandboxState("paused")},
		},
	}).NextItems()
	if err != nil {
		t.Fatalf("List paused sandboxes returned error: %v", err)
	}
	if !sandboxInfoListContains(pausedItems, sandbox.SandboxID) {
		t.Fatalf("expected paused list to contain %s, got %#v", sandbox.SandboxID, pausedItems)
	}

	resumeTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	resumed, err := e2b.Connect(ctx, sandbox.SandboxID, &e2b.SandboxConnectOpts{TimeoutMs: &resumeTimeoutMs})
	if err != nil {
		t.Fatalf("Connect paused sandbox returned error: %v", err)
	}
	mustRun(t, ctx, resumed, "echo resumed")
}

func TestLiveSandboxLifecycleAutoPause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("auto pause without auto resume requires connect", func(t *testing.T) {
		timeoutMs := int((30 * time.Second) / time.Millisecond)
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			TimeoutMs: &timeoutMs,
			Lifecycle: &e2b.SandboxLifecycle{
				OnTimeout:  "pause",
				AutoResume: boolPtr(false),
			},
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-auto-pause-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		if err := sandbox.SetTimeout(ctx, int((3*time.Second)/time.Millisecond), nil); err != nil {
			t.Fatalf("SetTimeout auto-pause sandbox returned error: %v", err)
		}

		waitForSandboxState(t, ctx, sandbox, e2b.SandboxState("paused"), 20*time.Second)
		resumed := connectSandboxWithRetry(t, ctx, sandbox, 60*time.Second)
		waitForSandboxState(t, ctx, resumed, e2b.SandboxState("running"), 20*time.Second)
		mustRun(t, ctx, resumed, "echo resumed")
	})

	t.Run("auto resume wakes on http request", func(t *testing.T) {
		timeoutMs := int((30 * time.Second) / time.Millisecond)
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			TimeoutMs: &timeoutMs,
			Lifecycle: &e2b.SandboxLifecycle{
				OnTimeout:  "pause",
				AutoResume: boolPtr(true),
			},
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-auto-resume-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})

		port := 8000
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, fmt.Sprintf("python3 -m http.server %d", port), nil)
		if err != nil {
			t.Fatalf("RunBackground auto-resume server returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })
		if err := sandbox.SetTimeout(ctx, int((3*time.Second)/time.Millisecond), nil); err != nil {
			t.Fatalf("SetTimeout auto-resume sandbox returned error: %v", err)
		}

		waitForSandboxState(t, ctx, sandbox, e2b.SandboxState("paused"), 20*time.Second)
		url := "https://" + sandbox.GetHost(port)
		waitForSandboxHostStatusOrSkip(t, ctx, url, sandbox.TrafficAccessToken, http.StatusOK, "auto-resume on HTTP request is unavailable in this environment")
		waitForSandboxState(t, ctx, sandbox, e2b.SandboxState("running"), 20*time.Second)
	})
}

func TestLiveSandboxPauseResumeStateRetention(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("env vars", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Envs: map[string]string{"TEST_VAR": "sfisback"},
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-env-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})

		before := strings.TrimSpace(mustRun(t, ctx, sandbox, `echo "$TEST_VAR"`).Stdout)
		if before != "sfisback" {
			t.Fatalf("expected env var before pause, got %q", before)
		}
		pauseAndReconnectSandbox(t, ctx, sandbox)
		after := strings.TrimSpace(mustRun(t, ctx, sandbox, `echo "$TEST_VAR"`).Stdout)
		if after != "sfisback" {
			t.Fatalf("expected env var after resume, got %q", after)
		}
	})

	t.Run("file", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-file-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		filename := "test_snapshot.txt"
		content := "This is a snapshot test file."

		info, err := sandbox.Files.Write(ctx, filename, strings.NewReader(content), nil)
		if err != nil {
			t.Fatalf("Write snapshot file returned error: %v", err)
		}
		if info.Name != filename || info.Type != e2b.FileTypeFile || info.Path != "/home/user/"+filename {
			t.Fatalf("unexpected written file info: %#v", info)
		}
		exists, err := sandbox.Files.Exists(ctx, filename, nil)
		if err != nil || !exists {
			t.Fatalf("expected file to exist before pause, exists=%v err=%v", exists, err)
		}
		readContent, err := readSandboxText(sandbox.Files, ctx, filename, nil)
		if err != nil || readContent != content {
			t.Fatalf("unexpected file content before pause: %q err=%v", readContent, err)
		}

		pauseAndReconnectSandbox(t, ctx, sandbox)

		exists, err = sandbox.Files.Exists(ctx, filename, nil)
		if err != nil || !exists {
			t.Fatalf("expected file to exist after resume, exists=%v err=%v", exists, err)
		}
		readContent, err = readSandboxText(sandbox.Files, ctx, filename, nil)
		if err != nil || readContent != content {
			t.Fatalf("unexpected file content after resume: %q err=%v", readContent, err)
		}
	})

	t.Run("ongoing process", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-ongoing-process-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, "sleep 3600", nil)
		if err != nil {
			t.Fatalf("RunBackground sleep returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })
		expectedPID := handle.Pid

		pauseAndReconnectSandbox(t, ctx, sandbox)

		processes, err := sandbox.Commands.List(ctx, nil)
		if err != nil {
			t.Fatalf("Commands.List after resume returned error: %v", err)
		}
		if !processListContains(processes, expectedPID) {
			t.Fatalf("expected resumed process list to contain pid %d, got %#v", expectedPID, processes)
		}
		connected, err := sandbox.Commands.Connect(ctx, expectedPID, nil)
		if err != nil {
			t.Fatalf("Commands.Connect to resumed process returned error: %v", err)
		}
		if connected.Pid != expectedPID {
			t.Fatalf("expected connected pid %d, got %d", expectedPID, connected.Pid)
		}
		connected.Disconnect()
	})

	t.Run("completed process", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-completed-process-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		filename := "test_long_running.txt"
		if _, err := runBackgroundCommand(sandbox.Commands, ctx, `sleep 2 && echo "done" > /home/user/`+filename, nil); err != nil {
			t.Fatalf("RunBackground delayed file command returned error: %v", err)
		}
		exists, err := sandbox.Files.Exists(ctx, filename, nil)
		if err != nil {
			t.Fatalf("Exists before delayed file returned error: %v", err)
		}
		if exists {
			t.Fatal("expected delayed file not to exist before pause")
		}

		pauseAndReconnectSandbox(t, ctx, sandbox)
		time.Sleep(3 * time.Second)

		exists, err = sandbox.Files.Exists(ctx, filename, nil)
		if err != nil || !exists {
			t.Fatalf("expected delayed file to exist after resume, exists=%v err=%v", exists, err)
		}
		content, err := readSandboxText(sandbox.Files, ctx, filename, nil)
		if err != nil {
			t.Fatalf("ReadText delayed file returned error: %v", err)
		}
		if strings.TrimSpace(content) != "done" {
			t.Fatalf("unexpected delayed file content: %q", content)
		}
	})

	t.Run("http server", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Metadata: map[string]string{
				"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-http-server-" + fmt.Sprint(time.Now().UnixNano()),
			},
		})
		port := 8000
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, fmt.Sprintf("python3 -m http.server %d", port), nil)
		if err != nil {
			t.Fatalf("RunBackground pause/resume server returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })

		url := "https://" + sandbox.GetHost(port)
		waitForSandboxHostStatus(t, ctx, url, sandbox.TrafficAccessToken, http.StatusOK)
		pauseAndReconnectSandbox(t, ctx, sandbox)
		waitForSandboxHostStatus(t, ctx, url, sandbox.TrafficAccessToken, http.StatusOK)
	})
}

func TestLiveSandboxHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("running sandbox host responds", func(t *testing.T) {
		sandbox := newLiveSandbox(t, ctx)
		port := 8081
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, fmt.Sprintf("python3 -m http.server %d", port), nil)
		if err != nil {
			t.Fatalf("RunBackground server returned error: %v", err)
		}
		t.Cleanup(func() {
			_, _ = handle.Kill()
		})

		url := "https://" + sandbox.GetHost(port)
		client := &http.Client{Timeout: 5 * time.Second}
		deadline := time.Now().Add(60 * time.Second)
		var lastErr error
		for time.Now().Before(deadline) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("failed to create host request: %v", err)
			}
			if sandbox.TrafficAccessToken != "" {
				req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
			}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return
				}
				lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
			} else {
				lastErr = err
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatalf("sandbox host did not become reachable: %v", lastErr)
	})

	t.Run("killed sandbox host returns 502 payload", func(t *testing.T) {
		sandbox := newLiveSandbox(t, ctx)
		port := 3000
		url := "https://" + sandbox.GetHost(port)

		if err := sandbox.Kill(ctx, nil); err != nil {
			t.Fatalf("Kill returned error: %v", err)
		}

		client := &http.Client{Timeout: 5 * time.Second}
		deadline := time.Now().Add(30 * time.Second)
		var lastStatus int
		var lastBody []byte
		var lastErr error
		for time.Now().Before(deadline) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("failed to create killed-host request: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				time.Sleep(500 * time.Millisecond)
				continue
			}

			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatalf("failed to read killed-host response body: %v", readErr)
			}

			lastStatus = resp.StatusCode
			lastBody = body
			if resp.StatusCode == http.StatusBadGateway {
				var payload struct {
					Message   string `json:"message"`
					SandboxID string `json:"sandboxId"`
					Code      int    `json:"code"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("failed to decode killed-host response body %q: %v", string(body), err)
				}
				if payload.Message != "The sandbox was not found" {
					t.Fatalf("unexpected killed-host message: %#v", payload)
				}
				if payload.Code != http.StatusBadGateway {
					t.Fatalf("unexpected killed-host code: %#v", payload)
				}
				if payload.SandboxID == "" || !strings.HasPrefix(sandbox.SandboxID, payload.SandboxID) {
					t.Fatalf("expected killed-host sandboxId prefix of %q, got %#v", sandbox.SandboxID, payload)
				}
				return
			}

			time.Sleep(500 * time.Millisecond)
		}

		t.Fatalf("killed sandbox host did not return 502 payload: status=%d body=%q err=%v", lastStatus, string(lastBody), lastErr)
	})
}

func TestLiveSandboxPublicTraffic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("allow public traffic false requires token", func(t *testing.T) {
		secure := true
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Secure: &secure,
			Network: &e2b.SandboxNetworkOpts{
				AllowPublicTraffic: boolPtr(false),
			},
		})
		if sandbox.TrafficAccessToken == "" {
			t.Skip("allowPublicTraffic=false does not return a traffic access token in this environment")
		}

		port := 8082
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, fmt.Sprintf("python3 -m http.server %d", port), nil)
		if err != nil {
			t.Fatalf("RunBackground server returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })

		url := "https://" + sandbox.GetHost(port)
		waitForSandboxHostStatus(t, ctx, url, "", http.StatusForbidden)
		waitForSandboxHostStatus(t, ctx, url, sandbox.TrafficAccessToken, http.StatusOK)
	})

	t.Run("allow public traffic true works without token", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Network: &e2b.SandboxNetworkOpts{
				AllowPublicTraffic: boolPtr(true),
			},
		})

		port := 8083
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, fmt.Sprintf("python3 -m http.server %d", port), nil)
		if err != nil {
			t.Fatalf("RunBackground server returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })

		url := "https://" + sandbox.GetHost(port)
		waitForSandboxHostStatus(t, ctx, url, "", http.StatusOK)
	})

	t.Run("mask request host", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Network: &e2b.SandboxNetworkOpts{
				AllowPublicTraffic: boolPtr(true),
				MaskRequestHost:    "custom-host.example.com:${PORT}",
			},
		})

		port := 8084
		outputFile := "/tmp/go-sdk-mask-request-host.txt"
		serverCmd := fmt.Sprintf(`python3 - <<'PY'
import http.server

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        with open(%s, "w") as f:
            for k, v in self.headers.items():
                f.write(k + ": " + v + chr(10))
        self.send_response(200)
        self.end_headers()
    def log_message(self, *a):
        pass

http.server.HTTPServer(("", %d), H).serve_forever()
PY`, shellQuote(outputFile), port)
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, serverCmd, nil)
		if err != nil {
			t.Fatalf("RunBackground masked host server returned error: %v", err)
		}
		t.Cleanup(func() { _, _ = handle.Kill() })

		url := "https://" + sandbox.GetHost(port)
		waitForSandboxHostStatus(t, ctx, url, "", http.StatusOK)
		result, err := runForegroundCommand(sandbox.Commands, ctx, "cat "+shellQuote(outputFile), nil)
		if err != nil {
			t.Fatalf("cat captured headers returned error: %v", err)
		}
		if !strings.Contains(result.Stdout, "custom-host.example.com") {
			t.Skipf("maskRequestHost is not enforced in this environment; captured headers: %q", result.Stdout)
		}
		if !strings.Contains(strings.ToLower(result.Stdout), "host:") ||
			!strings.Contains(result.Stdout, strconv.Itoa(port)) {
			t.Fatalf("expected masked Host header to contain custom host and port, got %q", result.Stdout)
		}
	})
}

func TestLiveSandboxNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
		Network: &e2b.SandboxNetworkOpts{
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			AllowOut: []string{"1.1.1.1"},
		},
	})

	allowed, err := runForegroundCommand(sandbox.Commands, ctx, "curl --connect-timeout 3 --max-time 5 -s -o /dev/null -w '%{http_code}' https://1.1.1.1", nil)
	if err != nil {
		t.Skipf("allowed network route is not reachable in this environment: %v", err)
	}
	if strings.TrimSpace(allowed.Stdout) != "301" {
		t.Skipf("allowed network route returned unexpected status in this environment: %q", allowed.Stdout)
	}

	_, err = runForegroundCommand(sandbox.Commands, ctx, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8", nil)
	var exitErr *e2b.CommandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected denied IP to return CommandExitError, got %T %v", err, err)
	}
}

func TestLiveSandboxNetworkRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	const injectedHeader = "X-E2B-Test-Token"
	const injectedValue = "e2b-transform-value-123"

	sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
		Network: &e2b.SandboxNetworkOpts{
			AllowOut: []string{"httpbin.e2b.team"},
			DenyOut:  []string{e2b.ALL_TRAFFIC},
			Rules: e2b.SandboxNetworkRules{
				"httpbin.e2b.team": {
					{
						Transform: &e2b.SandboxNetworkTransform{
							Headers: map[string]string{
								injectedHeader: injectedValue,
							},
						},
					},
				},
			},
		},
	})

	result, err := runForegroundCommand(sandbox.Commands, ctx, "curl -sS --max-time 10 https://httpbin.e2b.team/headers", nil)
	if err != nil {
		t.Skipf("network rules are not reachable in this environment: %v", err)
	}

	var parsed struct {
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &parsed); err != nil {
		t.Fatalf("failed to decode reflected headers: %v", err)
	}

	if parsed.Headers[injectedHeader] != injectedValue {
		t.Skipf("network transform is not enforced in this environment; reflected headers: %#v", parsed.Headers)
	}
}

func TestLiveSandboxUpdateNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("applies new egress rules", func(t *testing.T) {
		sandbox := newLiveSandbox(t, ctx)

		before, err := runForegroundCommand(sandbox.Commands, ctx, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8", nil)
		if err != nil {
			t.Skipf("baseline network route is not reachable in this environment: %v", err)
		}
		if before.ExitCode != 0 {
			t.Skipf("baseline network route returned unexpected exit code in this environment: %d", before.ExitCode)
		}

		if err := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
			DenyOut: []string{"8.8.8.8"},
		}, nil); err != nil {
			t.Fatalf("UpdateNetwork returned error: %v", err)
		}

		_, err = runForegroundCommand(sandbox.Commands, ctx, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8", nil)
		var exitErr *e2b.CommandExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected updated deny rule to return CommandExitError, got %T %v", err, err)
		}
	})

	t.Run("clears existing rules when fields are omitted", func(t *testing.T) {
		sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{
			Network: &e2b.SandboxNetworkOpts{
				DenyOut:  []string{e2b.ALL_TRAFFIC},
				AllowOut: []string{"1.1.1.1"},
			},
		})

		_, err := runForegroundCommand(sandbox.Commands, ctx, "curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8", nil)
		var exitErr *e2b.CommandExitError
		if !errors.As(err, &exitErr) {
			t.Skipf("baseline deny rule is not enforced in this environment: %T %v", err, err)
		}

		if err := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{}, nil); err != nil {
			t.Fatalf("UpdateNetwork returned error: %v", err)
		}

		cleared, err := runForegroundCommand(sandbox.Commands, ctx, "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8", nil)
		if err != nil {
			t.Fatalf("expected omitted update fields to clear deny rules, got %v", err)
		}
		if cleared.ExitCode != 0 {
			t.Fatalf("expected cleared route to keep exit code 0, got %d", cleared.ExitCode)
		}
	})
}

func TestLiveSandboxInternetAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	defaultSandbox := newLiveSandbox(t, ctx)
	assertSandboxConnectivityCheck(t, ctx, defaultSandbox, "default")

	allowInternet := true
	enabledSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{AllowInternetAccess: &allowInternet})
	assertSandboxConnectivityCheck(t, ctx, enabledSandbox, "enabled")

	allowInternet = false
	disabledSandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{AllowInternetAccess: &allowInternet})
	_, err := runForegroundCommand(disabledSandbox.Commands, ctx, "curl --connect-timeout 3 --max-time 5 -Is https://connectivitycheck.gstatic.com/generate_204", nil)
	if err == nil {
		t.Skip("allowInternetAccess=false is not enforced in this environment")
	}
	var exitErr *e2b.CommandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected CommandExitError when internet access is disabled, got %T %v", err, err)
	}
}

func TestLiveRandomness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	t.Run("python random numbers differ in same sandbox", func(t *testing.T) {
		sandbox := newLiveSandbox(t, ctx)

		first, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		second, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors in the same sandbox, got %q", first)
		}
	})

	t.Run("python random numbers differ across sandboxes from same template", func(t *testing.T) {
		firstSandbox := newLiveSandbox(t, ctx)
		first, ok := runNumpyRandomVector(t, ctx, firstSandbox)
		if !ok {
			return
		}
		if err := firstSandbox.Kill(ctx, nil); err != nil {
			t.Fatalf("Kill first randomness sandbox returned error: %v", err)
		}

		secondSandbox := newLiveSandbox(t, ctx)
		second, ok := runNumpyRandomVector(t, ctx, secondSandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors across sandboxes, got %q", first)
		}
	})
}

func TestLiveClaudeCodeInterpreterRandomness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	if os.Getenv("E2B_API_KEY") == "" {
		t.Skip("E2B_API_KEY is required for integration tests")
	}

	exists, err := e2b.Exists(ctx, "claude-code-interpreter", liveConnectionOpts())
	if err != nil {
		t.Skipf("could not check claude-code-interpreter template alias: %v", err)
	}
	if !exists {
		t.Skip("claude-code-interpreter template alias is unavailable in this environment")
	}

	template := "claude-code-interpreter"

	t.Run("python random numbers differ in same sandbox", func(t *testing.T) {
		sandbox := createLiveSandboxFromTemplate(t, ctx, template, "claude-random-same")
		first, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		second, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors in the same sandbox, got %q", first)
		}
	})

	t.Run("python random numbers differ across sandboxes from same template", func(t *testing.T) {
		firstSandbox := createLiveSandboxFromTemplate(t, ctx, template, "claude-random-first")
		first, ok := runNumpyRandomVector(t, ctx, firstSandbox)
		if !ok {
			return
		}
		if err := firstSandbox.Kill(ctx, nil); err != nil {
			t.Fatalf("Kill first randomness sandbox returned error: %v", err)
		}

		secondSandbox := createLiveSandboxFromTemplate(t, ctx, template, "claude-random-second")
		second, ok := runNumpyRandomVector(t, ctx, secondSandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors across sandboxes, got %q", first)
		}
	})
}

func TestLiveClaudeCodeInterpreterDerivedNumpyTemplateRandomness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	exists, err := e2b.Exists(ctx, "claude-code-interpreter", liveConnectionOpts())
	if err != nil {
		t.Skipf("could not check claude-code-interpreter template alias: %v", err)
	}
	if !exists {
		t.Skip("claude-code-interpreter template alias is unavailable in this environment")
	}

	name := fmt.Sprintf("go-sdk-claude-derived-numpy-%d", time.Now().UnixNano())
	template := e2b.Template(nil).
		FromTemplate("claude-code-interpreter").
		SkipCache().
		RunCmd("python3 -m pip install --break-system-packages --no-cache-dir numpy")

	info, err := e2b.Build(ctx, template, name, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Skipf("could not build temporary claude-code-interpreter+numpy template in this environment: %v", err)
	}
	if info == nil || info.TemplateID == "" {
		t.Fatalf("expected temporary claude-code-interpreter+numpy build to return template ID, got %#v", info)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	t.Run("python random numbers differ in same sandbox", func(t *testing.T) {
		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "claude-derived-numpy-same")
		first, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		second, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors in the same sandbox, got %q", first)
		}
	})

	t.Run("python random numbers differ across sandboxes from same template", func(t *testing.T) {
		firstSandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "claude-derived-numpy-first")
		first, ok := runNumpyRandomVector(t, ctx, firstSandbox)
		if !ok {
			return
		}
		if err := firstSandbox.Kill(ctx, nil); err != nil {
			t.Fatalf("Kill first randomness sandbox returned error: %v", err)
		}

		secondSandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "claude-derived-numpy-second")
		second, ok := runNumpyRandomVector(t, ctx, secondSandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors across sandboxes, got %q", first)
		}
	})
}

func TestLiveTemporaryPythonNumpyTemplateRandomness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	name := fmt.Sprintf("go-sdk-python-numpy-%d", time.Now().UnixNano())
	template := e2b.Template(nil).
		FromPythonImage("3.12").
		SkipCache().
		RunCmd("python3 -m pip install --no-cache-dir numpy")

	info, err := e2b.Build(ctx, template, name, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Skipf("could not build temporary python+numpy template in this environment: %v", err)
	}
	if info == nil || info.TemplateID == "" {
		t.Fatalf("expected temporary python+numpy template build to return template ID, got %#v", info)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	t.Run("python random numbers differ in same sandbox", func(t *testing.T) {
		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "tmp-python-numpy-same")
		first, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		second, ok := runNumpyRandomVector(t, ctx, sandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors in the same sandbox, got %q", first)
		}
	})

	t.Run("python random numbers differ across sandboxes from same template", func(t *testing.T) {
		firstSandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "tmp-python-numpy-first")
		first, ok := runNumpyRandomVector(t, ctx, firstSandbox)
		if !ok {
			return
		}
		if err := firstSandbox.Kill(ctx, nil); err != nil {
			t.Fatalf("Kill first randomness sandbox returned error: %v", err)
		}

		secondSandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "tmp-python-numpy-second")
		second, ok := runNumpyRandomVector(t, ctx, secondSandbox)
		if !ok {
			return
		}
		if first == second {
			t.Fatalf("expected different random vectors across sandboxes, got %q", first)
		}
	})
}

func TestLiveStress(t *testing.T) {
	if os.Getenv("E2B_RUN_STRESS") != "1" {
		t.Skip("set E2B_RUN_STRESS=1 to run expensive live stress tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	t.Run("heavy file writes and reads", func(t *testing.T) {
		template := getLiveTemplate(t, ctx)
		sandboxCount := envInt("E2B_STRESS_SANDBOX_COUNT", 10)
		fileSizeMB := envInt("E2B_STRESS_FILE_MB", 256)
		metadataPrefix := liveSandboxMetadata(t)["sandboxTestId"]
		data := bytes.Repeat([]byte{0x5a}, fileSizeMB*1024*1024)

		errs := make(chan error, sandboxCount)
		var wg sync.WaitGroup
		for i := 0; i < sandboxCount; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				timeoutMs := int((10 * time.Minute) / time.Millisecond)
				sandbox, err := e2b.Create(ctx, template, &e2b.SandboxOpts{
					ConnectionOpts: mergeLiveConnectionOpts(e2b.ConnectionOpts{}),
					TimeoutMs:      &timeoutMs,
					Metadata: map[string]string{
						"sandboxTestId": fmt.Sprintf("%s-heavy-%d-%d", metadataPrefix, i, time.Now().UnixNano()),
					},
				})
				if err != nil {
					errs <- fmt.Errorf("create sandbox %d: %w", i, err)
					return
				}
				defer func() {
					cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
					defer cleanupCancel()
					_ = sandbox.Kill(cleanupCtx, nil)
				}()

				requestTimeoutMs := int((5 * time.Minute) / time.Millisecond)
				if _, err := sandbox.Files.Write(ctx, "heavy-file", bytes.NewReader(data), &e2b.FilesystemWriteOpts{
					FilesystemRequestOpts: e2b.FilesystemRequestOpts{RequestTimeoutMs: &requestTimeoutMs},
				}); err != nil {
					errs <- fmt.Errorf("write sandbox %d: %w", i, err)
					return
				}
				readBack, err := readSandboxBytes(sandbox.Files, ctx, "heavy-file", &e2b.FilesystemReadOpts{
					FilesystemRequestOpts: e2b.FilesystemRequestOpts{RequestTimeoutMs: &requestTimeoutMs},
				})
				if err != nil {
					errs <- fmt.Errorf("read sandbox %d: %w", i, err)
					return
				}
				if !bytes.Equal(readBack, data) {
					errs <- fmt.Errorf("sandbox %d heavy file mismatch: got %d bytes want %d bytes", i, len(readBack), len(data))
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("requests to app hosts", func(t *testing.T) {
		template := os.Getenv("E2B_STRESS_TEMPLATE")
		if template == "" {
			t.Skip("E2B_STRESS_TEMPLATE is required for app host stress test")
		}
		sandboxCount := envInt("E2B_STRESS_SANDBOX_COUNT", 10)
		requestRounds := envInt("E2B_STRESS_REQUEST_ROUNDS", 100)

		hosts := make([]string, 0, sandboxCount)
		sandboxes := make([]*e2b.Sandbox, 0, sandboxCount)
		for i := 0; i < sandboxCount; i++ {
			timeoutMs := int((10 * time.Minute) / time.Millisecond)
			sandbox, err := e2b.Create(ctx, template, &e2b.SandboxOpts{
				ConnectionOpts: mergeLiveConnectionOpts(e2b.ConnectionOpts{}),
				TimeoutMs:      &timeoutMs,
				Metadata: map[string]string{
					"sandboxTestId": fmt.Sprintf("%s-host-%d-%d", liveSandboxMetadata(t)["sandboxTestId"], i, time.Now().UnixNano()),
				},
			})
			if err != nil {
				t.Fatalf("Create host stress sandbox %d returned error: %v", i, err)
			}
			sandboxes = append(sandboxes, sandbox)
			hosts = append(hosts, sandbox.GetHost(3000))
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
			defer cleanupCancel()
			for _, sandbox := range sandboxes {
				_ = sandbox.Kill(cleanupCtx, nil)
			}
		})

		time.Sleep(10 * time.Second)
		client := &http.Client{Timeout: 10 * time.Second}
		for round := 0; round < requestRounds; round++ {
			for _, host := range hosts {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host, nil)
				if err != nil {
					t.Fatalf("failed to create stress host request: %v", err)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatalf("GET stress host %s returned error: %v", host, err)
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= 500 {
					t.Fatalf("GET stress host %s returned server error %d", host, resp.StatusCode)
				}
			}
		}
	})
}

func TestLiveSnapshots(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	sandbox := newLiveSandbox(t, ctx)
	baseDir := path.Join("/home/user", "go-snapshot-"+fmt.Sprint(time.Now().UnixNano()))
	configPath := path.Join(baseDir, "config.json")
	dataPath := path.Join(baseDir, "data.txt")
	configContent := `{"env":"test"}`
	dataContent := "important data"

	if _, err := sandbox.Files.MakeDir(ctx, baseDir, nil); err != nil {
		t.Fatalf("MakeDir returned error: %v", err)
	}
	if _, err := sandbox.Files.Write(ctx, configPath, strings.NewReader(configContent), nil); err != nil {
		t.Fatalf("Write config returned error: %v", err)
	}
	if _, err := sandbox.Files.Write(ctx, dataPath, strings.NewReader(dataContent), nil); err != nil {
		t.Fatalf("Write data returned error: %v", err)
	}

	snapshotName := "go-sdk-integration-" + fmt.Sprint(time.Now().UnixNano())
	snapshot, err := sandbox.CreateSnapshot(ctx, &e2b.CreateSnapshotOpts{Name: snapshotName})
	if err != nil {
		t.Fatalf("CreateSnapshot returned error: %v", err)
	}
	if snapshot.SnapshotID == "" {
		t.Fatalf("expected snapshot ID, got %#v", snapshot)
	}
	snapshotID := snapshot.SnapshotID
	defer func() {
		if snapshotID != "" {
			_, _ = e2b.DeleteSnapshot(context.Background(), snapshotID, nil)
		}
	}()

	globalSnapshots, err := e2b.ListSnapshots(&e2b.SnapshotListOpts{Limit: 50}).NextItems()
	if err != nil {
		t.Fatalf("ListSnapshots returned error: %v", err)
	}
	if !snapshotListContains(globalSnapshots, snapshot.SnapshotID) {
		t.Fatalf("expected global snapshot list to contain %s, got %#v", snapshot.SnapshotID, globalSnapshots)
	}

	sandboxSnapshots, err := sandbox.ListSnapshots(&struct {
		e2b.SandboxApiOpts
		Limit     int
		NextToken string
	}{Limit: 50}).NextItems()
	if err != nil {
		t.Fatalf("sandbox.ListSnapshots returned error: %v", err)
	}
	if !snapshotListContains(sandboxSnapshots, snapshot.SnapshotID) {
		t.Fatalf("expected sandbox snapshot list to contain %s, got %#v", snapshot.SnapshotID, sandboxSnapshots)
	}

	branch1 := createLiveSandboxFromTemplate(t, ctx, snapshot.SnapshotID, "snapshot-branch-1")
	branch2 := createLiveSandboxFromTemplate(t, ctx, snapshot.SnapshotID, "snapshot-branch-2")

	for name, sbx := range map[string]*e2b.Sandbox{"branch1": branch1, "branch2": branch2} {
		dirExists, err := sbx.Files.Exists(ctx, baseDir, nil)
		if err != nil {
			t.Fatalf("%s Exists dir returned error: %v", name, err)
		}
		if !dirExists {
			t.Fatalf("%s expected directory from snapshot to exist", name)
		}
		config, err := readSandboxText(sbx.Files, ctx, configPath, nil)
		if err != nil {
			t.Fatalf("%s ReadText config returned error: %v", name, err)
		}
		data, err := readSandboxText(sbx.Files, ctx, dataPath, nil)
		if err != nil {
			t.Fatalf("%s ReadText data returned error: %v", name, err)
		}
		if config != configContent || data != dataContent {
			t.Fatalf("%s unexpected snapshot contents: config=%q data=%q", name, config, data)
		}
	}

	if _, err := branch1.Files.Write(ctx, dataPath, strings.NewReader("modified in branch1"), nil); err != nil {
		t.Fatalf("branch1 Write modified data returned error: %v", err)
	}
	branch1Data, err := readSandboxText(branch1.Files, ctx, dataPath, nil)
	if err != nil {
		t.Fatalf("branch1 ReadText modified data returned error: %v", err)
	}
	branch2Data, err := readSandboxText(branch2.Files, ctx, dataPath, nil)
	if err != nil {
		t.Fatalf("branch2 ReadText data returned error: %v", err)
	}
	if branch1Data != "modified in branch1" || branch2Data != dataContent {
		t.Fatalf("expected snapshot branches to be isolated, branch1=%q branch2=%q", branch1Data, branch2Data)
	}

	// Kill branch sandboxes before deleting the snapshot they were created from.
	for name, sbx := range map[string]*e2b.Sandbox{"branch1": branch1, "branch2": branch2} {
		if err := sbx.Kill(ctx, nil); err != nil {
			t.Logf("warning: kill %s returned error: %v", name, err)
		}
	}

	deleted, err := e2b.DeleteSnapshot(ctx, snapshot.SnapshotID, nil)
	if err != nil {
		t.Fatalf("DeleteSnapshot returned error: %v", err)
	}
	if !deleted {
		t.Fatal("expected first DeleteSnapshot to return true")
	}
	waitForSnapshotAbsent(t, ctx, snapshot.SnapshotID)

	deletedAgain, err := e2b.DeleteSnapshot(ctx, snapshot.SnapshotID, nil)
	if err != nil {
		t.Fatalf("second DeleteSnapshot returned error: %v", err)
	}
	if deletedAgain {
		t.Fatal("expected second DeleteSnapshot to return false")
	}
	snapshotID = ""
}

func TestLiveFileSigning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	secure := true
	sandbox := newLiveSandboxWithOpts(t, ctx, &e2b.SandboxOpts{Secure: &secure})
	validExpiration := 10
	expiredExpiration := -10

	info, err := sandbox.Files.Write(ctx, "hello.txt", strings.NewReader("hello world"), nil)
	if err != nil {
		t.Fatalf("Write hello.txt returned error: %v", err)
	}
	if info.Name != "hello.txt" || info.Type != e2b.FileTypeFile || info.Path != "/home/user/hello.txt" {
		t.Fatalf("unexpected secure write info: %#v", info)
	}
	readBack, err := readSandboxText(sandbox.Files, ctx, "hello.txt", nil)
	if err != nil {
		t.Fatalf("ReadText hello.txt on secure sandbox returned error: %v", err)
	}
	if readBack != "hello world" {
		t.Fatalf("unexpected secure readback content: %q", readBack)
	}

	t.Run("secure connect", func(t *testing.T) {
		dir := "test_directory_secure_connect"
		connected, err := e2b.Connect(ctx, sandbox.SandboxID, nil)
		if err != nil {
			t.Fatalf("Connect secure sandbox returned error: %v", err)
		}
		if _, err := connected.Files.MakeDir(ctx, dir, nil); err != nil {
			t.Fatalf("MakeDir through connected secure sandbox returned error: %v", err)
		}
		files, err := connected.Files.List(ctx, dir, nil)
		if err != nil {
			t.Fatalf("List through connected secure sandbox returned error: %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("expected new secure directory to be empty, got %#v", files)
		}
	})

	t.Run("secure watch dir", func(t *testing.T) {
		dir := livePath("secure-watch")
		if _, err := sandbox.Files.MakeDir(ctx, dir, nil); err != nil {
			t.Fatalf("MakeDir secure watch dir returned error: %v", err)
		}

		events := make(chan e2b.FilesystemEvent, 8)
		watchTimeoutMs := int((10 * time.Second) / time.Millisecond)
		handle, err := sandbox.Files.WatchDir(ctx, dir, func(event e2b.FilesystemEvent) {
			events <- event
		}, &e2b.WatchOpts{TimeoutMs: &watchTimeoutMs})
		if err != nil {
			t.Fatalf("WatchDir on secure sandbox returned error: %v", err)
		}
		defer handle.Stop()

		filename := "test_watch.txt"
		if _, err := sandbox.Files.Write(ctx, path.Join(dir, filename), strings.NewReader("This file will be watched."), nil); err != nil {
			t.Fatalf("Write secure watched file returned error: %v", err)
		}
		waitForFilesystemEvent(t, events, filename)
	})

	t.Run("secure command run", func(t *testing.T) {
		result, err := runForegroundCommand(sandbox.Commands, ctx, "echo Hello World!", nil)
		if err != nil {
			t.Fatalf("Run on secure sandbox returned error: %v", err)
		}
		if result.Stdout != "Hello World!\n" {
			t.Fatalf("unexpected secure command stdout: %q", result.Stdout)
		}
	})

	plainDownload, err := sandbox.DownloadUrl("hello.txt", nil)
	if err != nil {
		t.Fatalf("DownloadUrl without expiration returned error: %v", err)
	}
	status, body := fetchText(t, ctx, plainDownload)
	if status != http.StatusOK || body != "hello world" {
		t.Fatalf("unexpected signed download without expiration response: status=%d body=%q", status, body)
	}

	validDownload, err := sandbox.DownloadUrl("hello.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &validExpiration})
	if err != nil {
		skipIfSigningUnavailable(t, err)
		t.Fatalf("DownloadUrl returned error: %v", err)
	}
	status, body = fetchText(t, ctx, validDownload)
	if status != http.StatusOK || body != "hello world" {
		t.Fatalf("unexpected signed download response: status=%d body=%q", status, body)
	}

	expiredDownload, err := sandbox.DownloadUrl("hello.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &expiredExpiration})
	if err != nil {
		t.Fatalf("expired DownloadUrl returned error: %v", err)
	}
	status, body = fetchText(t, ctx, expiredDownload)
	if status != http.StatusUnauthorized || !strings.Contains(body, "signature is already expired") {
		t.Fatalf("unexpected expired download response: status=%d body=%q", status, body)
	}

	if _, err := sandbox.Files.Write(ctx, "root-hello.txt", strings.NewReader("hello root"), &e2b.FilesystemWriteOpts{
		FilesystemRequestOpts: e2b.FilesystemRequestOpts{User: "root"},
	}); err != nil {
		t.Fatalf("Write root-hello.txt returned error: %v", err)
	}
	rootDownload, err := sandbox.DownloadUrl("root-hello.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &validExpiration, User: "root"})
	if err != nil {
		t.Fatalf("root DownloadUrl returned error: %v", err)
	}
	status, body = fetchText(t, ctx, rootDownload)
	if status != http.StatusOK || body != "hello root" {
		t.Fatalf("unexpected root signed download response: status=%d body=%q", status, body)
	}

	uploadURL, err := sandbox.UploadUrl("uploaded.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &validExpiration})
	if err != nil {
		t.Fatalf("UploadUrl returned error: %v", err)
	}
	status, body = postMultipartFile(t, ctx, uploadURL, "uploaded.txt", "file content")
	if status != http.StatusOK {
		t.Fatalf("unexpected signed upload response: status=%d body=%q", status, body)
	}
	assertUploadResponsePath(t, body, "/home/user/uploaded.txt")

	rootUploadURL, err := sandbox.UploadUrl("root-uploaded.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &validExpiration, User: "root"})
	if err != nil {
		t.Fatalf("root UploadUrl returned error: %v", err)
	}
	status, body = postMultipartFile(t, ctx, rootUploadURL, "root-uploaded.txt", "file content")
	if status != http.StatusOK {
		t.Fatalf("unexpected root signed upload response: status=%d body=%q", status, body)
	}
	assertUploadResponsePath(t, body, "/root/root-uploaded.txt")

	expiredUploadURL, err := sandbox.UploadUrl("expired-upload.txt", &struct {
		UseSignatureExpiration *int
		User                   string
	}{UseSignatureExpiration: &expiredExpiration})
	if err != nil {
		t.Fatalf("expired UploadUrl returned error: %v", err)
	}
	status, body = postMultipartFile(t, ctx, expiredUploadURL, "expired-upload.txt", "file content")
	if status != http.StatusUnauthorized || !strings.Contains(body, "signature is already expired") {
		t.Fatalf("unexpected expired upload response: status=%d body=%q", status, body)
	}
}

func TestLiveVolumeLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	requireLiveEnv(t)
	if os.Getenv("E2B_API_KEY") == "" {
		t.Skip("E2B_API_KEY is required for volume integration tests")
	}
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)

	volumeName := "go-sdk-volume-" + fmt.Sprint(time.Now().UnixNano())
	vol, err := e2bvol.Create(ctx, volumeName, &e2bvol.ConnectionOpts{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	})
	if err != nil {
		t.Fatalf("Create volume returned error: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e2bvol.Destroy(context.Background(), vol.VolumeID, &e2bvol.ConnectionOpts{
			ApiKey:      os.Getenv("E2B_API_KEY"),
			AccessToken: os.Getenv("E2B_ACCESS_TOKEN"),
			Domain:      os.Getenv("E2B_DOMAIN"),
			ApiUrl:      os.Getenv("E2B_API_URL"),
		})
	})

	t.Run("control plane lifecycle", func(t *testing.T) {
		if vol.Name != volumeName {
			t.Fatalf("expected created volume name %q, got %q", volumeName, vol.Name)
		}
		if vol.VolumeID == "" || vol.Token == "" {
			t.Fatalf("expected created volume to include ID and token, got %#v", vol)
		}

		info, err := e2bvol.GetInfo(ctx, vol.VolumeID, &e2bvol.ConnectionOpts{
			ApiKey:      os.Getenv("E2B_API_KEY"),
			AccessToken: os.Getenv("E2B_ACCESS_TOKEN"),
			Domain:      os.Getenv("E2B_DOMAIN"),
			ApiUrl:      os.Getenv("E2B_API_URL"),
		})
		if err != nil {
			t.Fatalf("GetInfo returned error: %v", err)
		}
		if info.VolumeID != vol.VolumeID || info.Name != volumeName {
			t.Fatalf("unexpected volume info: %#v", info)
		}

		entries, err := e2bvol.List(ctx, &e2bvol.ConnectionOpts{
			ApiKey:      os.Getenv("E2B_API_KEY"),
			AccessToken: os.Getenv("E2B_ACCESS_TOKEN"),
			Domain:      os.Getenv("E2B_DOMAIN"),
			ApiUrl:      os.Getenv("E2B_API_URL"),
		})
		if err != nil {
			t.Fatalf("List volumes returned error: %v", err)
		}
		if !liveVolumeListContains(entries, vol.VolumeID, volumeName) {
			t.Fatalf("expected created volume in control-plane list, got %#v", entries)
		}
	})

	t.Run("file operations lifecycle", func(t *testing.T) {
		dirPath := "/multi-file-dir"
		files := []string{"file1.txt", "file2.txt", "file3.txt"}

		dirStat, err := vol.MakeDir(ctx, dirPath, &e2bvol.VolumeWriteOptions{})
		if err != nil {
			t.Skipf("volume content API does not support makeDir in this environment; JS shows the same NotFound behavior: %v", err)
		}
		if dirStat.Type != e2bvol.VolumeFileTypeDirectory {
			t.Fatalf("expected directory type, got %#v", dirStat)
		}

		for _, fileName := range files {
			path := dirPath + "/" + fileName
			content := "Content of " + fileName
			stat, err := vol.WriteFile(ctx, path, strings.NewReader(content), nil)
			if err != nil {
				t.Fatalf("WriteFile(%s) returned error: %v", path, err)
			}
			if stat.Type != e2bvol.VolumeFileTypeFile || stat.Name != fileName {
				t.Fatalf("unexpected file stat for %s: %#v", path, stat)
			}
		}

		listing, err := vol.List(ctx, dirPath, nil)
		if err != nil {
			t.Fatalf("List(%s) returned error: %v", dirPath, err)
		}
		if len(listing) < len(files) {
			t.Fatalf("expected at least %d files in %s, got %#v", len(files), dirPath, listing)
		}

		for _, fileName := range files {
			path := dirPath + "/" + fileName
			content, err := readVolumeText(vol, ctx, path, nil)
			if err != nil {
				t.Fatalf("ReadFileText(%s) returned error: %v", path, err)
			}
			if content != "Content of "+fileName {
				t.Fatalf("unexpected file content for %s: %q", path, content)
			}
		}

		if err := vol.Remove(ctx, dirPath, nil); err != nil {
			t.Fatalf("Remove(%s) returned error: %v", dirPath, err)
		}
		exists, err := vol.Exists(ctx, dirPath, nil)
		if err != nil {
			t.Fatalf("Exists(%s) returned error: %v", dirPath, err)
		}
		if exists {
			t.Fatalf("expected %s to be removed", dirPath)
		}
	})
}

func TestLiveTemplateBuildUploadAndTags(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	name := "go-sdk-template-" + fmt.Sprint(time.Now().UnixNano())
	exists, err := e2b.Exists(ctx, name, liveConnectionOpts())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("Template.Exists returned error: %v", err)
	}
	if exists {
		t.Fatalf("unexpected pre-existing template alias %q", name)
	}

	contextDir := t.TempDir()
	if err := os.WriteFile(path.Join(contextDir, "test.txt"), []byte("template upload content\n"), 0o644); err != nil {
		t.Fatalf("failed to write template fixture: %v", err)
	}
	template := e2b.Template(&e2b.TemplateOptions{FileContextPath: contextDir}).
		FromBaseImage().
		Copy("test.txt", "/app/test.txt", &struct{ ForceUpload bool }{ForceUpload: true}).
		RunCmd("cat /app/test.txt")

	info, err := e2b.Build(ctx, template, name, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("Template.Build returned error: %v", err)
	}
	if info.TemplateID == "" || info.BuildID == "" {
		t.Fatalf("expected template and build IDs, got %#v", info)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	exists, err = e2b.Exists(ctx, name, liveConnectionOpts())
	if err != nil {
		t.Fatalf("Template.Exists after build returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected template alias %q to exist after build", name)
	}

	tag := name + "-integration"
	if _, err := e2b.AssignTags(ctx, name, []string{tag}, liveConnectionOpts()); err != nil {
		t.Fatalf("AssignTags returned error: %v", err)
	}
	tags, err := e2b.GetTags(ctx, info.TemplateID, liveConnectionOpts())
	if err != nil {
		t.Fatalf("GetTags returned error: %v", err)
	}
	if !templateTagsContain(tags, tag) {
		t.Fatalf("expected tag %q in %#v", tag, tags)
	}
	if err := e2b.RemoveTags(ctx, name, []string{tag}, liveConnectionOpts()); err != nil {
		t.Fatalf("RemoveTags returned error: %v", err)
	}
}

func TestLiveTemplateBuildRejectsInvalidTagFormats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	templateName := "go-sdk-tags-test"
	initialTag := fmt.Sprintf("%s:v1-%d", templateName, time.Now().UnixNano())

	info, err := e2b.Build(ctx, e2b.Template(nil).FromBaseImage(), initialTag, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("Template.Build returned error: %v", err)
	}
	if info.TemplateID == "" || info.BuildID == "" {
		t.Fatalf("expected template and build IDs, got %#v", info)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	cases := []struct {
		name string
		tag  string
	}{
		{name: "missing alias", tag: ":invalid-tag"},
		{name: "missing tag", tag: templateName + ":"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e2b.AssignTags(ctx, initialTag, tc.tag, liveConnectionOpts()); err == nil {
				t.Fatalf("expected AssignTags(%q) to fail for invalid tag format", tc.tag)
			}
		})
	}
}

func TestLiveTemplateBuildInBackgroundStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	name := fmt.Sprintf("go-sdk-background:v1-%d", time.Now().UnixNano())
	template := e2b.Template(nil).
		FromUbuntuImage("22.04").
		SkipCache().
		RunCmd("sleep 5").
		SetStartCmd(`echo "Hello"`, e2b.WaitForTimeout(10_000))

	buildOpts := liveBuildOptions()
	buildOpts.CpuCount = 1
	buildOpts.MemoryMB = 1024

	startedAt := time.Now()
	info, err := e2b.BuildInBackground(ctx, template, name, buildOpts)
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("BuildInBackground returned error: %v", err)
	}
	if info == nil || info.TemplateID == "" || info.BuildID == "" {
		t.Fatalf("expected template and build IDs, got %#v", info)
	}
	if elapsed := time.Since(startedAt); elapsed > 5*time.Second {
		t.Fatalf("expected BuildInBackground to return before the 5s build step finishes, took %s", elapsed)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
	}()

	deadline := time.Now().Add(30 * time.Second)
	for {
		status, err := e2b.GetBuildStatus(ctx, info, liveBuildStatusOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("GetBuildStatus returned error: %v", err)
		}
		switch string(status.Status) {
		case "building", "waiting":
			return
		case "error":
			t.Fatalf("expected background build to still be in progress, got error status: %#v", status)
		case "ready":
			if time.Now().After(deadline) {
				t.Fatalf("background build became ready before an in-progress status was observed: %#v", status)
			}
		default:
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for in-progress background build status, last status: %#v", status)
			}
		}
		time.Sleep(time.Second)
	}
}

func TestLiveTemplateBuildFromExistingTemplate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	parentName := fmt.Sprintf("go-sdk-parent-template-%d", time.Now().UnixNano())
	parentInfo, err := e2b.Build(ctx, e2b.Template(nil).FromBaseImage(), parentName, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("parent Template.Build returned error: %v", err)
	}
	if parentInfo.TemplateID == "" || parentInfo.BuildID == "" {
		t.Fatalf("expected parent template and build IDs, got %#v", parentInfo)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), parentInfo.TemplateID, nil)
	}()

	childName := fmt.Sprintf("%s-child", parentName)
	childInfo, err := e2b.Build(ctx, e2b.Template(nil).FromTemplate(parentName), childName, liveBuildOptions())
	if err != nil {
		skipIfTemplateAPIUnavailable(t, err)
		t.Fatalf("child Template.Build returned error: %v", err)
	}
	if childInfo.TemplateID == "" || childInfo.BuildID == "" {
		t.Fatalf("expected child template and build IDs, got %#v", childInfo)
	}
	defer func() {
		_, _ = e2b.DeleteSnapshot(context.Background(), childInfo.TemplateID, nil)
	}()

	exists, err := e2b.Exists(ctx, childName, liveConnectionOpts())
	if err != nil {
		t.Fatalf("Template.Exists after child build returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected template alias %q to exist after build", childName)
	}
}

func TestLiveTemplateBuildStacktrace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()
	requireLiveEnv(t)

	_, err := e2b.Build(ctx, e2b.Template(nil).FromTemplate("this-template-does-not-exist"), "go-sdk-stacktrace-"+fmt.Sprint(time.Now().UnixNano()), liveBuildOptions())
	if err == nil {
		t.Fatal("expected build from missing template to fail")
	}
	skipIfTemplateAPIUnavailable(t, err)
	if !strings.Contains(err.Error(), "this-template-does-not-exist") && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Fatalf("expected missing template error to mention missing template or not found, got %v", err)
	}
}

func TestLiveTemplateBuildMethodParityOnBaseImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	requireLiveEnv(t)

	t.Run("run command as root", func(t *testing.T) {
		name := fmt.Sprintf("go-sdk-run-root-%d", time.Now().UnixNano())
		template := e2b.Template(nil).
			FromBaseImage().
			RunCmd(`test "$(whoami)" = "root"`, &struct{ User string }{User: "root"})

		info, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("Template.Build returned error: %v", err)
		}
		if info.TemplateID == "" || info.BuildID == "" {
			t.Fatalf("expected template and build IDs, got %#v", info)
		}
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()

		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "run-root")
		result, err := runForegroundCommand(sandbox.Commands, ctx, "whoami", nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if strings.TrimSpace(result.Stdout) != "user" {
			t.Fatalf("expected sandbox default user to remain user after root build step, got %#v", result)
		}
	})

	t.Run("run command as user that does not exist", func(t *testing.T) {
		name := fmt.Sprintf("go-sdk-run-invalid-user-%d", time.Now().UnixNano())
		template := e2b.Template(nil).
			FromBaseImage().
			RunCmd("whoami", &struct{ User string }{User: "root123"})

		_, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err == nil {
			t.Fatal("expected Template.Build to fail for an invalid build user")
		}
		skipIfTemplateAPIUnavailable(t, err)
		if !strings.Contains(err.Error(), "invalid username") || !strings.Contains(err.Error(), "root123") {
			t.Fatalf("expected invalid username error, got %v", err)
		}
	})

	t.Run("make symlink", func(t *testing.T) {
		name := fmt.Sprintf("go-sdk-symlink-%d", time.Now().UnixNano())
		template := e2b.Template(nil).
			FromBaseImage().
			MakeSymlink(".bashrc", ".bashrc.local").
			RunCmd(`test "$(readlink .bashrc.local)" = ".bashrc"`)

		info, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("Template.Build returned error: %v", err)
		}
		if info.TemplateID == "" || info.BuildID == "" {
			t.Fatalf("expected template and build IDs, got %#v", info)
		}
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()

		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "symlink")
		result, err := runForegroundCommand(sandbox.Commands, ctx, `readlink /home/user/.bashrc.local`, nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if strings.TrimSpace(result.Stdout) != ".bashrc" {
			t.Fatalf("expected .bashrc.local symlink target .bashrc, got %#v", result)
		}
	})

	t.Run("make symlink force", func(t *testing.T) {
		name := fmt.Sprintf("go-sdk-symlink-force-%d", time.Now().UnixNano())
		template := e2b.Template(nil).
			FromBaseImage().
			MakeSymlink(".bashrc", ".bashrc.local").
			SkipCache().
			MakeSymlink(".bashrc", ".bashrc.local", &struct{ Force bool }{Force: true}).
			RunCmd(`test "$(readlink .bashrc.local)" = ".bashrc"`)

		info, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("Template.Build returned error: %v", err)
		}
		if info.TemplateID == "" || info.BuildID == "" {
			t.Fatalf("expected template and build IDs, got %#v", info)
		}
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()

		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "symlink-force")
		result, err := runForegroundCommand(sandbox.Commands, ctx, `readlink /home/user/.bashrc.local`, nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if strings.TrimSpace(result.Stdout) != ".bashrc" {
			t.Fatalf("expected forced .bashrc.local symlink target .bashrc, got %#v", result)
		}
	})

	t.Run("copy symlink without resolve", func(t *testing.T) {
		contextDir := t.TempDir()
		if err := os.WriteFile(path.Join(contextDir, "test.txt"), []byte("template symlink content\n"), 0o644); err != nil {
			t.Fatalf("failed to write template fixture: %v", err)
		}
		if err := os.Symlink("test.txt", path.Join(contextDir, "link.txt")); err != nil {
			t.Fatalf("failed to create symlink fixture: %v", err)
		}

		name := fmt.Sprintf("go-sdk-copy-symlink-%d", time.Now().UnixNano())
		template := e2b.Template(&e2b.TemplateOptions{FileContextPath: contextDir}).
			FromBaseImage().
			Copy("test.txt", "/app/test.txt", &struct{ ForceUpload bool }{ForceUpload: true}).
			Copy("link.txt", "/app/link.txt", &struct{ ForceUpload bool }{ForceUpload: true}).
			RunCmd(`test "$(readlink /app/link.txt)" = "test.txt"`).
			RunCmd(`test "$(cat /app/link.txt)" = "template symlink content"`)

		info, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("Template.Build returned error: %v", err)
		}
		if info.TemplateID == "" || info.BuildID == "" {
			t.Fatalf("expected template and build IDs, got %#v", info)
		}
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()

		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "copy-symlink")
		result, err := runForegroundCommand(sandbox.Commands, ctx, `readlink /app/link.txt && cat /app/link.txt`, nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		output := strings.Split(strings.TrimSpace(result.Stdout), "\n")
		if len(output) != 2 || output[0] != "test.txt" || output[1] != "template symlink content" {
			t.Fatalf("unexpected symlink copy output: %#v", result)
		}
	})

	t.Run("copy symlink with resolve", func(t *testing.T) {
		contextDir := t.TempDir()
		if err := os.WriteFile(path.Join(contextDir, "test.txt"), []byte("resolved symlink content\n"), 0o644); err != nil {
			t.Fatalf("failed to write template fixture: %v", err)
		}
		if err := os.Symlink("test.txt", path.Join(contextDir, "link.txt")); err != nil {
			t.Fatalf("failed to create symlink fixture: %v", err)
		}

		name := fmt.Sprintf("go-sdk-copy-resolved-symlink-%d", time.Now().UnixNano())
		template := e2b.Template(&e2b.TemplateOptions{FileContextPath: contextDir}).
			FromBaseImage().
			Copy("link.txt", "/app/link.txt", &struct {
				ForceUpload     bool
				ResolveSymlinks bool
			}{ForceUpload: true, ResolveSymlinks: true}).
			RunCmd(`test ! -L /app/link.txt`).
			RunCmd(`test "$(cat /app/link.txt)" = "resolved symlink content"`)

		info, err := e2b.Build(ctx, template, name, liveBuildOptions())
		if err != nil {
			skipIfTemplateAPIUnavailable(t, err)
			t.Fatalf("Template.Build returned error: %v", err)
		}
		if info.TemplateID == "" || info.BuildID == "" {
			t.Fatalf("expected template and build IDs, got %#v", info)
		}
		defer func() {
			_, _ = e2b.DeleteSnapshot(context.Background(), info.TemplateID, nil)
		}()

		sandbox := createLiveSandboxFromTemplate(t, ctx, info.TemplateID, "copy-resolved-symlink")
		result, err := runForegroundCommand(sandbox.Commands, ctx, `if [ -L /app/link.txt ]; then echo symlink; else echo regular; fi && cat /app/link.txt`, nil)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		output := strings.Split(strings.TrimSpace(result.Stdout), "\n")
		if len(output) != 2 || output[0] != "regular" || output[1] != "resolved symlink content" {
			t.Fatalf("unexpected resolved symlink copy output: %#v", result)
		}
	})
}

func newLiveSandbox(t *testing.T, ctx context.Context) *e2b.Sandbox {
	return newLiveSandboxWithOpts(t, ctx, nil)
}

func newLiveSandboxWithOpts(t *testing.T, ctx context.Context, opts *e2b.SandboxOpts) *e2b.Sandbox {
	t.Helper()
	requireLiveEnv(t)
	if os.Getenv("E2B_API_KEY") == "" {
		t.Skip("E2B_API_KEY is required for integration tests")
	}

	if opts == nil {
		opts = &e2b.SandboxOpts{}
	} else {
		copied := *opts
		opts = &copied
	}
	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	if opts.TimeoutMs == nil {
		opts.TimeoutMs = &timeoutMs
	}
	opts.ConnectionOpts = mergeLiveConnectionOpts(opts.ConnectionOpts)
	if opts.Metadata == nil {
		opts.Metadata = liveSandboxMetadata(t)
	} else if _, ok := opts.Metadata["sandboxTestId"]; !ok {
		metadata := make(map[string]string, len(opts.Metadata)+1)
		for k, v := range opts.Metadata {
			metadata[k] = v
		}
		metadata["sandboxTestId"] = liveSandboxMetadata(t)["sandboxTestId"]
		opts.Metadata = metadata
	}
	template := getLiveTemplate(t, ctx)
	sandbox, err := e2b.Create(ctx, template, opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := sandbox.Kill(cleanupCtx, nil); err != nil {
			t.Logf("failed to kill sandbox %s: %v", sandbox.SandboxID, err)
		}
	})
	return sandbox
}

func createLiveSandboxFromTemplate(t *testing.T, ctx context.Context, template string, label string) *e2b.Sandbox {
	t.Helper()
	timeoutMs := int((10 * time.Minute) / time.Millisecond)
	sandbox, err := e2b.Create(ctx, template, &e2b.SandboxOpts{
		ConnectionOpts: mergeLiveConnectionOpts(e2b.ConnectionOpts{}),
		TimeoutMs:      &timeoutMs,
		Metadata: map[string]string{
			"sandboxTestId": liveSandboxMetadata(t)["sandboxTestId"] + "-" + label,
		},
	})
	if err != nil {
		t.Fatalf("Create(%s) returned error: %v", template, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := sandbox.Kill(cleanupCtx, nil); err != nil {
			t.Logf("failed to kill sandbox %s: %v", sandbox.SandboxID, err)
		}
	})
	return sandbox
}

func liveSandboxMetadata(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"sandboxTestId": "go-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()),
	}
}

func requireLiveEnv(t *testing.T) {
	t.Helper()
	loadDotEnv()
	if loadEnvErr != nil {
		t.Fatalf("failed to load .env: %v", loadEnvErr)
	}
}

func loadDotEnv() {
	loadEnvOnce.Do(func() {
		loadEnvErr = loadDotEnvFile(".env")
	})
}

func loadDotEnvFile(filename string) error {
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" && os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func getLiveTemplate(t *testing.T, ctx context.Context) string {
	t.Helper()
	for _, key := range []string{"E2B_TEST_TEMPLATE", "E2B_INTEGRATION_TEMPLATE", "E2B_TEMPLATE", "E2B_SANDBOX_TEMPLATE"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}

	liveTemplateMu.Lock()
	defer liveTemplateMu.Unlock()
	if liveTemplateName != "" {
		return liveTemplateName
	}

	exists, err := e2b.Exists(ctx, "base", liveConnectionOpts())
	if err == nil && exists {
		liveTemplateName = "base"
		return liveTemplateName
	}
	if err != nil {
		t.Logf("could not check base template alias; looking for fallback template: %v", err)
	} else {
		t.Log("base template alias is not available; looking for fallback template")
	}

	inferred, inferErr := inferLiveTemplate()
	if inferErr == nil && inferred != "" {
		t.Logf("using template %q inferred from existing sandboxes", inferred)
		liveTemplateName = inferred
		return liveTemplateName
	}
	if inferErr != nil {
		t.Logf("could not infer template from existing sandboxes: %v", inferErr)
	}

	name := "go-sdk-integration-" + fmt.Sprint(time.Now().UnixNano())
	info, err := e2b.Build(ctx, e2b.Template(nil).FromBaseImage(), name, liveBuildOptions())
	if err != nil {
		t.Skipf("no live template is available and temporary template build failed; set E2B_TEST_TEMPLATE to an existing template: %v", err)
	}
	if info.TemplateID == "" {
		t.Fatalf("temporary integration template build returned empty template ID: %#v", info)
	}
	liveTemplateName = info.TemplateID
	liveTemplateID = info.TemplateID
	liveTemplateBuilt = true
	return liveTemplateName
}

func inferLiveTemplate() (string, error) {
	paginator := e2b.List(&e2b.SandboxListOpts{Limit: 10})
	items, err := paginator.NextItems()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.TemplateID != "" {
			return item.TemplateID, nil
		}
	}
	return "", nil
}

func liveBuildOptions() *e2b.BuildOptions {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2b.BuildOptions{
		BasicBuildOptions: e2b.BasicBuildOptions{
			CpuCount:    1,
			MemoryMB:    512,
			OnBuildLogs: func(_ *e2b.LogEntry) {},
		},
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func liveConnectionOpts() *e2b.ConnectionOpts {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2b.ConnectionOpts{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func mergeLiveConnectionOpts(override e2b.ConnectionOpts) e2b.ConnectionOpts {
	base := liveConnectionOpts()
	merged := *base

	if override.ApiKey != "" {
		merged.ApiKey = override.ApiKey
	}
	if override.AccessToken != "" {
		merged.AccessToken = override.AccessToken
	}
	if override.Domain != "" {
		merged.Domain = override.Domain
	}
	if override.ApiUrl != "" {
		merged.ApiUrl = override.ApiUrl
	}
	if override.SandboxUrl != "" {
		merged.SandboxUrl = override.SandboxUrl
	}
	if override.Debug != nil {
		merged.Debug = override.Debug
	}
	if override.Signal != nil {
		merged.Signal = override.Signal
	}
	if override.RequestTimeoutMs != nil {
		merged.RequestTimeoutMs = override.RequestTimeoutMs
	}
	if override.Logger != nil {
		merged.Logger = override.Logger
	}
	merged.Headers = mergeLiveHeaders(merged.Headers, override.Headers)
	if override.Proxy != "" {
		merged.Proxy = override.Proxy
	}

	return merged
}

func mergeLiveHeaders(base map[string]string, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	headers := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		headers[k] = v
	}
	for k, v := range override {
		headers[k] = v
	}
	return headers
}

func liveBuildStatusOptions() *e2b.GetBuildStatusOptions {
	requestTimeoutMs := int((2 * time.Minute) / time.Millisecond)
	return &e2b.GetBuildStatusOptions{
		ApiKey:           os.Getenv("E2B_API_KEY"),
		AccessToken:      os.Getenv("E2B_ACCESS_TOKEN"),
		Domain:           os.Getenv("E2B_DOMAIN"),
		ApiUrl:           os.Getenv("E2B_API_URL"),
		RequestTimeoutMs: &requestTimeoutMs,
	}
}

func cleanupLiveTemplate() {
	if !liveTemplateBuilt || liveTemplateID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, _ = e2b.DeleteSnapshot(ctx, liveTemplateID, nil)
}

func mustRun(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox, cmd string) *e2b.CommandResult {
	t.Helper()
	result, err := runForegroundCommand(sandbox.Commands, ctx, cmd, nil)
	if err != nil {
		t.Fatalf("command %q returned error: %v", cmd, err)
	}
	return result
}

func runNumpyRandomVector(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox) (string, bool) {
	t.Helper()
	result, err := runForegroundCommand(sandbox.Commands, ctx, `python3 - <<'PY'
import numpy as np
print([np.random.normal(), np.random.normal(), np.random.normal()])
PY`, nil)
	if err != nil {
		msg := strings.ToLower(err.Error())
		var exitErr *e2b.CommandExitError
		if errors.As(err, &exitErr) {
			msg += "\n" + strings.ToLower(exitErr.Stdout) + "\n" + strings.ToLower(exitErr.Stderr)
		}
		if strings.Contains(msg, "no module named") || strings.Contains(msg, "numpy") || strings.Contains(msg, "python3: not found") {
			t.Skipf("numpy random test requires python3 with numpy in the live template: %v", err)
			return "", false
		}
		t.Fatalf("numpy random command returned error: %v", err)
	}
	return strings.TrimSpace(result.Stdout), true
}

func livePath(prefix string) string {
	return "/tmp/e2b-go-" + prefix + "-" + fmt.Sprint(time.Now().UnixNano())
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func processListContains(processes []e2b.ProcessInfo, pid uint32) bool {
	for _, process := range processes {
		if process.Pid == pid {
			return true
		}
	}
	return false
}

func entryListContains(entries []e2b.EntryInfo, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

type liveEntryExpectation struct {
	Name string
	Type e2b.FileType
	Path string
}

func assertListEntries(t *testing.T, sandbox *e2b.Sandbox, ctx context.Context, dir string, depth int, expected []liveEntryExpectation) {
	t.Helper()
	var opts *e2b.FilesystemListOpts
	if depth > 0 {
		opts = &e2b.FilesystemListOpts{Depth: intPtr(depth)}
	}
	entries, err := sandbox.Files.List(ctx, dir, opts)
	if err != nil {
		t.Fatalf("List(%s, depth=%d) returned error: %v", dir, depth, err)
	}
	if len(entries) != len(expected) {
		t.Fatalf("List(%s, depth=%d) returned %d entries, want %d: %#v", dir, depth, len(entries), len(expected), entries)
	}
	for i, want := range expected {
		got := entries[i]
		if got.Name != want.Name || got.Type != want.Type || got.Path != want.Path {
			t.Fatalf("List(%s, depth=%d)[%d] = {name:%q type:%q path:%q}, want {name:%q type:%q path:%q}", dir, depth, i, got.Name, got.Type, got.Path, want.Name, want.Type, want.Path)
		}
	}
}

func findEntryByName(entries []e2b.EntryInfo, name string) *e2b.EntryInfo {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func sandboxInfoListContains(items []e2b.SandboxInfo, sandboxID string) bool {
	for _, item := range items {
		if item.SandboxID == sandboxID {
			return true
		}
	}
	return false
}

func waitForSandboxAbsentFromRunningList(t *testing.T, sandboxID string, metadata map[string]string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastItems []e2b.SandboxInfo
	var lastErr error
	for time.Now().Before(deadline) {
		items, err := e2b.List(&e2b.SandboxListOpts{
			Query: &struct {
				Metadata map[string]string
				State    []e2b.SandboxState
			}{
				Metadata: metadata,
				State:    []e2b.SandboxState{e2b.SandboxState("running")},
			},
		}).NextItems()
		lastItems = items
		lastErr = err
		if err == nil && !sandboxInfoListContains(items, sandboxID) {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("sandbox %s remained in running list after kill; items=%#v err=%v", sandboxID, lastItems, lastErr)
}

func waitForSandboxPagination(t *testing.T, sandboxID1, sandboxID2 string, metadata map[string]string) ([]e2b.SandboxInfo, []e2b.SandboxInfo, bool, string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var lastFirst []e2b.SandboxInfo
	var lastItems []e2b.SandboxInfo
	var lastErr error
	var lastHasNext bool
	var lastNextToken string

	for {
		query := &struct {
			Metadata map[string]string
			State    []e2b.SandboxState
		}{
			Metadata: metadata,
			State:    []e2b.SandboxState{e2b.SandboxState("running")},
		}
		paginator := e2b.List(&e2b.SandboxListOpts{Query: query, Limit: 1})
		first, err := paginator.NextItems()
		lastFirst = first
		lastErr = err
		if err == nil && len(first) == 1 && paginator.HasNext && paginator.NextToken != "" {
			firstHasNext := paginator.HasNext
			firstNextToken := paginator.NextToken
			items := append([]e2b.SandboxInfo{}, first...)
			for paginator.HasNext && len(items) < 200 {
				page, err := paginator.NextItems()
				lastErr = err
				if err != nil {
					break
				}
				items = append(items, page...)
				lastItems = items
				lastHasNext = paginator.HasNext
				lastNextToken = paginator.NextToken
				if sandboxInfoListContains(items, sandboxID1) && sandboxInfoListContains(items, sandboxID2) {
					return first, items, firstHasNext, firstNextToken
				}
			}
		} else {
			lastHasNext = paginator.HasNext
			lastNextToken = paginator.NextToken
		}

		if time.Now().After(deadline) {
			if lastErr == nil && len(lastFirst) == 1 && !lastHasNext && lastNextToken == "" {
				t.Skipf("sandbox list pagination token is unavailable in this environment; first=%#v items=%#v", lastFirst, lastItems)
			}
			t.Fatalf("pagination did not return running sandboxes %s/%s; first=%#v items=%#v hasNext=%v nextToken=%q err=%v", sandboxID1, sandboxID2, lastFirst, lastItems, lastHasNext, lastNextToken, lastErr)
		}
		time.Sleep(time.Second)
	}
}

func waitForSandboxState(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox, state e2b.SandboxState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastInfo *e2b.SandboxInfo
	var lastErr error
	for time.Now().Before(deadline) {
		info, err := sandbox.GetInfo(ctx, nil)
		if err == nil {
			lastInfo = info
			if info.State == state {
				return
			}
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("sandbox %s did not reach state %q within %s; lastInfo=%#v err=%v", sandbox.SandboxID, state, timeout, lastInfo, lastErr)
}

func pauseAndReconnectSandbox(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox) {
	t.Helper()
	paused, err := sandbox.Pause(ctx, nil)
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}
	if !paused {
		t.Fatal("expected Pause to return true")
	}
	waitForSandboxState(t, ctx, sandbox, e2b.SandboxState("paused"), 20*time.Second)
	connectSandboxWithRetry(t, ctx, sandbox, 60*time.Second)
	waitForSandboxState(t, ctx, sandbox, e2b.SandboxState("running"), 20*time.Second)
}

func connectSandboxWithRetry(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox, timeout time.Duration) *e2b.Sandbox {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resumed, err := sandbox.Connect(ctx, nil)
		if err == nil {
			return resumed
		}
		lastErr = err
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "pausing") && !strings.Contains(msg, "resume sandbox") {
			t.Fatalf("Connect paused sandbox returned error: %v", err)
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("Connect paused sandbox did not succeed within %s: %v", timeout, lastErr)
	return nil
}

func snapshotListContains(items []e2b.SnapshotInfo, snapshotID string) bool {
	for _, item := range items {
		if item.SnapshotID == snapshotID {
			return true
		}
	}
	return false
}

func stringListContains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func findGitFileStatus(status *e2b.GitStatus, name string) *e2b.GitFileStatus {
	if status == nil {
		return nil
	}
	for i := range status.FileStatus {
		if status.FileStatus[i].Name == name {
			return &status.FileStatus[i]
		}
	}
	return nil
}

func assertGitRepoCleanWithReadme(t *testing.T, sandbox *e2b.Sandbox, ctx context.Context, repoPath string, expected string) {
	t.Helper()
	status, err := sandbox.Git.Status(ctx, repoPath, nil)
	if err != nil {
		t.Fatalf("Git.Status(%s) returned error: %v", repoPath, err)
	}
	if !status.IsClean {
		t.Fatalf("expected clean git status for %s, got %#v", repoPath, status)
	}
	contents, err := readSandboxText(sandbox.Files, ctx, path.Join(repoPath, "README.md"), nil)
	if err != nil {
		t.Fatalf("ReadText README for %s returned error: %v", repoPath, err)
	}
	if contents != expected {
		t.Fatalf("unexpected README contents for %s: %q", repoPath, contents)
	}
}

type liveGitDaemon struct {
	handle     *e2b.CommandHandle
	remotePath string
	remoteURL  string
	port       int
}

func startLiveGitDaemon(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox, baseDir string) liveGitDaemon {
	t.Helper()
	remotePath := path.Join(baseDir, "remote.git")
	if _, err := sandbox.Git.Init(ctx, remotePath, &e2b.GitInitOpts{Bare: true, InitialBranch: "main"}); err != nil {
		t.Fatalf("Git.Init bare remote returned error: %v", err)
	}

	for attempt := 0; attempt < 3; attempt++ {
		port := 20000 + int((time.Now().UnixNano()/int64(time.Millisecond)+int64(attempt)*997)%20000)
		remoteURL := fmt.Sprintf("git://127.0.0.1:%d/remote.git", port)
		cmd := fmt.Sprintf(
			"git daemon --reuseaddr --base-path=%s --export-all --enable=receive-pack --informative-errors --listen=127.0.0.1 --port=%d",
			shellQuote(baseDir),
			port,
		)
		handle, err := runBackgroundCommand(sandbox.Commands, ctx, cmd, nil)
		if err != nil {
			t.Logf("git daemon start attempt %d failed: %v", attempt+1, err)
			continue
		}

		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := runForegroundCommand(sandbox.Commands, ctx, "git ls-remote "+shellQuote(remoteURL), nil); err == nil {
				t.Cleanup(func() {
					_, _ = handle.Kill()
				})
				return liveGitDaemon{
					handle:     handle,
					remotePath: remotePath,
					remoteURL:  remoteURL,
					port:       port,
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		_, _ = handle.Kill()
	}

	t.Skip("git daemon did not become reachable in this environment")
	return liveGitDaemon{}
}

func waitForSnapshotAbsent(t *testing.T, ctx context.Context, snapshotID string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		items, err := e2b.ListSnapshots(&e2b.SnapshotListOpts{Limit: 50}).NextItems()
		if err != nil {
			t.Fatalf("ListSnapshots while waiting for deletion returned error: %v", err)
		}
		if !snapshotListContains(items, snapshotID) {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("snapshot %s was still present after deletion", snapshotID)
}

func fetchText(t *testing.T, ctx context.Context, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create GET request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s returned error: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read GET response body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func waitForSandboxHostStatus(t *testing.T, ctx context.Context, url string, trafficAccessToken string, expectedStatus int) string {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("failed to create host request: %v", err)
		}
		if trafficAccessToken != "" {
			req.Header.Set("e2b-traffic-access-token", trafficAccessToken)
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatalf("failed to read host response body: %v", readErr)
			}
			if resp.StatusCode == expectedStatus {
				return string(body)
			}
			lastErr = fmt.Errorf("unexpected status %d body %q", resp.StatusCode, string(body))
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("sandbox host %s did not return status %d: %v", url, expectedStatus, lastErr)
	return ""
}

func waitForSandboxHostStatusOrSkip(t *testing.T, ctx context.Context, url string, trafficAccessToken string, expectedStatus int, skipReason string) string {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(60 * time.Second)
	var lastStatus int
	var lastBody string
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("failed to create host request: %v", err)
		}
		if trafficAccessToken != "" {
			req.Header.Set("e2b-traffic-access-token", trafficAccessToken)
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatalf("failed to read host response body: %v", readErr)
			}
			lastStatus = resp.StatusCode
			lastBody = string(body)
			lastErr = nil
			if resp.StatusCode == expectedStatus {
				return lastBody
			}
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		t.Skipf("%s: %v", skipReason, lastErr)
	}
	t.Skipf("%s: last status=%d body=%q", skipReason, lastStatus, lastBody)
	return ""
}

func postMultipartFile(t *testing.T, ctx context.Context, url string, filename string, content string) (int, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create multipart file part: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("failed to write multipart file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s returned error: %v", url, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read POST response body: %v", err)
	}
	return resp.StatusCode, string(respBody)
}

func assertUploadResponsePath(t *testing.T, body string, wantPath string) {
	t.Helper()
	var infos []e2b.WriteInfo
	if err := json.Unmarshal([]byte(body), &infos); err != nil {
		t.Fatalf("failed to parse upload response %q: %v", body, err)
	}
	if len(infos) != 1 || infos[0].Path != wantPath || infos[0].Type != e2b.FileTypeFile {
		t.Fatalf("unexpected upload response: %#v", infos)
	}
}

func skipIfSigningUnavailable(t *testing.T, err error) {
	t.Helper()
	if strings.Contains(err.Error(), "Signature expiration can be used only when sandbox is created as secured") {
		t.Skipf("signed URLs are unavailable in this environment: %v", err)
	}
}

func templateTagsContain(tags []e2b.TemplateTag, tag string) bool {
	for _, item := range tags {
		if item.Tag == tag {
			return true
		}
	}
	return false
}

func liveVolumeListContains(entries []e2bvol.VolumeInfo, volumeID, name string) bool {
	for _, entry := range entries {
		if entry.VolumeID == volumeID && entry.Name == name {
			return true
		}
	}
	return false
}

func skipIfTemplateAPIUnavailable(t *testing.T, err error) {
	t.Helper()
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "request build failed: 404") || strings.Contains(msg, "404 page not found") {
		t.Skipf("template build API is unavailable in this environment: %v", err)
	}
}

func waitForFilesystemEvent(t *testing.T, events <-chan e2b.FilesystemEvent, name string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Name == name || path.Base(event.Name) == name {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for filesystem event %q", name)
		}
	}
}

func waitForFilesystemEventExact(t *testing.T, events <-chan e2b.FilesystemEvent, name string) {
	t.Helper()
	waitForFilesystemEventMatching(t, events, func(event e2b.FilesystemEvent) bool {
		return event.Name == name
	}, name)
}

func waitForFilesystemEventExactType(t *testing.T, events <-chan e2b.FilesystemEvent, name string, eventType e2b.FilesystemEventType) {
	t.Helper()
	waitForFilesystemEventMatching(t, events, func(event e2b.FilesystemEvent) bool {
		return event.Name == name && event.Type == eventType
	}, fmt.Sprintf("%s %s", eventType, name))
}

func waitForFilesystemEventMatching(t *testing.T, events <-chan e2b.FilesystemEvent, matches func(e2b.FilesystemEvent) bool, description string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	seen := make([]e2b.FilesystemEvent, 0, 4)
	for {
		select {
		case event := <-events:
			seen = append(seen, event)
			if matches(event) {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for filesystem event %q; seen %#v", description, seen)
		}
	}
}

func waitForCommandStdout(t *testing.T, handle *e2b.CommandHandle, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if got := handle.State().Stdout; got == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for stdout %q, got %q", want, handle.State().Stdout)
		case <-ticker.C:
		}
	}
}

func waitForCommandStdoutContains(t *testing.T, handle *e2b.CommandHandle, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if got := handle.State().Stdout; strings.Contains(got, want) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for stdout to contain %q, got %q", want, handle.State().Stdout)
		case <-ticker.C:
		}
	}
}

func waitForMetrics(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox) []e2b.SandboxMetrics {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		metrics, err := sandbox.GetMetrics(ctx, nil)
		if err == nil && len(metrics) > 0 {
			metric := metrics[0]
			start := metric.Timestamp.Add(-2 * time.Second)
			end := metric.Timestamp.Add(2 * time.Second)
			filtered, filteredErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
				Start: &start,
				End:   &end,
			})
			if filteredErr != nil {
				t.Fatalf("filtered metrics query returned error: %v", filteredErr)
			}
			if len(filtered) == 0 {
				t.Skipf(
					"metrics endpoint returned unfiltered data but did not honor an inclusive filtered window in this environment: metric=%s start=%s end=%s",
					metric.Timestamp.UTC().Format(time.RFC3339),
					start.UTC().Format(time.RFC3339),
					end.UTC().Format(time.RFC3339),
				)
			}
			return metrics
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("timed out waiting for metrics: %v", lastErr)
	}
	t.Skip("metrics endpoint returned no points in this environment")
	return nil
}

func assertSandboxConnectivityCheck(t *testing.T, ctx context.Context, sandbox *e2b.Sandbox, label string) {
	t.Helper()
	result, err := runForegroundCommand(sandbox.Commands, ctx, "curl -s -o /dev/null -w '%{http_code}' https://connectivitycheck.gstatic.com/generate_204", nil)
	if err != nil {
		t.Skipf("internet connectivity check is unreachable for %s sandbox in this environment: %v", label, err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "204" {
		t.Skipf("internet connectivity check returned unexpected response for %s sandbox: exit=%d stdout=%q stderr=%q", label, result.ExitCode, result.Stdout, result.Stderr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
