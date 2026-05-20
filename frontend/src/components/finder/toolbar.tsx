"use client";

import * as React from "react";
import {
  ArrowLeft,
  ArrowRight,
  Columns3,
  LayoutGrid,
  List,
  Search,
  SlidersHorizontal,
  ArrowUpDown,
  Share2,
  Tag as TagIcon,
  ChevronRight,
  MoreHorizontal,
  Settings,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ThemeToggle } from "@/components/theme-toggle";
import { cn } from "@/lib/utils";

export type ViewMode = "icons" | "list" | "columns";
export type SortKey = "name" | "modified" | "size" | "kind";

interface ToolbarProps {
  canBack: boolean;
  canForward: boolean;
  onBack: () => void;
  onForward: () => void;
  view: ViewMode;
  onView: (v: ViewMode) => void;
  sort: SortKey;
  onSort: (s: SortKey) => void;
  ascending: boolean;
  onToggleAscending: () => void;
  query: string;
  onQuery: (q: string) => void;
  breadcrumbs: { label: string; onClick?: () => void }[];
  onOpenPalette: () => void;
  onOpenManage: () => void;
}

export function Toolbar({
  canBack,
  canForward,
  onBack,
  onForward,
  view,
  onView,
  sort,
  onSort,
  ascending,
  onToggleAscending,
  query,
  onQuery,
  breadcrumbs,
  onOpenPalette,
  onOpenManage,
}: ToolbarProps) {
  return (
    <div className="flex h-12 shrink-0 items-center gap-1 border-b border-border/60 bg-background/80 px-3 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="flex items-center gap-0.5">
        <IconButton tip="Back (⌘[)" onClick={onBack} disabled={!canBack}>
          <ArrowLeft className="size-4" />
        </IconButton>
        <IconButton tip="Forward (⌘])" onClick={onForward} disabled={!canForward}>
          <ArrowRight className="size-4" />
        </IconButton>
      </div>

      <Separator orientation="vertical" className="mx-1 h-5" />

      <div className="flex items-center rounded-md border border-border/60 bg-muted/30 p-0.5">
        <ViewButton active={view === "icons"} onClick={() => onView("icons")} tip="Icons">
          <LayoutGrid className="size-4" />
        </ViewButton>
        <ViewButton active={view === "list"} onClick={() => onView("list")} tip="List">
          <List className="size-4" />
        </ViewButton>
        <ViewButton active={view === "columns"} onClick={() => onView("columns")} tip="Columns">
          <Columns3 className="size-4" />
        </ViewButton>
      </div>

      <Separator orientation="vertical" className="mx-1 h-5" />

      <DropdownMenu>
        <DropdownMenuTrigger
          className="inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-foreground/80 hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground"
        >
          <ArrowUpDown className="size-3.5" /> Sort
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuLabel>Sort by</DropdownMenuLabel>
          <DropdownMenuCheckboxItem checked={sort === "name"} onCheckedChange={() => onSort("name")}>
            Name
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={sort === "modified"} onCheckedChange={() => onSort("modified")}>
            Date Modified
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={sort === "size"} onCheckedChange={() => onSort("size")}>
            Size
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={sort === "kind"} onCheckedChange={() => onSort("kind")}>
            Kind
          </DropdownMenuCheckboxItem>
          <DropdownMenuSeparator />
          <DropdownMenuCheckboxItem checked={ascending} onCheckedChange={onToggleAscending}>
            Ascending
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <IconButton tip="Tag">
        <TagIcon className="size-4" />
      </IconButton>
      <IconButton tip="Share">
        <Share2 className="size-4" />
      </IconButton>
      <IconButton tip="More">
        <MoreHorizontal className="size-4" />
      </IconButton>

      <div className="mx-2 flex min-w-0 flex-1 items-center gap-1 overflow-hidden text-xs text-muted-foreground">
        {breadcrumbs.map((b, i) => (
          <React.Fragment key={`${b.label}-${i}`}>
            {i > 0 && <ChevronRight className="size-3 shrink-0 opacity-50" />}
            <button
              type="button"
              onClick={b.onClick}
              disabled={!b.onClick || i === breadcrumbs.length - 1}
              className={cn(
                "max-w-[10rem] truncate rounded px-1 py-0.5 transition-colors",
                i === breadcrumbs.length - 1
                  ? "text-foreground"
                  : "hover:bg-accent hover:text-accent-foreground",
              )}
            >
              {b.label}
            </button>
          </React.Fragment>
        ))}
      </div>

      <div className="flex items-center gap-1">
        <div className="relative">
          <Search className="absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => onQuery(e.target.value)}
            placeholder="Search"
            onFocus={(e) => {
              if (e.target.value === "") onOpenPalette();
            }}
            className="h-7 w-56 pl-7 text-xs"
          />
        </div>
        <IconButton tip="Filters">
          <SlidersHorizontal className="size-4" />
        </IconButton>
        <IconButton tip="Manage" onClick={onOpenManage}>
          <Settings className="size-4" />
        </IconButton>
        <ThemeToggle />
      </div>
    </div>
  );
}

function IconButton({
  children,
  tip,
  onClick,
  disabled,
}: {
  children: React.ReactNode;
  tip: string;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        disabled={disabled}
        onClick={onClick}
        className={cn(
          "inline-flex size-7 items-center justify-center rounded-md text-foreground/80 transition-colors hover:bg-muted hover:text-foreground",
          "disabled:pointer-events-none disabled:opacity-40",
        )}
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{tip}</TooltipContent>
    </Tooltip>
  );
}

function ViewButton({
  active,
  onClick,
  tip,
  children,
}: {
  active: boolean;
  onClick: () => void;
  tip: string;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        onClick={onClick}
        aria-pressed={active}
        aria-label={tip}
        className={cn(
          "flex size-6 items-center justify-center rounded transition-colors",
          active
            ? "bg-background text-foreground shadow-sm"
            : "text-muted-foreground hover:text-foreground",
        )}
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side="bottom">{tip}</TooltipContent>
    </Tooltip>
  );
}
