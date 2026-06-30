"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Card } from "@/components/ui/card";
import { ProjectSelect } from "@/components/ai/project-select";
import { ConfidenceBadge, SeverityBadge } from "@/components/ai/badges";
import { listAnalyses } from "@/lib/ai";

export default function AIInsightsPage() {
  const [projectId, setProjectId] = useState("");
  const { data: analyses, isLoading } = useQuery({
    queryKey: ["ai", "analyses", projectId],
    queryFn: () => listAnalyses(projectId),
    enabled: !!projectId,
    refetchInterval: 10000,
  });

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-brand-gradient">AI Insights</h1>
          <p className="text-sm text-muted-foreground">Model-generated analyses of your traffic.</p>
        </div>
        <ProjectSelect value={projectId} onChange={setProjectId} />
      </div>

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : analyses && analyses.length > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {analyses.map((a) => (
            <Card key={a.id} className="space-y-3 border-purple-800/20">
              <div className="flex items-center gap-2">
                <span className="font-mono text-xs text-purple-300">{a.analysis_type}</span>
                <span className="ml-auto" />
                <SeverityBadge value={a.severity ?? a.result?.severity ?? "medium"} />
                <ConfidenceBadge value={a.confidence_score} />
              </div>
              <p className="text-sm text-foreground/90">{a.result?.explanation}</p>
              {a.result?.suggested_fix && (
                <p className="text-xs text-muted-foreground">
                  <span className="text-purple-300">Fix:</span> {a.result.suggested_fix}
                </p>
              )}
              <p className="text-xs text-muted-foreground">via {a.provider}</p>
            </Card>
          ))}
        </div>
      ) : (
        <Card>
          <p className="text-sm text-muted-foreground">
            No insights yet. Run an analysis from <span className="text-purple-300">Investigations</span> or the
            Request Inspector, or generate an incident.
          </p>
        </Card>
      )}
    </div>
  );
}
