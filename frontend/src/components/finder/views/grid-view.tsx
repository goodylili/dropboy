"use client";

import * as React from "react";
import type { FileNode } from "@/lib/types";
import { cn } from "@/lib/utils";
import { FileIcon } from "../file-icon";
import { SyncIndicator } from "../status-indicator";

interface GridViewProps {
  files: FileNode[];
  selected: string | null;
  onSelect: (id: string) => void;
  onOpen: (file: FileNode) => void;
}

export function GridView({ files, selected, onSelect, onOpen }: GridViewProps) {
  return (
    <div className="h-full overflow-auto p-4">
      {files.length === 0 ? (
        <p className="py-16 text-center text-sm text-muted-foreground">No items</p>
      ) : (
        <div
          className="grid gap-2"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(8rem, 1fr))" }}
        >
          {files.map((file) => (
            <button
              key={file.id}
              type="button"
              onClick={() => onSelect(file.id)}
              onDoubleClick={() => onOpen(file)}
              className={cn(
                "group flex flex-col items-center gap-2 rounded-lg px-2 py-3 text-center transition-colors",
                selected === file.id
                  ? "bg-primary/10"
                  : "hover:bg-accent/40",
              )}
            >
              <div className="relative">
                <FileIcon kind={file.kind} className="size-14" />
                <span className="absolute -bottom-1 -right-1 rounded-full bg-background p-0.5 shadow-sm">
                  <SyncIndicator status={file.status} />
                </span>
              </div>
              <span
                className={cn(
                  "line-clamp-2 max-w-full break-words text-xs",
                  selected === file.id && "rounded bg-primary px-1.5 py-0.5 text-primary-foreground",
                )}
              >
                {file.name}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
