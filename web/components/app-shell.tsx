"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { Button, Drawer, Dropdown, toast } from "@heroui/react";
import { ChevronDown, ChevronRight, Globe, LogOut, Menu, Moon, Receipt, ShieldCheck, Sparkles, Sun, User, Wallet } from "lucide-react";
import { cn } from "@/lib/cn";
import { isNavActive, navGroupsForRole, type NavGroup, type NavItem } from "@/lib/nav";
import { useSession, useUser } from "@/lib/session-context";
import { useLocale } from "@/lib/i18n/locale-context";
import { useHydrated } from "@/lib/use-hydrated";
import { useNavT } from "@/lib/i18n/dict/nav";
import { useShellT } from "@/lib/i18n/dict/shell";
import { useCommonT } from "@/lib/i18n/dict/common";
import { Logo } from "./logo";
import { Icon } from "./icon";
import { GradientAvatar } from "./gradient-avatar";
import { EmailVerifyBanner } from "./email-verify-banner";
import { WalletPopover } from "./wallet-popover";
import { PageHeading, type Crumb } from "./page-heading";
import { ThemeToggle } from "./theme-toggle";
import { LanguageToggle } from "./language-toggle";
import type { Role } from "@/lib/types";

const focusRing = "outline-none focus-visible:ring-2 focus-visible:ring-sidebar-accent";

/** stacked = icon over label (student rail, 88px); row = icon beside label (expanded); icon = icon only (admin collapsed). */
type NavLayout = "stacked" | "row" | "icon";

function NavLink({ item, active, layout, showBar = true, barInset, onNavigate, className }: { item: NavItem; active: boolean; layout: NavLayout; showBar?: boolean; barInset?: string; onNavigate?: () => void; className?: string }) {
  const navT = useNavT();
  const label = navT[item.href] ?? item.label;
  return (
    <Link
      href={item.href}
      title={layout === "icon" ? label : undefined}
      aria-label={layout === "icon" ? label : undefined}
      aria-current={active ? "page" : undefined}
      onClick={onNavigate}
      className={cn(
        "relative flex items-center transition-colors",
        focusRing,
        layout === "stacked" && "min-h-18 flex-col justify-center gap-2 rounded-tile text-nano",
        layout === "row" && "min-h-12.5 gap-3.5 rounded-tile px-4 text-small",
        layout === "icon" && "min-h-12 justify-center rounded-tile",
        active ? "bg-rail-active text-sidebar-foreground" : "text-sidebar-muted hover:bg-sidebar-hover hover:text-sidebar-foreground",
        className,
      )}
    >
      {active && showBar && <span className={cn("absolute inset-y-5 w-0.75 rounded-full bg-sidebar-accent shadow-rail-glow", barInset ?? "-left-2.5")} aria-hidden />}
      <Icon icon={item.icon} size="rail" />
      {layout !== "icon" && <span className="truncate">{label}</span>}
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
      <NavLink item={item} active={active} layout="row" showBar={false} onNavigate={onNavigate} className="min-h-10 gap-3 rounded-control px-3 py-1.5 font-normal" />
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
        className={cn("flex w-full items-center justify-between rounded-control px-3 py-1.5 text-label font-medium transition-colors hover:text-sidebar-foreground", focusRing, hasActive && !open ? "text-sidebar-foreground" : "text-sidebar-muted")}
      >
        <span>{name}</span>
        <ChevronDown className={cn("size-3.5 transition-transform", !open && "-rotate-90")} aria-hidden />
      </button>
      {open && (
        <ul id={panelId} className="ml-5 mt-0.5 flex flex-col">
          {group.items.map((item, index) => (
            <ChildLink key={item.href} item={item} active={isNavActive(item, pathname)} last={index === group.items.length - 1} onNavigate={onNavigate} />
          ))}
        </ul>
      )}
    </div>
  );
}

