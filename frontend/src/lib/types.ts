export type SyncStatus = "synced" | "pending" | "uploading" | "downloading" | "conflict" | "error";

export type FileKind =
  | "folder"
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "code"
  | "archive"
  | "document"
  | "spreadsheet"
  | "presentation"
  | "text"
  | "binary";

export type Tag = "red" | "orange" | "yellow" | "green" | "blue" | "purple" | "gray";

export interface FileNode {
  id: string;
  name: string;
  kind: FileKind;
  size: number; // bytes; folders use sum of children
  modified: string; // ISO
  machine: string; // machine-id
  path: string; // absolute virtual path within machine root
  status: SyncStatus;
  encrypted: boolean;
  tags?: Tag[];
  children?: FileNode[];
}

export interface Machine {
  id: string;
  label: string;
  online: boolean;
}
