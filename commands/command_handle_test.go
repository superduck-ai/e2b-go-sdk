package commands

import (
	"errors"
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
