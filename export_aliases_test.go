package e2b

import (
	rootcmd "github.com/e2b-dev/e2b-go-sdk/commands"
	rootfs "github.com/e2b-dev/e2b-go-sdk/filesystem"
	rootgit "github.com/e2b-dev/e2b-go-sdk/git"
	"testing"
)

func TestRootAliasesExposeJsStyleFilesystemCommandAndGitTypes(t *testing.T) {
	if FileTypeFile != rootfs.FileTypeFile || FileTypeDir != rootfs.FileTypeDir {
		t.Fatalf("expected root file type constants to match filesystem package exports")
	}
	if FilesystemEventChmod != rootfs.FilesystemEventChmod ||
		FilesystemEventCreate != rootfs.FilesystemEventCreate ||
		FilesystemEventRemove != rootfs.FilesystemEventRemove ||
		FilesystemEventRename != rootfs.FilesystemEventRename ||
		FilesystemEventWrite != rootfs.FilesystemEventWrite {
		t.Fatalf("expected root filesystem event constants to match filesystem package exports")
	}

	var _ FileType = rootfs.FileTypeFile
	var _ WriteInfo = rootfs.WriteInfo{}
	var _ EntryInfo = rootfs.EntryInfo{}
	var _ *Filesystem = (*rootfs.Filesystem)(nil)
	var _ FilesystemRequestOpts = rootfs.FilesystemRequestOpts{}
	var _ FilesystemWriteOpts = rootfs.FilesystemWriteOpts{}
	var _ FilesystemReadOpts = rootfs.FilesystemReadOpts{}
	var _ FilesystemListOpts = rootfs.FilesystemListOpts{}
	var _ WatchOpts = rootfs.WatchOpts{}
	var _ WriteEntry = rootfs.WriteEntry{}
	var _ FilesystemEventType = rootfs.FilesystemEventCreate
	var _ FilesystemEvent = rootfs.FilesystemEvent{}
	var _ *WatchHandle = (*rootfs.WatchHandle)(nil)

	var _ *CommandExitError = (*rootcmd.CommandExitError)(nil)
	var _ CommandResult = rootcmd.CommandResult{}
	var _ Stdout = rootcmd.Stdout("stdout")
	var _ Stderr = rootcmd.Stderr("stderr")
	var _ PtyOutput = rootcmd.PtyOutput([]byte("pty"))
	var _ *CommandHandle = (*rootcmd.CommandHandle)(nil)
	var _ ProcessInfo = rootcmd.ProcessInfo{}
	var _ CommandRequestOpts = rootcmd.CommandRequestOpts{}
	var _ CommandConnectOpts = rootcmd.CommandConnectOpts{}
	var _ CommandStartOpts = rootcmd.CommandStartOpts{}
	var _ *Commands = (*rootcmd.Commands)(nil)
	var _ *Pty = (*rootcmd.Pty)(nil)
	var _ PtyCreateOpts = rootcmd.PtyCreateOpts{}
	var _ PtyConnectOpts = rootcmd.PtyConnectOpts{}

	var _ *Git = (*rootgit.Git)(nil)
	var _ GitRequestOpts = rootgit.GitRequestOpts{}
	var _ GitCloneOpts = rootgit.GitCloneOpts{}
	var _ GitInitOpts = rootgit.GitInitOpts{}
	var _ GitRemoteAddOpts = rootgit.GitRemoteAddOpts{}
	var _ GitCommitOpts = rootgit.GitCommitOpts{}
	var _ GitAddOpts = rootgit.GitAddOpts{}
	var _ GitDeleteBranchOpts = rootgit.GitDeleteBranchOpts{}
	var _ GitPushOpts = rootgit.GitPushOpts{}
	var _ GitPullOpts = rootgit.GitPullOpts{}
	var _ GitResetOpts = rootgit.GitResetOpts{}
	var _ GitRestoreOpts = rootgit.GitRestoreOpts{}
	var _ GitResetMode = rootgit.GitResetHard
	var _ GitDangerouslyAuthenticateOpts = rootgit.GitDangerouslyAuthenticateOpts{}
	var _ GitConfigOpts = rootgit.GitConfigOpts{}
	var _ GitConfigScope = rootgit.GitConfigLocal
	var _ GitBranches = rootgit.GitBranches{}
	var _ GitFileStatus = rootgit.GitFileStatus{}
	var _ GitStatus = rootgit.GitStatus{}

	// This is a compile-time surface check; runtime assertions are unnecessary.
}
