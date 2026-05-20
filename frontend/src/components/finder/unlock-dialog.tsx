"use client";

import * as React from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { useData } from "@/lib/data-provider";

export function UnlockDialog() {
  const { status, refresh } = useData();
  const [pass, setPass] = React.useState("");
  const [remember, setRemember] = React.useState(true);
  const [err, setErr] = React.useState<string | null>(null);
  const [busy, setBusy] = React.useState(false);

  const open = !!status?.locked;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api.unlock(pass, remember);
      setPass("");
      await refresh();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open}>
      <DialogContent className="sm:max-w-sm" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>Unlock dropboy</DialogTitle>
          <DialogDescription>
            Enter your encryption passphrase to start syncing. Saving it to the OS keychain lets
            the daemon auto-unlock on next boot.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <Input
            type="password"
            placeholder="Passphrase"
            value={pass}
            onChange={(e) => setPass(e.target.value)}
            autoFocus
          />
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember(e.target.checked)}
            />
            <span>Remember in keychain</span>
          </label>
          <Button type="submit" disabled={!pass || busy}>
            {busy ? "Unlocking…" : "Unlock"}
          </Button>
          {err && <p className="text-xs text-red-500">{err}</p>}
        </form>
      </DialogContent>
    </Dialog>
  );
}
