"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  getRequest,
  listProjects,
  listRequests,
  replayRequest,
  type ReplayResult,
} from "@/lib/resources";

function statusColor(status?: number) {
  if (!status) return "text-muted-foreground";
  if (status < 300) return "text-emerald-400";
  if (status < 400) return "text-amber-400";
  return "text-red-400";
}

export default function RequestsPage() {
  const qc = useQueryClient();
  const [projectId, setProjectId] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [replay, setReplay] = useState<ReplayResult | null>(null);

  const { data: projects } = useQuery({ queryKey: ["projects"], queryFn: listProjects });
  useEffect(() => {
    if (!projectId && projects && projects.length > 0) setProjectId(projects[0]!.id);
  }, [projects, projectId]);

  const { data: requests, isLoading } = useQuery({
    queryKey: ["requests", projectId],
    queryFn: () => listRequests(projectId),
    enabled: !!projectId,
    refetchInterval: 5000,
  });

  const { data: detail } = useQuery({
    queryKey: ["request", selectedId],
    queryFn: () => getRequest(selectedId!),
    enabled: !!selectedId,
  });

  const doReplay = useMutation({
    mutationFn: () => replayRequest(selectedId!),
    onSuccess: (r) => {
      setReplay(r);
      void qc.invalidateQueries({ queryKey: ["requests", projectId] });
    },
  });

  useEffect(() => setReplay(null), [selectedId]);

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Request Inspector</h1>
          <p className="text-sm text-muted-foreground">Captured traffic through your tunnels.</p>
        </div>
        <select
          value={projectId}
          onChange={(e) => { setProjectId(e.target.value); setSelectedId(null); }}
          className="h-10 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500"
        >
          {projects?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {/* Request list */}
        <Card className="p-0">
          {isLoading ? (
            <p className="p-6 text-sm text-muted-foreground">Loading…</p>
          ) : requests && requests.length > 0 ? (
            <ul className="divide-y divide-[var(--border)]">
              {requests.map((r) => (
                <li key={r.id}>
                  <button
                    onClick={() => setSelectedId(r.id)}
                    className={`flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition hover:bg-[var(--card-elevated)] ${selectedId === r.id ? "bg-[var(--card-elevated)]" : ""}`}
                  >
                    <span className="w-14 font-mono text-xs font-semibold">{r.method}</span>
                    <span className="flex-1 truncate font-mono text-xs">{r.path}</span>
                    <span className={`w-10 text-right font-mono text-xs ${statusColor(r.status)}`}>{r.status ?? "—"}</span>
                    <span className="w-14 text-right text-xs text-muted-foreground">{r.duration_ms ?? "—"}ms</span>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="p-6 text-sm text-muted-foreground">
              No captured requests yet. Start a tunnel and send traffic to it.
            </p>
          )}
        </Card>

        {/* Detail panel */}
        <Card>
          {!detail ? (
            <p className="text-sm text-muted-foreground">Select a request to inspect it.</p>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="font-mono text-sm">
                  <span className="font-semibold">{detail.method}</span>{" "}
                  <span className={statusColor(detail.response?.status)}>{detail.response?.status}</span>
                </h3>
                <Button size="sm" onClick={() => doReplay.mutate()} disabled={doReplay.isPending}>
                  {doReplay.isPending ? "Replaying…" : "Replay"}
                </Button>
              </div>
              <p className="break-all font-mono text-xs text-muted-foreground">{detail.path}{detail.query ? `?${detail.query}` : ""}</p>

              <Section title="Request headers"><HeaderList headers={detail.headers} /></Section>
              {detail.body && <Section title="Request body"><Pre>{detail.body}</Pre></Section>}
              {detail.response && (
                <>
                  <Section title="Response headers"><HeaderList headers={detail.response.headers} /></Section>
                  <Section title="Response body"><Pre>{detail.response.body}</Pre></Section>
                </>
              )}

              {replay && (
                <div className="rounded-[var(--radius-md)] border border-purple-700/40 bg-purple-950/20 p-3">
                  <p className="text-xs font-medium text-purple-300">
                    Replay → <span className={statusColor(replay.status)}>{replay.status}</span> ({replay.duration_ms}ms)
                  </p>
                  <Pre>{replay.body.slice(0, 2000)}</Pre>
                </div>
              )}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  );
}

function HeaderList({ headers }: { headers: Record<string, string[]> }) {
  const entries = Object.entries(headers ?? {});
  if (entries.length === 0) return <p className="text-xs text-muted-foreground">none</p>;
  return (
    <div className="space-y-0.5 font-mono text-xs">
      {entries.map(([k, vs]) => (
        <div key={k} className="flex gap-2">
          <span className="text-purple-300">{k}:</span>
          <span className="break-all text-muted-foreground">{vs.join(", ")}</span>
        </div>
      ))}
    </div>
  );
}

function Pre({ children }: { children: React.ReactNode }) {
  return (
    <pre className="max-h-48 overflow-auto rounded-[var(--radius-sm)] bg-[var(--background)] p-3 font-mono text-xs text-foreground/80">
      {children}
    </pre>
  );
}
