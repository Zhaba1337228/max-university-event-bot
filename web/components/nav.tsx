"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";

const tabs = [
  { href: "/dashboard", label: "Дашборд" },
  { href: "/events", label: "Мероприятия" },
  { href: "/checkin", label: "Check-in" },
];

export function Nav() {
  const pathname = usePathname();
  const router = useRouter();

  async function onLogout() {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // ignore — cookie всё равно очистим клиентом
    }
    router.replace("/auth/login");
  }

  return (
    <nav className="border-b border-border bg-surface">
      <div className="container flex h-14 items-center justify-between">
        <div className="flex items-center gap-5">
          <Link href="/dashboard" className="text-base font-semibold text-text no-underline hover:text-accent">
            MAX Bot Admin
          </Link>
          <div className="flex items-center gap-1">
            {tabs.map((t) => {
              const active = pathname === t.href || pathname.startsWith(t.href + "/");
              return (
                <Link
                  key={t.href}
                  href={t.href}
                  className={
                    "rounded-md px-3 py-1.5 text-sm no-underline transition-colors " +
                    (active
                      ? "bg-muted text-text"
                      : "text-subtle hover:bg-muted hover:text-text")
                  }
                >
                  {t.label}
                </Link>
              );
            })}
          </div>
        </div>
        <button
          type="button"
          onClick={onLogout}
          className="rounded-md border border-border bg-muted px-3 py-1.5 text-sm text-text hover:bg-border"
        >
          Выйти
        </button>
      </div>
    </nav>
  );
}
