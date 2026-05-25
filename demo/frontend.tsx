import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type CmdResult = {
  command: string;
  stdout: string;
  stderr: string;
  exitCode: number;
  error?: string;
};


type LifecycleEvent = {
  version: string;
  id: string;
  type: string;
  eventData: Record<string, any>;
  sandboxId?: string;
  sandboxExecutionId?: string;
  timestamp: string;
};

type Status = "idle" | "creating" | "running" | "executing" | "killing" | "killed" | "pausing" | "paused" | "resuming" | "error";

type FileEntry = {
  name: string;
  path: string;
  type: string;
  size: number;
  permissions: string;
};

/** Snapshot list row — aligned with E2B Sandbox snapshots API. */
type SnapshotRow = {
  snapshotId: string;
  names?: string[];
  templateId?: string;
  createdAt?: string;
  sandboxId?: string;
};

type VolumeRow = { volumeId: string; name: string };

type SandboxState = {
  sandboxId: string;
  startedAt: string;
  endAt?: string;
  templateId?: string;
  name?: string;
  cpuCount?: number;
  memoryMB?: number;
  metadata?: Record<string, string>;
  /** From GET /api/sandboxes `state` (E2B sandbox.state). */
  remoteState?: string;
  aliases?: { alias: string; namespace: string }[];
  status: Status;
  host?: string;
  hostPort?: number;
  results: CmdResult[];
};

const STATUS_STYLE: Record<Status, { label: string; cls: string; dot: string }> = {
  idle:      { label: "未创建",   cls: "bg-slate-700 text-slate-200",        dot: "bg-slate-400" },
  creating:  { label: "创建中",   cls: "bg-amber-700/40 text-amber-200",     dot: "bg-amber-400 animate-pulse" },
  running:   { label: "运行中",   cls: "bg-emerald-700/40 text-emerald-200", dot: "bg-emerald-400" },
  executing: { label: "执行命令", cls: "bg-blue-700/40 text-blue-200",       dot: "bg-blue-400 animate-pulse" },
  killing:   { label: "销毁中",   cls: "bg-orange-700/40 text-orange-200",   dot: "bg-orange-400 animate-pulse" },
  killed:    { label: "已销毁",   cls: "bg-slate-700 text-slate-300",        dot: "bg-slate-500" },
  pausing:   { label: "暂停中",   cls: "bg-yellow-700/40 text-yellow-200",   dot: "bg-yellow-400 animate-pulse" },
  paused:    { label: "已暂停",   cls: "bg-yellow-700/40 text-yellow-200",   dot: "bg-yellow-400" },
  resuming:  { label: "恢复中",   cls: "bg-cyan-700/40 text-cyan-200",       dot: "bg-cyan-400 animate-pulse" },
  error:     { label: "错误",     cls: "bg-red-800/50 text-red-200",         dot: "bg-red-400" },
};

const EVENT_COLOR: Record<string, string> = {
  "sandbox.lifecycle.created":      "text-emerald-300",
  "sandbox.lifecycle.killed":       "text-orange-300",
  "sandbox.lifecycle.paused":       "text-yellow-300",
  "sandbox.lifecycle.resumed":      "text-emerald-300",
  "sandbox.lifecycle.updated":      "text-sky-300",
  "sandbox.lifecycle.checkpointed": "text-purple-300",
  "sandbox.command.started":        "text-blue-300",
  "sandbox.command.completed":      "text-blue-200",
  "sandbox.error":                  "text-red-300",
};

