import type { LucideIcon } from "lucide-react";
import {
  Activity,
  Banknote,
  BarChart3,
  BookOpen,
  ClipboardList,
  FileQuestion,
  Gauge,
  Layers,
  LayoutGrid,
  PanelsTopLeft,
  Shapes,
  LayoutDashboard,
  MonitorDot,
  Package,
  Receipt,
  School,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  Wallet,
} from "lucide-react";
import type { Role } from "./types";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  adminOnly?: boolean;
  /** Match the href exactly instead of by prefix (dashboard roots). */
  exact?: boolean;
  /** Extra path prefixes that keep this item highlighted, e.g. the exam runtime under a hub. */
  also?: string[];
}

export interface NavGroup {
  id: string;
  /** Collapsed-rail button for multi-item groups; must differ from every child icon. */
  icon?: LucideIcon;
  items: NavItem[];
}

export const studentNav: NavItem[] = [
  { href: "/student", label: "Dashboard", icon: LayoutDashboard, exact: true },
  { href: "/student/tryouts", label: "Tryout", icon: BookOpen, also: ["/student/tryout", "/student/kecermatan"] },
  { href: "/catalog", label: "Katalog", icon: LayoutGrid },
];

// Group ids double as dictionary keys ("group:<id>") in lib/i18n/dict/nav.ts.
const adminGroups: NavGroup[] = [
  { id: "overview", items: [{ href: "/admin", label: "Ringkasan", icon: LayoutDashboard, exact: true }] },
  {
    id: "operations",
    icon: MonitorDot,
    items: [
      { href: "/admin/monitoring", label: "Monitoring", icon: Activity },
      { href: "/admin/students", label: "Siswa", icon: Users },
      { href: "/admin/results", label: "Hasil", icon: BarChart3 },
      { href: "/admin/kecermatan-results", label: "Hasil Kecermatan", icon: BarChart3 },
    ],
  },
  {
    id: "content",
    icon: Layers,
    items: [
      { href: "/admin/tryouts", label: "Tryout", icon: ClipboardList },
      { href: "/admin/kecermatan", label: "Tes Kecermatan", icon: Gauge },
      { href: "/admin/item-packages", label: "Paket Item", icon: Shapes },
      { href: "/admin/questions", label: "Bank Soal", icon: FileQuestion },
      { href: "/admin/products", label: "Produk", icon: Package },
      { href: "/admin/programs", label: "Program", icon: School, adminOnly: true },
      { href: "/admin/website", label: "Kelola Website", icon: PanelsTopLeft, adminOnly: true },
    ],
  },
  {
    id: "finance",
    icon: Wallet,
    items: [
      { href: "/admin/orders", label: "Pesanan", icon: Receipt },
      { href: "/admin/topups", label: "Top-up", icon: Banknote },
    ],
  },
  {
    id: "system",
    icon: SlidersHorizontal,
    items: [
      { href: "/admin/users", label: "Akun Pengguna", icon: ShieldCheck, adminOnly: true },
      { href: "/admin/settings", label: "Pengaturan", icon: Settings, adminOnly: true },
    ],
  },
];

export const navGroupsForRole = (role: Role): NavGroup[] => {
  if (role === "student") return [{ id: "student", items: studentNav }];
  return adminGroups
    .map((group) => ({ ...group, items: group.items.filter((item) => !item.adminOnly || role === "admin") }))
    .filter((group) => group.items.length > 0);
};

export function isNavActive(item: NavItem, pathname: string): boolean {
  if (item.exact) return pathname === item.href;
  return [item.href, ...(item.also ?? [])].some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}
