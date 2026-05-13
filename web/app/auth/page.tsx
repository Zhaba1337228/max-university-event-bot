"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

// /auth?t=<magic-jwt>
// 1. читаем t из query
// 2. POST /api/auth/exchange — backend ставит cookie session_jwt
// 3. редирект на /dashboard
export default function AuthExchangePage() {
  return (
    <Suspense fallback={<Loading />}>
      <Exchange />
    </Suspense>
  );
}

function Loading() {
  return (
    <div className="container max-w-md py-16">
      <Card>
        <CardHeader>
          <CardTitle>Вход в админку</CardTitle>
        </CardHeader>
        <CardBody>
          <p className="text-subtle">Подготавливаем форму…</p>
        </CardBody>
      </Card>
    </div>
  );
}

function Exchange() {
  const router = useRouter();
  const sp = useSearchParams();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(true);

  useEffect(() => {
    const token = sp.get("t");
    if (!token) {
      setError("В ссылке нет параметра t. Запросите /admin_login в боте ещё раз.");
      setBusy(false);
      return;
    }
    (async () => {
      try {
        await api.post("/api/auth/exchange", { t: token });
        router.replace("/dashboard");
      } catch (e) {
        const msg =
          e instanceof HttpError && e.body?.message
            ? e.body.message
            : "Не удалось войти. Запросите новую ссылку через /admin_login.";
        setError(msg);
        setBusy(false);
      }
    })();
  }, [router, sp]);

  return (
    <div className="container max-w-md py-16">
      <Card>
        <CardHeader>
          <CardTitle>Вход в админку</CardTitle>
        </CardHeader>
        <CardBody>
          {busy && <p className="text-subtle">Проверяем ссылку…</p>}
          {error && (
            <>
              <p className="mb-4 text-danger">{error}</p>
              <Button variant="secondary" onClick={() => router.replace("/auth/login")}>
                Что делать?
              </Button>
            </>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
