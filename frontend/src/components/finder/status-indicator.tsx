import { Check, CircleAlert, CircleDot, Cloud, CloudUpload, CloudDownload, Lock } from "lucide-react";
import type { SyncStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

type Entry = { Icon: React.ComponentType<{ className?: string }>; color: string; label: string };

const map: Record<SyncStatus, Entry> = {
  synced: { Icon: Check, color: "text-emerald-500", label: "Synced" },
  pending: { Icon: CircleDot, color: "text-amber-500", label: "Pending" },
  uploading: { Icon: CloudUpload, color: "text-sky-500", label: "Uploading" },
  downloading: { Icon: CloudDownload, color: "text-sky-500", label: "Downloading" },
  conflict: { Icon: CircleAlert, color: "text-red-500", label: "Conflict" },
  error: { Icon: CircleAlert, color: "text-red-500", label: "Error" },
};

const fallback: Entry = { Icon: CircleDot, color: "text-muted-foreground", label: "Unknown" };

export function SyncIndicator({ status, encrypted, className }: { status: SyncStatus; encrypted?: boolean; className?: string }) {
  const { Icon, color, label } = map[status] ?? fallback;
  return (
    <span className={cn("inline-flex items-center gap-1", className)} aria-label={label} title={label}>
      <Icon className={cn("size-3.5", color)} />
      {encrypted ? <Lock className="size-3 text-muted-foreground" aria-label="Encrypted" /> : null}
    </span>
  );
}

export function CloudIcon({ className }: { className?: string }) {
  return <Cloud className={cn("size-3.5 text-muted-foreground", className)} />;
}
