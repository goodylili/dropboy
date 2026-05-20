"use client";

import * as React from "react";
import { ChevronRight, Lock, MapPin } from "lucide-react";
import type { FileNode } from "@/lib/types";
import { cn } from "@/lib/utils";
import { FileIcon } from "../file-icon";
import { SyncIndicator } from "../status-indicator";
import { formatBytes, formatDate } from "@/lib/format";

interface ColumnViewProps {
  root: FileNode;
  /** segments after root, e.g. ["Projects","dropboy"] */
  pathSegments: string[];
  /** path string for the file selected at the deepest column */
  selectedLeafId: string | null;
  onNavigate: (segments: string[]) => void;
  onSelectLeaf: (id: string | null) => void;
  onOpen: (file: FileNode) => void;
}

export function ColumnView({
  root,
  pathSegments,
  selectedLeafId,
  onNavigate,
  onSelectLeaf,
  onOpen,
}: ColumnViewProps) {
  // Build the chain of nodes for each column.
  const columns: FileNode[] = [root];
  let cur: FileNode | undefined = root;
  for (const seg of pathSegments) {
    cur = cur?.children?.find((c) => c.name === seg);
    if (!cur) break;
    columns.push(cur);
  }

  // The preview file is the selected leaf if it's a file at the rightmost level.
  const lastDir = columns[columns.length - 1];
  const previewFile =
    selectedLeafId && lastDir?.children?.find((c) => c.id === selectedLeafId);

  return (
    <div className="flex h-full divide-x divide-border/60 overflow-x-auto">
      {columns.map((col, idx) => {
        if (!col.children) return null;
        const activeChildName = pathSegments[idx];
        return (
          <div key={col.id} className="flex h-full w-64 shrink-0 flex-col">
            <div className="px-3 py-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70">
              {col.name === "/" ? "Root" : col.name}
            </div>
            <ul className="flex-1 overflow-y-auto px-1 pb-2">
              {col.children.map((child) => {
                const isActive = activeChildName === child.name;
                const isSelectedFile = idx === columns.length - 1 && selectedLeafId === child.id;
                return (
                  <li key={child.id}>
                    <button
                      type="button"
                      onClick={() => {
                        if (child.kind === "folder") {
                          onNavigate([...pathSegments.slice(0, idx), child.name]);
                          onSelectLeaf(null);
                        } else {
                          onNavigate(pathSegments.slice(0, idx));
                          onSelectLeaf(child.id);
                        }
                      }}
                      onDoubleClick={() => onOpen(child)}
                      className={cn(
                        "flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-sm transition-colors",
                        (isActive || isSelectedFile)
                          ? "bg-primary text-primary-foreground"
                          : "hover:bg-accent/60",
                      )}
                    >
                      <FileIcon kind={child.kind} className="size-4 shrink-0" />
                      <span className="flex-1 truncate">{child.name}</span>
                      {child.kind === "folder" ? (
                        <ChevronRight className={cn("size-3 opacity-60", (isActive) && "opacity-100")} />
                      ) : (
                        <SyncIndicator status={child.status} />
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        );
      })}

      <div className="flex h-full w-72 shrink-0 flex-col bg-muted/20 p-4">
        {previewFile ? (
          <Preview file={previewFile} />
        ) : (
          <Preview file={columns[columns.length - 1]} />
        )}
      </div>
    </div>
  );
}

function Preview({ file }: { file: FileNode }) {
  return (
    <div className="flex h-full flex-col items-center gap-3 text-center">
      <FileIcon kind={file.kind} className="size-24" />
      <div className="w-full">
        <p className="truncate text-sm font-medium">{file.name}</p>
        <p className="text-xs capitalize text-muted-foreground">{file.kind}</p>
      </div>

      <dl className="grid w-full gap-1.5 pt-3 text-left text-xs">
        <Row label="Size" value={file.kind === "folder" ? `${file.children?.length ?? 0} items` : formatBytes(file.size)} />
        <Row label="Modified" value={formatDate(file.modified)} />
        <Row label="Machine" value={file.machine} />
        <Row
          label="Where"
          value={
            <span className="inline-flex items-center gap-1">
              <MapPin className="size-3" />
              <span className="truncate">{file.path}</span>
            </span>
          }
        />
        <Row
          label="Status"
          value={
            <span className="inline-flex items-center gap-1.5 capitalize">
              <SyncIndicator status={file.status} /> {file.status}
            </span>
          }
        />
        {file.encrypted && (
          <Row
            label="Encryption"
            value={
              <span className="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                <Lock className="size-3" /> AES-256-GCM
              </span>
            }
          />
        )}
      </dl>
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-2 border-b border-border/40 pb-1.5 last:border-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="max-w-[60%] truncate text-right text-foreground">{value}</dd>
    </div>
  );
}
