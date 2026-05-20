"use client";

import { Cloud, CloudUpload, CloudDownload, Pause, Lock } from "lucide-react";
import type { FileNode } from "@/lib/types";
import { formatBytes } from "@/lib/format";

interface StatusBarProps {
  itemCount: number;
  selected: FileNode | null;
  queue: { uploads: number; downloads: number; bandwidth: string };
  paused: boolean;
  onTogglePause: () => void;
}

export function StatusBar({ itemCount, selected, queue, paused, onTogglePause }: StatusBarProps) {
  return (
    <div className="flex h-7 shrink-0 items-center justify-between border-t border-border/60 bg-muted/30 px-3 text-[11px] text-muted-foreground">
      <div className="flex items-center gap-3">
        <span>
          {selected
            ? `${selected.name} — ${formatBytes(selected.size)}`
            : `${itemCount} item${itemCount === 1 ? "" : "s"}`}
        </span>
        <span className="inline-flex items-center gap-1">
          <Lock className="size-3 text-emerald-500" /> Client-side encrypted
        </span>
      </div>
      <div className="flex items-center gap-3">
        <span className="inline-flex items-center gap-1">
          <CloudUpload className="size-3 text-sky-500" /> {queue.uploads}
        </span>
        <span className="inline-flex items-center gap-1">
          <CloudDownload className="size-3 text-sky-500" /> {queue.downloads}
        </span>
        <span className="inline-flex items-center gap-1">
          <Cloud className="size-3" /> {queue.bandwidth}
        </span>
        <button
          type="button"
          onClick={onTogglePause}
          className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 hover:bg-accent hover:text-accent-foreground"
        >
          <Pause className="size-3" /> {paused ? "Paused" : "Pause"}
        </button>
      </div>
    </div>
  );
}
