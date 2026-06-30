"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { login, oauthURL, useAuth } from "@/lib/auth";
import { ApiError } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const setSession = useAuth((s) => s.setSession);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await login({ email, password });
      setSession(res);
      router.replace("/dashboard");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Card className="p-8">
      <h1 className="text-2xl font-semibold tracking-tight">Welcome back</h1>
      <p className="mt-1 text-sm text-muted-foreground">Sign in to your Gotra workspace.</p>

      <div className="mt-6 space-y-3">
        <a href={oauthURL("google")}>
          <Button type="button" variant="secondary" className="w-full">Continue with Google</Button>
        </a>
        <a href={oauthURL("github")}>
          <Button type="button" variant="secondary" className="w-full">Continue with GitHub</Button>
        </a>
        <a href={oauthURL("oidc")}>
          <Button type="button" variant="ghost" className="w-full">Continue with SSO</Button>
        </a>
      </div>

      <div className="my-6 flex items-center gap-3 text-xs text-muted-foreground">
        <span className="h-px flex-1 bg-[var(--border)]" /> or <span className="h-px flex-1 bg-[var(--border)]" />
      </div>

      <form className="space-y-4" onSubmit={onSubmit}>
        <Field label="Email" type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" />
        <Field label="Password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} placeholder="••••••••" />
        {error && <p className="text-sm text-red-400">{error}</p>}
        <Button className="w-full" type="submit" disabled={loading}>{loading ? "Signing in…" : "Sign in"}</Button>
      </form>

      <div className="mt-4 text-center text-sm">
        <Link href="/forgot-password" className="text-muted-foreground hover:text-foreground">Forgot password?</Link>
      </div>

      <p className="mt-4 text-center text-sm text-muted-foreground">
        No account? <Link href="/register" className="text-purple-400 hover:underline">Create one</Link>
      </p>
    </Card>
  );
}
