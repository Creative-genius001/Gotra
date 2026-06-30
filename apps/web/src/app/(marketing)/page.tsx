import Link from "next/link";
import { Button } from "@/components/ui/button";

const features = [
  { title: "Instant tunnels", body: "Expose localhost over TLS with a single command from the Gotra agent." },
  { title: "Capture & replay", body: "Inspect every request and replay it with modified variables." },
  { title: "AI debugging", body: "Explain errors, investigate incidents and get recommended fixes — inline." },
];

export default function LandingPage() {
  return (
    <main className="relative min-h-screen overflow-hidden">
      {/* Ambient brand glow */}
      <div className="pointer-events-none absolute -top-40 left-1/2 h-[600px] w-[600px] -translate-x-1/2 rounded-full bg-purple-700/20 blur-[160px]" />

      <header className="mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
        <span className="text-lg font-semibold tracking-tight">Gotra</span>
        <nav className="flex items-center gap-3">
          <Link href="/login">
            <Button variant="ghost" size="sm">Sign in</Button>
          </Link>
          <Link href="/register">
            <Button size="sm">Get started</Button>
          </Link>
        </nav>
      </header>

      <section className="mx-auto max-w-4xl px-6 pb-24 pt-20 text-center">
        <span className="inline-block rounded-full border border-[var(--border)] bg-[var(--card)] px-4 py-1 text-xs text-muted-foreground">
          Go-native tunneling · request replay · AI debugging
        </span>
        <h1 className="mt-6 text-5xl font-semibold leading-tight tracking-tight sm:text-6xl">
          Ship from <span className="text-brand-gradient">localhost</span> to the world,
          <br /> debugged by AI.
        </h1>
        <p className="mx-auto mt-6 max-w-2xl text-lg text-muted-foreground">
          Securely expose your local apps, capture and replay every request, and let an embedded
          AI layer explain errors and investigate incidents like a senior engineer.
        </p>
        <div className="mt-10 flex items-center justify-center gap-4">
          <Link href="/register">
            <Button size="lg">Start free</Button>
          </Link>
          <Link href="/login">
            <Button variant="secondary" size="lg">Live demo</Button>
          </Link>
        </div>
      </section>

      <section className="mx-auto grid max-w-6xl gap-6 px-6 pb-32 sm:grid-cols-3">
        {features.map((f) => (
          <div
            key={f.title}
            className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--card)] p-6"
          >
            <h3 className="text-base font-semibold">{f.title}</h3>
            <p className="mt-2 text-sm text-muted-foreground">{f.body}</p>
          </div>
        ))}
      </section>
    </main>
  );
}