function NavLinks({ expanded, onNavigate }: { expanded: boolean; onNavigate?: () => void }) {
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
  const isStudent = role === "student";

  return (
    <nav aria-label={shellT.mainNavigation} className="flex flex-col gap-2">
      {groups.map((group, index) => {
        const flat = group.items.length === 1 || isStudent;
        if (!expanded || flat) {
          const layout: NavLayout = isStudent ? (expanded ? "row" : "stacked") : expanded ? "row" : "icon";
          return (
            <div key={group.id} className={cn("flex flex-col gap-2", !expanded && !isStudent && index > 0 && "mt-1 border-t border-sidebar-border pt-3")}>
              {group.items.map((item) => (
                <NavLink key={item.href} item={item} active={isNavActive(item, pathname)} layout={layout} barInset={expanded ? "-left-4" : "-left-2.5"} onNavigate={onNavigate} />
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
    <nav aria-label={shellT.mainNavigation} className="fixed inset-x-4 bottom-4 z-30 flex h-17.5 gap-1.5 rounded-hero bg-sidebar px-3 py-1.5 text-sidebar-foreground shadow-menu lg:hidden">
      {items.map((item) => {
        const active = isNavActive(item, pathname);
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "relative flex min-h-15 flex-1 flex-col items-center justify-center gap-1 rounded-tile text-micro transition-colors",
              focusRing,
              active ? "bg-rail-active text-sidebar-foreground" : "text-sidebar-muted",
            )}
          >
            {active && <span className="absolute left-1/2 top-0.5 h-0.5 w-5 -translate-x-1/2 rounded-full bg-sidebar-accent shadow-rail-glow" aria-hidden />}
            <Icon icon={item.icon} size="nav" />
            {navT[item.href] ?? item.label}
          </Link>
        );
      })}
    </nav>
  );
}

function RailBrand({ expanded, staff, homeHref }: { expanded: boolean; staff: boolean; homeHref: string }) {
  const shellT = useShellT();
  return (
    <Link
      href={homeHref}
      aria-label="Taruna Eduka"
      className={cn("flex min-h-12 items-center gap-3 whitespace-nowrap rounded-tile", focusRing, expanded ? "justify-start pl-2.5" : "justify-center", staff ? "mb-6" : "mb-16.5")}
    >
      <span className="font-display text-brand text-sidebar-foreground">
        T<span className="text-sidebar-accent">.</span>
      </span>
      {expanded && (
        <span className="font-display text-title text-sidebar-foreground">
          Taruna Eduka
          <small className="mt-0.5 block font-sans text-micro font-normal text-sidebar-muted">{staff ? shellT.staffTagline : shellT.studentTagline}</small>
        </span>
      )}
    </Link>
  );
}

function RailThemeButton({ expanded }: { expanded: boolean }) {
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useHydrated();
  const shellT = useShellT();
  const isDark = mounted && resolvedTheme === "dark";
  return (
    <button
      type="button"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label={shellT.changeTheme}
      title={shellT.changeTheme}
      className={cn("flex h-12 w-full items-center rounded-tile text-sidebar-muted transition-colors hover:bg-sidebar-hover hover:text-sidebar-foreground", focusRing, expanded ? "justify-start gap-3.5 px-4 text-caption" : "justify-center")}
    >
      <Icon icon={isDark ? Sun : Moon} size="rail" />
      {expanded && <span>{shellT.changeTheme}</span>}
    </button>
  );
}

function Rail({ expanded, onToggle, staff, homeHref }: { expanded: boolean; onToggle: () => void; staff: boolean; homeHref: string }) {
  const shellT = useShellT();
  const { role } = useSession();
  const commonT = useCommonT();
  const roleLabel: Record<Role, string> = { student: commonT.student, operator: commonT.operator, admin: commonT.admin };
  return (
    <aside
      className={cn(
        "sticky top-inset z-30 hidden h-rail shrink-0 flex-col rounded-rail bg-sidebar pt-6.5 pb-4 text-sidebar-foreground shadow-rail transition-[width] duration-300 ease-out motion-reduce:transition-none lg:flex",
        expanded ? (staff ? "w-rail-admin px-4" : "w-rail-open px-4") : "w-rail px-2.5",
      )}
    >
      <RailBrand expanded={expanded} staff={staff} homeHref={homeHref} />
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        aria-label={expanded ? shellT.collapse : shellT.expand}
        title={expanded ? shellT.collapse : shellT.expand}
        className="absolute -right-3 top-9 grid size-6.5 place-items-center rounded-full border border-border bg-surface text-foreground shadow-soft outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <ChevronRight className={cn("size-3.5 transition-transform", expanded && "rotate-180")} aria-hidden />
      </button>
      {staff && expanded && role && (
        <div className="mb-5 flex items-center gap-2.5 rounded-control bg-sidebar-hover px-3 py-2.5">
          <ShieldCheck className="size-4.5 text-sidebar-muted" aria-hidden />
          <p className="text-caption font-medium">
            {roleLabel[role]}
            <small className="block text-micro font-normal text-sidebar-muted">Taruna Eduka</small>
          </p>
        </div>
      )}
      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 py-0.5">
        <NavLinks expanded={expanded} />
      </div>
      <div className="mt-2">
        {!staff && expanded && (
          <p className="flex items-center gap-2.5 px-3 py-4 text-caption text-sidebar-muted">
            <Sparkles className="size-6 opacity-80" aria-hidden />
            <span>{shellT.railNote}</span>
          </p>
        )}
        <RailThemeButton expanded={expanded} />
      </div>
    </aside>
  );
}

function MenuRow({ id, icon, label, hint, arrow = true, danger }: { id: string; icon: React.ComponentType<{ className?: string }>; label: string; hint?: string; arrow?: boolean; danger?: boolean }) {
  const RowIcon = icon;
  return (
    <Dropdown.Item id={id} textValue={label} className={cn("gap-3 rounded-control px-2.5 py-3", danger && "text-danger")}>
      <RowIcon className={cn("size-4.5 shrink-0", danger ? "text-danger" : "text-muted")} />
      <span className="min-w-0 flex-1">
        <span className="block text-caption">{label}</span>
        {hint && <span className="mt-0.5 block text-micro text-muted">{hint}</span>}
      </span>
      {arrow && <ChevronRight className="size-3.5 text-muted" aria-hidden />}
    </Dropdown.Item>
  );
}

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
  const pages =
    role === "student"
      ? [
          { id: "/student/profile", label: shellT.profile, hint: shellT.profileHint, icon: User },
          { id: "/student/wallet", label: shellT.walletTransactions, hint: shellT.walletHint, icon: Wallet },
          { id: "/student/orders", label: shellT.orders, hint: shellT.ordersHint, icon: Receipt },
        ]
      : [];

  return (
    <Dropdown>
      <Dropdown.Trigger
        aria-label={`${shellT.accountMenu}: ${user.name}`}
        className="flex min-h-12 items-center gap-2.5 rounded-control p-1 text-left outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <GradientAvatar user={user} size="md" className="max-md:size-8.5" />
        <span className="hidden min-w-0 max-w-44 xl:block">
          <span className="block truncate text-caption font-medium leading-tight text-foreground">{user.name}</span>
          <span className="mt-0.5 block truncate text-micro leading-tight text-muted">{role ? roleLabel[role] : ""}</span>
        </span>
        <ChevronDown className="hidden size-3.5 text-muted md:block" aria-hidden />
      </Dropdown.Trigger>
      <Dropdown.Popover placement="bottom end" className="w-82.5 max-w-[calc(100vw-2rem)] p-2.5">
        <div className="mb-1.5 flex items-center gap-3 border-b border-border px-2 pt-2 pb-4">
          <GradientAvatar user={user} size="md" />
          <div className="min-w-0">
            <p className="truncate text-caption font-medium text-foreground">{user.name}</p>
            <p className="mt-0.5 truncate text-micro text-muted">{user.email ?? user.phone}</p>
          </div>
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
              {pages.map(({ id, label, hint, icon }) => (
                <MenuRow key={id} id={id} icon={icon} label={label} hint={hint} />
              ))}
            </Dropdown.Section>
          )}
          <Dropdown.Section aria-label={shellT.preferences}>
            <Dropdown.Item id="theme" textValue={shellT.theme} className="gap-3 rounded-control px-2.5 py-3">
              {isDark ? <Sun className="size-4.5 text-muted" aria-hidden /> : <Moon className="size-4.5 text-muted" aria-hidden />}
              <span className="flex-1 text-caption">{shellT.theme}</span>
              <span className="text-micro text-muted">{isDark ? shellT.themeDark : shellT.themeLight}</span>
            </Dropdown.Item>
            <Dropdown.Item id="locale" textValue={shellT.language} className="gap-3 rounded-control px-2.5 py-3">
              <Globe className="size-4.5 text-muted" aria-hidden />
              <span className="flex-1 text-caption">{shellT.language}</span>
              <span className="text-micro text-muted">{shellT.languageName}</span>
            </Dropdown.Item>
          </Dropdown.Section>
          <Dropdown.Section aria-label={commonT.logout}>
            <MenuRow id="logout" icon={LogOut} label={commonT.logout} arrow={false} danger />
          </Dropdown.Section>
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}

/** Admin topbar crumb: "Group > Page", derived from the nav so pages only pass a title. */
function useStaffCrumb(title?: string) {
  const pathname = usePathname();
  const navT = useNavT();
  const { role } = useSession();
  const groups = role && role !== "student" ? navGroupsForRole(role) : [];
  const group = groups.find((g) => g.items.some((item) => isNavActive(item, pathname)));
  return { group: group && group.items.length > 1 ? (navT[`group:${group.id}`] ?? group.id) : null, title };
}

export function AppShell({
  children,
  title,
  description,
  actions,
  breadcrumb,
  maxWidth,
}: {
  children: React.ReactNode;
  title?: string;
  description?: string;
  /** Rendered on the right of the page heading, beside the title. */
  actions?: React.ReactNode;
  /** Trail shown above the page title. */
  breadcrumb?: Crumb[];
  /** Constrains and centers main content — e.g. "max-w-5xl". Omit for full-width (tables, dashboards). */
  maxWidth?: string;
}) {
  const [pref, setPref] = useState<boolean | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const shellT = useShellT();
  const { role } = useSession();
  const isStudent = role === "student";
  const expanded = pref ?? !isStudent;
  const homeHref = isStudent ? "/student" : "/admin";
  const crumb = useStaffCrumb(title);

  return (
    <div className="flex min-h-dvh bg-background lg:gap-8 lg:p-inset lg:pr-6">
      <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-control focus:bg-surface focus:px-4 focus:py-2 focus:text-caption">
        {shellT.skipToContent}
      </a>
      <Rail expanded={expanded} onToggle={() => setPref(!expanded)} staff={!isStudent} homeHref={homeHref} />

      {!isStudent && (
        <Drawer isOpen={mobileOpen} onOpenChange={setMobileOpen}>
          <Drawer.Backdrop>
            <Drawer.Content placement="left">
              <Drawer.Dialog className="flex h-full w-72 flex-col bg-sidebar px-4 pt-6 text-sidebar-foreground" aria-label={shellT.navigation}>
                <Drawer.Header className="mb-6 p-0">
                  <RailBrand expanded staff homeHref={homeHref} />
                </Drawer.Header>
                <Drawer.Body className="flex-1 overflow-y-auto p-0 pb-4">
                  <NavLinks expanded onNavigate={() => setMobileOpen(false)} />
                </Drawer.Body>
              </Drawer.Dialog>
            </Drawer.Content>
          </Drawer.Backdrop>
        </Drawer>
      )}

      <div className="flex min-h-dvh min-w-0 flex-1 flex-col px-5 lg:min-h-0 lg:px-0">
        <header className="flex min-h-19 items-center gap-3 py-3 lg:min-h-topbar lg:gap-6 lg:px-1 lg:pt-0 lg:pb-4">
          {isStudent ? (
            <Logo size="md" plate className="shrink-0 [&_img]:max-lg:h-9" />
          ) : (
            <>
              <Button variant="ghost" isIconOnly size="sm" className="lg:hidden" onPress={() => setMobileOpen(true)} aria-label={shellT.openNavigation}>
                <Menu className="size-5" />
              </Button>
              <p className="flex min-w-0 items-center gap-2.5 text-caption text-muted">
                {crumb.group && (
                  <>
                    <span className="hidden truncate sm:inline">{crumb.group}</span>
                    <ChevronRight className="hidden size-3.5 shrink-0 sm:block" aria-hidden />
                  </>
                )}
                <strong className="truncate font-medium text-foreground">{crumb.title}</strong>
              </p>
            </>
          )}
          <div className="ml-auto flex items-center gap-3 lg:gap-6">
            {isStudent && <WalletPopover />}
            {!isStudent && (
              <div className="hidden items-center gap-1 md:flex">
                <LanguageToggle />
                <ThemeToggle />
              </div>
            )}
            <AccountMenu />
          </div>
        </header>
        <main id="main-content" tabIndex={-1} className={cn("flex-1 px-0 pt-4 outline-none lg:px-1 lg:pt-5", isStudent && "pb-28 lg:pb-0")}>
          <div className={cn(maxWidth && "mx-auto", maxWidth)}>
            <EmailVerifyBanner />
            {title && <PageHeading title={title} description={description} breadcrumb={breadcrumb} actions={actions} size={isStudent ? "display" : "compact"} />}
            {children}
          </div>
        </main>
        {isStudent && (
          <footer className="hidden items-center justify-between gap-4 px-1 pt-7 pb-5 text-micro text-muted lg:flex">
            <span>{shellT.footer}</span>
          </footer>
        )}
      </div>
      {isStudent && <MobileStudentNav />}
    </div>
  );
}
