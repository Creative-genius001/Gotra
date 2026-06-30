"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { createProject, deleteProject, listProjects } from "@/lib/resources";

export default function ProjectsPage() {
  const qc = useQueryClient();
  const [name, setName] = useState("");

  const { data: projects, isLoading } = useQuery({ queryKey: ["projects"], queryFn: listProjects });

  const create = useMutation({
    mutationFn: () => createProject(name || "Untitled Project"),
    onSuccess: () => {
      setName("");
      void qc.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteProject(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["projects"] }),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground">Group your tunnels and traffic by project.</p>
        </div>
      </div>

      <Card className="flex items-center gap-3">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="New project name"
          className="h-10 flex-1 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500"
        />
        <Button onClick={() => create.mutate()} disabled={create.isPending}>
          {create.isPending ? "Creating…" : "Create project"}
        </Button>
      </Card>

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {projects?.map((p) => (
            <Card key={p.id} className="flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-semibold">{p.name}</h3>
                  <p className="text-xs text-muted-foreground">{p.slug}</p>
                </div>
                <span className="rounded-full bg-[var(--card-elevated)] px-2 py-0.5 text-xs text-muted-foreground">
                  {p.role}
                </span>
              </div>
              {(p.role === "owner" || p.role === "admin") && (
                <Button variant="ghost" size="sm" className="self-start text-red-400" onClick={() => remove.mutate(p.id)}>
                  Delete
                </Button>
              )}
            </Card>
          ))}
          {projects?.length === 0 && <p className="text-sm text-muted-foreground">No projects yet.</p>}
        </div>
      )}
    </div>
  );
}
