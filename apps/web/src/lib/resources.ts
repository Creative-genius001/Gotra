/** Typed API calls for projects and tunnels. */
import { apiFetch } from "@/lib/api";

export interface Project {
  id: string;
  workspace_id: string;
  owner_id: string;
  name: string;
  slug: string;
  role?: string;
  created_at: string;
  updated_at: string;
}

export interface Tunnel {
  id: string;
  project_id: string;
  public_url: string;
  local_port: number;
  status: string;
  created_at: string;
  updated_at: string;
}

// --- Projects ---------------------------------------------------------------

export async function listProjects(): Promise<Project[]> {
  const res = await apiFetch<{ projects: Project[] }>("/projects");
  return res.projects;
}

export function createProject(name: string) {
  return apiFetch<Project>("/projects", { method: "POST", body: JSON.stringify({ name }) });
}

export function deleteProject(id: string) {
  return apiFetch<{ status: string }>(`/projects/${id}`, { method: "DELETE" });
}

// --- Tunnels ----------------------------------------------------------------

export async function listTunnels(projectId: string): Promise<Tunnel[]> {
  const res = await apiFetch<{ tunnels: Tunnel[] }>(`/tunnels?project_id=${encodeURIComponent(projectId)}`);
  return res.tunnels;
}

export function createTunnel(projectId: string, localPort: number) {
  return apiFetch<Tunnel>("/tunnels", {
    method: "POST",
    body: JSON.stringify({ project_id: projectId, local_port: localPort }),
  });
}

export function deleteTunnel(id: string) {
  return apiFetch<{ status: string }>(`/tunnels/${id}`, { method: "DELETE" });
}

// --- Requests (capture + replay) --------------------------------------------

export interface RequestSummary {
  id: string;
  tunnel_id: string;
  method: string;
  path: string;
  status?: number;
  duration_ms?: number;
  received_at: string;
}

export interface ResponseDetail {
  status: number;
  headers: Record<string, string[]>;
  body: string;
  duration_ms: number;
}

export interface RequestDetail {
  id: string;
  tunnel_id: string;
  project_id: string;
  method: string;
  path: string;
  query?: string;
  headers: Record<string, string[]>;
  body: string;
  response?: ResponseDetail;
  received_at: string;
}

export interface ReplayResult {
  replay_id: string;
  status: number;
  body: string;
  duration_ms: number;
}

export async function listRequests(projectId: string): Promise<RequestSummary[]> {
  const res = await apiFetch<{ requests: RequestSummary[] }>(
    `/requests?project_id=${encodeURIComponent(projectId)}`,
  );
  return res.requests;
}

export function getRequest(id: string): Promise<RequestDetail> {
  return apiFetch<RequestDetail>(`/requests/${id}`);
}

export function replayRequest(id: string): Promise<ReplayResult> {
  return apiFetch<ReplayResult>(`/requests/${id}/replay`, { method: "POST", body: "{}" });
}
