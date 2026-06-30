"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Card, CardTitle, CardValue } from "@/components/ui/card";
import { ProjectSelect } from "@/components/ai/project-select";
import { getAnalytics } from "@/lib/analytics";

export default function AnalyticsPage() {
  const [projectId, setProjectId] = useState("");
  const { data, isLoading } = useQuery({
    queryKey: ["analytics", projectId],
    queryFn: () => getAnalytics(projectId, 24),
    enabled: !!projectId,
    refetchInterval: 15000,
  });

  const maxReq = Math.max(1, ...(data?.series ?? []).map((p) => p.requests));

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Analytics</h1>
          <p className="text-sm text-muted-foreground">Traffic over the last 24 hours.</p>
        </div>
        <ProjectSelect value={projectId} onChange={setProjectId} />
      </div>

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : data && !data.enabled ? (
        <Card>
          <p className="text-sm text-muted-foreground">
            Analytics is disabled. Start the ClickHouse profile
            (<span className="font-mono text-xs">docker compose --profile analytics up -d</span>) and set
            <span className="font-mono text-xs"> CLICKHOUSE_URL</span> to enable the pipeline.
          </p>
        </Card>
      ) : (
        <>
          <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-4">
            <Card><CardTitle>Requests (24h)</CardTitle><CardValue>{data?.requests ?? 0}</CardValue></Card>
            <Card><CardTitle>Error rate</CardTitle><CardValue>{((data?.error_rate ?? 0) * 100).toFixed(1)}%</CardValue></Card>
            <Card><CardTitle>Avg latency</CardTitle><CardValue>{Math.round(data?.avg_latency_ms ?? 0)}ms</CardValue></Card>
            <Card><CardTitle>p95 latency</CardTitle><CardValue>{Math.round(data?.p95_latency_ms ?? 0)}ms</CardValue></Card>
          </div>

          <Card>
            <CardTitle>Traffic by hour</CardTitle>
            {data && data.series.length > 0 ? (
              <div className="mt-6 flex h-48 items-end gap-1">
                {data.series.map((p) => (
                  <div key={p.bucket} className="flex flex-1 flex-col items-center gap-1" title={`${p.requests} req, ${p.errors} err`}>
                    <div className="flex w-full flex-col justify-end" style={{ height: "100%" }}>
                      <div
                        className="w-full rounded-t bg-red-500/70"
                        style={{ height: `${(p.errors / maxReq) * 100}%` }}
                      />
                      <div
                        className="w-full rounded-t bg-brand-gradient"
                        style={{ height: `${((p.requests - p.errors) / maxReq) * 100}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="mt-4 flex h-48 items-center justify-center rounded-[var(--radius-md)] border border-dashed border-[var(--border)] text-sm text-muted-foreground">
                No traffic captured in this window yet.
              </div>
            )}
          </Card>
        </>
      )}
    </div>
  );
}
