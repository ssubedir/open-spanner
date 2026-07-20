"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSelector } from "@tanstack/react-store";
import { Check, ChevronDown, LogOut, Menu, X } from "lucide-react";
import { useEffect, useState } from "react";

import { BrandMark } from "@/components/brand-mark";
import { Button } from "@/components/ui/button";
import { navigation } from "@/lib/navigation";
import { cn } from "@/lib/utils";
import { appStore, appStoreActions } from "@/product/app-store";
import { listWorkspaces, type Workspace } from "@/product/api";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const session = useSelector(appStore, (state) => state.auth.session);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const activeItem = navigation.flatMap((group) => group.items).find((item) => pathname === item.href || pathname.startsWith(item.href + "/"));

  useEffect(() => {
    if (!session) return;
    void listWorkspaces().then((response) => setWorkspaces(response.items)).catch(() => setWorkspaces([]));
  }, [session]);

  async function switchTo(workspaceID: string) {
    if (!session || workspaceID === session.user.workspace_id) return;
    await appStoreActions.switchWorkspace(workspaceID);
    window.location.href = "/overview";
  }

  return (
    <div data-testid="dashboard-root" className="min-h-screen bg-background">
      <div data-testid="dashboard-frame" className="min-h-screen overflow-hidden bg-background">
        <div className="grid min-h-screen lg:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="hidden border-r bg-card lg:block">
            <Sidebar currentWorkspaceID={session?.user.workspace_id} onSwitch={switchTo} pathname={pathname} userEmail={session?.user.email} workspaceName={session?.user.workspace_name} workspaces={workspaces} />
          </aside>

          {mobileOpen && (
            <div className="fixed inset-0 z-50 lg:hidden">
              <button className="absolute inset-0 bg-black/20" aria-label="Close navigation" onClick={() => setMobileOpen(false)} />
              <aside className="relative h-full w-[278px] border-r bg-card shadow-xl">
                <button className="absolute right-3 top-3 grid size-8 place-items-center rounded-md hover:bg-muted" onClick={() => setMobileOpen(false)} aria-label="Close navigation">
                  <X className="size-4" />
                </button>
                <Sidebar currentWorkspaceID={session?.user.workspace_id} onNavigate={() => setMobileOpen(false)} onSwitch={switchTo} pathname={pathname} userEmail={session?.user.email} workspaceName={session?.user.workspace_name} workspaces={workspaces} />
              </aside>
            </div>
          )}

          <div className="min-w-0">
            <header className="flex h-[58px] items-center gap-3 border-b bg-card px-4 md:px-6">
              <Button variant="ghost" size="icon" className="-ml-2 lg:hidden" onClick={() => setMobileOpen(true)} aria-label="Open navigation">
                <Menu />
              </Button>
              <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
                Open Spanner <span className="px-1.5 text-border">/</span> <span className="font-medium text-foreground">{activeItem?.label ?? "Overview"}</span>
              </div>
            </header>
            <main className="product-page">{children}</main>
          </div>
        </div>
      </div>
    </div>
  );
}

function Sidebar({ currentWorkspaceID, pathname, onNavigate, onSwitch, userEmail, workspaceName, workspaces }: { currentWorkspaceID?: string; pathname: string; onNavigate?: () => void; onSwitch: (workspaceID: string) => Promise<void>; userEmail?: string; workspaceName?: string; workspaces: Workspace[] }) {
  return (
    <div className="flex h-full min-h-screen flex-col px-3 py-3">
      <details className="group relative mb-4">
        <summary className="flex cursor-pointer list-none items-center gap-2.5 rounded-lg px-2 py-1 text-left hover:bg-muted">
          <BrandMark />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-semibold">{workspaceName || "Open Spanner"}</span>
            <span className="block text-xs capitalize text-muted-foreground">Workspace</span>
          </span>
          <ChevronDown className="size-3.5 transition-transform group-open:rotate-180" />
        </summary>
        <div className="absolute left-0 right-0 top-full z-30 mt-1 rounded-md border bg-popover p-1 shadow-md">
          {workspaces.map((workspace) => (
            <button className="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left text-xs hover:bg-muted" key={workspace.id} onClick={() => void onSwitch(workspace.id)} type="button">
              <span className="min-w-0 flex-1"><span className="block truncate font-medium">{workspace.name}</span><span className="capitalize text-muted-foreground">{workspace.role}</span></span>
              {workspace.id === currentWorkspaceID ? <Check className="size-3.5 text-primary" /> : null}
            </button>
          ))}
          {workspaces.length === 0 ? <p className="px-2 py-2 text-xs text-muted-foreground">Loading workspace…</p> : null}
        </div>
      </details>
      <nav className="scrollbar-subtle flex-1 overflow-y-auto">
        {navigation.map((group) => (
          <div key={group.label} className="mb-4">
            <p className="mb-1 px-2 text-[11px] font-medium tracking-wide text-muted-foreground/70">{group.label}</p>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const active = pathname === item.href || pathname.startsWith(item.href + "/");
                return (
                  <Link key={item.href} href={item.href} onClick={onNavigate} className={cn("flex h-8 items-center gap-2.5 rounded-md px-2 text-[13px] transition-colors", active ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:bg-muted")}>
                    <item.icon className={cn("size-4", active ? "text-foreground" : "text-muted-foreground")} strokeWidth={1.7} />
                    {item.label}
                    {item.label === "Alerts" && <span className="ml-auto size-1.5 rounded-full bg-amber-500" />}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>
      <div className="border-t pt-3">
        <div className="flex items-center gap-2.5 rounded-md px-2 py-1.5">
          <div className="grid size-7 place-items-center rounded-full bg-neutral-900 text-[10px] font-semibold text-white">SG</div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium">Signed in</p>
            <p className="truncate text-[11px] text-muted-foreground">{userEmail ?? "Unknown user"}</p>
          </div>
          <button
            aria-label="Sign out"
            className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={() => void appStoreActions.logout().then(() => { window.location.href = "/login"; })}
            type="button"
          >
            <LogOut className="size-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
}
