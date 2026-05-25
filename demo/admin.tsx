import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type TemplateBuild = {
  buildId: string;
  status: string;
  statusGroup: string;
  dockerfile: string;
  startCmd: string;
  vcpu: number;
  ramMB: number;
  freeDiskSizeMB: number;
  totalDiskSizeMB: number;
  kernelVersion: string;
  firecrackerVersion: string;
  envdVersion: string;
  finishedAt: string | null;
};

type TemplateRow = {
  id: string;
  aliases: { alias: string; namespace: string }[];
  createdAt: string;
  updatedAt: string;
  public: boolean;
  buildCount: number;
  spawnCount: number;
  lastSpawnedAt: string | null;
  teamId: string;
  source: string;
  latestBuild: TemplateBuild | null;
};

type VolumeRow = { volumeId: string; name: string };

type ApiKeyRow = {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  lastUsed: string | null;
  teamId: string;
  teamName: string | null;
  maskedKey: string;
  keyLength: number;
};

type AccessTokenRow = {
  id: string;
  name: string;
  createdAt: string;
  userId: string;
  userEmail: string | null;
  maskedToken: string;
  tokenLength: number;
};

type Tab = "templates" | "volumes" | "apikeys" | "database";

function TemplatesTab() {
  const [templates, setTemplates] = useState<TemplateRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState<"updated" | "spawns" | "created">("updated");

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const r = await fetch("/api/templates");
      const d = (await r.json()) as { templates?: TemplateRow[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setTemplates(d.templates ?? []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { refresh(); }, []);

  const filtered = useMemo(() => {
    let list = templates;
    if (search.trim()) {
      const q = search.toLowerCase();
      list = list.filter(
        (t) =>
          t.id.toLowerCase().includes(q) ||
          t.aliases.some((a) => a.alias.toLowerCase().includes(q)) ||
          t.teamId.toLowerCase().includes(q)
      );
    }
    if (sortBy === "updated") list = [...list].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    else if (sortBy === "spawns") list = [...list].sort((a, b) => b.spawnCount - a.spawnCount);
    else list = [...list].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
    return list;
  }, [templates, search, sortBy]);

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center justify-between">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索模板 ID、alias 或 Team ID..."
          className="w-full sm:w-96 bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
        />
        <div className="flex items-center gap-2">
          <span className="text-xs text-slate-500">{templates.length} 个模板</span>
          <span className="text-slate-700">|</span>
          <span className="text-xs text-slate-500">排序:</span>
          {(["updated", "spawns", "created"] as const).map((s) => (
            <button
              key={s}
              onClick={() => setSortBy(s)}
              className={`px-2.5 py-1 rounded text-xs font-medium transition ${
                sortBy === s ? "bg-emerald-700 text-emerald-100" : "bg-slate-800 text-slate-400 hover:text-slate-200"
              }`}
            >
              {{ updated: "最近更新", spawns: "启动次数", created: "创建时间" }[s]}
            </button>
          ))}
          <button
            onClick={refresh}
            disabled={loading}
            className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 rounded text-xs font-medium transition ml-1"
          >
            {loading ? "..." : "↻ 刷新"}
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-950/50 border border-red-800 text-red-300 rounded-lg p-4 text-sm">{error}</div>
      )}

      <div className="grid gap-4">
        {filtered.length === 0 && !loading && (
          <div className="text-center text-slate-500 py-12">
            {search ? "没有匹配的模板" : "暂无模板数据"}
          </div>
        )}
        {filtered.map((t) => {
          const isExpanded = expandedId === t.id;
          return (
            <div
              key={t.id}
              className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden hover:border-slate-700 transition"
            >
              <div
                className="px-6 py-4 cursor-pointer flex items-start gap-4"
                onClick={() => setExpandedId(isExpanded ? null : t.id)}
              >
                <span className="text-slate-500 mt-1 select-none text-sm">{isExpanded ? "▾" : "▸"}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 flex-wrap">
                    <span className="font-mono text-sm text-slate-100 font-medium">{t.id}</span>
                    {t.public && (
                      <span className="text-[11px] px-2 py-0.5 rounded-full bg-emerald-900/50 text-emerald-300 border border-emerald-800/50">公开</span>
                    )}
                    {t.latestBuild && (
                      <span className={`text-[11px] px-2 py-0.5 rounded-full border ${
                        t.latestBuild.statusGroup === "succeeded"
                          ? "bg-emerald-900/30 text-emerald-300 border-emerald-800/50"
                          : t.latestBuild.statusGroup === "failed"
                          ? "bg-red-900/30 text-red-300 border-red-800/50"
                          : "bg-amber-900/30 text-amber-300 border-amber-800/50"
                      }`}>
                        {t.latestBuild.status}
                      </span>
                    )}
                  </div>
                  {t.aliases.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 mt-2">
                      {t.aliases.map((a) => (
                        <span
                          key={a.alias}
                          className="text-xs px-2 py-0.5 rounded-md bg-indigo-900/40 text-indigo-300 font-mono border border-indigo-800/40"
                          title={`namespace: ${a.namespace}`}
                        >
                          {a.alias}
                        </span>
                      ))}
                    </div>
                  )}
                  <div className="flex flex-wrap gap-x-5 gap-y-1 mt-2 text-xs text-slate-500">
                    <span>构建 <span className="text-slate-300">{t.buildCount}</span> 次</span>
                    <span>启动 <span className="text-slate-300">{t.spawnCount}</span> 次</span>
                    {t.lastSpawnedAt && (
                      <span>最近启动 <span className="text-slate-400">{new Date(t.lastSpawnedAt).toLocaleString()}</span></span>
                    )}
                    <span>更新于 <span className="text-slate-400">{new Date(t.updatedAt).toLocaleString()}</span></span>
                  </div>
                </div>
                {t.latestBuild && (
                  <div className="hidden sm:flex items-center gap-3 text-xs text-slate-400 shrink-0">
                    <span className="bg-slate-800 px-2 py-1 rounded">{t.latestBuild.vcpu} vCPU</span>
                    <span className="bg-slate-800 px-2 py-1 rounded">{t.latestBuild.ramMB} MB</span>
                  </div>
                )}
              </div>
              {isExpanded && (
                <div className="border-t border-slate-800 px-6 py-5 bg-slate-950/50">
                  <div className="grid md:grid-cols-2 gap-6">
                    <div className="space-y-4">
                      <div>
                        <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wide mb-2">基本信息</h3>
                        <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                          <table className="w-full text-sm">
                            <tbody className="divide-y divide-slate-800">
                              <tr><td className="px-4 py-2 text-slate-500 w-28">Template ID</td><td className="px-4 py-2 font-mono text-slate-200">{t.id}</td></tr>
                              <tr><td className="px-4 py-2 text-slate-500">Team ID</td><td className="px-4 py-2 font-mono text-slate-200 break-all">{t.teamId}</td></tr>
                              <tr><td className="px-4 py-2 text-slate-500">Source</td><td className="px-4 py-2 text-slate-200">{t.source || "—"}</td></tr>
                              <tr><td className="px-4 py-2 text-slate-500">创建时间</td><td className="px-4 py-2 text-slate-200">{new Date(t.createdAt).toLocaleString()}</td></tr>
                              <tr><td className="px-4 py-2 text-slate-500">更新时间</td><td className="px-4 py-2 text-slate-200">{new Date(t.updatedAt).toLocaleString()}</td></tr>
                            </tbody>
                          </table>
                        </div>
                      </div>
                      {t.aliases.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wide mb-2">别名 (Aliases)</h3>
                          <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                            <table className="w-full text-sm">
                              <thead>
                                <tr className="border-b border-slate-800 bg-slate-800/40">
                                  <th className="px-4 py-2 text-left text-xs font-medium text-slate-400">别名</th>
                                  <th className="px-4 py-2 text-left text-xs font-medium text-slate-400">命名空间</th>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-slate-800">
                                {t.aliases.map((a) => (
                                  <tr key={a.alias}>
                                    <td className="px-4 py-2 font-mono text-indigo-300">{a.alias}</td>
                                    <td className="px-4 py-2 font-mono text-slate-400">{a.namespace}</td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        </div>
                      )}
                    </div>
                    <div className="space-y-4">
                      {t.latestBuild ? (
                        <>
                          <div>
                            <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wide mb-2">最新构建</h3>
                            <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
                              <table className="w-full text-sm">
                                <tbody className="divide-y divide-slate-800">
                                  <tr><td className="px-4 py-2 text-slate-500 w-32">Build ID</td><td className="px-4 py-2 font-mono text-slate-200 break-all text-xs">{t.latestBuild.buildId}</td></tr>
                                  <tr>
                                    <td className="px-4 py-2 text-slate-500">状态</td>
                                    <td className="px-4 py-2">
                                      <span className={`text-xs px-2 py-0.5 rounded ${
                                        t.latestBuild.statusGroup === "succeeded" ? "bg-emerald-900/40 text-emerald-300"
                                        : t.latestBuild.statusGroup === "failed" ? "bg-red-900/40 text-red-300"
                                        : "bg-amber-900/40 text-amber-300"
                                      }`}>{t.latestBuild.status}</span>
                                    </td>
                                  </tr>
                                  <tr><td className="px-4 py-2 text-slate-500">规格</td><td className="px-4 py-2 text-slate-200">{t.latestBuild.vcpu} vCPU · {t.latestBuild.ramMB} MB RAM</td></tr>
                                  <tr><td className="px-4 py-2 text-slate-500">磁盘</td><td className="px-4 py-2 text-slate-200">{t.latestBuild.freeDiskSizeMB} MB 可用 / {t.latestBuild.totalDiskSizeMB} MB 总计</td></tr>
                                  <tr><td className="px-4 py-2 text-slate-500">内核</td><td className="px-4 py-2 font-mono text-slate-200 text-xs">{t.latestBuild.kernelVersion || "—"}</td></tr>
                                  <tr><td className="px-4 py-2 text-slate-500">Firecracker</td><td className="px-4 py-2 font-mono text-slate-200 text-xs">{t.latestBuild.firecrackerVersion || "—"}</td></tr>
                                  <tr><td className="px-4 py-2 text-slate-500">envd</td><td className="px-4 py-2 font-mono text-slate-200 text-xs">{t.latestBuild.envdVersion || "—"}</td></tr>
                                  {t.latestBuild.startCmd && (
                                    <tr><td className="px-4 py-2 text-slate-500">启动命令</td><td className="px-4 py-2 font-mono text-slate-200 text-xs break-all">{t.latestBuild.startCmd}</td></tr>
                                  )}
                                  {t.latestBuild.finishedAt && (
                                    <tr><td className="px-4 py-2 text-slate-500">完成时间</td><td className="px-4 py-2 text-slate-200">{new Date(t.latestBuild.finishedAt).toLocaleString()}</td></tr>
                                  )}
                                </tbody>
                              </table>
                            </div>
                          </div>
                          {t.latestBuild.dockerfile && (
                            <div>
                              <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wide mb-2">Dockerfile</h3>
                              <pre className="bg-slate-900 border border-slate-800 rounded-lg p-4 text-xs font-mono text-slate-300 whitespace-pre-wrap max-h-60 overflow-auto">
                                {t.latestBuild.dockerfile}
                              </pre>
                            </div>
                          )}
                        </>
                      ) : (
                        <div className="text-slate-500 text-sm">暂无构建信息</div>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function VolumesTab() {
  const [volumes, setVolumes] = useState<VolumeRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState("");
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [infoId, setInfoId] = useState("");
  const [infoResult, setInfoResult] = useState<VolumeRow | null>(null);
  const [infoError, setInfoError] = useState<string | null>(null);
  const [infoLoading, setInfoLoading] = useState(false);

  async function refresh() {
    setLoading(true);
    setError(null);
    try {
      const r = await fetch("/api/volumes");
      const d = (await r.json()) as { volumes?: VolumeRow[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setVolumes(d.volumes ?? []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { refresh(); }, []);

  async function createVolume() {
    const name = newName.trim();
    if (!name) return;
    if (!/^[a-zA-Z0-9-]+$/.test(name)) {
      setError("Volume 名称只能包含字母、数字和连字符");
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const r = await fetch("/api/volumes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const d = (await r.json()) as { error?: string };
      if (!r.ok) throw new Error(d.error || "创建失败");
      setNewName("");
      await refresh();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setCreating(false);
    }
  }

  async function destroyVolume(id: string, name: string) {
    if (!confirm(`确认销毁 Volume?\n名称: ${name}\nID: ${id}`)) return;
    setError(null);
    try {
      const r = await fetch(`/api/volumes/${encodeURIComponent(id)}`, { method: "DELETE" });
      const d = (await r.json()) as { error?: string };
      if (!r.ok) throw new Error(d.error || "删除失败");
      if (infoResult?.volumeId === id) setInfoResult(null);
      await refresh();
    } catch (e: any) {
      setError(e.message);
    }
  }

  async function getInfo(id: string) {
    if (!id.trim()) return;
    setInfoLoading(true);
    setInfoError(null);
    setInfoResult(null);
    try {
      const r = await fetch(`/api/volumes/${encodeURIComponent(id.trim())}`);
      const d = (await r.json()) as VolumeRow & { error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setInfoResult({ volumeId: d.volumeId, name: d.name });
    } catch (e: any) {
      setInfoError(e.message);
    } finally {
      setInfoLoading(false);
    }
  }

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(text);
      setTimeout(() => setCopiedId((c) => (c === text ? null : c)), 2000);
    } catch {}
  }

  const filtered = useMemo(() => {
    if (!search.trim()) return volumes;
    const q = search.toLowerCase();
    return volumes.filter(
      (v) => v.name.toLowerCase().includes(q) || v.volumeId.toLowerCase().includes(q)
    );
  }, [volumes, search]);

  return (
    <div className="space-y-6">
      {/* Top bar */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center justify-between">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索 Volume 名称或 ID..."
          className="w-full sm:w-96 bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
        />
        <div className="flex items-center gap-2">
          <span className="text-xs text-slate-500">{volumes.length} 个 Volume</span>
          <button
            onClick={refresh}
            disabled={loading}
            className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 rounded text-xs font-medium transition ml-1"
          >
            {loading ? "..." : "↻ 刷新"}
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-950/50 border border-red-800 text-red-300 rounded-lg p-4 text-sm flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-red-200 shrink-0 ml-3">✕</button>
        </div>
      )}

      {/* Create + GetInfo */}
      <div className="grid md:grid-cols-2 gap-4">
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3">
          <h3 className="text-sm font-semibold text-slate-200">创建 Volume</h3>
          <p className="text-xs text-slate-500">名称只能包含字母、数字和连字符 (a-z, 0-9, -)</p>
          <div className="flex gap-2">
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !creating && createVolume()}
              placeholder="my-volume"
              className="flex-1 bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
            />
            <button
              onClick={createVolume}
              disabled={creating || !newName.trim()}
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-700 disabled:text-slate-500 rounded-lg text-sm font-medium transition"
            >
              {creating ? "创建中..." : "创建"}
            </button>
          </div>
        </div>

        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3">
          <h3 className="text-sm font-semibold text-slate-200">查询 Volume 信息</h3>
          <p className="text-xs text-slate-500">通过 Volume ID 查询详细信息</p>
          <div className="flex gap-2">
            <input
              value={infoId}
              onChange={(e) => setInfoId(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && infoId.trim() && getInfo(infoId)}
              placeholder="volume-id"
              className="flex-1 bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
            />
            <button
              onClick={() => getInfo(infoId)}
              disabled={infoLoading || !infoId.trim()}
              className="px-4 py-2 bg-sky-600 hover:bg-sky-500 disabled:bg-slate-700 disabled:text-slate-500 rounded-lg text-sm font-medium transition"
            >
              {infoLoading ? "..." : "查询"}
            </button>
          </div>
          {infoError && (
            <div className="text-xs text-red-300 bg-red-950/40 border border-red-900/60 rounded-lg px-3 py-2">{infoError}</div>
          )}
          {infoResult && (
            <div className="bg-slate-950/80 border border-slate-800 rounded-lg p-3">
              <table className="w-full text-sm">
                <tbody className="divide-y divide-slate-800">
                  <tr>
                    <td className="py-1.5 pr-3 text-slate-500">Volume ID</td>
                    <td className="py-1.5 font-mono text-slate-200 break-all">{infoResult.volumeId}</td>
                  </tr>
                  <tr>
                    <td className="py-1.5 pr-3 text-slate-500">名称</td>
                    <td className="py-1.5 font-mono text-slate-200">{infoResult.name}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Volume list */}
      <div className="grid gap-3">
        {filtered.length === 0 && !loading && (
          <div className="text-center text-slate-500 py-12">
            {search ? "没有匹配的 Volume" : "暂无 Volume"}
          </div>
        )}
        {filtered.map((v) => (
          <div
            key={v.volumeId}
            className="bg-slate-900 border border-slate-800 rounded-xl px-6 py-4 flex items-center gap-4 hover:border-slate-700 transition"
          >
            <div className="w-10 h-10 rounded-lg bg-cyan-900/30 border border-cyan-800/40 flex items-center justify-center text-cyan-300 text-lg shrink-0">
              V
            </div>
            <div className="flex-1 min-w-0">
              <div className="font-mono text-sm text-slate-100 font-medium truncate">{v.name}</div>
              <div className="font-mono text-xs text-slate-500 mt-0.5 break-all">{v.volumeId}</div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                onClick={() => copyText(v.volumeId)}
                className="px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
              >
                {copiedId === v.volumeId ? "已复制" : "复制 ID"}
              </button>
              <button
                onClick={() => { setInfoId(v.volumeId); getInfo(v.volumeId); }}
                className="px-3 py-1.5 rounded-lg text-xs bg-sky-800 hover:bg-sky-700 text-slate-100 transition"
              >
                详情
              </button>
              <button
                onClick={() => destroyVolume(v.volumeId, v.name)}
                className="px-3 py-1.5 rounded-lg text-xs bg-red-900/40 hover:bg-red-800/60 text-red-300 transition"
              >
                销毁
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ApiKeysTab() {
  const [apiKeys, setApiKeys] = useState<ApiKeyRow[]>([]);
  const [accessTokens, setAccessTokens] = useState<AccessTokenRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [subTab, setSubTab] = useState<"apikeys" | "access_tokens">("apikeys");
  const [copiedId, setCopiedId] = useState<string | null>(null);

  async function refreshKeys() {
    setLoading(true);
    setError(null);
    try {
      const r = await fetch("/api/api-keys");
      const text = await r.text();
      if (!text) throw new Error("空响应");
      const d = JSON.parse(text) as { keys?: ApiKeyRow[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setApiKeys(d.keys ?? []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  async function refreshTokens() {
    setLoading(true);
    setError(null);
    try {
      const r = await fetch("/api/access-tokens");
      const text = await r.text();
      if (!text) throw new Error("空响应");
      const d = JSON.parse(text) as { tokens?: AccessTokenRow[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setAccessTokens(d.tokens ?? []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refreshKeys();
    refreshTokens();
  }, []);

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(text);
      setTimeout(() => setCopiedId((c) => (c === text ? null : c)), 2000);
    } catch {}
  }

  const filteredKeys = useMemo(() => {
    if (!search.trim()) return apiKeys;
    const q = search.toLowerCase();
    return apiKeys.filter(
      (k) =>
        k.name.toLowerCase().includes(q) ||
        k.maskedKey.toLowerCase().includes(q) ||
        k.teamId.toLowerCase().includes(q) ||
        (k.teamName ?? "").toLowerCase().includes(q)
    );
  }, [apiKeys, search]);

  const filteredTokens = useMemo(() => {
    if (!search.trim()) return accessTokens;
    const q = search.toLowerCase();
    return accessTokens.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.maskedToken.toLowerCase().includes(q) ||
        (t.userEmail ?? "").toLowerCase().includes(q)
    );
  }, [accessTokens, search]);

  function relativeTime(dateStr: string | null): string {
    if (!dateStr) return "从未";
    const diff = Date.now() - Date.parse(dateStr);
    if (diff < 60000) return "刚刚";
    if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;
    return `${Math.floor(diff / 86400000)} 天前`;
  }

  return (
    <div className="space-y-6">
      {/* Top bar */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center justify-between">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索名称、Key、Team..."
          className="w-full sm:w-96 bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 text-sm font-mono focus:outline-none focus:border-emerald-500 placeholder:text-slate-600"
        />
        <div className="flex items-center gap-2">
          <button
            onClick={() => { refreshKeys(); refreshTokens(); }}
            disabled={loading}
            className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 rounded text-xs font-medium transition"
          >
            {loading ? "..." : "↻ 刷新"}
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-950/50 border border-red-800 text-red-300 rounded-lg p-4 text-sm flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-red-200 shrink-0 ml-3">✕</button>
        </div>
      )}

      {/* Sub tabs */}
      <div className="flex items-center gap-1 bg-slate-800/60 rounded-lg p-1 w-fit">
        <button
          onClick={() => setSubTab("apikeys")}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
            subTab === "apikeys" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          API Keys <span className="text-xs text-slate-500 ml-1">{apiKeys.length}</span>
        </button>
        <button
          onClick={() => setSubTab("access_tokens")}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
            subTab === "access_tokens" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
          }`}
        >
          Access Tokens <span className="text-xs text-slate-500 ml-1">{accessTokens.length}</span>
        </button>
      </div>

      {/* API Keys list */}
      {subTab === "apikeys" && (
        <div className="space-y-4">
          <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-4 text-xs text-slate-400 space-y-1">
            <p><span className="text-slate-200 font-medium">API Key</span> 用于 SDK 认证，通过 <code className="text-emerald-400/80">E2B_API_KEY</code> 环境变量或直接传入 Sandbox 构造函数。</p>
            <p>每个 Key 关联一个 Team，Key 的原始值仅在创建时可见，此处显示掩码。</p>
          </div>

          <div className="grid gap-3">
            {filteredKeys.length === 0 && !loading && (
              <div className="text-center text-slate-500 py-12">{search ? "没有匹配的 API Key" : "暂无 API Key"}</div>
            )}
            {filteredKeys.map((k) => (
              <div key={k.id} className="bg-slate-900 border border-slate-800 rounded-xl px-6 py-4 hover:border-slate-700 transition">
                <div className="flex items-start gap-4">
                  <div className="w-10 h-10 rounded-lg bg-emerald-900/30 border border-emerald-800/40 flex items-center justify-center text-emerald-300 text-sm font-bold shrink-0">
                    K
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 flex-wrap">
                      <span className="text-sm text-slate-100 font-medium">{k.name}</span>
                      <code className="text-xs px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">{k.maskedKey}</code>
                    </div>
                    <div className="flex flex-wrap gap-x-5 gap-y-1 mt-2 text-xs text-slate-500">
                      <span>Team: <span className="text-slate-300">{k.teamName || k.teamId}</span></span>
                      <span>创建于 <span className="text-slate-400">{new Date(k.createdAt).toLocaleString()}</span></span>
                      <span>最近使用 <span className={k.lastUsed ? "text-emerald-400/80" : "text-slate-600"}>{relativeTime(k.lastUsed)}</span></span>
                      <span>长度 <span className="text-slate-400">{k.keyLength}</span></span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={() => copyText(k.id)}
                      className="px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
                    >
                      {copiedId === k.id ? "已复制" : "复制 ID"}
                    </button>
                    <button
                      onClick={() => copyText(k.maskedKey)}
                      className="px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
                    >
                      {copiedId === k.maskedKey ? "已复制" : "复制 Key"}
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Access Tokens list */}
      {subTab === "access_tokens" && (
        <div className="space-y-4">
          <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-4 text-xs text-slate-400 space-y-1">
            <p><span className="text-slate-200 font-medium">Access Token</span> 仅用于 CLI 认证，通过 <code className="text-emerald-400/80">E2B_ACCESS_TOKEN</code> 环境变量设置，适用于 CI/CD 场景。</p>
            <p>每个 Token 关联一个用户，Token 原始值仅在创建时可见。</p>
          </div>

          <div className="grid gap-3">
            {filteredTokens.length === 0 && !loading && (
              <div className="text-center text-slate-500 py-12">{search ? "没有匹配的 Access Token" : "暂无 Access Token"}</div>
            )}
            {filteredTokens.map((t) => (
              <div key={t.id} className="bg-slate-900 border border-slate-800 rounded-xl px-6 py-4 hover:border-slate-700 transition">
                <div className="flex items-start gap-4">
                  <div className="w-10 h-10 rounded-lg bg-sky-900/30 border border-sky-800/40 flex items-center justify-center text-sky-300 text-sm font-bold shrink-0">
                    T
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 flex-wrap">
                      <span className="text-sm text-slate-100 font-medium">{t.name}</span>
                      <code className="text-xs px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">{t.maskedToken}</code>
                    </div>
                    <div className="flex flex-wrap gap-x-5 gap-y-1 mt-2 text-xs text-slate-500">
                      <span>用户: <span className="text-slate-300">{t.userEmail || t.userId}</span></span>
                      <span>创建于 <span className="text-slate-400">{new Date(t.createdAt).toLocaleString()}</span></span>
                      <span>长度 <span className="text-slate-400">{t.tokenLength}</span></span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={() => copyText(t.id)}
                      className="px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 transition"
                    >
                      {copiedId === t.id ? "已复制" : "复制 ID"}
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

type TableInfo = { table_schema: string; table_name: string };
type ColumnInfo = { column_name: string; data_type: string; is_nullable: string };

function DatabaseTab() {
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTable, setSelectedTable] = useState<{ schema: string; name: string } | null>(null);
  const [columns, setColumns] = useState<ColumnInfo[]>([]);
  const [rows, setRows] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [tableLoading, setTableLoading] = useState(false);
  const limit = 50;

  async function refreshTables() {
    setLoading(true);
    setError(null);
    try {
      const r = await fetch("/api/db-tables");
      const text = await r.text();
      if (!text) throw new Error("空响应");
      const d = JSON.parse(text) as { tables?: TableInfo[]; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setTables(d.tables ?? []);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  async function loadTable(schema: string, name: string, newOffset = 0) {
    setTableLoading(true);
    setError(null);
    try {
      const r = await fetch(`/api/db-table/${encodeURIComponent(schema)}/${encodeURIComponent(name)}?limit=${limit}&offset=${newOffset}`);
      const text = await r.text();
      if (!text) throw new Error("空响应");
      const d = JSON.parse(text) as { columns?: ColumnInfo[]; rows?: any[]; total?: number; error?: string };
      if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
      setSelectedTable({ schema, name });
      setColumns(d.columns ?? []);
      setRows(d.rows ?? []);
      setTotal(d.total ?? 0);
      setOffset(newOffset);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setTableLoading(false);
    }
  }

  useEffect(() => { refreshTables(); }, []);

  const grouped = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const t of tables) {
      const list = map.get(t.table_schema) ?? [];
      list.push(t.table_name);
      map.set(t.table_schema, list);
    }
    return map;
  }, [tables]);

  function formatCell(value: any): string {
    if (value === null || value === undefined) return "NULL";
    if (typeof value === "object") return JSON.stringify(value);
    return String(value);
  }

  const totalPages = Math.ceil(total / limit);
  const currentPage = Math.floor(offset / limit) + 1;

  return (
    <div className="flex gap-6 min-h-[calc(100vh-120px)]">
      {/* Sidebar: table list */}
      <div className="w-64 shrink-0 bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col">
        <div className="px-4 py-3 border-b border-slate-800 flex items-center justify-between">
          <span className="text-sm font-medium text-slate-200">Tables</span>
          <button
            onClick={refreshTables}
            disabled={loading}
            className="text-xs text-slate-400 hover:text-slate-200"
          >
            {loading ? "..." : "↻"}
          </button>
        </div>
        <div className="flex-1 overflow-auto">
          {Array.from(grouped.entries()).map(([schema, tableNames]) => (
            <div key={schema}>
              <div className="px-4 py-1.5 text-[10px] uppercase tracking-wider text-slate-500 bg-slate-950/50 sticky top-0">
                {schema}
              </div>
              {tableNames.map((name) => {
                const isSelected = selectedTable?.schema === schema && selectedTable?.name === name;
                return (
                  <button
                    key={`${schema}.${name}`}
                    onClick={() => loadTable(schema, name)}
                    className={`w-full text-left px-4 py-2 text-xs font-mono truncate transition ${
                      isSelected
                        ? "bg-emerald-900/30 text-emerald-300 border-l-2 border-emerald-500"
                        : "text-slate-300 hover:bg-slate-800/60 border-l-2 border-transparent"
                    }`}
                  >
                    {name}
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Main: table data */}
      <div className="flex-1 min-w-0 space-y-4">
        {error && (
          <div className="bg-red-950/50 border border-red-800 text-red-300 rounded-lg p-3 text-sm flex items-center justify-between">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="text-red-400 hover:text-red-200 ml-3">✕</button>
          </div>
        )}

        {!selectedTable && (
          <div className="text-center text-slate-500 py-20">
            <p className="text-lg">选择左侧的表查看数据</p>
            <p className="text-sm mt-2">{tables.length} 个表</p>
          </div>
        )}

        {selectedTable && (
          <>
            {/* Header */}
            <div className="flex items-center justify-between flex-wrap gap-3">
              <div>
                <h3 className="text-lg font-semibold text-slate-100">
                  <span className="text-slate-500">{selectedTable.schema}.</span>{selectedTable.name}
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  {total} 行 · {columns.length} 列
                  {tableLoading && <span className="ml-2 text-amber-400">加载中...</span>}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => loadTable(selectedTable.schema, selectedTable.name, offset)}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 rounded text-xs font-medium transition"
                >
                  ↻ 刷新
                </button>
              </div>
            </div>

            {/* Columns info */}
            <details className="bg-slate-900 border border-slate-800 rounded-lg">
              <summary className="px-4 py-2.5 text-xs text-slate-400 cursor-pointer hover:text-slate-200">
                列定义 ({columns.length} 列)
              </summary>
              <div className="px-4 pb-3 flex flex-wrap gap-2">
                {columns.map((c) => (
                  <span key={c.column_name} className="text-[11px] px-2 py-1 rounded bg-slate-800 text-slate-300 font-mono">
                    {c.column_name} <span className="text-slate-500">{c.data_type}</span>
                    {c.is_nullable === "YES" && <span className="text-slate-600 ml-0.5">?</span>}
                  </span>
                ))}
              </div>
            </details>

            {/* Table data */}
            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="overflow-auto max-h-[60vh]">
                <table className="w-full text-xs">
                  <thead className="sticky top-0 bg-slate-800/90 backdrop-blur z-10">
                    <tr>
                      <th className="px-3 py-2 text-left text-slate-400 font-medium w-10">#</th>
                      {columns.map((c) => (
                        <th key={c.column_name} className="px-3 py-2 text-left text-slate-400 font-medium font-mono whitespace-nowrap">
                          {c.column_name}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60">
                    {rows.map((row, i) => (
                      <tr key={i} className="hover:bg-slate-800/30">
                        <td className="px-3 py-2 text-slate-600">{offset + i + 1}</td>
                        {columns.map((c) => (
                          <td key={c.column_name} className="px-3 py-2 text-slate-200 font-mono whitespace-nowrap max-w-[300px] truncate" title={formatCell(row[c.column_name])}>
                            {row[c.column_name] === null ? (
                              <span className="text-slate-600 italic">NULL</span>
                            ) : typeof row[c.column_name] === "object" ? (
                              <span className="text-amber-300/80">{JSON.stringify(row[c.column_name])}</span>
                            ) : (
                              formatCell(row[c.column_name])
                            )}
                          </td>
                        ))}
                      </tr>
                    ))}
                    {rows.length === 0 && (
                      <tr><td colSpan={columns.length + 1} className="px-4 py-8 text-center text-slate-500">空表</td></tr>
                    )}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              {total > limit && (
                <div className="flex items-center justify-between px-4 py-3 border-t border-slate-800">
                  <span className="text-xs text-slate-500">
                    第 {currentPage} / {totalPages} 页
                  </span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => loadTable(selectedTable.schema, selectedTable.name, Math.max(0, offset - limit))}
                      disabled={offset === 0 || tableLoading}
                      className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded text-xs transition"
                    >
                      上一页
                    </button>
                    <button
                      onClick={() => loadTable(selectedTable.schema, selectedTable.name, offset + limit)}
                      disabled={offset + limit >= total || tableLoading}
                      className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded text-xs transition"
                    >
                      下一页
                    </button>
                  </div>
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function Admin() {
  const [tab, setTab] = useState<Tab>("templates");

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800 bg-slate-900/80 sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-xl font-bold">E2B 管理后台</h1>
            <a href="/" className="text-xs text-slate-400 hover:text-slate-200 border border-slate-700 rounded px-2 py-1">
              ← Sandbox 控制台
            </a>
          </div>
          <div className="flex items-center gap-1 bg-slate-800/60 rounded-lg p-1">
            <button
              onClick={() => setTab("templates")}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
                tab === "templates" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              模板
            </button>
            <button
              onClick={() => setTab("volumes")}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
                tab === "volumes" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              Volumes
            </button>
            <button
              onClick={() => setTab("apikeys")}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
                tab === "apikeys" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              API Keys
            </button>
            <button
              onClick={() => setTab("database")}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition ${
                tab === "database" ? "bg-slate-700 text-slate-100 shadow" : "text-slate-400 hover:text-slate-200"
              }`}
            >
              数据库
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-6">
        {tab === "templates" && <TemplatesTab />}
        {tab === "volumes" && <VolumesTab />}
        {tab === "apikeys" && <ApiKeysTab />}
        {tab === "database" && <DatabaseTab />}
      </main>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<Admin />);
