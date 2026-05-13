"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { Me } from "@/lib/types";
import { Nav } from "@/components/nav";

// Shell для всех страниц после входа.
// При 401 → редирект на /auth/login.
export default function AuthedLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
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

  if (loading) {
    return (
      <div className="container py-16 text-subtle">Загрузка…</div>
    );
  }
  if (!me) {
    return null;
  }
  return (
    <div className="min-h-screen">
      <Nav />
      <main className="container py-6">{children}</main>
    </div>
  );
}
