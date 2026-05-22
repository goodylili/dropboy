"use client";

import * as React from "react";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { api, type Conflict, type Folder } from "@/lib/api";
import { useData } from "@/lib/data-provider";

interface ManagePanelProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}

export function ManagePanel({ open, onOpenChange }: ManagePanelProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[480px] sm:max-w-[480px]">
        <SheetHeader>
          <SheetTitle>Manage</SheetTitle>
          <SheetDescription>Folders, conflicts, and daemon settings.</SheetDescription>
        </SheetHeader>
        <Tabs defaultValue="folders" className="mt-4">
          <TabsList className="grid w-full grid-cols-4">
            <TabsTrigger value="folders">Folders</TabsTrigger>
            <TabsTrigger value="conflicts">Conflicts</TabsTrigger>
            <TabsTrigger value="restore">Restore</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>
          <TabsContent value="folders" className="mt-4">
            <FoldersTab />
          </TabsContent>
          <TabsContent value="conflicts" className="mt-4">
            <ConflictsTab />
          </TabsContent>
          <TabsContent value="restore" className="mt-4">
            <RestoreTab />
          </TabsContent>
          <TabsContent value="settings" className="mt-4">
            <SettingsTab />
          </TabsContent>
        </Tabs>
      </SheetContent>
    </Sheet>
  );
}

function FoldersTab() {
  const { refresh } = useData();
  const [folders, setFolders] = React.useState<Folder[]>([]);
  const [adding, setAdding] = React.useState("");
  const [excludes, setExcludes] = React.useState("");
  const [err, setErr] = React.useState<string | null>(null);

  const load = React.useCallback(async () => {
    try {
      setFolders(await api.folders());
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, []);

  React.useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetches from the dropboy daemon (external system).
    void load();
  }, [load]);

  const addFolder = async () => {
    if (!adding) return;
    try {
      await api.addFolder({
        path: adding,
        exclude: excludes
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setAdding("");
      setExcludes("");
      await load();
      await refresh();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  };

  const removeFolder = async (p: string) => {
    try {
      await api.removeFolder(p);
      await load();
      await refresh();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Input placeholder="/absolute/path/to/folder" value={adding} onChange={(e) => setAdding(e.target.value)} />
        <Input
          placeholder="exclude patterns (comma separated, e.g. node_modules/**, *.log)"
          value={excludes}
          onChange={(e) => setExcludes(e.target.value)}
        />
        <Button onClick={addFolder} disabled={!adding}>Add folder</Button>
      </div>
      <Separator />
      <ul className="space-y-1 text-sm">
        {folders.length === 0 && <li className="text-muted-foreground">No folders watched yet.</li>}
        {folders.map((f) => (
          <li key={f.path} className="flex items-center justify-between gap-2 rounded border border-border/60 p-2">
            <div className="min-w-0">
              <div className="truncate font-mono text-xs">{f.path}</div>
              {f.exclude && f.exclude.length > 0 && (
                <div className="truncate text-[11px] text-muted-foreground">exclude: {f.exclude.join(", ")}</div>
              )}
            </div>
            <Button size="sm" variant="ghost" onClick={() => removeFolder(f.path)}>Remove</Button>
          </li>
        ))}
      </ul>
      {err && <p className="text-xs text-red-500">{err}</p>}
    </div>
  );
}

function ConflictsTab() {
  const [conflicts, setConflicts] = React.useState<Conflict[]>([]);
  const [err, setErr] = React.useState<string | null>(null);

  const load = React.useCallback(async () => {
    try {
      setConflicts(await api.conflicts());
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, []);

  React.useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetches from the dropboy daemon (external system).
    void load();
  }, [load]);

  const resolve = async (id: string, res: "local" | "remote" | "both") => {
    try {
      await api.resolveConflict(id, res);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  };

  if (conflicts.length === 0) {
    return <p className="text-sm text-muted-foreground">No conflicts. ✓</p>;
  }
  return (
    <ul className="space-y-2">
      {conflicts.map((c) => (
        <li key={c.id} className="space-y-2 rounded border border-border/60 p-2">
          <div className="font-mono text-xs">{c.path}</div>
          <div className="text-[11px] text-muted-foreground">
            local: {c.local} · remote: {c.remote} · detected {c.detected}
          </div>
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => resolve(c.id, "local")}>Keep local</Button>
            <Button size="sm" variant="outline" onClick={() => resolve(c.id, "remote")}>Keep remote</Button>
            <Button size="sm" variant="outline" onClick={() => resolve(c.id, "both")}>Keep both</Button>
          </div>
        </li>
      ))}
      {err && <p className="text-xs text-red-500">{err}</p>}
    </ul>
  );
}

function RestoreTab() {
  const { machines } = useData();
  const [machine, setMachine] = React.useState(machines[0]?.id ?? "");
  const [path, setPath] = React.useState("/");
  const [into, setInto] = React.useState("");
  const [status, setStatus] = React.useState<string | null>(null);
  const [err, setErr] = React.useState<string | null>(null);

  const submit = async () => {
    try {
      setErr(null);
      const r = await api.restore({ machine, path, into: into || undefined });
      setStatus(r.status);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <div className="space-y-3">
      <label className="block space-y-1 text-sm">
        <span>Machine</span>
        <select
          className="w-full rounded border border-border/60 bg-background p-2 text-sm"
          value={machine}
          onChange={(e) => setMachine(e.target.value)}
        >
          {machines.map((m) => (
            <option key={m.id} value={m.id}>{m.label}</option>
          ))}
        </select>
      </label>
      <Input placeholder="Subtree (e.g. /Documents/Contracts)" value={path} onChange={(e) => setPath(e.target.value)} />
      <Input placeholder="Destination (blank = original paths)" value={into} onChange={(e) => setInto(e.target.value)} />
      <Button onClick={submit} disabled={!machine}>Start restore</Button>
      {status && <p className="text-xs text-muted-foreground">Status: {status}</p>}
      {err && <p className="text-xs text-red-500">{err}</p>}
    </div>
  );
}

function SettingsTab() {
  const { status } = useData();
  if (!status) return <p className="text-sm text-muted-foreground">Daemon offline.</p>;
  return (
    <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
      <dt className="text-muted-foreground">Bucket</dt><dd className="font-mono text-xs">{status.bucket || "—"}</dd>
      <dt className="text-muted-foreground">Region</dt><dd className="font-mono text-xs">{status.region || "—"}</dd>
      <dt className="text-muted-foreground">Machine</dt><dd className="font-mono text-xs">{status.machineId || "—"}</dd>
      <dt className="text-muted-foreground">Running</dt><dd>{String(status.running)}</dd>
      <dt className="text-muted-foreground">Paused</dt><dd>{String(status.paused)}</dd>
      <dt className="text-muted-foreground">Queue ↑</dt><dd>{status.queueUploads}</dd>
      <dt className="text-muted-foreground">Queue ↓</dt><dd>{status.queueDownloads}</dd>
      <dt className="text-muted-foreground">Bytes ↑</dt><dd>{status.bytesUp}</dd>
      <dt className="text-muted-foreground">Bytes ↓</dt><dd>{status.bytesDown}</dd>
      <dt className="text-muted-foreground">Conflicts</dt><dd>{status.conflicts}</dd>
      <dt className="text-muted-foreground">Last sync</dt><dd className="text-xs">{status.lastSyncAt}</dd>
    </dl>
  );
}
