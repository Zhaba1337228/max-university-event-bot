"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Role, canCheckin, canManageEvents, roleLabel } from "@/lib/types";
import clsx from "clsx";

type Tab = { href: string; label: string };

// tabsForRole — какие табы видит пользователь в навигации.
//
//   • admin           — Дашборд + Мероприятия + Check-in
//   • organizer       — Дашборд + Мероприятия (без Check-in: гостей сканирует staff)
//   • staff           — Check-in (организаторские разделы скрыты)
//   • applicant       — не должно случиться: backend режет на этапе auth
function tabsForRole(role: Role): Tab[] {
  const out: Tab[] = [];
  if (canManageEvents(role)) {
    out.push({ href: "/dashboard", label: "Дашборд" });
    out.push({ href: "/events", label: "Мероприятия" });
  }
  if (canCheckin(role)) {
    out.push({ href: "/checkin", label: "Check-in" });
  }
  if (role === "admin") {
    out.push({ href: "/users", label: "Пользователи" });
  }
  return out;
}

export function Nav({ role }: { role: Role }) {
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);

  // Закрываем мобильное меню при смене страницы.
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
        <div className="flex items-center gap-2 sm:gap-5">
          <Link
            href={role === "staff" ? "/checkin" : "/dashboard"}
            className="flex items-center gap-2 text-base font-semibold text-text no-underline hover:text-accent"
          >
            <span aria-hidden className="inline-block h-6 w-6 rounded-md bg-gradient-to-br from-accent to-accentHover" />
            <span>MAX Bot Admin</span>
          </Link>
          {/* Desktop tabs */}
          <div className="hidden items-center gap-1 sm:flex">
            {tabs.map((t) => {
              const active = pathname === t.href || pathname.startsWith(t.href + "/");
              return (
                <Link
                  key={t.href}
                  href={t.href}
                  className={clsx(
                    "rounded-md px-3 py-1.5 text-sm no-underline transition-colors",
                    active
                      ? "bg-muted text-text"
                      : "text-subtle hover:bg-muted hover:text-text",
                  )}
                >
                  {t.label}
                </Link>
              );
            })}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <span
            className="hidden truncate rounded-full border border-border bg-muted/60 px-2.5 py-0.5 text-xs text-subtle md:inline"
            title="Ваша роль"
          >
            {roleLabel(role)}
          </span>
          <button
            type="button"
            onClick={onLogout}
            className="hidden rounded-md border border-border bg-muted px-3 py-1.5 text-sm text-text hover:bg-border sm:inline-flex"
          >
            Выйти
          </button>
          {/* Burger */}
          <button
            type="button"
            aria-label={mobileOpen ? "Закрыть меню" : "Открыть меню"}
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((v) => !v)}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border bg-muted text-text hover:bg-border sm:hidden"
          >
            {mobileOpen ? <IconClose /> : <IconBurger />}
          </button>
        </div>
      </div>

      {/* Mobile sheet */}
      {mobileOpen && (
        <div className="border-t border-border bg-surface sm:hidden">
          <div className="container flex flex-col gap-1 py-3">
            {tabs.map((t) => {
              const active = pathname === t.href || pathname.startsWith(t.href + "/");
              return (
                <Link
                  key={t.href}
                  href={t.href}
                  className={clsx(
                    "rounded-md px-3 py-2 text-base no-underline transition-colors",
                    active
                      ? "bg-muted text-text"
                      : "text-subtle hover:bg-muted hover:text-text",
                  )}
                >
                  {t.label}
                </Link>
              );
            })}
            <div className="mt-2 flex items-center justify-between gap-2 border-t border-border pt-3">
              <span className="text-xs text-subtle">{roleLabel(role)}</span>
              <button
                type="button"
                onClick={onLogout}
                className="rounded-md border border-border bg-muted px-3 py-2 text-sm text-text hover:bg-border"
              >
                Выйти
              </button>
            </div>
          </div>
        </div>
      )}
    </nav>
  );
}

function IconBurger() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="3" y1="6" x2="21" y2="6" />
      <line x1="3" y1="12" x2="21" y2="12" />
      <line x1="3" y1="18" x2="21" y2="18" />
    </svg>
  );
}

function IconClose() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  );
}
