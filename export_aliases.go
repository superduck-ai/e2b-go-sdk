package e2b

import (
	rootcmd "github.com/e2b-dev/e2b-go-sdk/commands"
	rootfs "github.com/e2b-dev/e2b-go-sdk/filesystem"
	rootgit "github.com/e2b-dev/e2b-go-sdk/git"
)

const (
	FileTypeFile = rootfs.FileTypeFile
	FileTypeDir  = rootfs.FileTypeDir
)

const (
	FilesystemEventChmod  = rootfs.FilesystemEventChmod
	FilesystemEventCreate = rootfs.FilesystemEventCreate
	FilesystemEventRemove = rootfs.FilesystemEventRemove
	FilesystemEventRename = rootfs.FilesystemEventRename
	FilesystemEventWrite  = rootfs.FilesystemEventWrite
)

type FileType = rootfs.FileType
type WriteInfo = rootfs.WriteInfo
type EntryInfo = rootfs.EntryInfo
type Filesystem = rootfs.Filesystem
type FilesystemWriteOpts = rootfs.FilesystemWriteOpts
type FilesystemReadOpts = rootfs.FilesystemReadOpts
type FilesystemEventType = rootfs.FilesystemEventType
type FilesystemEvent = rootfs.FilesystemEvent
type WatchHandle = rootfs.WatchHandle

type CommandExitError = rootcmd.CommandExitError
type CommandResult = rootcmd.CommandResult
type Stdout = rootcmd.Stdout
type Stderr = rootcmd.Stderr
type PtyOutput = rootcmd.PtyOutput
type CommandHandle = rootcmd.CommandHandle
type ProcessInfo = rootcmd.ProcessInfo
type CommandRequestOpts = rootcmd.CommandRequestOpts
type CommandConnectOpts = rootcmd.CommandConnectOpts
type CommandStartOpts = rootcmd.CommandStartOpts
type Commands = rootcmd.Commands
type Pty = rootcmd.Pty

type Git = rootgit.Git
type GitRequestOpts = rootgit.GitRequestOpts
type GitCloneOpts = rootgit.GitCloneOpts
type GitInitOpts = rootgit.GitInitOpts
type GitRemoteAddOpts = rootgit.GitRemoteAddOpts
type GitCommitOpts = rootgit.GitCommitOpts
type GitAddOpts = rootgit.GitAddOpts
type GitDeleteBranchOpts = rootgit.GitDeleteBranchOpts
type GitPushOpts = rootgit.GitPushOpts
type GitPullOpts = rootgit.GitPullOpts
type GitDangerouslyAuthenticateOpts = rootgit.GitDangerouslyAuthenticateOpts
type GitConfigOpts = rootgit.GitConfigOpts
type GitConfigScope = rootgit.GitConfigScope
type GitBranches = rootgit.GitBranches
type GitFileStatus = rootgit.GitFileStatus
type GitStatus = rootgit.GitStatus
