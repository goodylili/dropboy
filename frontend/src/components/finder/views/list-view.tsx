"use client";

import * as React from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import type { FileNode } from "@/lib/types";
import { cn } from "@/lib/utils";
import { FileIcon } from "../file-icon";
import { SyncIndicator } from "../status-indicator";
import { formatBytes, formatRelative } from "@/lib/format";
import type { SortKey } from "../toolbar";

interface ListViewProps {
  files: FileNode[];
  selected: string | null;
  onSelect: (id: string) => void;
  onOpen: (file: FileNode) => void;
  sort: SortKey;
  ascending: boolean;
  onSort: (key: SortKey) => void;
}

export function ListView({ files, selected, onSelect, onOpen, sort, ascending, onSort }: ListViewProps) {
  return (
    <div className="h-full overflow-auto">
      <table className="w-full text-sm">
        <thead className="sticky top-0 z-10 bg-muted/40 backdrop-blur">
          <tr className="text-left text-xs font-medium text-muted-foreground">
            <Th onClick={() => onSort("name")} active={sort === "name"} ascending={ascending} className="w-1/2 pl-3">
              Name
            </Th>
            <Th onClick={() => onSort("modified")} active={sort === "modified"} ascending={ascending} className="w-44">
              Date Modified
            </Th>
            <Th onClick={() => onSort("size")} active={sort === "size"} ascending={ascending} className="w-24">
              Size
            </Th>
            <Th onClick={() => onSort("kind")} active={sort === "kind"} ascending={ascending} className="w-28">
              Kind
            </Th>
            <th className="w-20 px-2 py-2">Status</th>
          </tr>
        </thead>
        <tbody>
          {files.map((file) => (
            <tr
              key={file.id}
              onClick={() => onSelect(file.id)}
              onDoubleClick={() => onOpen(file)}
              className={cn(
                "group cursor-default border-b border-border/30 transition-colors",
                selected === file.id
                  ? "bg-primary/10 hover:bg-primary/15"
                  : "hover:bg-accent/40",
              )}
            >
              <td className="pl-3 pr-2 py-1.5">
                <div className="flex items-center gap-2">
                  <FileIcon kind={file.kind} className="size-4 shrink-0" />
                  <span className="truncate">{file.name}</span>
                  {file.tags?.length ? (
                    <span className="flex items-center gap-0.5">
                      {file.tags.map((t) => (
                        <span
                          key={t}
                          className={cn("size-1.5 rounded-full", tagColor(t))}
                          aria-hidden
                        />
                      ))}
                    </span>
                  ) : null}
                </div>
              </td>
              <td className="px-2 py-1.5 text-xs text-muted-foreground">
                {formatRelative(file.modified)}
              </td>
              <td className="px-2 py-1.5 text-xs tabular-nums text-muted-foreground">
                {file.kind === "folder" ? "—" : formatBytes(file.size)}
              </td>
              <td className="px-2 py-1.5 text-xs capitalize text-muted-foreground">{file.kind}</td>
              <td className="px-2 py-1.5">
                <SyncIndicator status={file.status} encrypted={file.encrypted} />
              </td>
            </tr>
          ))}
          {files.length === 0 && (
            <tr>
              <td colSpan={5} className="px-4 py-16 text-center text-sm text-muted-foreground">
                No items
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

function Th({
  children,
  onClick,
  active,
  ascending,
  className,
}: {
  children: React.ReactNode;
  onClick: () => void;
  active: boolean;
  ascending: boolean;
  className?: string;
}) {
  return (
    <th className={cn("px-2 py-2", className)}>
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "inline-flex items-center gap-1 transition-colors",
          active ? "text-foreground" : "hover:text-foreground",
        )}
      >
        {children}
        {active && (ascending ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />)}
      </button>
    </th>
  );
}

function tagColor(t: string): string {
  switch (t) {
    case "red":
      return "bg-red-500";
    case "orange":
      return "bg-orange-500";
    case "yellow":
      return "bg-yellow-500";
    case "green":
      return "bg-emerald-500";
    case "blue":
      return "bg-sky-500";
    case "purple":
      return "bg-violet-500";
    default:
      return "bg-zinc-400";
  }
}
