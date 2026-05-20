"use client";

import { FinderShell } from "@/components/finder/finder-shell";
import { DataProvider } from "@/lib/data-provider";

export default function Page() {
  return (
    <DataProvider>
      <FinderShell />
    </DataProvider>
  );
}
