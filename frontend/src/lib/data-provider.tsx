"use client";

import * as React from "react";
import { api, captureTokenFromURL, subscribeEvents, type Status } from "./api";
import type { FileNode, Machine } from "./types";

interface DataContextValue {
  ready: boolean;
  error: string | null;
  machines: Machine[];
  trees: Record<string, FileNode>;
  status: Status | null;
  refresh: () => Promise<void>;
}

const DataContext = React.createContext<DataContextValue | null>(null);

export function DataProvider({ children }: { children: React.ReactNode }) {
  const [machines, setMachines] = React.useState<Machine[]>([]);
  const [trees, setTrees] = React.useState<Record<string, FileNode>>({});
  const [status, setStatus] = React.useState<Status | null>(null);
  const [ready, setReady] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const load = React.useCallback(async () => {
    try {
      const [ms, st] = await Promise.all([api.machines(), api.status()]);
      setMachines(ms);
      setStatus(st);
      const trees: Record<string, FileNode> = {};
      await Promise.all(
        ms.map(async (m) => {
          trees[m.id] = await api.tree(m.id, "/");
        }),
      );
      setTrees(trees);
      setReady(true);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setReady(true);
    }
  }, []);

  React.useEffect(() => {
    captureTokenFromURL();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- initial fetch from the dropboy daemon (external system).
    void load();
  }, [load]);

  React.useEffect(() => {
    if (!ready) return;
    const stop = subscribeEvents((ev) => {
      if (ev.type === "status" && ev.payload) {
        setStatus((s) => ({ ...(s ?? ({} as Status)), ...(ev.payload as Status) }));
      }
      if (ev.type === "unlocked") {
        void load();
      }
    });
    return stop;
  }, [ready, load]);

  const value = React.useMemo<DataContextValue>(
    () => ({ ready, error, machines, trees, status, refresh: load }),
    [ready, error, machines, trees, status, load],
  );

  return <DataContext.Provider value={value}>{children}</DataContext.Provider>;
}

export function useData(): DataContextValue {
  const ctx = React.useContext(DataContext);
  if (!ctx) throw new Error("useData must be used inside <DataProvider>");
  return ctx;
}

export function findByPath(root: FileNode | undefined, path: string): FileNode | null {
  if (!root) return null;
  if (path === "/" || path === "") return root;
  const segs = path.split("/").filter(Boolean);
  let cur: FileNode | undefined = root;
  for (const seg of segs) {
    cur = cur?.children?.find((c) => c.name === seg);
    if (!cur) return null;
  }
  return cur;
}

export function flatten(node: FileNode, out: FileNode[] = []): FileNode[] {
  out.push(node);
  node.children?.forEach((c) => flatten(c, out));
  return out;
}
