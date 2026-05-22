"use client";

import * as React from "react";
import { Sidebar } from "./sidebar";
import { Toolbar, type SortKey, type ViewMode } from "./toolbar";
import { StatusBar } from "./status-bar";
import { ListView } from "./views/list-view";
import { GridView } from "./views/grid-view";
import { ColumnView } from "./views/column-view";
import { CommandPalette } from "./command-palette";
import { ManagePanel } from "./manage-panel";
import { UnlockDialog } from "./unlock-dialog";
import { findByPath, flatten, useData } from "@/lib/data-provider";
import { api } from "@/lib/api";
import type { FileNode } from "@/lib/types";

type Loc = { machine: string; path: string };

function sortFiles(files: FileNode[], sort: SortKey, ascending: boolean): FileNode[] {
  const dir = ascending ? 1 : -1;
  const folders = files.filter((f) => f.kind === "folder");
  const others = files.filter((f) => f.kind !== "folder");
  const cmp = (a: FileNode, b: FileNode) => {
    switch (sort) {
      case "name":
        return a.name.localeCompare(b.name) * dir;
      case "modified":
        return (+new Date(a.modified) - +new Date(b.modified)) * dir;
      case "size":
        return (a.size - b.size) * dir;
      case "kind":
        return a.kind.localeCompare(b.kind) * dir;
    }
  };
  return [...folders.sort(cmp), ...others.sort(cmp)];
}

