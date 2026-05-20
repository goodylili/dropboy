"use client";

import * as React from "react";
import {
  Clock,
  Trash2,
  Folder,
  FolderHeart,
  Image as ImageIcon,
  Code,
  Music,
  Film,
  Download,
  Cloud,
  Laptop,
  Server,
  AlertTriangle,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useData } from "@/lib/data-provider";

type Loc = { machine: string; path: string };

interface SidebarProps {
  current: Loc;
  onNavigate: (loc: Loc) => void;
}

function buildFavorites(machineId: string) {
  return [
    { label: "Recents", icon: Clock, target: { machine: "__recents", path: "/" } },
    { label: "Documents", icon: Folder, target: { machine: machineId, path: "/Documents" } },
    { label: "Projects", icon: FolderHeart, target: { machine: machineId, path: "/Projects" } },
    { label: "Pictures", icon: ImageIcon, target: { machine: machineId, path: "/Pictures" } },
    { label: "Music", icon: Music, target: { machine: machineId, path: "/Music" } },
    { label: "Movies", icon: Film, target: { machine: machineId, path: "/Movies" } },
    { label: "Downloads", icon: Download, target: { machine: machineId, path: "/Downloads" } },
    { label: "Code", icon: Code, target: { machine: machineId, path: "/Projects" } },
  ];
}

const tags = [
  { label: "Important", color: "bg-red-500" },
  { label: "Work", color: "bg-amber-500" },
  { label: "Personal", color: "bg-emerald-500" },
  { label: "Reading", color: "bg-sky-500" },
  { label: "Archive", color: "bg-violet-500" },
];

export function Sidebar({ current, onNavigate }: SidebarProps) {
  const { machines } = useData();
  const primary = machines[0]?.id ?? "";
  const favorites = buildFavorites(primary);
  return (
    <aside className="hidden w-60 shrink-0 flex-col border-r border-border/60 bg-muted/30 md:flex">
      <ScrollArea className="flex-1">
        <div className="px-3 py-3">
          <Section title="Favorites">
            {favorites.map((f) => (
              <NavItem
                key={f.label}
                icon={<f.icon className="size-4" />}
                label={f.label}
                active={current.machine === f.target.machine && current.path === f.target.path}
                onClick={() => onNavigate(f.target as Loc)}
              />
            ))}
          </Section>

          <Section title="Machines">
            {machines.map((m) => (
              <NavItem
                key={m.id}
                icon={m.id === "linux-rig" ? <Server className="size-4" /> : <Laptop className="size-4" />}
                label={m.label}
                trailing={
                  <span
                    className={cn(
                      "size-1.5 rounded-full",
                      m.online ? "bg-emerald-500" : "bg-zinc-400",
                    )}
                    aria-label={m.online ? "Online" : "Offline"}
                  />
                }
                active={current.machine === m.id && current.path === "/"}
                onClick={() => onNavigate({ machine: m.id, path: "/" })}
              />
            ))}
          </Section>

          <Section title="iCloud-like">
            <NavItem
              icon={<Cloud className="size-4" />}
              label="All Devices"
              active={current.machine === "__all" && current.path === "/"}
              onClick={() => onNavigate({ machine: "__all", path: "/" })}
            />
            <NavItem
              icon={<AlertTriangle className="size-4" />}
              label="Conflicts"
              active={current.machine === "__conflicts"}
              onClick={() => onNavigate({ machine: "__conflicts", path: "/" })}
            />
            <NavItem
              icon={<Trash2 className="size-4" />}
              label="Trash"
              active={current.machine === "__trash"}
              onClick={() => onNavigate({ machine: "__trash", path: "/" })}
            />
          </Section>

          <Section title="Tags">
            {tags.map((t) => (
              <button
                key={t.label}
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              >
                <span className={cn("size-2 rounded-full", t.color)} />
                <span className="truncate">{t.label}</span>
              </button>
            ))}
          </Section>
        </div>
      </ScrollArea>
    </aside>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-4">
      <h3 className="px-2 pb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70">
        {title}
      </h3>
      <div className="space-y-0.5">{children}</div>
    </div>
  );
}

function NavItem({
  icon,
  label,
  active,
  trailing,
  onClick,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  trailing?: React.ReactNode;
  onClick?: () => void;
  children?: React.ReactNode;
}) {
  return (
    <>
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "group flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-sm transition-colors",
          active
            ? "bg-accent text-accent-foreground"
            : "text-foreground/80 hover:bg-accent/60 hover:text-accent-foreground",
        )}
      >
        <span className={cn("text-muted-foreground", active && "text-accent-foreground")}>{icon}</span>
        <span className="flex-1 truncate">{label}</span>
        {trailing}
      </button>
      {children}
    </>
  );
}
