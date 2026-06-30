"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ProjectSelect } from "@/components/ai/project-select";
import { AnalysisPanel } from "@/components/ai/analysis-panel";
import { listRequests } from "@/lib/resources";
import { analyzeRequest, explainError, type StoredAnalysis } from "@/lib/ai";

function statusColor(status?: number) {
  if (!status) return "text-muted-foreground";
  if (status < 300) return "text-emerald-400";
  if (status < 400) return "text-amber-400";
  return "text-red-400";
}

export default function AIInvestigationsPage() {
  const [projectId, setProjectId] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [result, setResult] = useState<StoredAnalysis | null>(null);

  const { data: requests } = useQuery({
    queryKey: ["requests", projectId],
    queryFn: () => listRequests(projectId),
    enabled: !!projectId,
  });

  const investigate = useMutation({
    mutationFn: (kind: "explain" | "analyze") =>
      kind === "explain" ? explainError(selected!) : analyzeRequest(selected!),
    onSuccess: (r) => setResult(r),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-brand-gradient">AI Investigations</h1>
          <p className="text-sm text-muted-foreground">Pick a captured request and let AI investigate it.</p>
        </div>
        <ProjectSelect value={projectId} onChange={(id) => { setProjectId(id); setSelected(null); setResult(null); }} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="p-0">
          {requests && requests.length > 0 ? (
            <ul className="max-h-[28rem] divide-y divide-[var(--border)] overflow-auto">
              {requests.map((r) => (
                <li key={r.id}>
                  <button
                    onClick={() => { setSelected(r.id); setResult(null); }}
                    className={`flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition hover:bg-[var(--card-elevated)] ${selected === r.id ? "bg-[var(--card-elevated)]" : ""}`}
                  >
                    <span className="w-14 font-mono text-xs font-semibold">{r.method}</span>
                    <span className="flex-1 truncate font-mono text-xs">{r.path}</span>
                    <span className={`font-mono text-xs ${statusColor(r.status)}`}>{r.status ?? "—"}</span>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="p-6 text-sm text-muted-foreground">No captured requests for this project yet.</p>
          )}
        </Card>

        <div className="space-y-4">
          <div className="flex gap-2">
            <Button onClick={() => investigate.mutate("explain")} disabled={!selected || investigate.isPending}>
              {investigate.isPending ? "Investigating…" : "Explain error"}
            </Button>
            <Button variant="secondary" onClick={() => investigate.mutate("analyze")} disabled={!selected || investigate.isPending}>
              Analyze request
            </Button>
          </div>
          {result ? (
            <AnalysisPanel result={result.result} provider={result.provider} />
          ) : (
            <Card>
              <p className="text-sm text-muted-foreground">
                Select a request, then run an investigation. Results are saved to AI Insights.
              </p>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
