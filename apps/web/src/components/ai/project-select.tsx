"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { listProjects } from "@/lib/resources";

/** Project picker shared across AI pages; auto-selects the first project. */
export function ProjectSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (id: string) => void;
}) {
  const { data: projects } = useQuery({ queryKey: ["projects"], queryFn: listProjects });

  useEffect(() => {
    if (!value && projects && projects.length > 0) onChange(projects[0]!.id);
  }, [projects, value, onChange]);

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="h-10 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background-elevated)] px-3 text-sm outline-none focus:border-purple-500"
    >
      {projects?.map((p) => (
        <option key={p.id} value={p.id}>
          {p.name}
        </option>
      ))}
    </select>
  );
}
