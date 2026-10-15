"use client";

import { FormEvent, useState } from "react";
import { Button } from "@/components/ui";

export function DevLogin({ enabled, email }: { enabled: boolean; email?: string }) {
  const [editing, setEditing] = useState(!email);
  const [value, setValue] = useState(email || "");
  if (!enabled) return null;
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = value.trim();
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(next) || next.length > 100) return;
    document.cookie = `dev_email=${encodeURIComponent(next)}; Path=/; SameSite=Lax; Max-Age=2592000`;
    window.location.reload();
  }
  return editing ? <form className="dev-login" onSubmit={submit}>
    <label className="sr-only" htmlFor="dev-email">Continue as email</label>
    <input id="dev-email" type="email" maxLength={100} required placeholder="you@example.com" value={value} onChange={(event) => setValue(event.target.value)} />
    <Button type="submit">Continue as</Button>
  </form> : <div className="dev-login"><span className="muted">{email}</span><Button variant="ghost" onClick={() => setEditing(true)}>Switch</Button></div>;
}
