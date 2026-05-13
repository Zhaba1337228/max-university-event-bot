"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { Me } from "@/lib/types";
import { Nav } from "@/components/nav";
import { MeContext } from "@/components/me-context";

// Shell для всех страниц после входа.
//
// Обязанности:
//   1. Получить /api/auth/me; при 401 → /auth/login.
//   2. Применить role-based gate:
//      • staff попадает только на /checkin (всё остальное → /checkin)
//      • organizer не имеет права на /checkin (→ /forbidden)
//      • admin имеет доступ ко всему.
//   3. Прокинуть `me` в children через MeContext (без повторных fetch).
export default function AuthedLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await api.get<Me>("/api/auth/me");
        if (!cancelled) {
          setMe(data);
          setLoading(false);
        }
      } catch (e) {
        if (e instanceof HttpError && e.status === 401) {
          router.replace("/auth/login");
        } else {
          if (!cancelled) setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router]);

  // Role-based redirects (выполняем только после загрузки me).
  useEffect(() => {
    if (!me) return;
    const role = me.user.role;

    // Staff: только /checkin. Любая попытка зайти на организаторские страницы
    // → редирект на /checkin. Так у волонтёра нет лишних разделов.
    if (role === "staff") {
      const allowed = pathname === "/checkin" || pathname.startsWith("/forbidden");
      if (!allowed) {
        router.replace("/checkin");
      }
      return;
    }

    // Organizer (не admin): нет доступа к /checkin.
    if (role === "organizer" && pathname.startsWith("/checkin")) {
      router.replace("/forbidden?reason=checkin_organizer");
      return;
    }
  }, [me, pathname, router]);

  if (loading) {
    return (
      <div className="container py-16 text-subtle">Загрузка…</div>
    );
  }
  if (!me) {
    return null;
  }

  // Пока выполняется client-side редирект — рендерим пустой shell, чтобы
  // не мигало содержимое страницы, на которую staff/organizer не имеют прав.
  const role = me.user.role;
  if (role === "staff" && pathname !== "/checkin" && !pathname.startsWith("/forbidden")) {
    return <div className="container py-16 text-subtle">Перенаправляем…</div>;
  }
  if (role === "organizer" && pathname.startsWith("/checkin")) {
    return <div className="container py-16 text-subtle">Перенаправляем…</div>;
  }

  return (
    <MeContext.Provider value={me}>
      <div className="min-h-screen">
        <Nav role={role} />
        <main className="container py-6 sm:py-8">{children}</main>
      </div>
    </MeContext.Provider>
  );
}

