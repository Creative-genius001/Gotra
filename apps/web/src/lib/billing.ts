/** Typed API calls for billing. */
import { apiFetch } from "@/lib/api";

export interface Plan {
  name: string;
  price_usd: number;
  max_projects: number;
  max_tunnels: number;
  max_requests_per_day: number;
}

export interface Usage {
  projects: number;
  active_tunnels: number;
  requests_today: number;
}

export interface BillingInfo {
  plan: string;
  status: string;
  limits: Plan;
  usage: Usage;
  available_plans: Plan[];
}

/** Plan change result: applied immediately (info) or needs checkout (URL). */
export interface ChangeResult {
  info?: BillingInfo;
  checkout_url?: string;
}

export function getBilling(): Promise<BillingInfo> {
  return apiFetch<BillingInfo>("/billing");
}

export function changePlan(plan: string): Promise<ChangeResult> {
  return apiFetch<ChangeResult>("/billing/plan", {
    method: "POST",
    body: JSON.stringify({ plan }),
  });
}
