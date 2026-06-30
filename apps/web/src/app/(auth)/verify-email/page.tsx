"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { verifyEmail } from "@/lib/auth";

type State = "verifying" | "success" | "error";

export default function VerifyEmailPage() {
  const [state, setState] = useState<State>("verifying");
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return;
    ran.current = true;

    const token = new URLSearchParams(window.location.search).get("token");
    if (!token) {
      setState("error");
      return;
    }
    verifyEmail(token)
      .then(() => setState("success"))
      .catch(() => setState("error"));
  }, []);

  return (
    <Card className="p-8 text-center">
      <h1 className="text-2xl font-semibold tracking-tight">Email verification</h1>
      {state === "verifying" && <p className="mt-3 text-sm text-muted-foreground">Verifying your email…</p>}
      {state === "success" && (
        <>
          <p className="mt-3 text-sm text-muted-foreground">Your email has been verified. 🎉</p>
          <Link href="/dashboard" className="mt-6 inline-block"><Button>Go to dashboard</Button></Link>
        </>
      )}
      {state === "error" && (
        <>
          <p className="mt-3 text-sm text-red-400">This verification link is invalid or has expired.</p>
          <Link href="/login" className="mt-6 inline-block"><Button variant="secondary">Back to sign in</Button></Link>
        </>
      )}
    </Card>
  );
}
