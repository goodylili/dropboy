"use client";

import * as React from "react";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { FileIcon } from "./file-icon";
import { formatRelative } from "@/lib/format";
import type { FileNode } from "@/lib/types";

interface PaletteProps {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  allFiles: FileNode[];
  onNavigate: (file: FileNode) => void;
}

export function CommandPalette({ open, onOpenChange, allFiles, onNavigate }: PaletteProps) {
  const recents = React.useMemo(
    () =>
      [...allFiles]
        .filter((f) => f.kind !== "folder")
        .sort((a, b) => +new Date(b.modified) - +new Date(a.modified))
        .slice(0, 6),
    [allFiles],
  );

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="Search files" description="Find anything in dropboy">
      <CommandInput placeholder="Search files, folders, and machines…" />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Recents">
          {recents.map((f) => (
            <CommandItem
              key={f.id}
              value={`${f.name} ${f.path}`}
              onSelect={() => {
                onNavigate(f);
                onOpenChange(false);
              }}
            >
              <FileIcon kind={f.kind} className="size-4" />
              <span>{f.name}</span>
              <span className="ml-auto text-xs text-muted-foreground">{formatRelative(f.modified)}</span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="All Files">
          {allFiles.slice(0, 200).map((f) => (
            <CommandItem
              key={f.id}
              value={`${f.name} ${f.path} ${f.machine}`}
              onSelect={() => {
                onNavigate(f);
                onOpenChange(false);
              }}
            >
              <FileIcon kind={f.kind} className="size-4" />
              <div className="flex min-w-0 flex-col">
                <span className="truncate">{f.name}</span>
                <span className="truncate text-[10px] text-muted-foreground">
                  {f.machine} · {f.path}
                </span>
              </div>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
