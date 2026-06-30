"use client";

import Link from "next/link";
import { useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { forgotPassword } from "@/lib/auth";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [devToken, setDevToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await forgotPassword(email);
      setDevToken(res.dev_token ?? null);
    } catch {
      // Intentionally ignore — never reveal whether the email exists.
    } finally {
      setSent(true);
      setLoading(false);
    }
  }

  return (
    <Card className="p-8">
      <h1 className="text-2xl font-semibold tracking-tight">Reset your password</h1>
      <p className="mt-1 text-sm text-muted-foreground">We&apos;ll email you a reset link.</p>

      {sent ? (
        <div className="mt-6 space-y-4">
          <p className="rounded-[var(--radius-md)] bg-[var(--card-elevated)] p-3 text-sm text-muted-foreground">
            If an account exists for <span className="text-foreground">{email}</span>, a reset link has been sent.
          </p>
          {devToken && (
            <p className="break-all rounded-[var(--radius-md)] border border-dashed border-[var(--border)] p-3 text-xs text-muted-foreground">
              Dev mode — open the reset link:{" "}
              <Link href={`/reset-password?token=${devToken}`} className="text-purple-400 underline">
                /reset-password
              </Link>
            </p>
          )}
          <Link href="/login" className="inline-block"><Button variant="secondary">Back to sign in</Button></Link>
        </div>
      ) : (
        <form className="mt-6 space-y-4" onSubmit={onSubmit}>
          <Field label="Email" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" />
          <Button className="w-full" type="submit" disabled={loading}>{loading ? "Sending…" : "Send reset link"}</Button>
        </form>
      )}
    </Card>
  );
}
