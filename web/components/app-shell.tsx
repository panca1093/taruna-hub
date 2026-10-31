"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { Button, Drawer, Dropdown, toast } from "@heroui/react";
import { ChevronDown, ChevronsLeft, Globe, LogOut, Menu, Moon, Receipt, Sun, User, Wallet } from "lucide-react";
import { cn } from "@/lib/cn";
import { isNavActive, navGroupsForRole, type NavGroup, type NavItem } from "@/lib/nav";
import { useSession, useUser } from "@/lib/session-context";
import { useLocale } from "@/lib/i18n/locale-context";
import { useHydrated } from "@/lib/use-hydrated";
import { useNavT } from "@/lib/i18n/dict/nav";
import { useShellT } from "@/lib/i18n/dict/shell";
import { useCommonT } from "@/lib/i18n/dict/common";
import { Logo } from "./logo";
import { GradientAvatar } from "./gradient-avatar";
import { EmailVerifyBanner } from "./email-verify-banner";
import { WalletPopover } from "./wallet-popover";
import type { Role } from "@/lib/types";

const focusRing = "outline-none focus-visible:ring-2 focus-visible:ring-sidebar-accent";

function NavLink({ item, active, collapsed, onNavigate, className }: { item: NavItem; active: boolean; collapsed?: boolean; onNavigate?: () => void; className?: string }) {
  const navT = useNavT();
  const label = navT[item.href] ?? item.label;
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      title={collapsed ? label : undefined}
      aria-label={collapsed ? label : undefined}
      aria-current={active ? "page" : undefined}
      onClick={onNavigate}
      className={cn(
        "relative flex items-center gap-3 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors",
        focusRing,
        collapsed && "justify-center px-0",
        active ? "bg-sidebar-active text-sidebar-foreground" : "text-sidebar-muted hover:bg-sidebar-active hover:text-sidebar-foreground",
        className,
      )}
    >
      {active && <span className="absolute inset-y-1.5 left-0 w-0.5 rounded-full bg-sidebar-accent" aria-hidden />}
      <Icon className="size-4.5 shrink-0" strokeWidth={2} />
      {!collapsed && <span className="truncate">{label}</span>}
    </Link>
  );
}

function ChildLink({ item, active, last, onNavigate }: { item: NavItem; active: boolean; last: boolean; onNavigate?: () => void }) {
  return (
    <li
      className={cn(
        "relative pl-4",
        "before:absolute before:left-0 before:top-0 before:border-l before:border-dashed before:border-sidebar-muted/60",
        last ? "before:h-1/2" : "before:h-full",
        "after:absolute after:left-0 after:top-1/2 after:w-3 after:border-t after:border-dashed after:border-sidebar-muted/60",
      )}
    >
      <NavLink item={item} active={active} onNavigate={onNavigate} className="py-1.5 font-normal" />
    </li>
  );
}

function AdminGroup({ group, open, onToggle, pathname, onNavigate }: { group: NavGroup; open: boolean; onToggle: () => void; pathname: string; onNavigate?: () => void }) {
  const navT = useNavT();
  const shellT = useShellT();
  const name = navT[`group:${group.id}`] ?? group.id;
  const panelId = `nav-group-${group.id}`;
  const hasActive = group.items.some((item) => isNavActive(item, pathname));
  return (
    <div>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={shellT.toggleGroup(name)}
        className={cn("flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wider transition-colors hover:text-sidebar-foreground", focusRing, hasActive && !open ? "text-sidebar-foreground" : "text-sidebar-muted")}
      >
        <span>{name}</span>
        <ChevronDown className={cn("size-3.5 transition-transform", !open && "-rotate-90")} aria-hidden />
      </button>
      {open && (
        <ul id={panelId} className="ml-4.5 mt-0.5 flex flex-col">
          {group.items.map((item, index) => (
            <ChildLink key={item.href} item={item} active={isNavActive(item, pathname)} last={index === group.items.length - 1} onNavigate={onNavigate} />
          ))}
        </ul>
      )}
    </div>
  );
}

