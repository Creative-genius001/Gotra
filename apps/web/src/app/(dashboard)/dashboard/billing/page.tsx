"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card, CardTitle } from "@/components/ui/card";
import { changePlan, getBilling, type Plan } from "@/lib/billing";

function limitLabel(n: number) {
  return n < 0 ? "Unlimited" : n.toLocaleString();
}

function UsageBar({ used, limit }: { used: number; limit: number }) {
  const pct = limit < 0 ? 0 : Math.min(100, (used / Math.max(1, limit)) * 100);
  return (
    <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-[var(--background)]">
      <div className="h-full bg-brand-gradient" style={{ width: `${pct}%` }} />
    </div>
  );
}

export default function BillingPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["billing"], queryFn: getBilling });

  const switchPlan = useMutation({
    mutationFn: (plan: string) => changePlan(plan),
    onSuccess: (res) => {
      if (res.checkout_url) {
        window.location.href = res.checkout_url; // redirect to Stripe Checkout
        return;
      }
      void qc.invalidateQueries({ queryKey: ["billing"] });
    },
  });

  if (isLoading || !data) return <p className="text-sm text-muted-foreground">Loading…</p>;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Billing</h1>
        <p className="text-sm text-muted-foreground">
          You are on the <span className="capitalize text-purple-300">{data.plan}</span> plan.
        </p>
      </div>

      <Card className="space-y-4">
        <CardTitle>Usage this period</CardTitle>
        <div className="grid gap-6 sm:grid-cols-3">
          <div>
            <p className="text-sm">Projects <span className="text-muted-foreground">{data.usage.projects} / {limitLabel(data.limits.max_projects)}</span></p>
            <UsageBar used={data.usage.projects} limit={data.limits.max_projects} />
          </div>
          <div>
            <p className="text-sm">Active tunnels <span className="text-muted-foreground">{data.usage.active_tunnels} / {limitLabel(data.limits.max_tunnels)}</span></p>
            <UsageBar used={data.usage.active_tunnels} limit={data.limits.max_tunnels} />
          </div>
          <div>
            <p className="text-sm">Requests today <span className="text-muted-foreground">{data.usage.requests_today.toLocaleString()} / {limitLabel(data.limits.max_requests_per_day)}</span></p>
            <UsageBar used={data.usage.requests_today} limit={data.limits.max_requests_per_day} />
          </div>
        </div>
      </Card>

      <div className="grid gap-4 sm:grid-cols-3">
        {data.available_plans.map((p: Plan) => {
          const current = p.name === data.plan;
          return (
            <Card key={p.name} className={current ? "border-purple-600/50" : ""}>
              <div className="flex items-baseline justify-between">
                <h3 className="font-semibold capitalize">{p.name}</h3>
                <span className="text-2xl font-semibold">${p.price_usd}<span className="text-sm text-muted-foreground">/mo</span></span>
              </div>
              <ul className="mt-4 space-y-1 text-sm text-muted-foreground">
                <li>{limitLabel(p.max_projects)} projects</li>
                <li>{limitLabel(p.max_tunnels)} tunnels</li>
                <li>{limitLabel(p.max_requests_per_day)} requests/day</li>
              </ul>
              <div className="mt-5">
                {current ? (
                  <Button variant="secondary" className="w-full" disabled>Current plan</Button>
                ) : (
                  <Button className="w-full" onClick={() => switchPlan.mutate(p.name)} disabled={switchPlan.isPending}>
                    {switchPlan.isPending ? "Switching…" : `Switch to ${p.name}`}
                  </Button>
                )}
              </div>
            </Card>
          );
        })}
      </div>

      <p className="text-xs text-muted-foreground">
        Plan changes apply immediately. Payment processing is stubbed — wire a provider (Stripe) via the
        billing Processor interface to charge for paid tiers.
      </p>
    </div>
  );
}
