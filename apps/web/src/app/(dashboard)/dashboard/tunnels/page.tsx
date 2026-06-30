"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { createTunnel, deleteTunnel, listProjects, listTunnels } from "@/lib/resources";

export default function TunnelsPage() {
  const qc = useQueryClient();
  const [projectId, setProjectId] = useState<string>("");
  const [port, setPort] = useState("3000");

  const { data: projects } = useQuery({ queryKey: ["projects"], queryFn: listProjects });

  // Default to the first project once loaded.
  useEffect(() => {
    if (!projectId && projects && projects.length > 0) setProjectId(projects[0]!.id);
  }, [projects, projectId]);

  const { data: tunnels, isLoading } = useQuery({
    queryKey: ["tunnels", projectId],
    queryFn: () => listTunnels(projectId),
    enabled: !!projectId,
  });

  const create = useMutation({
    mutationFn: () => createTunnel(projectId, Number(port) || 3000),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["tunnels", projectId] }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteTunnel(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["tunnels", projectId] }),
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Tunnels</h1>
        <p className="text-sm text-muted-foreground">Expose a local port to a public URL.</p>
      </div>

      <Card className="flex flex-wrap items-center gap-3">
        <select
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          className="h-10 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500"
        >
          {projects?.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <input
          value={port}
          onChange={(e) => setPort(e.target.value)}
          type="number"
          placeholder="Local port"
          className="h-10 w-32 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500"
        />
        <Button onClick={() => create.mutate()} disabled={!projectId || create.isPending}>
          {create.isPending ? "Creating…" : "Create tunnel"}
        </Button>
      </Card>

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="space-y-3">
          {tunnels?.map((t) => (
            <Card key={t.id} className="flex items-center justify-between py-4">
              <div>
                <p className="font-mono text-sm text-purple-300">{t.public_url}</p>
                <p className="text-xs text-muted-foreground">→ localhost:{t.local_port}</p>
              </div>
              <div className="flex items-center gap-3">
                <span className="rounded-full bg-[var(--card-elevated)] px-2 py-0.5 text-xs text-muted-foreground">{t.status}</span>
                <Button variant="ghost" size="sm" className="text-red-400" onClick={() => remove.mutate(t.id)}>Delete</Button>
              </div>
            </Card>
          ))}
          {tunnels?.length === 0 && <p className="text-sm text-muted-foreground">No tunnels for this project yet.</p>}
        </div>
      )}
    </div>
  );
}
