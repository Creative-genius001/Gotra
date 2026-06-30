"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ProjectSelect } from "@/components/ai/project-select";
import { ConfidenceBadge, SeverityBadge } from "@/components/ai/badges";
import { generateIncident, listIncidents } from "@/lib/ai";
import { ApiError } from "@/lib/api";

export default function AIIncidentsPage() {
  const qc = useQueryClient();
  const [projectId, setProjectId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const { data: incidents, isLoading } = useQuery({
    queryKey: ["ai", "incidents", projectId],
    queryFn: () => listIncidents(projectId),
    enabled: !!projectId,
  });

  const generate = useMutation({
    mutationFn: () => generateIncident(projectId),
    onSuccess: () => {
      setError(null);
      void qc.invalidateQueries({ queryKey: ["ai", "incidents", projectId] });
    },
    onError: (e) =>
      setError(
        e instanceof ApiError && e.status === 422
          ? "No recent failed requests to analyze. Send some failing traffic through a tunnel first."
          : "Failed to generate incident.",
      ),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-brand-gradient">AI Incident Center</h1>
          <p className="text-sm text-muted-foreground">AI-generated incidents from recent failures.</p>
        </div>
        <div className="flex items-center gap-3">
          <ProjectSelect value={projectId} onChange={setProjectId} />
          <Button onClick={() => generate.mutate()} disabled={!projectId || generate.isPending}>
            {generate.isPending ? "Generating…" : "Generate incident"}
          </Button>
        </div>
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : incidents && incidents.length > 0 ? (
        <div className="space-y-4">
          {incidents.map((i) => (
            <Card key={i.id} className="space-y-4 border-purple-800/20">
              <div className="flex items-start gap-2">
                <h3 className="font-semibold">{i.summary}</h3>
                <span className="ml-auto" />
                <SeverityBadge value={i.report?.severity ?? "medium"} />
                <ConfidenceBadge value={i.confidence_score} />
              </div>
              {i.root_cause && (
                <p className="text-sm">
                  <span className="text-purple-300">Root cause:</span> {i.root_cause}
                </p>
              )}
              {i.report?.timeline?.length > 0 && (
                <div>
                  <p className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Timeline</p>
                  <ol className="space-y-1 text-sm text-muted-foreground">
                    {i.report.timeline.map((t, idx) => (
                      <li key={idx}>• {t}</li>
                    ))}
                  </ol>
                </div>
              )}
              {i.report?.recommended_actions?.length > 0 && (
                <div>
                  <p className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">Recommended actions</p>
                  <ul className="space-y-1 text-sm text-foreground/90">
                    {i.report.recommended_actions.map((a, idx) => (
                      <li key={idx}>→ {a}</li>
                    ))}
                  </ul>
                </div>
              )}
              <p className="text-xs text-muted-foreground">status: {i.status}</p>
            </Card>
          ))}
        </div>
      ) : (
        <Card>
          <p className="text-sm text-muted-foreground">No incidents yet. Click “Generate incident”.</p>
        </Card>
      )}
    </div>
  );
}
