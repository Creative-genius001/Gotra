/** Typed API call for the analytics pipeline. */
import { apiFetch } from "@/lib/api";

export interface SeriesPoint {
  bucket: string;
  requests: number;
  errors: number;
}

export interface AnalyticsSummary {
  enabled: boolean;
  requests: number;
  errors: number;
  error_rate: number;
  avg_latency_ms: number;
  p95_latency_ms: number;
  series: SeriesPoint[];
}

export function getAnalytics(projectId: string, hours = 24): Promise<AnalyticsSummary> {
  return apiFetch<AnalyticsSummary>(
    `/analytics?project_id=${encodeURIComponent(projectId)}&hours=${hours}`,
  );
}
