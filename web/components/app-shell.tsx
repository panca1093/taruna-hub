"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Button,
  Drawer,
  Dropdown,
  Header,
  Separator,
} from "@heroui/react";
import { ChevronsLeft, GraduationCap, LogOut, Menu, ShieldCheck, UserCog } from "lucide-react";
import { cn } from "@/lib/cn";
import { navForRole } from "@/lib/nav";
import { getSessionSettings, useSession } from "@/lib/session-context";
import { useNavT } from "@/lib/i18n/dict/nav";
import { useShellT } from "@/lib/i18n/dict/shell";
import { useCommonT } from "@/lib/i18n/dict/common";
import { Logo } from "./logo";
import { GradientAvatar } from "./gradient-avatar";
import { ThemeToggle } from "./theme-toggle";
import { LanguageToggle } from "./language-toggle";
import type { Role } from "@/lib/types";

const roleIcon: Record<Role, React.ElementType> = { student: GraduationCap, operator: UserCog, admin: ShieldCheck };

function NavLinks({ collapsed, onNavigate }: { collapsed?: boolean; onNavigate?: () => void }) {
  const pathname = usePathname();
  const { role } = useSession();
  const items = navForRole(role);
  const navT = useNavT();

  return (
    <nav className="flex flex-col gap-0.5 px-2">
      {items.map((item) => {
        const active = item.href === "/admin" || item.href === "/student"
          ? pathname === item.href
          : pathname.startsWith(item.href);
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            onClick={onNavigate}
            className={cn(
              "group relative flex items-center gap-3 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors",
              active
                ? "bg-accent-soft text-accent-soft-foreground"
                : "text-muted hover:bg-surface-secondary hover:text-foreground",
            )}
          >
            {active && (
              <span className="absolute inset-y-1 left-0 w-0.5 rounded-full bg-gradient-ring" aria-hidden />
            )}
            <Icon className="size-[18px] shrink-0" strokeWidth={2} />
            {!collapsed && <span className="truncate">{navT[item.href] ?? item.label}</span>}
          </Link>
        );
      })}
    </nav>
  );
}

