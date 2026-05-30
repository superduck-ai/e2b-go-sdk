package commands

import (
	"errors"
	"reflect"
	"testing"
)

func TestCommandHandleWaitReturnsCommandExitErrorWithProcessMessage(t *testing.T) {
	handle := newCommandHandle(123, func() {}, func() (bool, error) { return true, nil }, nil, nil)
	handle.appendStdout("hello")
	handle.appendStderr("boom")
	handle.setEnd(7, "process failed")

	_, err := handle.Wait()
	if err == nil {
		t.Fatal("expected non-zero exit to return error")
	}

	var exitErr *CommandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected CommandExitError, got %T %v", err, err)
	}
	if exitErr.Error() != "process failed" {
		t.Fatalf("expected error message to use process error, got %q", exitErr.Error())
	}
	if exitErr.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", exitErr.ExitCode)
	}
	if exitErr.Stdout != "hello" || exitErr.Stderr != "boom" {
		t.Fatalf("expected stdout/stderr to be preserved, got %#v", exitErr.CommandResult)
	}
	if exitErr.CommandResult.Error != "process failed" {
		t.Fatalf("expected result error field to be preserved, got %q", exitErr.CommandResult.Error)
	}
}

func TestCommandHandleStateSnapshotsLiveOutputAndCopiesExitCode(t *testing.T) {
	handle := newCommandHandle(123, func() {}, func() (bool, error) { return true, nil }, nil, nil)
	handle.appendStdout("hello")
	handle.appendStderr("boom")

	running := handle.State()
	if handle.Pid != 123 {
		t.Fatalf("expected handle pid 123, got %d", handle.Pid)
	}
	if running.Stdout != "hello" || running.Stderr != "boom" {
		t.Fatalf("unexpected running snapshot: %#v", running)
	}
	if _, ok := reflect.TypeOf(running).FieldByName("Pid"); ok {
		t.Fatal("expected state snapshot to omit pid; pid lives on CommandHandle")
	}
	if running.ExitCode != nil {
		t.Fatalf("expected running snapshot exit code to be nil, got %v", *running.ExitCode)
	}

	handle.setEnd(7, "process failed")
	finished := handle.State()
	if finished.Error != "process failed" {
		t.Fatalf("expected error snapshot to be preserved, got %q", finished.Error)
	}
	if finished.ExitCode == nil || *finished.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %#v", finished.ExitCode)
	}

	*finished.ExitCode = 999
	again := handle.State()
	if again.ExitCode == nil || *again.ExitCode != 7 {
		t.Fatalf("expected fresh state snapshot to preserve exit code 7, got %#v", again.ExitCode)
	}
}
