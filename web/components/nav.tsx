"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Role, canCheckin, canManageEvents, roleLabel } from "@/lib/types";
import clsx from "clsx";
import {
  IconHome,
  IconCalendar,
  IconQr,
  IconUsers,
  IconShield,
  IconLogOut,
  IconMenu,
  IconX,
  IconLock,
} from "@/components/ui/icons";

type Tab = { href: string; label: string; icon: React.ReactNode };

// tabsForRole — какие табы видит пользователь в навигации.
function tabsForRole(role: Role): Tab[] {
  const out: Tab[] = [];
  if (canManageEvents(role)) {
    out.push({ href: "/dashboard", label: "Дашборд", icon: <IconHome size={16} /> });
    out.push({ href: "/events", label: "Мероприятия", icon: <IconCalendar size={16} /> });
  }
  if (canCheckin(role)) {
    out.push({ href: "/checkin", label: "Check-in", icon: <IconQr size={16} /> });
  }
  if (role === "admin") {
    out.push({ href: "/users", label: "Пользователи", icon: <IconUsers size={16} /> });
  }
  // Страница прав доступа видна всем аутентифицированным (кроме staff)
  if (canManageEvents(role)) {
    out.push({ href: "/roles", label: "Доступы", icon: <IconLock size={16} /> });
  }
  return out;
}

// Role color scheme
const roleColors: Record<Role, { bg: string; text: string; dot: string }> = {
  admin: {
    bg: "bg-accent/15",
    text: "text-accent",
    dot: "bg-accent",
  },
  organizer: {
    bg: "bg-blue-500/15",
    text: "text-blue-400",
    dot: "bg-blue-400",
  },
  staff: {
    bg: "bg-emerald-500/15",
    text: "text-emerald-400",
    dot: "bg-emerald-400",
  },
  applicant: {
    bg: "bg-muted",
    text: "text-subtle",
    dot: "bg-subtle",
  },
};

function RoleBadge({ role }: { role: Role }) {
  const c = roleColors[role];
  return (
    <span
      className={clsx(
        "hidden items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium md:inline-flex",
        c.bg,
        c.text,
      )}
      title="Ваша роль"
    >
      {role === "admin" && <IconShield size={12} />}
      {roleLabel(role)}
    </span>
  );
}

export function Nav({ role }: { role: Role }) {
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  const tabs = tabsForRole(role);

  async function onLogout() {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // ignore — cookie всё равно очистим клиентом
    }
    router.replace("/auth/login");
  }

  return (
    <nav className="sticky top-0 z-20 border-b border-border bg-surface/95 backdrop-blur supports-[backdrop-filter]:bg-surface/80">
      <div className="container flex h-14 items-center justify-between gap-3">
        {/* Logo + Desktop tabs */}
        <div className="flex items-center gap-1 sm:gap-4">
          <Link
            href={role === "staff" ? "/checkin" : "/dashboard"}
            className="flex items-center gap-2 text-sm font-semibold text-text no-underline hover:text-accent transition-colors"
          >
            <span
              aria-hidden
              className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-accent to-accentHover shadow-card text-white"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
              </svg>
            </span>
            <span className="hidden sm:inline">MAX Admin</span>
          </Link>

          {/* Divider */}
          <div className="hidden h-5 w-px bg-border sm:block" />

          {/* Desktop tabs */}
          <div className="hidden items-center gap-0.5 sm:flex">
            {tabs.map((t) => {
              const active = pathname === t.href || pathname.startsWith(t.href + "/");
              return (
                <Link
                  key={t.href}
                  href={t.href}
                  className={clsx(
                    "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm no-underline transition-colors",
                    active
                      ? "bg-accent/10 text-accent font-medium"
                      : "text-subtle hover:bg-muted hover:text-text",
                  )}
                >
                  {t.icon}
                  {t.label}
                </Link>
              );
            })}
          </div>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-2">
          <RoleBadge role={role} />

          <button
            type="button"
            onClick={onLogout}
            className="hidden items-center gap-1.5 rounded-md border border-border bg-muted px-3 py-1.5 text-sm text-text hover:bg-border transition-colors sm:inline-flex"
          >
            <IconLogOut size={14} />
            <span className="hidden md:inline">Выйти</span>
          </button>

          {/* Burger */}
          <button
            type="button"
            aria-label={mobileOpen ? "Закрыть меню" : "Открыть меню"}
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((v) => !v)}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border bg-muted text-text hover:bg-border transition-colors sm:hidden"
          >
            {mobileOpen ? <IconX size={16} /> : <IconMenu size={16} />}
          </button>
        </div>
      </div>

      {/* Mobile sheet */}
      {mobileOpen && (
        <div className="border-t border-border bg-surface sm:hidden animate-in slide-in-from-top-2 duration-200">
          <div className="container flex flex-col gap-1 py-3">
            {tabs.map((t) => {
              const active = pathname === t.href || pathname.startsWith(t.href + "/");
              return (
                <Link
                  key={t.href}
                  href={t.href}
                  className={clsx(
                    "flex items-center gap-2.5 rounded-md px-3 py-2.5 text-sm no-underline transition-colors",
                    active
                      ? "bg-accent/10 text-accent font-medium"
                      : "text-subtle hover:bg-muted hover:text-text",
                  )}
                >
                  {t.icon}
                  {t.label}
                </Link>
              );
            })}
            <div className="mt-2 flex items-center justify-between gap-2 border-t border-border pt-3">
              <RoleBadge role={role} />
              <span className="inline-flex md:hidden">
                <RoleBadge role={role} />
              </span>
              <button
                type="button"
                onClick={onLogout}
                className="flex items-center gap-1.5 rounded-md border border-border bg-muted px-3 py-2 text-sm text-text hover:bg-border transition-colors"
              >
                <IconLogOut size={14} />
                Выйти
              </button>
            </div>
          </div>
        </div>
      )}
    </nav>
  );
}