function NavLinks({ collapsed, onNavigate }: { collapsed?: boolean; onNavigate?: () => void }) {
  const pathname = usePathname();
  const { role } = useSession();
  const shellT = useShellT();
  const groups = role ? navGroupsForRole(role) : [];
  const activeGroup = groups.find((group) => group.items.some((item) => isNavActive(item, pathname)))?.id ?? null;
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const [seenGroup, setSeenGroup] = useState(activeGroup);
  if (seenGroup !== activeGroup) {
    setSeenGroup(activeGroup);
    if (activeGroup) setToggled((prev) => ({ ...prev, [activeGroup]: true }));
  }

  return (
    <nav aria-label={shellT.mainNavigation} className="flex flex-col gap-1 px-2">
      {groups.map((group, index) => {
        const flat = group.items.length === 1 || role === "student";
        if (collapsed || flat) {
          return (
            <div key={group.id} className={cn("flex flex-col gap-0.5", collapsed && index > 0 && "mt-1 border-t border-sidebar-border pt-1")}>
              {group.items.map((item) => (
                <NavLink key={item.href} item={item} active={isNavActive(item, pathname)} collapsed={collapsed} onNavigate={onNavigate} />
              ))}
            </div>
          );
        }
        return (
          <AdminGroup
            key={group.id}
            group={group}
            open={toggled[group.id] ?? group.id === activeGroup}
            onToggle={() => setToggled((prev) => ({ ...prev, [group.id]: !(prev[group.id] ?? group.id === activeGroup) }))}
            pathname={pathname}
            onNavigate={onNavigate}
          />
        );
      })}
    </nav>
  );
}

function MobileStudentNav() {
  const pathname = usePathname();
  const navT = useNavT();
  const shellT = useShellT();
  const { role } = useSession();
  const items = role ? navGroupsForRole(role).flatMap((group) => group.items) : [];
  return (
    <nav aria-label={shellT.mainNavigation} className="fixed inset-x-0 bottom-0 z-30 flex border-t border-sidebar-border bg-sidebar text-sidebar-foreground lg:hidden">
      {items.map((item) => {
        const active = isNavActive(item, pathname);
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            className={cn("flex min-h-14 flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium", focusRing, active ? "text-sidebar-accent" : "text-sidebar-muted")}
          >
            <Icon className="size-5" strokeWidth={2} aria-hidden />
            {navT[item.href] ?? item.label}
          </Link>
        );
      })}
    </nav>
  );
}

