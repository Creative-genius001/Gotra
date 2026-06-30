import { Card, CardTitle, CardValue } from "@/components/ui/card";

const metrics = [
  { label: "Active tunnels", value: "3" },
  { label: "Requests (24h)", value: "12.4k" },
  { label: "Error rate", value: "0.8%" },
  { label: "Open incidents", value: "1" },
];

export default function DashboardOverview() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
        <p className="text-sm text-muted-foreground">Your workspace at a glance.</p>
      </div>

      {/* Hero metrics */}
      <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-4">
        {metrics.map((m) => (
          <Card key={m.label}>
            <CardTitle>{m.label}</CardTitle>
            <CardValue>{m.value}</CardValue>
          </Card>
        ))}
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardTitle>Traffic trends</CardTitle>
          <div className="mt-4 flex h-48 items-center justify-center rounded-[var(--radius-md)] border border-dashed border-[var(--border)] text-sm text-muted-foreground">
            Chart placeholder (purple-first visualization)
          </div>
        </Card>
        <Card>
          <CardTitle>AI insights</CardTitle>
          <div className="mt-4 space-y-3 text-sm text-muted-foreground">
            <p className="rounded-[var(--radius-md)] bg-[var(--card-elevated)] p-3">
              No incidents detected in the last 24 hours.
            </p>
          </div>
        </Card>
      </div>
    </div>
  );
}