function App() {
  const [sandboxes, setSandboxes] = useState<Record<string, SandboxState>>({});
  const [activeId, setActiveId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [running, setRunning] = useState(false);
  const [command, setCommand] = useState('echo "Hello from E2B!"');

  const [err, setErr] = useState<string | null>(null);
  const errTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const setErrAuto = (msg: string | null) => {
    if (errTimer.current) clearTimeout(errTimer.current);
    setErr(msg);
    if (msg) {
      errTimer.current = setTimeout(() => setErrAuto(null), 5000);
    }
  };
  const [events, setEvents] = useState<LifecycleEvent[]>([]);
  const [now, setNow] = useState(Date.now());
  // File browser state
  const [filePath, setFilePath] = useState("/");
  const [fileEntries, setFileEntries] = useState<FileEntry[]>([]);
  const [fileContent, setFileContent] = useState<{ path: string; content: string } | null>(null);
  const [fileEditorText, setFileEditorText] = useState("");
  const [savingFile, setSavingFile] = useState(false);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [quickWritePath, setQuickWritePath] = useState("/tmp/e2b-fs-test.txt");
  const [quickWriteBody, setQuickWriteBody] = useState("hello from e2b files.write()");
  // files.getInfo() — https://e2b.dev/docs/filesystem/info
  const [infoPath, setInfoPath] = useState("/home/user");
  const [infoResult, setInfoResult] = useState<{ path: string; info: any } | null>(null);
  const [infoError, setInfoError] = useState<string | null>(null);
  const [infoLoading, setInfoLoading] = useState(false);
  // Right panel tab
  const [rightTab, setRightTab] = useState<"events" | "files" | "fileops" | "snapshots" | "volumes">("events");
  // Snapshots — https://e2b.dev/docs/sandbox/snapshots
  const [snapshots, setSnapshots] = useState<SnapshotRow[]>([]);
  const [snapshotting, setSnapshotting] = useState(false);
  const [snapshotListFilter, setSnapshotListFilter] = useState<"all" | "current">("all");
  const [snapshotHighlightId, setSnapshotHighlightId] = useState<string | null>(null);
  const [snapshotCopiedId, setSnapshotCopiedId] = useState<string | null>(null);
  // Volumes — https://e2b.dev/docs/volumes/manage
  const [volumes, setVolumes] = useState<VolumeRow[]>([]);
  const [volumesLoading, setVolumesLoading] = useState(false);
  const [volumesError, setVolumesError] = useState<string | null>(null);
  const [newVolumeName, setNewVolumeName] = useState("");
  const [creatingVolume, setCreatingVolume] = useState(false);
  const [volumeInfoId, setVolumeInfoId] = useState("");
  const [volumeInfoResult, setVolumeInfoResult] = useState<VolumeRow | null>(null);
  const [volumeInfoError, setVolumeInfoError] = useState<string | null>(null);
  const [volumeInfoLoading, setVolumeInfoLoading] = useState(false);
  const [volumeCopiedId, setVolumeCopiedId] = useState<string | null>(null);
  const activeIdRef = React.useRef<string | null>(null);
  const snapshotFilterRef = React.useRef<"all" | "current">("all");
  useEffect(() => {
    activeIdRef.current = activeId;
  }, [activeId]);
  useEffect(() => {
    snapshotFilterRef.current = snapshotListFilter;
  }, [snapshotListFilter]);
  // Template ID for creating sandboxes + history from localStorage
  const [templateId, setTemplateId] = useState(() => {
    try { const h = JSON.parse(localStorage.getItem("e2b_template_history") ?? "[]"); return h[0]?.id ?? ""; } catch { return ""; }
  });
  const [templateHistory, setTemplateHistory] = useState<{ id: string; ts: number }[]>(() => {
    try { return JSON.parse(localStorage.getItem("e2b_template_history") ?? "[]"); } catch { return []; }
  });
  // Env vars
  const [envVars, setEnvVars] = useState<Record<string, string>>({});

  // Network policy — https://e2b.dev/docs/sandbox/internet-access
  const [showNetwork, setShowNetwork] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [allowInternet, setAllowInternet] = useState(true);
  const [allowOutText, setAllowOutText] = useState("");
  const [denyOutText, setDenyOutText] = useState("");
  const [denyAll, setDenyAll] = useState(false);
  const [allowPublicTraffic, setAllowPublicTraffic] = useState(true);

  // Volume mounts
  const [volumeMounts, setVolumeMounts] = useState<{ mountPath: string; volumeName: string }[]>([]);

  // Metadata
  const [metadata, setMetadata] = useState<{ key: string; value: string }[]>([]);

  /** Avoid concurrent pause/resume per sandbox (e.g. double-click before React re-renders). No retry — duplicate POST is user/agent initiated. */
  const sandboxPauseResumeBusy = React.useRef(new Set<string>());
  /** Only auto-fetch /host once per sandbox id; clears on Resume success or manual "Resolve" so failed connect does not loop 500s. */
  const hostAutoFetchAttemptedRef = React.useRef(new Set<string>());

  function saveTemplateToHistory(tid: string) {
    setTemplateHistory((prev) => {
      const next = [{ id: tid, ts: Date.now() }, ...prev.filter((x) => x.id !== tid)].slice(0, 20);
      localStorage.setItem("e2b_template_history", JSON.stringify(next));
      return next;
    });
  }

  const active = activeId ? sandboxes[activeId] : null;

  function patchSb(id: string, patch: Partial<SandboxState>) {
    setSandboxes((m) => (m[id] ? { ...m, [id]: { ...m[id], ...patch } } : m));
  }

  async function refreshList() {
    try {
      const r = await fetch("/api/sandboxes");
      const d = (await r.json()) as any;
      if (!r.ok) throw new Error(d.error || "list failed");
      const remote: string[] = d.sandboxes.map((s: any) => s.sandboxId);
      setSandboxes((m) => {
        const next: Record<string, SandboxState> = {};
        for (const s of d.sandboxes as any[]) {
          const prev = m[s.sandboxId];
          const newStatus: Status =
            s.state === "paused"
              ? "paused"
              : s.state === "pausing"
              ? "pausing"
              : s.state === "resuming"
              ? "resuming"
              : prev?.status === "executing"
              ? "executing"
              : "running";
          next[s.sandboxId] = {
            ...(prev ?? {}),
            sandboxId: s.sandboxId,
            startedAt: prev?.startedAt ?? s.startedAt,
            endAt: s.endAt,
            templateId: s.templateId,
            name: s.name,
            cpuCount: s.cpuCount,
            memoryMB: s.memoryMB,
            metadata: s.metadata,
            remoteState: s.state,
            aliases: s.aliases,
            status: newStatus,
            results: prev?.results ?? [],
          } as SandboxState;
        }
        for (const [id, v] of Object.entries(m)) {
          if (!next[id] && (v.status === "killed" || v.status === "error")) {
            next[id] = v;
          }
        }
        // Only update if something actually changed
        const mKeys = Object.keys(m);
        const nKeys = Object.keys(next);
        if (mKeys.length !== nKeys.length) return next;
        for (const k of nKeys) {
          const p = m[k], n = next[k]!;
          if (!p || p.remoteState !== n.remoteState || p.status !== n.status ||
              p.templateId !== n.templateId || p.endAt !== n.endAt) return next;
        }
        return m;
      });
      setActiveId((cur) => cur ?? remote[0] ?? null);
    } catch (e: any) {
      setErrAuto(e.message);
    }
  }

  useEffect(() => {
    refreshList();
    fetch("/api/env").then(r => r.json()).then(d => setEnvVars(d)).catch(() => {});
    const t = setInterval(refreshList, 5000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    const es = new EventSource("/api/events");
    es.onmessage = (m) => {
      try {
        const evt: LifecycleEvent = JSON.parse(m.data);
        setEvents((xs) => [evt, ...xs].slice(0, 300));
        const id = evt.sandboxId;
        if (!id) return;
        switch (evt.type) {
          case "sandbox.lifecycle.created":
            setSandboxes((m) =>
              m[id]
                ? m
                : {
                    ...m,
                    [id]: {
                      sandboxId: id,
                      startedAt: evt.eventData?.execution?.started_at ?? evt.timestamp,
                      status: "running",
                      results: [],
                    },
                  }
            );
            setActiveId((cur) => cur ?? id);
            break;
          case "sandbox.command.started":
            patchSb(id, { status: "executing" });
            break;
          case "sandbox.command.completed":
            setSandboxes((m) =>
              m[id] ? { ...m, [id]: { ...m[id], status: "running" } } : m
            );
            break;
          case "sandbox.lifecycle.killed":
            patchSb(id, { status: "killed" });
            break;
          case "sandbox.lifecycle.paused":
            patchSb(id, { status: "paused" });
            break;
          case "sandbox.lifecycle.resumed":
            patchSb(id, { status: "running" });
            break;
          case "sandbox.lifecycle.checkpointed": {
            const sid = String(evt.eventData?.snapshotId ?? "");
            if (sid) setSnapshotHighlightId(sid);
            void (async () => {
              const filter = snapshotFilterRef.current;
              const aid = activeIdRef.current;
              const q =
                filter === "current" && aid
                  ? `/api/snapshots?sandboxId=${encodeURIComponent(aid)}`
                  : "/api/snapshots";
              try {
                const r = await fetch(q);
                const d = (await r.json()) as { snapshots?: SnapshotRow[] };
                if (r.ok) setSnapshots(d.snapshots ?? []);
              } catch {}
            })();
            break;
          }
          case "sandbox.error":
            patchSb(id, { status: "error" });
            break;
        }
      } catch {}
    };
    return () => es.close();
  }, []);

  useEffect(() => {
    if (!snapshotHighlightId) return;
    const t = setTimeout(() => setSnapshotHighlightId(null), 14000);
    return () => clearTimeout(t);
  }, [snapshotHighlightId]);

  useEffect(() => {
    if (rightTab !== "snapshots") return;
    const url =
      snapshotListFilter === "current" && activeId
        ? `/api/snapshots?sandboxId=${encodeURIComponent(activeId)}`
        : "/api/snapshots";
    void (async () => {
      try {
        const r = await fetch(url);
        const d = (await r.json()) as { snapshots?: SnapshotRow[]; error?: string };
        if (!r.ok) {
          if (d.error) setErrAuto(d.error);
          return;
        }
        setSnapshots(d.snapshots ?? []);
      } catch {}
    })();
  }, [rightTab, snapshotListFilter, activeId]);

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, []);

  async function createSandbox() {
    if (!templateId.trim()) { setErrAuto("请输入 Template ID"); return; }
    setErrAuto(null);
    setCreating(true);
    try {
      // Build optional network config — only send fields the user changed from defaults.
      const splitList = (s: string) =>
        s.split(/[\s,\n]+/).map((x) => x.trim()).filter(Boolean);
      const allowOut = splitList(allowOutText);
      const denyOutItems = splitList(denyOutText);
      if (denyAll && !denyOutItems.includes("ALL_TRAFFIC")) denyOutItems.push("ALL_TRAFFIC");

      const payload: Record<string, unknown> = { templateId: templateId.trim() };
      if (!allowInternet) payload.allowInternetAccess = false;
      const network: Record<string, unknown> = {};
      if (allowOut.length) network.allowOut = allowOut;
      if (denyOutItems.length) network.denyOut = denyOutItems;
      if (!allowPublicTraffic) network.allowPublicTraffic = false;
      if (Object.keys(network).length) payload.network = network;

      const mounts: Record<string, string> = {};
      for (const m of volumeMounts) {
        if (m.mountPath.trim() && m.volumeName.trim()) {
          mounts[m.mountPath.trim()] = m.volumeName.trim();
        }
      }
      if (Object.keys(mounts).length) payload.volumeMounts = mounts;

      const meta: Record<string, string> = {};
      for (const m of metadata) {
        if (m.key.trim() && m.value.trim()) {
          meta[m.key.trim()] = m.value.trim();
        }
      }
      if (Object.keys(meta).length) payload.metadata = meta;

      const res = await fetch("/api/sandbox", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const data = (await res.json()) as any;
      if (!res.ok) throw new Error(data.error || "创建失败");
      saveTemplateToHistory(templateId.trim());
      setSandboxes((m) => ({
        ...m,
        [data.sandboxId]: m[data.sandboxId] ?? {
          sandboxId: data.sandboxId,
          startedAt: data.startedAt,
          status: "running",
          results: [],
        },
      }));
      setActiveId(data.sandboxId);
      setShowCreateModal(false);
      refreshList();
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setCreating(false);
    }
  }

  async function runCommand() {
    if (!activeId) return;
    setErrAuto(null);
    setRunning(true);
    const targetId = activeId;
    try {
      const res = await fetch(`/api/sandbox/${targetId}/run`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }),
      });
      const data = (await res.json()) as any;
      if (!res.ok) throw new Error(data.error || "执行失败");
      setSandboxes((m) =>
        m[targetId]
          ? {
              ...m,
              [targetId]: {
                ...m[targetId],
                results: [{ command, ...data }, ...m[targetId].results],
              },
            }
          : m
      );
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setRunning(false);
    }
  }

  async function killSandbox(id: string) {
    patchSb(id, { status: "killing" });
    try {
      const res = await fetch(`/api/sandbox/${id}`, { method: "DELETE" });
      const data = (await res.json().catch(() => ({}))) as { error?: string };
      if (!res.ok) throw new Error(data.error || "Kill failed");
      patchSb(id, { status: "killed", host: undefined });
      await refreshList();
    } catch (e: any) {
      setErrAuto(e.message);
      await refreshList();
    }
  }

  async function pauseSandbox(id: string) {
    const lockKey = `pause:${id}`;
    if (sandboxPauseResumeBusy.current.has(lockKey)) return;
    sandboxPauseResumeBusy.current.add(lockKey);
    patchSb(id, { status: "pausing" });
    try {
      const res = await fetch(`/api/sandbox/${id}/pause`, { method: "POST" });
      const data = (await res.json()) as any;
      if (!res.ok) throw new Error(data.error || "暂停失败");
      refreshList();
    } catch (e: any) {
      setErrAuto(e.message);
      patchSb(id, { status: "running" });
    } finally {
      sandboxPauseResumeBusy.current.delete(lockKey);
    }
  }

  async function resumeSandbox(id: string) {
    const lockKey = `resume:${id}`;
    if (sandboxPauseResumeBusy.current.has(lockKey)) return;
    sandboxPauseResumeBusy.current.add(lockKey);
    patchSb(id, { status: "resuming" });
    try {
      const res = await fetch(`/api/sandbox/${id}/resume`, { method: "POST" });
      const data = (await res.json()) as any;
      if (!res.ok) throw new Error(data.error || "恢复失败");
      hostAutoFetchAttemptedRef.current.delete(id);
      patchSb(id, { host: undefined });
      refreshList();
    } catch (e: any) {
      setErrAuto(e.message);
      patchSb(id, { status: "paused" });
    } finally {
      sandboxPauseResumeBusy.current.delete(lockKey);
    }
  }

  function removeSandbox(id: string) {
    setSandboxes((m) => {
      const { [id]: _, ...rest } = m;
      return rest;
    });
    setActiveId((cur) => {
      if (cur !== id) return cur;
      const remaining = Object.keys(sandboxes).filter((x) => x !== id);
      return remaining[0] ?? null;
    });
  }

  async function createSnapshot(id: string) {
    setSnapshotting(true);
    setErrAuto(null);
    try {
      const res = await fetch(`/api/sandbox/${id}/snapshot`, { method: "POST" });
      const data = (await res.json()) as { snapshotId?: string; error?: string };
      if (!res.ok) throw new Error(data.error || "快照失败");
      if (data.snapshotId) setSnapshotHighlightId(data.snapshotId);
      await refreshSnapshots();
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setSnapshotting(false);
    }
  }

  async function refreshSnapshots() {
    try {
      const filter = snapshotListFilter;
      const aid = activeId;
      const url =
        filter === "current" && aid
          ? `/api/snapshots?sandboxId=${encodeURIComponent(aid)}`
          : "/api/snapshots";
      const r = await fetch(url);
      const d = (await r.json()) as { snapshots?: SnapshotRow[]; error?: string };
      if (!r.ok) {
        if (d.error) setErrAuto(d.error);
        return;
      }
      setSnapshots(d.snapshots ?? []);
    } catch {}
  }

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setSnapshotCopiedId(text);
      setTimeout(() => {
        setSnapshotCopiedId((c) => (c === text ? null : c));
      }, 2000);
    } catch {
      setErrAuto("Copy failed");
    }
  }

  async function deleteSnapshot(snapshotId: string) {
    try {
      const r = await fetch(
        `/api/snapshots/${encodeURIComponent(snapshotId)}`,
        { method: "DELETE" }
      );
      const d = (await r.json()) as { error?: string };
      if (!r.ok) throw new Error(d.error || "Delete failed");
      await refreshSnapshots();
    } catch (e: any) {
      setErrAuto(e.message);
    }
  }

  async function createFromSnapshot(snapshotId: string) {
    setErrAuto(null);
    setCreating(true);
    try {
      const res = await fetch("/api/sandbox", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ templateId: snapshotId }),
      });
      const data = (await res.json()) as any;
      if (!res.ok) throw new Error(data.error || "创建失败");
      setSandboxes((m) => ({
        ...m,
        [data.sandboxId]: m[data.sandboxId] ?? {
          sandboxId: data.sandboxId,
          startedAt: data.startedAt,
          status: "running",
          results: [],
        },
      }));
      setActiveId(data.sandboxId);
      refreshList();
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setCreating(false);
    }
  }

  // File browser
  async function loadFiles(id: string, path: string) {
    setLoadingFiles(true);
    setFileContent(null);
    setFileEditorText("");
    try {
      const r = await fetch(`/api/sandbox/${id}/files?path=${encodeURIComponent(path)}`);
      const d = (await r.json()) as any;
      if (!r.ok) throw new Error(d.error);
      setFileEntries(d.entries);
      setFilePath(path);
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setLoadingFiles(false);
    }
  }

  async function readFile(id: string, path: string) {
    try {
      const r = await fetch(`/api/sandbox/${id}/files/read?path=${encodeURIComponent(path)}`);
      const d = (await r.json()) as any;
      if (!r.ok) throw new Error(d.error);
      setFileContent({ path: d.path, content: d.content });
      setFileEditorText(typeof d.content === "string" ? d.content : String(d.content ?? ""));
    } catch (e: any) {
      setErrAuto(e.message);
    }
  }

  async function saveFileToSandbox(path: string, content: string) {
    if (!activeId) return;
    setSavingFile(true);
    setErrAuto(null);
    try {
      const res = await fetch(`/api/sandbox/${activeId}/files/write`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path, content }),
      });
      const data = (await res.json()) as { error?: string; ok?: boolean };
      if (!res.ok) throw new Error(data.error || "Write failed");
      setFileContent((prev) =>
        prev && prev.path === path ? { path, content } : prev
      );
      setErrAuto(null);
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setSavingFile(false);
    }
  }

  async function getFileInfo(path: string) {
    if (!activeId) return;
    setInfoLoading(true);
    setInfoError(null);
    try {
      const r = await fetch(
        `/api/sandbox/${activeId}/files/info?path=${encodeURIComponent(path)}`
      );
      const d = (await r.json()) as { info?: any; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setInfoResult({ path, info: d.info });
    } catch (e: any) {
      setInfoResult(null);
      setInfoError(e.message);
    } finally {
      setInfoLoading(false);
    }
  }

  async function quickWriteFile() {
    if (!activeId) return;
    setSavingFile(true);
    setErrAuto(null);
    try {
      const res = await fetch(`/api/sandbox/${activeId}/files/write`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: quickWritePath.trim(), content: quickWriteBody }),
      });
      const data = (await res.json()) as { error?: string };
      if (!res.ok) throw new Error(data.error || "Write failed");
      await loadFiles(activeId, filePath);
    } catch (e: any) {
      setErrAuto(e.message);
    } finally {
      setSavingFile(false);
    }
  }

  const fileDirty =
    fileContent !== null && fileEditorText !== fileContent.content;

  // Auto-load files when switching to files tab
  useEffect(() => {
    if (rightTab === "files" && activeId && active && (active.status === "running" || active.status === "executing")) {
      loadFiles(activeId, filePath);
    }
  }, [rightTab, activeId]);

  // Volumes — list / create / getInfo / destroy (https://e2b.dev/docs/volumes/manage)
  async function refreshVolumes() {
    setVolumesLoading(true);
    setVolumesError(null);
    try {
      const r = await fetch("/api/volumes");
      const d = (await r.json()) as { volumes?: VolumeRow[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setVolumes(d.volumes ?? []);
    } catch (e: any) {
      setVolumesError(e.message);
    } finally {
      setVolumesLoading(false);
    }
  }

  async function createVolume() {
    const name = newVolumeName.trim();
    if (!name) return;
    setCreatingVolume(true);
    setVolumesError(null);
    try {
      const r = await fetch("/api/volumes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const d = (await r.json()) as { error?: string };
      if (!r.ok) throw new Error(d.error || "Create failed");
      setNewVolumeName("");
      await refreshVolumes();
    } catch (e: any) {
      setVolumesError(e.message);
    } finally {
      setCreatingVolume(false);
    }
  }

  async function destroyVolume(id: string) {
    if (!confirm(`Destroy volume?\n${id}`)) return;
    try {
      const r = await fetch(`/api/volumes/${encodeURIComponent(id)}`, { method: "DELETE" });
      const d = (await r.json()) as { error?: string };
      if (!r.ok) throw new Error(d.error || "Delete failed");
      if (volumeInfoResult?.volumeId === id) setVolumeInfoResult(null);
      await refreshVolumes();
    } catch (e: any) {
      setVolumesError(e.message);
    }
  }

  async function getVolumeInfo(id: string) {
    const target = id.trim();
    if (!target) return;
    setVolumeInfoLoading(true);
    setVolumeInfoError(null);
    setVolumeInfoResult(null);
    try {
      const r = await fetch(`/api/volumes/${encodeURIComponent(target)}`);
      const d = (await r.json()) as VolumeRow & { error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setVolumeInfoResult({ volumeId: d.volumeId, name: d.name });
    } catch (e: any) {
      setVolumeInfoError(e.message);
    } finally {
      setVolumeInfoLoading(false);
    }
  }

  useEffect(() => {
    if (rightTab === "volumes") refreshVolumes();
  }, [rightTab]);

  const list = useMemo(
    () =>
      Object.values(sandboxes).sort(
        (a, b) => Date.parse(b.startedAt) - Date.parse(a.startedAt)
      ),
    [sandboxes]
  );

  const activeEvents = useMemo(
    () => (activeId ? events.filter((e) => !e.sandboxId || e.sandboxId === activeId) : events),
    [events, activeId]
  );

  async function fetchHost(id: string, port = 3000, opts?: { quiet?: boolean }) {
    try {
      const r = await fetch(`/api/sandbox/${id}/host?port=${port}`);
      const d = (await r.json()) as any;
      if (!r.ok) throw new Error(d.error || "Host request failed");
      patchSb(id, { host: d.host, hostPort: port });
      return true;
    } catch (e: any) {
      if (!opts?.quiet) setErrAuto(e.message);
      return false;
    }
  }

  // Resolve preview URL via Sandbox.connect + getHost — only for the selected sandbox, once per attempt, no list-wide fan-out (avoids unexplained /host requests and 500 retries).
  useEffect(() => {
    if (!activeId) return;
    const s = sandboxes[activeId];
    if (!s || s.host) return;
    if (hostAutoFetchAttemptedRef.current.has(activeId)) return;
    if (s.remoteState === "paused" || s.status === "paused") return;
    if (s.remoteState === "pausing" || s.remoteState === "resuming") return;
    const eligible =
      s.remoteState === "running" ||
      s.status === "running" ||
      s.status === "executing";
    if (!eligible) return;

    hostAutoFetchAttemptedRef.current.add(activeId);
    void fetchHost(activeId, s.hostPort ?? 3000, { quiet: true });
  }, [activeId, sandboxes]);

  const st = STATUS_STYLE[active?.status ?? "idle"];
  const uptime =
    active && (active.status === "running" || active.status === "executing")
      ? now - Date.parse(active.startedAt)
      : 0;
  const canRun = active && active.status === "running";

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="flex min-h-screen">
        {/* Sidebar */}
        <aside className="w-72 border-r border-slate-800 bg-slate-900/50 flex flex-col">
          <div className="p-4 border-b border-slate-800">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-slate-300">Sandboxes</h2>
              <button
                onClick={refreshList}
                className="text-xs text-slate-400 hover:text-slate-200"
              >
                ↻ 刷新
              </button>
            </div>
            {/* Env vars */}
            {Object.keys(envVars).length > 0 && (
              <div className="mb-3 p-2 bg-slate-800/60 rounded text-[11px] font-mono space-y-0.5">
                {Object.entries(envVars).map(([k, v]) => (
                  <div key={k} className="flex items-center gap-1">
                    <span className="text-slate-400">{k}:</span>
                    <span className="text-slate-200 truncate">{v || <span className="text-slate-500 italic">未设置</span>}</span>
                  </div>
                ))}
              </div>
            )}
            <button
              onClick={() => setShowCreateModal(true)}
              className="w-full px-2 py-2 bg-emerald-600 hover:bg-emerald-500 rounded text-xs font-medium transition"
            >
              + 创建 Sandbox
            </button>
          </div>
          <div className="flex-1 overflow-auto">
            {list.length === 0 && (
              <div className="p-4 text-sm text-slate-500">暂无 sandbox</div>
            )}
            {list.map((s) => {
              const style = STATUS_STYLE[s.status];
              const isActive = s.sandboxId === activeId;
              const alive = s.status === "running" || s.status === "executing";
              const isPsd = s.status === "paused";
              return (
                <div
                  key={s.sandboxId}
                  onClick={() => setActiveId(s.sandboxId)}
                  className={`px-4 py-3 border-b border-slate-800 cursor-pointer group transition ${
                    isActive ? "bg-slate-800/80" : "hover:bg-slate-800/40"
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <span className={`w-2 h-2 rounded-full shrink-0 ${style.dot}`} />
                    <span className="font-mono text-xs text-slate-200 truncate flex-1">
                      {s.sandboxId}
                    </span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-700 text-slate-400">
                      SH
                    </span>
                  </div>
                  {s.host && (
                    <div className="mt-1 text-[11px] text-emerald-400/80 font-mono truncate" title={s.host}>
                      {s.host}
                    </div>
                  )}
                  {(s.templateId || s.cpuCount) && (
                    <div className="mt-1 text-[11px] text-slate-500 truncate">
                      {s.templateId && <span>{s.templateId}</span>}
                      {s.cpuCount && (
                        <span className="ml-2">
                          {s.cpuCount}vCPU · {s.memoryMB}MB
                        </span>
                      )}
                    </div>
                  )}
                  {s.aliases && s.aliases.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {s.aliases.map((a) => (
                        <span
                          key={a.alias}
                          className="text-[10px] px-1.5 py-0.5 rounded bg-indigo-900/40 text-indigo-300 font-mono"
                          title={`namespace: ${a.namespace}`}
                        >
                          {a.alias}
                        </span>
                      ))}
                    </div>
                  )}
                  {s.startedAt && (
                    <div
                      className="mt-1 text-[11px] text-slate-500 truncate"
                      title={new Date(s.startedAt).toLocaleString()}
                    >
                      创建于 {new Date(s.startedAt).toLocaleString()}
                    </div>
                  )}
                  <div className="flex items-center justify-between mt-2">
                    <span className={`text-[11px] px-2 py-0.5 rounded ${style.cls}`}>
                      {style.label}
                    </span>
                    {!alive && !isPsd && (
                      <button
                        onClick={(e) => { e.stopPropagation(); removeSandbox(s.sandboxId); }}
                        className="text-[11px] text-slate-400 hover:text-slate-200"
                      >
                        移除
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </aside>

        {/* Main area */}
        <main className="flex-1 p-6">
          <div className="max-w-5xl mx-auto space-y-6">
            <header className="flex items-end justify-between">
              <div>
                <h1 className="text-3xl font-bold">E2B Sandbox Demo</h1>
                <p className="text-slate-400 text-sm mt-1">
                  Shell / Python Code Interpreter /{" "}
                  <a
                    href="https://e2b.dev/docs/filesystem/read-write"
                    target="_blank"
                    rel="noreferrer"
                    className="text-emerald-400/90 hover:text-emerald-300"
                  >
                    沙箱文件读写
                  </a>
                </p>
              </div>
              <div className={`px-3 py-1.5 rounded-full text-sm font-medium flex items-center gap-2 ${st.cls}`}>
                <span className={`w-2 h-2 rounded-full ${st.dot}`} />
                {st.label}
                {active && (
                  <span className="ml-1 text-xs text-slate-400">
                    Shell
                  </span>
                )}
                {uptime > 0 && (
                  <span className="ml-2 font-mono text-xs opacity-80">
                    {(uptime / 1000).toFixed(1)}s
                  </span>
                )}
              </div>
            </header>

            {/* Sandbox info + host */}
            <section className="bg-slate-900 rounded-lg p-4 border border-slate-800">
              <div className="flex items-center justify-between gap-4 flex-wrap">
                <div className="text-sm text-slate-400 space-y-0.5">
                  <div>
                    当前 Sandbox:{" "}
                    <span className="font-mono text-slate-200">
                      {active?.sandboxId ?? "—"}
                    </span>
                  </div>
                  {active?.startedAt && (
                    <div className="text-xs text-slate-500">
                      创建于{" "}
                      <span
                        className="font-mono text-slate-400"
                        title={new Date(active.startedAt).toISOString()}
                      >
                        {new Date(active.startedAt).toLocaleString()}
                      </span>
                    </div>
                  )}
                </div>
                {active && (
                  <div className="flex items-center gap-2 text-sm flex-wrap">
                    <span className="text-slate-400">Host:</span>
                    {active.host ? (
                      <a
                        href={`https://${active.host}`}
                        target="_blank"
                        rel="noreferrer"
                        className="font-mono text-emerald-300 hover:underline"
                      >
                        {active.host}
                      </a>
                    ) : (
                      <span className="text-slate-500 font-mono">—</span>
                    )}
                    <input
                      type="number"
                      defaultValue={active.hostPort ?? 3000}
                      id="host-port"
                      className="w-20 bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs font-mono"
                    />
                    <button
                      type="button"
                      onClick={() => {
                        hostAutoFetchAttemptedRef.current.delete(active.sandboxId);
                        const el = document.getElementById("host-port") as HTMLInputElement;
                        void fetchHost(active.sandboxId, Number(el.value) || 3000);
                      }}
                      className="text-xs px-2 py-1 bg-slate-800 hover:bg-slate-700 rounded"
                    >
                      解析
                    </button>
                  </div>
                )}
              </div>
            </section>

            {/* Persistence: pause / resume (connect) / snapshot / kill — https://e2b.dev/docs/sandbox/persistence */}
            {active &&
              active.status !== "killed" &&
              (active.status === "killing" ||
                [
                  "running",
                  "executing",
                  "paused",
                  "pausing",
                  "resuming",
                  "error",
                ].includes(active.status)) && (
                <section className="bg-slate-900 rounded-lg p-4 border border-slate-800 space-y-3">
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h2 className="text-sm font-semibold text-slate-200">Persistence</h2>
                      <p className="text-[11px] text-slate-500 mt-0.5">
                        <code className="text-slate-400">pause</code>
                        {" / "}
                        <code className="text-slate-400">connect</code>
                        （恢复） ·{" "}
                        <code className="text-slate-400">kill</code>
                        （永久删除，无法再 resume）
                      </p>
                    </div>
                    <a
                      href="https://e2b.dev/docs/sandbox/persistence"
                      target="_blank"
                      rel="noreferrer"
                      className="text-xs text-emerald-400/90 hover:text-emerald-300 shrink-0"
                    >
                      E2B：Sandbox persistence →
                    </a>
                  </div>
                  {active.status === "killing" ? (
                    <p className="text-sm text-orange-300/90">销毁中…（kill）</p>
                  ) : (
                    <div className="flex flex-wrap items-center gap-2">
                      {(active.status === "running" || active.status === "executing") && (
                        <button
                          type="button"
                          onClick={() => pauseSandbox(active.sandboxId)}
                          disabled={
                            active.status === "executing" ||
                            active.status === "pausing" ||
                            active.status === "resuming"
                          }
                          className="px-3 py-1.5 bg-yellow-600 hover:bg-yellow-500 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                          title="Sandbox.pause() — Running → Paused"
                        >
                          暂停
                        </button>
                      )}
                      {active.status === "paused" && (
                        <button
                          type="button"
                          onClick={() => resumeSandbox(active.sandboxId)}
                          className="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 rounded text-xs font-medium transition"
                          title="Sandbox.connect() — Paused → Running"
                        >
                          恢复（connect）
                        </button>
                      )}
                      {(active.status === "running" || active.status === "executing") && (
                    <button
                      type="button"
                      onClick={() => createSnapshot(active.sandboxId)}
                      disabled={snapshotting || active.status === "executing"}
                      className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                      title="Sandbox.createSnapshot(sandboxId) — sandbox pauses briefly; connections may drop (see Snapshots docs)"
                    >
                      {snapshotting ? "快照中…" : "创建快照"}
                    </button>
                      )}
                      <button
                        type="button"
                        onClick={() => killSandbox(active.sandboxId)}
                        disabled={
                          active.status === "pausing" ||
                          active.status === "resuming" ||
                          active.status === "killing"
                        }
                        className="px-3 py-1.5 bg-red-700 hover:bg-red-600 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                        title="Sandbox.kill() — terminal; cannot resume after kill"
                      >
                        销毁（kill）
                      </button>
                      {(active.status === "pausing" || active.status === "resuming") && (
                        <span className="px-3 py-1.5 bg-slate-800 rounded text-xs text-slate-400">
                          {STATUS_STYLE[active.status].label}…
                        </span>
                      )}
                    </div>
                  )}
                </section>
              )}

            {/* Command input */}
            <section className="bg-slate-900 rounded-lg p-4 border border-slate-800 space-y-3">
              <label className="block text-sm font-medium">Shell Command</label>
              <div className="flex gap-2">
                <input
                  value={command}
                  onChange={(e) => setCommand(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && runCommand()}
                  disabled={!canRun}
                  className="flex-1 bg-slate-950 border border-slate-700 rounded px-3 py-2 font-mono text-sm focus:outline-none focus:border-emerald-500 disabled:opacity-50"
                  placeholder='echo "hello"'
                />
                <button
                  onClick={runCommand}
                  disabled={running || !canRun || !command.trim()}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-slate-700 disabled:text-slate-500 rounded font-medium transition"
                >
                  {running ? "执行中…" : "Run"}
                </button>
              </div>
            </section>

            {err && (
              <div className="bg-red-950/50 border border-red-800 text-red-300 rounded p-3 text-sm flex items-center justify-between">
                <span>{err}</span>
                <button onClick={() => setErrAuto(null)} className="ml-3 text-red-400 hover:text-red-200 shrink-0">✕</button>
              </div>
            )}

            <div className="grid md:grid-cols-2 gap-6">
              {/* Left: output history */}
              <section className="space-y-3">
                <h2 className="text-lg font-semibold">输出历史</h2>
                {!active || active.results.length === 0 ? (
                  <div className="text-slate-500 text-sm">暂无结果</div>
                ) : (
                  active.results.map((r, i) => (
                    <div key={i} className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                      <div className="bg-slate-800/60 px-4 py-2 flex items-center justify-between text-sm">
                        <code className="text-emerald-300 truncate">$ {r.command}</code>
                        <span className={r.exitCode === 0 ? "text-emerald-400" : "text-red-400"}>
                          exit {r.exitCode}
                        </span>
                      </div>
                      {r.stdout && (
                        <pre className="px-4 py-3 text-sm font-mono whitespace-pre-wrap text-slate-200 border-t border-slate-800">
                          {r.stdout}
                        </pre>
                      )}
                      {r.stderr && (
                        <pre className="px-4 py-3 text-sm font-mono whitespace-pre-wrap text-red-300 border-t border-slate-800">
                          {r.stderr}
                        </pre>
                      )}
                    </div>
                  ))
                )}
              </section>

              {/* Right: events or file browser */}
              <section className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex gap-1">
                    <button
                      onClick={() => setRightTab("events")}
                      className={`px-3 py-1 rounded text-sm font-medium transition ${
                        rightTab === "events" ? "bg-slate-700 text-slate-100" : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      事件流
                    </button>
                    <button
                      onClick={() => setRightTab("files")}
                      className={`px-3 py-1 rounded text-sm font-medium transition ${
                        rightTab === "files" ? "bg-slate-700 text-slate-100" : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      文件浏览
                    </button>
                    <button
                      onClick={() => setRightTab("fileops")}
                      className={`px-3 py-1 rounded text-sm font-medium transition ${
                        rightTab === "fileops" ? "bg-slate-700 text-slate-100" : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      文件操作
                    </button>
                    <button
                      onClick={() => { setRightTab("snapshots"); refreshSnapshots(); }}
                      className={`px-3 py-1 rounded text-sm font-medium transition ${
                        rightTab === "snapshots" ? "bg-slate-700 text-slate-100" : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      快照
                    </button>
                    <button
                      onClick={() => setRightTab("volumes")}
                      className={`px-3 py-1 rounded text-sm font-medium transition ${
                        rightTab === "volumes" ? "bg-slate-700 text-slate-100" : "text-slate-400 hover:text-slate-200"
                      }`}
                    >
                      Volumes
                    </button>
                  </div>
                  {rightTab === "events" && (
                    <button
                      onClick={() => setEvents([])}
                      className="text-xs text-slate-400 hover:text-slate-200"
                    >
                      清空
                    </button>
                  )}
                  {rightTab === "files" && activeId && canRun && (
                    <button
                      onClick={() => loadFiles(activeId, filePath)}
                      className="text-xs text-slate-400 hover:text-slate-200"
                    >
                      ↻ 刷新
                    </button>
                  )}
                  {rightTab === "snapshots" && (
                    <button
                      onClick={refreshSnapshots}
                      className="text-xs text-slate-400 hover:text-slate-200"
                    >
                      ↻ 刷新
                    </button>
                  )}
                  {rightTab === "volumes" && (
                    <button
                      onClick={refreshVolumes}
                      className="text-xs text-slate-400 hover:text-slate-200"
                    >
                      ↻ 刷新
                    </button>
                  )}
                </div>

                {rightTab === "events" && (
                  <div className="bg-slate-900 border border-slate-800 rounded-lg max-h-[600px] overflow-auto divide-y divide-slate-800">
                    {activeEvents.length === 0 && (
                      <div className="px-4 py-3 text-slate-500 text-sm">等待事件…</div>
                    )}
                    {activeEvents.map((e) => (
                      <details key={e.id} className="px-4 py-2 text-sm">
                        <summary className="cursor-pointer flex items-center gap-2">
                          <span className="text-slate-500 font-mono text-xs">
                            {new Date(e.timestamp).toLocaleTimeString()}
                          </span>
                          <span className={`font-mono ${EVENT_COLOR[e.type] ?? "text-slate-200"}`}>
                            {e.type}
                          </span>
                          {e.eventData?.exitCode !== undefined && (
                            <span className={`ml-auto font-mono text-xs ${e.eventData.exitCode === 0 ? "text-emerald-400" : "text-red-400"}`}>
                              exit {e.eventData.exitCode}
                            </span>
                          )}
                          {e.eventData?.duration_ms !== undefined && (
                            <span className="font-mono text-xs text-slate-400">
                              {e.eventData.duration_ms}ms
                            </span>
                          )}
                        </summary>
                        <pre className="mt-2 text-xs font-mono text-slate-400 whitespace-pre-wrap bg-slate-950/60 p-2 rounded">
                          {JSON.stringify(e, null, 2)}
                        </pre>
                      </details>
                    ))}
                  </div>
                )}

                {rightTab === "snapshots" && (
                  <div className="space-y-3">
                    <div className="rounded-lg border border-violet-900/50 bg-violet-950/25 p-3 text-xs text-slate-300 space-y-2">
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <h3 className="text-sm font-semibold text-violet-200">Sandbox snapshots</h3>
                          <p className="text-[11px] text-slate-500 mt-1 leading-relaxed">
                            Captures filesystem and memory at a point in time. The source sandbox keeps running after the snapshot (brief pause). New sandboxes use{" "}
                            <code className="text-slate-400">Sandbox.create(snapshotId)</code>.
                          </p>
                        </div>
                        <a
                          href="https://e2b.dev/docs/sandbox/snapshots"
                          target="_blank"
                          rel="noreferrer"
                          className="shrink-0 text-violet-300 hover:text-violet-200 underline-offset-2"
                        >
                          Docs →
                        </a>
                      </div>
                      <div
                        className="rounded border border-amber-800/60 bg-amber-950/30 px-2.5 py-2 text-[11px] text-amber-100/90"
                        role="status"
                      >
                        <span className="font-medium text-amber-200">Connections: </span>
                        While snapshotting, the sandbox is briefly paused. Active WebSocket / PTY / command streams may drop — reconnect clients if needed.
                      </div>
                      <div className="overflow-hidden rounded border border-slate-800">
                        <table className="w-full text-left text-[11px]">
                          <thead>
                            <tr className="border-b border-slate-800 bg-slate-950/80 text-slate-500">
                              <th className="px-2 py-1.5 font-medium">Aspect</th>
                              <th className="px-2 py-1.5 font-medium">Pause / Resume</th>
                              <th className="px-2 py-1.5 font-medium">Snapshots</th>
                            </tr>
                          </thead>
                          <tbody className="text-slate-400">
                            <tr className="border-b border-slate-800/80">
                              <td className="px-2 py-1.5 text-slate-300">Original sandbox</td>
                              <td className="px-2 py-1.5">Stays paused until resume</td>
                              <td className="px-2 py-1.5">Continues after snapshot</td>
                            </tr>
                            <tr className="border-b border-slate-800/80">
                              <td className="px-2 py-1.5 text-slate-300">Relationship</td>
                              <td className="px-2 py-1.5">One-to-one (same instance)</td>
                              <td className="px-2 py-1.5">One-to-many (fork state)</td>
                            </tr>
                            <tr>
                              <td className="px-2 py-1.5 text-slate-300">Typical use</td>
                              <td className="px-2 py-1.5">Suspend one machine</td>
                              <td className="px-2 py-1.5">Checkpoints, parallel forks</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                    </div>

                    {snapshotHighlightId && (
                      <div className="rounded-lg border border-emerald-800/60 bg-emerald-950/30 px-3 py-2 flex flex-wrap items-center gap-2 text-xs">
                        <span className="text-emerald-200/90">Latest snapshot</span>
                        <code className="font-mono text-emerald-300 truncate max-w-[min(100%,320px)]">
                          {snapshotHighlightId}
                        </code>
                        <button
                          type="button"
                          onClick={() => copyText(snapshotHighlightId)}
                          className="text-emerald-400 hover:text-emerald-300 underline text-[11px]"
                        >
                          {snapshotCopiedId === snapshotHighlightId ? "Copied" : "Copy ID"}
                        </button>
                      </div>
                    )}

                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[11px] text-slate-500">List:</span>
                      <button
                        type="button"
                        onClick={() => setSnapshotListFilter("all")}
                        className={`px-2 py-1 rounded text-[11px] font-medium transition ${
                          snapshotListFilter === "all"
                            ? "bg-slate-700 text-slate-100"
                            : "text-slate-400 hover:text-slate-200"
                        }`}
                      >
                        All snapshots
                      </button>
                      <button
                        type="button"
                        onClick={() => setSnapshotListFilter("current")}
                        disabled={!activeId}
                        className={`px-2 py-1 rounded text-[11px] font-medium transition disabled:opacity-40 ${
                          snapshotListFilter === "current"
                            ? "bg-slate-700 text-slate-100"
                            : "text-slate-400 hover:text-slate-200"
                        }`}
                        title={!activeId ? "Select a sandbox in the sidebar first" : undefined}
                      >
                        From selected sandbox
                      </button>
                      <span className="text-[10px] text-slate-600 ml-auto">
                        API: <code className="text-slate-500">listSnapshots{`({ sandboxId })`}</code>
                      </span>
                    </div>

                    <div className="bg-slate-900 border border-slate-800 rounded-lg max-h-[480px] overflow-auto divide-y divide-slate-800">
                      {snapshotListFilter === "current" && !activeId ? (
                        <div className="px-4 py-3 text-slate-500 text-sm">Select a sandbox to filter snapshots by source.</div>
                      ) : snapshots.length === 0 ? (
                        <div className="px-4 py-3 text-slate-500 text-sm">
                          No snapshots yet. Create one from a running sandbox (Persistence → Create snapshot).
                        </div>
                      ) : (
                        snapshots.map((snap) => {
                          const isNew = snap.snapshotId === snapshotHighlightId;
                          return (
                            <div
                              key={snap.snapshotId}
                              className={`px-4 py-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3 ${
                                isNew ? "bg-violet-950/35 border-l-2 border-violet-500" : ""
                              }`}
                            >
                              <div className="flex-1 min-w-0">
                                <div className="font-mono text-xs text-slate-200 break-all">{snap.snapshotId}</div>
                                {snap.names && snap.names.length > 0 && (
                                  <div className="text-[10px] text-slate-500 mt-1 font-mono">
                                    {snap.names.join(", ")}
                                  </div>
                                )}
                                <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-slate-500 mt-1">
                                  {snap.sandboxId && (
                                    <span>
                                      Source: <span className="font-mono text-slate-400">{snap.sandboxId}</span>
                                    </span>
                                  )}
                                  {snap.createdAt && (
                                    <span>{new Date(snap.createdAt).toLocaleString()}</span>
                                  )}
                                </div>
                              </div>
                              <div className="flex flex-wrap items-center gap-2 shrink-0">
                                <button
                                  type="button"
                                  onClick={() => copyText(snap.snapshotId)}
                                  className="px-2 py-1 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
                                >
                                  {snapshotCopiedId === snap.snapshotId ? "Copied" : "Copy"}
                                </button>
                                <button
                                  type="button"
                                  onClick={() => createFromSnapshot(snap.snapshotId)}
                                  disabled={creating}
                                  className="px-2 py-1 bg-emerald-700 hover:bg-emerald-600 disabled:bg-slate-700 rounded text-[11px] font-medium transition"
                                >
                                  Spawn from snapshot
                                </button>
                                <button
                                  type="button"
                                  onClick={() => {
                                    if (confirm(`Delete snapshot?\n${snap.snapshotId}`)) {
                                      void deleteSnapshot(snap.snapshotId);
                                    }
                                  }}
                                  className="text-[11px] text-red-400 hover:text-red-300"
                                >
                                  Delete
                                </button>
                              </div>
                            </div>
                          );
                        })
                      )}
                    </div>
                  </div>
                )}

                {rightTab === "fileops" && (
                  <div className="bg-slate-900 border border-slate-800 rounded-lg max-h-[640px] overflow-auto">
                    {!activeId || !canRun ? (
                      <div className="px-4 py-3 text-slate-500 text-sm">请选择运行中的 sandbox</div>
                    ) : (
                      <div className="divide-y divide-slate-800">
                        {/* files.write — Quick write */}
                        <div className="px-4 py-3 space-y-2">
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <span className="text-[11px] font-medium text-slate-400">
                              Quick write{" "}
                              <code className="text-slate-500">files.write(path, data)</code>
                            </span>
                            <a
                              href="https://e2b.dev/docs/filesystem/read-write"
                              target="_blank"
                              rel="noreferrer"
                              className="text-[11px] text-emerald-400/90 hover:text-emerald-300"
                            >
                              Docs →
                            </a>
                          </div>
                          <input
                            value={quickWritePath}
                            onChange={(e) => setQuickWritePath(e.target.value)}
                            className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500"
                            placeholder="/path/to/file"
                          />
                          <textarea
                            value={quickWriteBody}
                            onChange={(e) => setQuickWriteBody(e.target.value)}
                            rows={3}
                            className="w-full bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs font-mono text-slate-200 focus:outline-none focus:border-emerald-500 resize-y min-h-[60px]"
                            placeholder="File content"
                          />
                          <button
                            type="button"
                            onClick={() => void quickWriteFile()}
                            disabled={savingFile || !quickWritePath.trim()}
                            className="px-3 py-1.5 bg-emerald-700 hover:bg-emerald-600 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                          >
                            {savingFile ? "Writing…" : "Write file"}
                          </button>
                        </div>

                        {/* files.getInfo() — https://e2b.dev/docs/filesystem/info */}
                        <div className="px-4 py-3 space-y-2">
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <span className="text-[11px] font-medium text-slate-400">
                              File / dir info{" "}
                              <code className="text-slate-500">files.getInfo(path)</code>
                            </span>
                            <a
                              href="https://e2b.dev/docs/filesystem/info"
                              target="_blank"
                              rel="noreferrer"
                              className="text-[11px] text-emerald-400/90 hover:text-emerald-300"
                            >
                              Docs →
                            </a>
                          </div>
                          <div className="flex gap-2">
                            <input
                              value={infoPath}
                              onChange={(e) => setInfoPath(e.target.value)}
                              onKeyDown={(e) =>
                                e.key === "Enter" && infoPath.trim() && getFileInfo(infoPath.trim())
                              }
                              className="flex-1 bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500"
                              placeholder="/home/user/test_file.txt"
                            />
                            <button
                              type="button"
                              onClick={() => infoPath.trim() && getFileInfo(infoPath.trim())}
                              disabled={infoLoading || !infoPath.trim()}
                              className="px-3 py-1.5 bg-sky-700 hover:bg-sky-600 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                            >
                              {infoLoading ? "…" : "Get info"}
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                setInfoPath(filePath);
                                getFileInfo(filePath);
                              }}
                              disabled={infoLoading}
                              className="px-2 py-1.5 bg-slate-800 hover:bg-slate-700 rounded text-xs font-medium transition"
                              title="Use the current browser path"
                            >
                              ↺ cwd
                            </button>
                          </div>
                          {infoError && (
                            <div className="text-[11px] text-red-300 bg-red-950/40 border border-red-900/60 rounded px-2 py-1 break-all">
                              {infoError}
                            </div>
                          )}
                          {infoResult && (
                            <div className="bg-slate-950/80 border border-slate-800 rounded p-2 space-y-1">
                              <div className="text-[10px] text-slate-500 font-mono break-all">
                                {infoResult.path}
                              </div>
                              {infoResult.info && typeof infoResult.info === "object" ? (
                                <table className="w-full text-[11px] font-mono">
                                  <tbody>
                                    {Object.entries(infoResult.info).map(([k, v]) => (
                                      <tr key={k} className="border-b border-slate-800/40 last:border-0">
                                        <td className="py-0.5 pr-2 text-slate-500 align-top">{k}</td>
                                        <td className="py-0.5 text-slate-200 break-all">
                                          {v === null
                                            ? <span className="text-slate-600 italic">null</span>
                                            : typeof v === "object"
                                            ? JSON.stringify(v)
                                            : String(v)}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              ) : (
                                <pre className="text-[11px] text-slate-300 whitespace-pre-wrap">
                                  {JSON.stringify(infoResult.info, null, 2)}
                                </pre>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {rightTab === "volumes" && (
                  <div className="space-y-3">
                    <div className="rounded-lg border border-cyan-900/50 bg-cyan-950/25 p-3 text-xs text-slate-300">
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <h3 className="text-sm font-semibold text-cyan-200">Volumes</h3>
                          <p className="text-[11px] text-slate-500 mt-1 leading-relaxed">
                            Persistent storage shared across sandboxes. Names: letters, numbers, hyphens.
                          </p>
                        </div>
                        <a
                          href="https://e2b.dev/docs/volumes/manage"
                          target="_blank"
                          rel="noreferrer"
                          className="shrink-0 text-cyan-300 hover:text-cyan-200 underline-offset-2"
                        >
                          Docs →
                        </a>
                      </div>
                    </div>

                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 space-y-2">
                      <div className="text-[11px] font-medium text-slate-400">
                        Create <code className="text-slate-500">Volume.create(name)</code>
                      </div>
                      <div className="flex gap-2">
                        <input
                          value={newVolumeName}
                          onChange={(e) => setNewVolumeName(e.target.value)}
                          onKeyDown={(e) => e.key === "Enter" && !creatingVolume && createVolume()}
                          placeholder="my-volume"
                          className="flex-1 bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500"
                        />
                        <button
                          type="button"
                          onClick={() => void createVolume()}
                          disabled={creatingVolume || !newVolumeName.trim()}
                          className="px-3 py-1.5 bg-emerald-700 hover:bg-emerald-600 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                        >
                          {creatingVolume ? "Creating…" : "Create"}
                        </button>
                      </div>
                    </div>

                    <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 space-y-2">
                      <div className="text-[11px] font-medium text-slate-400">
                        Get info <code className="text-slate-500">Volume.getInfo(volumeId)</code>
                      </div>
                      <div className="flex gap-2">
                        <input
                          value={volumeInfoId}
                          onChange={(e) => setVolumeInfoId(e.target.value)}
                          onKeyDown={(e) => e.key === "Enter" && volumeInfoId.trim() && getVolumeInfo(volumeInfoId)}
                          placeholder="volume-id"
                          className="flex-1 bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500"
                        />
                        <button
                          type="button"
                          onClick={() => getVolumeInfo(volumeInfoId)}
                          disabled={volumeInfoLoading || !volumeInfoId.trim()}
                          className="px-3 py-1.5 bg-sky-700 hover:bg-sky-600 disabled:bg-slate-700 disabled:text-slate-500 rounded text-xs font-medium transition"
                        >
                          {volumeInfoLoading ? "…" : "Get info"}
                        </button>
                      </div>
                      {volumeInfoError && (
                        <div className="text-[11px] text-red-300 bg-red-950/40 border border-red-900/60 rounded px-2 py-1 break-all">
                          {volumeInfoError}
                        </div>
                      )}
                      {volumeInfoResult && (
                        <div className="bg-slate-950/80 border border-slate-800 rounded p-2 text-[11px] font-mono">
                          <div><span className="text-slate-500">volumeId:</span> <span className="text-slate-200 break-all">{volumeInfoResult.volumeId}</span></div>
                          <div><span className="text-slate-500">name:</span> <span className="text-slate-200 break-all">{volumeInfoResult.name}</span></div>
                        </div>
                      )}
                    </div>

                    {volumesError && (
                      <div className="text-[11px] text-red-300 bg-red-950/40 border border-red-900/60 rounded px-2 py-1 break-all">
                        {volumesError}
                      </div>
                    )}

                    <div className="flex items-center justify-between text-[11px] text-slate-500">
                      <span>
                        List <code className="text-slate-600">Volume.list()</code> · {volumes.length} item{volumes.length === 1 ? "" : "s"}
                      </span>
                      {volumesLoading && <span>loading…</span>}
                    </div>

                    <div className="bg-slate-900 border border-slate-800 rounded-lg max-h-[420px] overflow-auto divide-y divide-slate-800">
                      {volumes.length === 0 && !volumesLoading ? (
                        <div className="px-4 py-3 text-slate-500 text-sm">No volumes yet.</div>
                      ) : (
                        volumes.map((v) => (
                          <div
                            key={v.volumeId}
                            className="px-4 py-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
                          >
                            <div className="flex-1 min-w-0">
                              <div className="text-xs text-slate-200 truncate" title={v.name}>{v.name}</div>
                              <div className="font-mono text-[10px] text-slate-500 break-all mt-0.5">{v.volumeId}</div>
                            </div>
                            <div className="flex flex-wrap items-center gap-2 shrink-0">
                              <button
                                type="button"
                                onClick={async () => {
                                  try {
                                    await navigator.clipboard.writeText(v.volumeId);
                                    setVolumeCopiedId(v.volumeId);
                                    setTimeout(() => setVolumeCopiedId((c) => (c === v.volumeId ? null : c)), 2000);
                                  } catch {}
                                }}
                                className="px-2 py-1 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
                              >
                                {volumeCopiedId === v.volumeId ? "Copied" : "Copy ID"}
                              </button>
                              <button
                                type="button"
                                onClick={() => {
                                  setVolumeInfoId(v.volumeId);
                                  void getVolumeInfo(v.volumeId);
                                }}
                                className="px-2 py-1 rounded text-[11px] bg-sky-800 hover:bg-sky-700 text-slate-100 transition"
                              >
                                Info
                              </button>
                              <button
                                type="button"
                                onClick={() => void destroyVolume(v.volumeId)}
                                className="text-[11px] text-red-400 hover:text-red-300"
                              >
                                Destroy
                              </button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                )}

                {rightTab === "files" && (
                  <div className="bg-slate-900 border border-slate-800 rounded-lg max-h-[640px] overflow-auto flex flex-col">
                    {!activeId || !canRun ? (
                      <div className="px-4 py-3 text-slate-500 text-sm">请选择运行中的 sandbox</div>
                    ) : loadingFiles ? (
                      <div className="px-4 py-3 text-slate-500 text-sm">加载中…</div>
                    ) : (
                      <>
                        {/* Breadcrumb */}
                        <div className="px-4 py-2 border-b border-slate-800 flex items-center gap-1 text-xs font-mono">
                          {filePath.split("/").filter(Boolean).length > 0 && (
                            <button
                              onClick={() => {
                                const parent = filePath.split("/").slice(0, -1).join("/") || "/";
                                loadFiles(activeId!, parent);
                              }}
                              className="text-blue-400 hover:text-blue-300"
                            >
                              ..
                            </button>
                          )}
                          <span className="text-slate-400">{filePath}</span>
                        </div>
                        {fileEntries.length === 0 && !fileContent && (
                          <div className="px-4 py-3 text-slate-500 text-sm">空目录</div>
                        )}
                        {!fileContent && fileEntries.map((f) => (
                          <div
                            key={f.path}
                            className="px-4 py-2 border-b border-slate-800/50 hover:bg-slate-800/40 flex items-center gap-3 text-sm"
                          >
                            <span
                              onClick={() => {
                                if (f.type === "dir") loadFiles(activeId!, f.path);
                                else readFile(activeId!, f.path);
                              }}
                              className={`cursor-pointer ${f.type === "dir" ? "text-blue-400" : "text-slate-400"}`}
                            >
                              {f.type === "dir" ? "📁" : "📄"}
                            </span>
                            <span
                              onClick={() => {
                                if (f.type === "dir") loadFiles(activeId!, f.path);
                                else readFile(activeId!, f.path);
                              }}
                              className={`font-mono flex-1 cursor-pointer ${f.type === "dir" ? "text-blue-300" : "text-slate-200"}`}
                            >
                              {f.name}
                            </span>
                            <span className="text-xs text-slate-500">{f.permissions}</span>
                            {f.type !== "dir" && (
                              <span className="text-xs text-slate-500">
                                {f.size < 1024 ? `${f.size}B` : `${(f.size / 1024).toFixed(1)}K`}
                              </span>
                            )}
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setInfoPath(f.path);
                                setRightTab("fileops");
                                getFileInfo(f.path);
                              }}
                              className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 hover:bg-sky-700 text-slate-300 transition"
                              title="files.getInfo()"
                            >
                              info
                            </button>
                          </div>
                        ))}
                        {fileContent && (
                          <div className="flex flex-col min-h-0 flex-1">
                            <div className="px-4 py-2 border-b border-slate-800 flex flex-wrap items-center justify-between gap-2">
                              <span className="font-mono text-xs text-slate-300 truncate max-w-[min(100%,280px)]" title={fileContent.path}>
                                {fileContent.path}
                              </span>
                              <div className="flex flex-wrap items-center gap-2">
                                {fileDirty && (
                                  <span className="text-[10px] text-amber-300/90">Unsaved</span>
                                )}
                                <button
                                  type="button"
                                  onClick={() => void readFile(activeId!, fileContent.path)}
                                  disabled={savingFile}
                                  className="text-xs px-2 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 rounded"
                                >
                                  Reload
                                </button>
                                <button
                                  type="button"
                                  onClick={() => void saveFileToSandbox(fileContent.path, fileEditorText)}
                                  disabled={savingFile || !fileDirty}
                                  className="text-xs px-2 py-1 bg-emerald-700 hover:bg-emerald-600 disabled:bg-slate-700 disabled:text-slate-500 rounded font-medium"
                                >
                                  {savingFile ? "Saving…" : "Save"}
                                </button>
                                <button
                                  onClick={() => {
                                    setFileContent(null);
                                    setFileEditorText("");
                                  }}
                                  className="text-xs text-slate-400 hover:text-slate-200"
                                >
                                  返回列表
                                </button>
                              </div>
                            </div>
                            <textarea
                              value={fileEditorText}
                              onChange={(e) => setFileEditorText(e.target.value)}
                              spellCheck={false}
                              className="px-4 py-3 text-xs font-mono text-slate-200 bg-slate-950/80 border-0 focus:outline-none focus:ring-0 min-h-[220px] flex-1 resize-y"
                              aria-label="File content editor"
                            />
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}
              </section>
            </div>
          </div>
        </main>
      </div>

      {/* Create Sandbox Modal */}
      {showCreateModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={(e) => { if (e.target === e.currentTarget) setShowCreateModal(false); }}
        >
          <div className="bg-slate-900 border border-slate-700 rounded-xl shadow-2xl w-full max-w-lg mx-4 max-h-[85vh] overflow-auto">
            <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800">
              <h2 className="text-lg font-semibold text-slate-100">创建 Sandbox</h2>
              <button
                onClick={() => setShowCreateModal(false)}
                className="text-slate-400 hover:text-slate-200 text-lg"
              >
                ✕
              </button>
            </div>

            <div className="px-6 py-5 space-y-5">
              {/* Template ID */}
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-1.5">Template ID</label>
                <input
                  value={templateId}
                  onChange={(e) => setTemplateId(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && createSandbox()}
                  className="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                  placeholder="输入 Template ID（必填）"
                  autoFocus
                />
              </div>

              {/* Template history */}
              {templateHistory.length > 0 && (
                <div>
                  <label className="block text-xs text-slate-500 mb-1">最近使用</label>
                  <div className="flex flex-wrap gap-1.5">
                    {templateHistory.map((t) => (
                      <button
                        key={t.id}
                        onClick={() => setTemplateId(t.id)}
                        className={`px-2 py-1 rounded text-xs font-mono transition ${
                          templateId === t.id
                            ? "bg-emerald-700 text-emerald-100"
                            : "bg-slate-800 text-slate-400 hover:text-slate-200 hover:bg-slate-700"
                        }`}
                        title={new Date(t.ts).toLocaleString()}
                      >
                        {t.id}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Network policy */}
              <div className="border border-slate-800 rounded-lg overflow-hidden">
                <button
                  type="button"
                  onClick={() => setShowNetwork((v) => !v)}
                  className="w-full flex items-center justify-between px-4 py-3 text-sm text-slate-300 hover:bg-slate-800/40 transition"
                >
                  <span className="flex items-center gap-2">
                    网络策略
                    {(!allowInternet || allowOutText || denyOutText || denyAll || !allowPublicTraffic) && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-900/40 text-amber-300">已配置</span>
                    )}
                  </span>
                  <span className="text-slate-500">{showNetwork ? "▾" : "▸"}</span>
                </button>
                {showNetwork && (
                  <div className="px-4 pb-4 space-y-3 text-sm text-slate-300 border-t border-slate-800">
                    <label className="flex items-center gap-2 cursor-pointer mt-3">
                      <input
                        type="checkbox"
                        checked={allowInternet}
                        onChange={(e) => setAllowInternet(e.target.checked)}
                        className="accent-emerald-500"
                      />
                      <span>允许访问外网 <code className="text-xs text-slate-500">allowInternetAccess</code></span>
                    </label>
                    <div>
                      <label className="block text-xs text-slate-500 mb-1">
                        allowOut <span className="text-slate-600">(IP / CIDR / 域名，逗号或换行分隔)</span>
                      </label>
                      <textarea
                        value={allowOutText}
                        onChange={(e) => setAllowOutText(e.target.value)}
                        rows={2}
                        placeholder="api.example.com, *.github.com, 8.8.8.8"
                        className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-slate-500 mb-1">
                        denyOut <span className="text-slate-600">(仅 IP / CIDR)</span>
                      </label>
                      <textarea
                        value={denyOutText}
                        onChange={(e) => setDenyOutText(e.target.value)}
                        rows={2}
                        placeholder="10.0.0.0/8"
                        className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-2 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                      />
                    </div>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={denyAll}
                        onChange={(e) => setDenyAll(e.target.checked)}
                        className="accent-amber-500"
                      />
                      <span className="text-xs">denyOut 加入 <code className="text-slate-500">ALL_TRAFFIC</code></span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={allowPublicTraffic}
                        onChange={(e) => setAllowPublicTraffic(e.target.checked)}
                        className="accent-emerald-500"
                      />
                      <span className="text-xs">允许公开流量 <code className="text-slate-500">allowPublicTraffic</code></span>
                    </label>
                  </div>
                )}
              </div>

              {/* Volume Mounts */}
              <div className="border border-slate-800 rounded-lg overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <span className="text-sm text-slate-300 flex items-center gap-2">
                    Volume 挂载
                    {volumeMounts.length > 0 && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-indigo-900/40 text-indigo-300">{volumeMounts.length}</span>
                    )}
                  </span>
                  <button
                    type="button"
                    onClick={() => setVolumeMounts((v) => [...v, { mountPath: "/mnt/data", volumeName: "" }])}
                    className="text-xs text-emerald-400 hover:text-emerald-300"
                  >
                    + 添加
                  </button>
                </div>
                {volumeMounts.length > 0 && (
                  <div className="px-4 pb-3 space-y-2 border-t border-slate-800 pt-3">
                    {volumeMounts.map((m, i) => (
                      <div key={i} className="flex gap-2 items-center">
                        <input
                          value={m.mountPath}
                          onChange={(e) => {
                            const next = [...volumeMounts];
                            next[i] = { ...next[i], mountPath: e.target.value };
                            setVolumeMounts(next);
                          }}
                          placeholder="挂载路径"
                          className="flex-1 bg-slate-950 border border-slate-700 rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                        />
                        <span className="text-slate-600 text-xs shrink-0">→</span>
                        <input
                          value={m.volumeName}
                          onChange={(e) => {
                            const next = [...volumeMounts];
                            next[i] = { ...next[i], volumeName: e.target.value };
                            setVolumeMounts(next);
                          }}
                          placeholder="Volume 名称"
                          className="flex-1 bg-slate-950 border border-slate-700 rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                        />
                        <button
                          type="button"
                          onClick={() => setVolumeMounts((v) => v.filter((_, j) => j !== i))}
                          className="text-red-400 hover:text-red-300 text-sm px-1 shrink-0"
                        >
                          ✕
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Metadata */}
              <div className="border border-slate-800 rounded-lg overflow-hidden">
                <div className="flex items-center justify-between px-4 py-3">
                  <span className="text-sm text-slate-300 flex items-center gap-2">
                    Metadata
                    {metadata.length > 0 && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-indigo-900/40 text-indigo-300">{metadata.length}</span>
                    )}
                  </span>
                  <button
                    type="button"
                    onClick={() => setMetadata((v) => [...v, { key: "e2b.agents.kruise.io/csi-volume-config", value: "" }])}
                    className="text-xs text-emerald-400 hover:text-emerald-300"
                  >
                    + 添加
                  </button>
                </div>
                {metadata.length > 0 && (
                  <div className="px-4 pb-3 space-y-2 border-t border-slate-800 pt-3">
                    {metadata.map((m, i) => (
                      <div key={i} className="space-y-1.5">
                        <div className="flex gap-2 items-center">
                          <input
                            value={m.key}
                            onChange={(e) => {
                              const next = [...metadata];
                              next[i] = { ...next[i], key: e.target.value };
                              setMetadata(next);
                            }}
                            placeholder="Key"
                            className="flex-1 bg-slate-950 border border-slate-700 rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
                          />
                          <button
                            type="button"
                            onClick={() => setMetadata((v) => v.filter((_, j) => j !== i))}
                            className="text-red-400 hover:text-red-300 text-sm px-1 shrink-0"
                          >
                            ✕
                          </button>
                        </div>
                        <textarea
                          value={m.value}
                          onChange={(e) => {
                            const next = [...metadata];
                            next[i] = { ...next[i], value: e.target.value };
                            setMetadata(next);
                          }}
                          onBlur={() => {
                            try {
                              const parsed = JSON.parse(m.value);
                              const formatted = JSON.stringify(parsed, null, 2);
                              if (formatted !== m.value) {
                                const next = [...metadata];
                                next[i] = { ...next[i], value: formatted };
                                setMetadata(next);
                              }
                            } catch {}
                          }}
                          rows={4}
                          placeholder="Value（支持多行，JSON 会自动格式化）"
                          className="w-full bg-slate-950 border border-slate-700 rounded px-3 py-1.5 text-xs font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600 resize-y min-h-[60px]"
                        />
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-slate-800">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 text-sm text-slate-400 hover:text-slate-200 transition"
              >
                取消
              </button>
              <button
                onClick={() => createSandbox()}
                disabled={creating || !templateId.trim()}
                className="px-5 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-700 disabled:text-slate-500 rounded-lg text-sm font-medium transition"
              >
                {creating ? "创建中…" : "创建"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
