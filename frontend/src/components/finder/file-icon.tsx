import * as React from "react";
import {
  Folder,
  FileText,
  FileImage,
  FileVideo,
  FileAudio,
  FileCode,
  FileArchive,
  FileSpreadsheet,
  Presentation,
  FileType,
  File as FileIco,
} from "lucide-react";
import type { FileKind } from "@/lib/types";
import { cn } from "@/lib/utils";

type Entry = { Icon: React.ComponentType<{ className?: string }>; color: string };

const map: Record<FileKind, Entry> = {
  folder: { Icon: Folder, color: "text-sky-500 dark:text-sky-400" },
  image: { Icon: FileImage, color: "text-violet-500 dark:text-violet-400" },
  video: { Icon: FileVideo, color: "text-pink-500 dark:text-pink-400" },
  audio: { Icon: FileAudio, color: "text-emerald-500 dark:text-emerald-400" },
  pdf: { Icon: FileType, color: "text-red-500 dark:text-red-400" },
  code: { Icon: FileCode, color: "text-amber-500 dark:text-amber-400" },
  archive: { Icon: FileArchive, color: "text-orange-500 dark:text-orange-400" },
  document: { Icon: FileText, color: "text-blue-500 dark:text-blue-400" },
  spreadsheet: { Icon: FileSpreadsheet, color: "text-green-600 dark:text-green-400" },
  presentation: { Icon: Presentation, color: "text-rose-500 dark:text-rose-400" },
  text: { Icon: FileText, color: "text-zinc-500 dark:text-zinc-400" },
  binary: { Icon: FileIco, color: "text-zinc-500 dark:text-zinc-400" },
};

const fallback: Entry = { Icon: FileIco, color: "text-zinc-500 dark:text-zinc-400" };

export function FileIcon({ kind, className }: { kind: FileKind; className?: string }) {
  const { Icon, color } = map[kind] ?? fallback;
  return <Icon className={cn(color, className)} />;
}
