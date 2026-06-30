import Link from "next/link";
import { AuthGuard } from "@/components/auth-guard";
import { UserMenu } from "@/components/user-menu";


const nav = [
  { section: "Platform", items: [
    { label: "Overview", href: "/dashboard" },
    { label: "Projects", href: "/dashboard/projects" },
    { label: "Tunnels", href: "/dashboard/tunnels" },
    { label: "Requests", href: "/dashboard/requests" },
    { label: "Replay Center", href: "/dashboard/replay" },
    { label: "Analytics", href: "/dashboard/analytics" },
  ]},
  { section: "AI", items: [
    { label: "Insights", href: "/dashboard/ai/insights" },
    { label: "Incidents", href: "/dashboard/ai/incidents" },
    { label: "Investigations", href: "/dashboard/ai/investigations" },
    { label: "Copilot", href: "/dashboard/ai/copilot" },
  ]},
  { section: "Workspace", items: [
    { label: "Teams", href: "/dashboard/teams" },
    { label: "Billing", href: "/dashboard/billing" },
    { label: "Settings", href: "/dashboard/settings" },
  ]},
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthGuard>
    <div className="flex min-h-screen">
      {/* Sidebar — 260px per layout spec */}
      <aside className="hidden w-[260px] shrink-0 border-r border-[var(--border)] bg-[var(--background-elevated)] lg:block">
        <div className="flex h-16 items-center px-6 text-lg font-semibold tracking-tight">Gotra</div>
        <nav className="space-y-6 px-4 py-2">
          {nav.map((group) => (
            <div key={group.section}>
              <p className="px-2 pb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                {group.section}
              </p>
              <ul className="space-y-1">
                {group.items.map((item) => (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      className="block rounded-[var(--radius-sm)] px-3 py-2 text-sm text-foreground/80 transition hover:bg-[var(--card-elevated)] hover:text-foreground"
                    >
                      {item.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar — 64px */}
        <header className="flex h-16 items-center justify-between border-b border-[var(--border)] px-6">
          <input
            placeholder="Search…"
            className="h-9 w-72 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--card)] px-3 text-sm outline-none focus:border-purple-500"
          />
          <UserMenu />
        </header>

        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
    </AuthGuard>
  );
}
