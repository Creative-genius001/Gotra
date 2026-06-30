"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ProjectSelect } from "@/components/ai/project-select";
import { AnalysisPanel } from "@/components/ai/analysis-panel";
import { explainLogs, type StoredAnalysis } from "@/lib/ai";

export default function AICopilotPage() {
  const [projectId, setProjectId] = useState("");
  const [logs, setLogs] = useState("");
  const [result, setResult] = useState<StoredAnalysis | null>(null);

  const ask = useMutation({
    mutationFn: () => explainLogs(projectId, logs),
    onSuccess: (r) => setResult(r),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-brand-gradient">AI Copilot</h1>
          <p className="text-sm text-muted-foreground">
            Paste logs, a stack trace, or an error and get an explanation with likely causes and fixes.
          </p>
        </div>
        <ProjectSelect value={projectId} onChange={setProjectId} />
      </div>

      <Card className="space-y-3 border-purple-800/20">
        <textarea
          value={logs}
          onChange={(e) => setLogs(e.target.value)}
          placeholder="Paste logs or an error message here…"
          rows={8}
          className="w-full resize-y rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background)] p-3 font-mono text-xs outline-none focus:border-purple-500"
        />
        <div className="flex justify-end">
          <Button onClick={() => ask.mutate()} disabled={!projectId || !logs.trim() || ask.isPending}>
            {ask.isPending ? "Thinking…" : "Ask Copilot"}
          </Button>
        </div>
      </Card>

      {result && <AnalysisPanel result={result.result} provider={result.provider} />}
    </div>
  );
}
