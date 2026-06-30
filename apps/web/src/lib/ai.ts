/** Typed API calls for the AI Debugging Service. */
import { apiFetch } from "@/lib/api";

export interface AnalysisResult {
  explanation: string;
  root_cause: string;
  suggested_fix: string;
  severity: string;
  confidence: number;
}

export interface StoredAnalysis {
  id: string;
  provider: string;
  analysis_type: string;
  prompt_version: string;
  result: AnalysisResult;
}

export interface IncidentResult {
  summary: string;
  timeline: string[];
  affected_services: string[];
  root_cause: string;
  recommended_actions: string[];
  severity: string;
  confidence: number;
}

export interface StoredIncident {
  id: string;
  provider: string;
  result: IncidentResult;
}

export interface AnalysisRow {
  id: string;
  project_id: string;
  request_id?: string;
  analysis_type: string;
  provider: string;
  confidence_score: number;
  severity?: string;
  result: AnalysisResult;
  created_at: string;
}

export interface IncidentRow {
  id: string;
  project_id: string;
  summary: string;
  root_cause?: string;
  confidence_score: number;
  status: string;
  report: IncidentResult;
  created_at: string;
}

// --- Operations -------------------------------------------------------------

export function explainError(requestId: string) {
  return apiFetch<StoredAnalysis>("/ai/explain-error", {
    method: "POST",
    body: JSON.stringify({ request_id: requestId }),
  });
}

export function analyzeRequest(requestId: string) {
  return apiFetch<StoredAnalysis>("/ai/analyze-request", {
    method: "POST",
    body: JSON.stringify({ request_id: requestId }),
  });
}

export function explainLogs(projectId: string, logs: string) {
  return apiFetch<StoredAnalysis>("/ai/explain-logs", {
    method: "POST",
    body: JSON.stringify({ project_id: projectId, logs }),
  });
}

export function generateIncident(projectId: string) {
  return apiFetch<StoredIncident>("/ai/generate-incident", {
    method: "POST",
    body: JSON.stringify({ project_id: projectId }),
  });
}

export async function listAnalyses(projectId: string): Promise<AnalysisRow[]> {
  const res = await apiFetch<{ analyses: AnalysisRow[] }>(
    `/ai/analyses?project_id=${encodeURIComponent(projectId)}`,
  );
  return res.analyses;
}

export async function listIncidents(projectId: string): Promise<IncidentRow[]> {
  const res = await apiFetch<{ incidents: IncidentRow[] }>(
    `/ai/incidents?project_id=${encodeURIComponent(projectId)}`,
  );
  return res.incidents;
}