function UserMenu({ collapsed }: { collapsed?: boolean }) {
  const { user, role, setRole } = useSession();
  const router = useRouter();
  const shellT = useShellT();
  const commonT = useCommonT();
  const roleLabel: Record<Role, string> = { student: commonT.student, operator: commonT.operator, admin: commonT.admin };

  return (
    <Dropdown>
      <Dropdown.Trigger
        className={cn(
          "flex w-full items-center gap-2.5 rounded-lg p-1.5 text-left outline-none hover:bg-surface-secondary",
          collapsed && "justify-center",
        )}
      >
        <GradientAvatar user={user} size="sm" />
        {!collapsed && (
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[13px] font-medium text-foreground">{user.name}</span>
            <span className="block truncate text-[11px] text-muted">{roleLabel[role]}</span>
          </span>
        )}
      </Dropdown.Trigger>
      <Dropdown.Popover placement="top start" className="w-64">
        <div className="px-3 py-2.5">
          <p className="truncate text-sm font-medium text-foreground">{user.name}</p>
          <p className="truncate text-xs text-muted">{user.email}</p>
        </div>
        <Separator />
        <Dropdown.Menu
          onAction={(key) => {
            if (key === "logout") router.push(role === "student" ? "/login" : "/login/staff");
            else if (typeof key === "string" && key.startsWith("role:")) setRole(key.slice(5) as Role);
          }}
        >
          <Dropdown.Section>
            <Header className="px-2.5 pt-2 pb-1 text-[11px] font-medium tracking-wide text-muted uppercase">
              {shellT.viewAsDemo}
            </Header>
            {(["student", "operator", "admin"] as Role[]).map((r) => {
              const Icon = roleIcon[r];
              return (
                <Dropdown.Item key={r} id={`role:${r}`} className="gap-2.5">
                  <Icon className="size-4 text-muted" />
                  <span className="flex-1">{roleLabel[r]}</span>
                  {role === r && <span className="size-1.5 rounded-full bg-gradient-ring" />}
                </Dropdown.Item>
              );
            })}
          </Dropdown.Section>
          <Separator className="my-1" />
          <Dropdown.Item id="logout" className="gap-2.5 text-danger">
            <LogOut className="size-4" />
            {commonT.logout}
          </Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}

export function AppShell({
  children,
  title,
  description,
  actions,
  maxWidth,
}: {
  children: React.ReactNode;
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  /** Constrains and centers main content — e.g. "max-w-5xl". Omit for full-width (tables, dashboards). */
  maxWidth?: string;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const { role } = useSession();
  const router = useRouter();
  const shellT = useShellT();

  useEffect(() => {
    const { autoLogout, idleMinutes } = getSessionSettings();
    if (!autoLogout) return;

    let timer: ReturnType<typeof setTimeout>;
    const reset = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        router.push(role === "student" ? "/login" : "/login/staff");
      }, idleMinutes * 60_000);
    };
    const events = ["mousemove", "keydown", "click", "scroll"] as const;
    events.forEach((e) => window.addEventListener(e, reset));
    reset();
    return () => {
      clearTimeout(timer);
      events.forEach((e) => window.removeEventListener(e, reset));
    };
  }, [role, router]);

  return (
    <div className="flex min-h-dvh bg-background">
      {/* Desktop sidebar */}
      <aside
        className={cn(
          "sticky top-0 hidden h-dvh shrink-0 flex-col border-r border-border bg-surface transition-[width] duration-200 lg:flex",
          collapsed ? "w-[68px]" : "w-64",
        )}
      >
        <div className={cn("flex h-14 items-center px-3.5", collapsed && "justify-center px-0")}>
          <Logo collapsed={collapsed} />
        </div>
        <div className="flex-1 overflow-y-auto py-2">
          <NavLinks collapsed={collapsed} />
        </div>
        <div className="border-t border-border p-2">
          <Button
            variant="ghost"
            size="sm"
            className={cn("mb-1 w-full justify-start gap-2.5 text-muted", collapsed && "justify-center px-0")}
            onPress={() => setCollapsed((c) => !c)}
          >
            <ChevronsLeft className={cn("size-4 shrink-0 transition-transform", collapsed && "rotate-180")} />
            {!collapsed && shellT.collapse}
          </Button>
          <UserMenu collapsed={collapsed} />
        </div>
      </aside>

      {/* Mobile drawer */}
      <Drawer isOpen={mobileOpen} onOpenChange={setMobileOpen}>
        <Drawer.Backdrop>
          <Drawer.Content placement="left">
            <Drawer.Dialog className="flex h-full w-72 flex-col">
              <Drawer.Header className="flex h-14 items-center px-3.5">
                <Logo />
              </Drawer.Header>
              <Drawer.Body className="flex-1 overflow-y-auto px-0 py-2">
                <NavLinks onNavigate={() => setMobileOpen(false)} />
              </Drawer.Body>
              <Drawer.Footer className="border-t border-border p-2">
                <UserMenu />
              </Drawer.Footer>
            </Drawer.Dialog>
          </Drawer.Content>
        </Drawer.Backdrop>
      </Drawer>

      <div className="flex min-h-dvh min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border bg-surface/85 px-4 backdrop-blur-sm lg:px-6">
          <Button variant="ghost" isIconOnly size="sm" className="lg:hidden" onPress={() => setMobileOpen(true)}>
            <Menu className="size-[18px]" />
          </Button>
          <div className="min-w-0 flex-1">
            {title && <h1 className="truncate text-[15px] font-semibold text-foreground">{title}</h1>}
            {description && <p className="hidden truncate text-xs text-muted sm:block">{description}</p>}
          </div>
          <div className="flex items-center gap-2">
            {actions}
            <LanguageToggle />
            <ThemeToggle />
          </div>
        </header>
        <main className="flex-1 px-4 py-6 lg:px-6 lg:py-8">
          {maxWidth ? <div className={cn("mx-auto", maxWidth)}>{children}</div> : children}
        </main>
      </div>
    </div>
  );
}
