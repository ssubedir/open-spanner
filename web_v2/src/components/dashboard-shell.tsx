"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSelector } from "@tanstack/react-store";
import { ChevronDown, LogOut, Menu, X } from "lucide-react";
import { useState } from "react";

import { BrandMark } from "@/components/brand-mark";
import { Button } from "@/components/ui/button";
import { navigation } from "@/lib/navigation";
import { cn } from "@/lib/utils";
import { appStore, appStoreActions } from "@/product/app-store";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const session = useSelector(appStore, (state) => state.auth.session);
  const activeItem = navigation.flatMap((group) => group.items).find((item) => pathname === item.href || pathname.startsWith(item.href + "/"));

  return (
    <div data-testid="dashboard-root" className="min-h-screen bg-background">
      <div data-testid="dashboard-frame" className="min-h-screen overflow-hidden bg-background">
        <div className="grid min-h-screen lg:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="hidden border-r bg-white lg:block">
            <Sidebar pathname={pathname} userEmail={session?.user.email} />
          </aside>

          {mobileOpen && (
            <div className="fixed inset-0 z-50 lg:hidden">
              <button className="absolute inset-0 bg-black/20" aria-label="Close navigation" onClick={() => setMobileOpen(false)} />
              <aside className="relative h-full w-[278px] border-r bg-white shadow-xl">
                <button className="absolute right-3 top-3 grid size-8 place-items-center rounded-md hover:bg-muted" onClick={() => setMobileOpen(false)} aria-label="Close navigation">
                  <X className="size-4" />
                </button>
                <Sidebar pathname={pathname} onNavigate={() => setMobileOpen(false)} userEmail={session?.user.email} />
              </aside>
            </div>
          )}

          <div className="min-w-0">
            <header className="flex h-[58px] items-center gap-3 border-b bg-white px-4 md:px-6">
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

function Sidebar({ pathname, onNavigate, userEmail }: { pathname: string; onNavigate?: () => void; userEmail?: string }) {
  return (
    <div className="flex h-full min-h-screen flex-col px-3 py-3">
      <button className="mb-4 flex items-center gap-2.5 rounded-lg px-2 py-1 text-left hover:bg-muted">
        <BrandMark />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-semibold">Open Spanner</span>
          <span className="block text-xs text-muted-foreground">Usage infrastructure</span>
        </span>
        <ChevronDown className="size-3.5" />
      </button>
      <nav className="scrollbar-subtle flex-1 overflow-y-auto">
        {navigation.map((group) => (
          <div key={group.label} className="mb-4">
            <p className="mb-1 px-2 text-[11px] font-medium tracking-wide text-muted-foreground/70">{group.label}</p>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const active = pathname === item.href || pathname.startsWith(item.href + "/");
                return (
                  <Link key={item.href} href={item.href} onClick={onNavigate} className={cn("flex h-8 items-center gap-2.5 rounded-md px-2 text-[13px] transition-colors", active ? "bg-neutral-100 font-medium text-foreground" : "text-neutral-700 hover:bg-muted")}>
                    <item.icon className={cn("size-4", active ? "text-neutral-900" : "text-neutral-500")} strokeWidth={1.7} />
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