/** Quick wallet entry. The wallet popover replaces this link without touching its siblings. */
function AccountMenu() {
  const { role, logout } = useSession();
  const user = useUser();
  const router = useRouter();
  const commonT = useCommonT();
  const shellT = useShellT();
  const { locale, setLocale } = useLocale();
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useHydrated();
  const isDark = mounted && resolvedTheme === "dark";
  const roleLabel: Record<Role, string> = { student: commonT.student, operator: commonT.operator, admin: commonT.admin };
  const goHome = role === "student" ? "/login" : "/portal";
  const pages = role === "student"
    ? [
        { id: "/student/profile", label: shellT.profile, icon: User },
        { id: "/student/orders", label: shellT.orders, icon: Receipt },
        { id: "/student/wallet", label: shellT.walletTransactions, icon: Wallet },
      ]
    : [];

  return (
    <Dropdown>
      <Dropdown.Trigger
        aria-label={`${shellT.accountMenu}: ${user.name}`}
        className="flex items-center gap-2.5 rounded-lg p-1 text-left outline-none hover:bg-surface-secondary focus-visible:ring-2 focus-visible:ring-focus"
      >
        <GradientAvatar user={user} size="sm" />
        <span className="hidden min-w-0 max-w-40 md:block">
          <span className="block truncate text-[13px] font-medium leading-tight text-foreground">{user.name}</span>
          <span className="block truncate text-[11px] leading-tight text-muted">{role ? roleLabel[role] : ""}</span>
        </span>
        <ChevronDown className="mr-1 hidden size-3.5 text-muted md:block" aria-hidden />
      </Dropdown.Trigger>
      <Dropdown.Popover placement="bottom end" className="w-64">
        <div className="px-3 py-2.5">
          <p className="truncate text-sm font-medium text-foreground">{user.name}</p>
          <p className="truncate text-xs text-muted">{user.email ?? user.phone}</p>
        </div>
        <Dropdown.Menu
          aria-label={shellT.accountMenu}
          onAction={(key) => {
            const id = String(key);
            if (id === "logout") logout().then(() => router.push(goHome), () => toast.danger(commonT.logoutFailed));
            else if (id === "theme") setTheme(isDark ? "light" : "dark");
            else if (id === "locale") setLocale(locale === "id" ? "en" : "id");
            else router.push(id);
          }}
        >
          {pages.length > 0 && (
            <Dropdown.Section aria-label={shellT.accountPages}>
              {pages.map(({ id, label, icon: Icon }) => (
                <Dropdown.Item key={id} id={id} textValue={label} className="gap-2.5">
                  <Icon className="size-4" aria-hidden />
                  {label}
                </Dropdown.Item>
              ))}
            </Dropdown.Section>
          )}
          <Dropdown.Section aria-label={shellT.preferences}>
            <Dropdown.Item id="theme" textValue={shellT.theme} className="gap-2.5">
              {isDark ? <Sun className="size-4" aria-hidden /> : <Moon className="size-4" aria-hidden />}
              <span className="flex-1">{shellT.theme}</span>
              <span className="text-xs text-muted">{isDark ? shellT.themeDark : shellT.themeLight}</span>
            </Dropdown.Item>
            <Dropdown.Item id="locale" textValue={shellT.language} className="gap-2.5">
              <Globe className="size-4" aria-hidden />
              <span className="flex-1">{shellT.language}</span>
              <span className="text-xs text-muted">{shellT.languageName}</span>
            </Dropdown.Item>
          </Dropdown.Section>
          <Dropdown.Section aria-label={commonT.logout}>
            <Dropdown.Item id="logout" textValue={commonT.logout} className="gap-2.5 text-danger">
              <LogOut className="size-4" aria-hidden />
              {commonT.logout}
            </Dropdown.Item>
          </Dropdown.Section>
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
  const shellT = useShellT();
  const { role } = useSession();
  const isStudent = role === "student";
  return (
    <div className="flex min-h-dvh bg-background">
      <aside
        className={cn(
          "sticky top-0 hidden h-dvh shrink-0 flex-col bg-sidebar text-sidebar-foreground transition-[width] duration-200 lg:flex",
          collapsed ? "w-17" : "w-64",
        )}
      >
        <div className={cn("flex h-14 items-center px-3.5", collapsed && "justify-center px-0")}>
          <Logo collapsed={collapsed} className={cn("rounded-lg bg-surface focus-visible:ring-2 focus-visible:ring-sidebar-accent", collapsed ? "px-1.5 py-0.5" : "px-2 py-1")} />
        </div>
        <div className="flex-1 overflow-y-auto py-2">
          <NavLinks collapsed={collapsed} />
        </div>
        <div className="border-t border-sidebar-border p-2">
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? shellT.expand : shellT.collapse}
            title={collapsed ? shellT.expand : shellT.collapse}
            className={cn("flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm text-sidebar-muted transition-colors hover:bg-sidebar-active hover:text-sidebar-foreground", focusRing, collapsed && "justify-center px-0")}
          >
            <ChevronsLeft className={cn("size-4 shrink-0 transition-transform", collapsed && "rotate-180")} aria-hidden />
            {!collapsed && shellT.collapse}
          </button>
        </div>
      </aside>

      {!isStudent && (
        <Drawer isOpen={mobileOpen} onOpenChange={setMobileOpen}>
          <Drawer.Backdrop>
            <Drawer.Content placement="left">
              <Drawer.Dialog className="flex h-full w-72 flex-col bg-sidebar text-sidebar-foreground" aria-label={shellT.navigation}>
                <Drawer.Header className="flex h-14 items-center px-3.5">
                  <Logo className="rounded-lg bg-surface px-2 py-1 focus-visible:ring-2 focus-visible:ring-sidebar-accent" />
                </Drawer.Header>
                <Drawer.Body className="flex-1 overflow-y-auto px-0 py-2">
                  <NavLinks onNavigate={() => setMobileOpen(false)} />
                </Drawer.Body>
              </Drawer.Dialog>
            </Drawer.Content>
          </Drawer.Backdrop>
        </Drawer>
      )}

      <div className="flex min-h-dvh min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border bg-surface/85 px-4 backdrop-blur-sm lg:px-6">
          {isStudent ? (
            <Logo className="lg:hidden" />
          ) : (
            <Button variant="ghost" isIconOnly size="sm" className="lg:hidden" onPress={() => setMobileOpen(true)} aria-label={shellT.openNavigation}>
              <Menu className="size-4.5" />
            </Button>
          )}
          <div className="min-w-0 flex-1">
            {title && <h1 className="truncate text-[15px] font-semibold text-foreground">{title}</h1>}
            {description && <p className="hidden truncate text-xs text-muted sm:block">{description}</p>}
          </div>
          <div className="flex items-center gap-1.5">
            {actions}
            {isStudent && <WalletPopover />}
            <AccountMenu />
          </div>
        </header>
        <main className={cn("flex-1 px-4 py-6 lg:px-6 lg:py-8", isStudent && "pb-24 lg:pb-8")}>
          <div className={cn(maxWidth && "mx-auto", maxWidth)}>
            <EmailVerifyBanner />
            {children}
          </div>
        </main>
      </div>
      {isStudent && <MobileStudentNav />}
    </div>
  );
}
