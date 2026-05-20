import type { FileNode, Machine } from "./types";

export const machines: Machine[] = [
  { id: "golilis-mbp", label: "Goody's MacBook Pro", online: true },
  { id: "studio-desktop", label: "Studio Desktop", online: true },
  { id: "linux-rig", label: "linux-rig", online: false },
];

let id = 0;
const nid = () => `n${++id}`;

type Draft = Omit<FileNode, "path" | "machine" | "children"> & { children?: Draft[] };

function f(name: string, partial: Partial<Omit<Draft, "id" | "name">> = {}): Draft {
  return {
    id: nid(),
    name,
    kind: partial.kind ?? "binary",
    size: partial.size ?? 0,
    modified: partial.modified ?? new Date().toISOString(),
    status: partial.status ?? "synced",
    encrypted: partial.encrypted ?? true,
    tags: partial.tags,
    children: partial.children,
  };
}

function dir(name: string, children: Draft[]): Draft {
  return f(name, {
    kind: "folder",
    size: children.reduce((s, c) => s + c.size, 0),
    modified: children.map((c) => c.modified).sort().at(-1) ?? new Date().toISOString(),
    children,
  });
}

const now = Date.now();
const t = (offsetMin: number) => new Date(now - offsetMin * 60_000).toISOString();

function build(machine: string): FileNode {
  const tree = dir("/", [
    dir("Documents", [
      dir("Contracts", [
        f("nda-acme-2026.pdf", { kind: "pdf", size: 248_103, modified: t(45), tags: ["red"] }),
        f("dropboy-incorporation.pdf", { kind: "pdf", size: 1_204_822, modified: t(60 * 24 * 3) }),
        f("invoice-template.docx", { kind: "document", size: 38_410, modified: t(60 * 24 * 12) }),
      ]),
      dir("Receipts 2026", [
        f("aws-january.pdf", { kind: "pdf", size: 84_213, modified: t(60 * 24 * 30) }),
        f("aws-february.pdf", { kind: "pdf", size: 87_002, modified: t(60 * 24 * 60), status: "uploading" }),
        f("aws-march.pdf", { kind: "pdf", size: 91_330, modified: t(60 * 24 * 75) }),
        f("groceries.csv", { kind: "spreadsheet", size: 12_410, modified: t(60 * 24 * 14) }),
      ]),
      f("resume.pdf", { kind: "pdf", size: 184_002, modified: t(60 * 24 * 6), tags: ["blue"] }),
      f("notes.md", { kind: "text", size: 8_192, modified: t(12) }),
    ]),
    dir("Projects", [
      dir("dropboy", [
        dir("backend", [
          f("main.go", { kind: "code", size: 5_400, modified: t(8) }),
          f("sync.go", { kind: "code", size: 18_322, modified: t(2), status: "pending" }),
          f("crypto.go", { kind: "code", size: 11_244, modified: t(35) }),
          f("go.mod", { kind: "code", size: 412, modified: t(60 * 24) }),
        ]),
        dir("frontend", [
          f("package.json", { kind: "code", size: 1_204, modified: t(20) }),
          f("README.md", { kind: "text", size: 2_488, modified: t(60) }),
        ]),
        f("PRD.md", { kind: "text", size: 17_253, modified: t(5), tags: ["green"] }),
        f("Makefile", { kind: "code", size: 612, modified: t(60 * 24) }),
      ]),
      dir("website", [
        f("index.html", { kind: "code", size: 12_004, modified: t(60 * 24 * 9) }),
        f("hero.png", { kind: "image", size: 2_104_022, modified: t(60 * 24 * 9), status: "conflict" }),
      ]),
    ]),
    dir("Pictures", [
      dir("Wallpapers", [
        f("aurora-01.jpg", { kind: "image", size: 4_204_022, modified: t(60 * 24 * 90) }),
        f("aurora-02.jpg", { kind: "image", size: 3_811_004, modified: t(60 * 24 * 90) }),
        f("mountains.heic", { kind: "image", size: 2_404_022, modified: t(60 * 24 * 200) }),
      ]),
      f("screenshot-2026-05-15.png", { kind: "image", size: 1_204_022, modified: t(120) }),
      f("ferris.svg", { kind: "image", size: 8_200, modified: t(60 * 24 * 14), tags: ["orange"] }),
    ]),
    dir("Music", [
      f("focus-mix.mp3", { kind: "audio", size: 12_400_022, modified: t(60 * 24 * 4) }),
      f("voice-memo.m4a", { kind: "audio", size: 1_204_022, modified: t(60 * 24 * 1) }),
    ]),
    dir("Movies", [
      f("kyoto-trip.mp4", { kind: "video", size: 412_044_022, modified: t(60 * 24 * 20), status: "downloading" }),
    ]),
    dir("Downloads", [
      f("ghidra-11.zip", { kind: "archive", size: 312_044_022, modified: t(60 * 24 * 50) }),
      f("dataset.csv", { kind: "spreadsheet", size: 22_044_022, modified: t(60 * 24 * 60) }),
      f("slides.key", { kind: "presentation", size: 8_044_022, modified: t(60 * 24 * 100) }),
    ]),
  ]);

  // attach paths and machine recursively
  const walk = (node: Draft, parentPath: string): FileNode => {
    const isRoot = node.name === "/";
    const path = isRoot ? "/" : parentPath === "" ? `/${node.name}` : `${parentPath}/${node.name}`;
    return {
      id: node.id,
      name: node.name,
      kind: node.kind,
      size: node.size,
      modified: node.modified,
      status: node.status,
      encrypted: node.encrypted,
      tags: node.tags,
      machine,
      path,
      children: node.children?.map((c) => walk(c, isRoot ? "" : path)),
    };
  };

  return walk(tree, "");
}

export const trees: Record<string, FileNode> = Object.fromEntries(
  machines.map((m) => [m.id, build(m.id)]),
);

export function findByPath(machine: string, path: string): FileNode | null {
  const root = trees[machine];
  if (!root) return null;
  if (path === "/" || path === "") return root;
  const segs = path.split("/").filter(Boolean);
  let cur: FileNode | undefined = root;
  for (const seg of segs) {
    cur = cur?.children?.find((c) => c.name === seg);
    if (!cur) return null;
  }
  return cur;
}

export function flatten(node: FileNode, out: FileNode[] = []): FileNode[] {
  out.push(node);
  node.children?.forEach((c) => flatten(c, out));
  return out;
}
