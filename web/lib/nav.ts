import type { LucideIcon } from "lucide-react";
import {
  Activity,
  BarChart3,
  BookOpen,
  ClipboardList,
  FileQuestion,
  Gauge,
  LayoutDashboard,
  Package,
  Receipt,
  School,
  Settings,
  ShieldCheck,
  User,
  Users,
  Wallet,
} from "lucide-react";
import type { Role } from "./types";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  adminOnly?: boolean;
}

export const studentNav: NavItem[] = [
  { href: "/student", label: "Dashboard", icon: LayoutDashboard },
  { href: "/catalog", label: "Katalog", icon: BookOpen },
  { href: "/student/tryouts", label: "Tryout Saya", icon: ClipboardList },
  { href: "/student/orders", label: "Riwayat Pesanan", icon: Receipt },
  { href: "/student/wallet", label: "Saldo", icon: Wallet },
  { href: "/student/profile", label: "Profil", icon: User },
];

export const dashboardNav: NavItem[] = [
  { href: "/admin", label: "Dashboard", icon: LayoutDashboard },
  { href: "/admin/programs", label: "Program", icon: School, adminOnly: true },
  { href: "/admin/questions", label: "Bank Soal", icon: FileQuestion },
  { href: "/admin/tryouts", label: "Tryout", icon: ClipboardList },
  { href: "/admin/kecermatan", label: "Tes Kecermatan", icon: Gauge },
  { href: "/admin/products", label: "Produk", icon: Package },
  { href: "/admin/orders", label: "Pesanan", icon: BarChart3 },
  { href: "/admin/students", label: "Siswa", icon: Users },
  { href: "/admin/results", label: "Hasil", icon: BarChart3 },
  { href: "/admin/kecermatan-results", label: "Hasil Kecermatan", icon: BarChart3 },
  { href: "/admin/monitoring", label: "Monitoring", icon: Activity },
  { href: "/admin/users", label: "Akun Pengguna", icon: ShieldCheck, adminOnly: true },
  { href: "/admin/settings", label: "Pengaturan", icon: Settings, adminOnly: true },
];

export const navForRole = (role: Role): NavItem[] => {
  if (role === "student") return studentNav;
  return dashboardNav.filter((item) => !item.adminOnly || role === "admin");
};