export function FinderShell() {
  const { machines, trees, status, ready, error } = useData();
  const primary = machines[0]?.id ?? "";
  const [history, setHistory] = React.useState<Loc[]>([{ machine: "", path: "/" }]);
  const [historyIdx, setHistoryIdx] = React.useState(0);

  const rawCurrent = history[historyIdx];
  const current: Loc = React.useMemo(
    () =>
      rawCurrent?.machine
        ? rawCurrent
        : { machine: primary, path: rawCurrent?.path ?? "/" },
    [rawCurrent, primary],
  );

  const [view, setView] = React.useState<ViewMode>("columns");
  const [sort, setSort] = React.useState<SortKey>("name");
  const [ascending, setAscending] = React.useState(true);
  const [query, setQuery] = React.useState("");
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [paletteOpen, setPaletteOpen] = React.useState(false);
  const [manageOpen, setManageOpen] = React.useState(false);
  const [paused, setPaused] = React.useState(false);

  const navigate = React.useCallback((loc: Loc) => {
    setHistory((h) => {
      const next = h.slice(0, historyIdx + 1);
      next.push(loc);
      return next;
    });
    setHistoryIdx((i) => i + 1);
    setSelectedId(null);
  }, [historyIdx]);

  const goBack = () => {
    if (historyIdx > 0) {
      setHistoryIdx((i) => i - 1);
      setSelectedId(null);
    }
  };
  const goForward = () => {
    if (historyIdx < history.length - 1) {
      setHistoryIdx((i) => i + 1);
      setSelectedId(null);
    }
  };

  // Resolve current directory node + listing.
  const allFilesForMachine = React.useMemo(() => {
    if (current.machine === "__all" || current.machine === "__recents") {
      return Object.values(trees).flatMap((t) => flatten(t)).filter((n) => n.kind !== "folder");
    }
    if (current.machine === "__conflicts") {
      return Object.values(trees)
        .flatMap((t) => flatten(t))
        .filter((n) => n.status === "conflict");
    }
    if (current.machine === "__trash") {
      return [];
    }
    const root = trees[current.machine];
    return root ? flatten(root) : [];
  }, [current.machine, trees]);

  const currentDir: FileNode | null = React.useMemo(() => {
    if (current.machine.startsWith("__")) {
      return {
        id: `virtual-${current.machine}`,
        name: virtualLabel(current.machine),
        kind: "folder",
        size: 0,
        modified: new Date().toISOString(),
        machine: current.machine,
        path: "/",
        status: "synced",
        encrypted: false,
        children: allFilesForMachine,
      };
    }
    return findByPath(trees[current.machine], current.path);
  }, [current, allFilesForMachine, trees]);

  const listing = React.useMemo(() => {
    if (!currentDir?.children) return [];
    const items = query
      ? currentDir.children.filter((f) => f.name.toLowerCase().includes(query.toLowerCase()))
      : currentDir.children;
    return sortFiles(items, sort, ascending);
  }, [currentDir, query, sort, ascending]);

  const selected = listing.find((f) => f.id === selectedId) ?? null;

  // Breadcrumbs
  const breadcrumbs = React.useMemo(() => {
    if (current.machine.startsWith("__")) {
      return [{ label: virtualLabel(current.machine) }];
    }
    const m = machines.find((mm) => mm.id === current.machine);
    const segs = current.path.split("/").filter(Boolean);
    const crumbs: { label: string; onClick?: () => void }[] = [
      { label: m?.label ?? current.machine, onClick: () => navigate({ machine: current.machine, path: "/" }) },
    ];
    segs.forEach((seg, i) => {
      const subPath = "/" + segs.slice(0, i + 1).join("/");
      crumbs.push({
        label: seg,
        onClick: i === segs.length - 1 ? undefined : () => navigate({ machine: current.machine, path: subPath }),
      });
    });
    return crumbs;
  }, [current, navigate, machines]);

  const openItem = React.useCallback(
    (file: FileNode) => {
      if (file.kind === "folder") {
        const newPath = current.path === "/" ? `/${file.name}` : `${current.path}/${file.name}`;
        navigate({ machine: current.machine, path: newPath });
      } else {
        setSelectedId(file.id);
        if (typeof window !== "undefined") {
          window.open(api.fileURL(file.machine, file.path), "_blank", "noopener");
        }
      }
    },
    [current, navigate],
  );

  const navigateToFile = React.useCallback(
    (file: FileNode) => {
      const parts = file.path.split("/").filter(Boolean);
      const dirSegs = file.kind === "folder" ? parts : parts.slice(0, -1);
      navigate({ machine: file.machine, path: "/" + dirSegs.join("/") });
      setSelectedId(file.id);
    },
    [navigate],
  );

  // Keyboard shortcuts
  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      const meta = e.metaKey || e.ctrlKey;
      if (meta && e.key === "k") {
        e.preventDefault();
        setPaletteOpen((o) => !o);
      } else if (meta && e.key === "[") {
        e.preventDefault();
        goBack();
      } else if (meta && e.key === "]") {
        e.preventDefault();
        goForward();
      } else if (meta && e.key === "1") {
        e.preventDefault();
        setView("icons");
      } else if (meta && e.key === "2") {
        e.preventDefault();
        setView("list");
      } else if (meta && e.key === "3") {
        e.preventDefault();
        setView("columns");
      }
    };
    window.addEventListener("keydown", down);
    return () => window.removeEventListener("keydown", down);
  });

  // Column view path segments relative to machine root.
  const pathSegments = React.useMemo(
    () => current.path.split("/").filter(Boolean),
    [current.path],
  );

  const machineRoot = !current.machine.startsWith("__") ? trees[current.machine] : null;

  const allFilesForPalette = React.useMemo(
    () => Object.values(trees).flatMap((t) => flatten(t)).filter((n) => n.kind !== "folder"),
    [trees],
  );

  if (!ready) {
    return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">Connecting to dropboy daemon…</div>;
  }
  if (error) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-2 p-8 text-center">
        <p className="text-sm font-medium">Can&apos;t reach the dropboy daemon.</p>
        <p className="max-w-md text-xs text-muted-foreground">{error}</p>
        <p className="text-xs text-muted-foreground">Start it with <code>dropboy ui --open</code> and reload this page.</p>
      </div>
    );
  }

  return (
    <div className="flex h-screen w-full flex-col bg-background text-foreground">
      <Toolbar
        canBack={historyIdx > 0}
        canForward={historyIdx < history.length - 1}
        onBack={goBack}
        onForward={goForward}
        view={view}
        onView={setView}
        sort={sort}
        onSort={(k) => setSort(k)}
        ascending={ascending}
        onToggleAscending={() => setAscending((a) => !a)}
        query={query}
        onQuery={setQuery}
        breadcrumbs={breadcrumbs}
        onOpenPalette={() => setPaletteOpen(true)}
        onOpenManage={() => setManageOpen(true)}
      />

      <div className="flex min-h-0 flex-1">
        <Sidebar current={current} onNavigate={navigate} />

        <main className="flex min-w-0 flex-1 flex-col">
          {view === "columns" && machineRoot ? (
            <ColumnView
              root={machineRoot}
              pathSegments={pathSegments}
              selectedLeafId={selectedId}
              onNavigate={(segs) => navigate({ machine: current.machine, path: "/" + segs.join("/") })}
              onSelectLeaf={setSelectedId}
              onOpen={openItem}
            />
          ) : view === "list" || !machineRoot ? (
            <ListView
              files={listing}
              selected={selectedId}
              onSelect={setSelectedId}
              onOpen={openItem}
              sort={sort}
              ascending={ascending}
              onSort={(k) => setSort(k)}
            />
          ) : (
            <GridView
              files={listing}
              selected={selectedId}
              onSelect={setSelectedId}
              onOpen={openItem}
            />
          )}
        </main>
      </div>

      <StatusBar
        itemCount={listing.length}
        selected={selected}
        queue={{
          uploads: status?.queueUploads ?? 0,
          downloads: status?.queueDownloads ?? 0,
          bandwidth: paused ? "0 KB/s" : status ? `${Math.round(status.bytesUp / 1024)} KB/s` : "—",
        }}
        paused={paused || (status?.paused ?? false)}
        onTogglePause={() => {
          setPaused((p) => !p);
          void (paused ? api.resume() : api.pause());
        }}
      />

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        allFiles={allFilesForPalette}
        onNavigate={navigateToFile}
      />

      <ManagePanel open={manageOpen} onOpenChange={setManageOpen} />
      <UnlockDialog />
    </div>
  );
}

function virtualLabel(id: string): string {
  switch (id) {
    case "__all":
      return "All Devices";
    case "__recents":
      return "Recents";
    case "__conflicts":
      return "Conflicts";
    case "__trash":
      return "Trash";
    default:
      return id;
  }
}
