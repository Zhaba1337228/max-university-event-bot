"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { api, HttpError } from "@/lib/api";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export default function LoginInfoPage() {
  const router = useRouter();
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const t = token.trim();
    if (!t) return;
    setBusy(true);
    setError(null);
    try {
      await api.post("/api/auth/exchange", { t });
      router.replace("/dashboard");
    } catch (err) {
      if (err instanceof HttpError) {
        setError(
          err.body?.message ??
            (err.status === 401
              ? "Токен недействителен или истёк. Запросите новый через /admin_login в боте."
              : `Ошибка ${err.status}`),
        );
      } else {
        setError("Не удалось подключиться к серверу.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container max-w-md py-16 space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Вход в панель управления</CardTitle>
        </CardHeader>
        <CardBody>
          <p className="mb-4 text-sm text-subtle">
            Отправьте команду{" "}
            <code className="rounded bg-muted px-1.5 py-0.5">/admin_login</code>{" "}
            боту в MAX — он пришлёт ссылку. Нажмите на неё или скопируйте токен из
            адресной строки и вставьте ниже.
          </p>

          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <Input
              placeholder="Вставьте токен сюда..."
              value={token}
              onChange={(e) => setToken(e.target.value)}
              autoFocus
              autoComplete="off"
              spellCheck={false}
            />
            {error && <p className="text-sm text-danger">{error}</p>}
            <Button type="submit" disabled={busy || !token.trim()}>
              {busy ? "Входим…" : "Войти"}
            </Button>
          </form>

          <p className="mt-6 text-xs text-subtle">
            Токен действует 5 минут. Если истёк — повторите команду{" "}
            <code className="rounded bg-muted px-1 py-0.5">/admin_login</code>.
          </p>

          <p className="mt-4 text-xs">
            <Link href="/" className="text-subtle hover:text-text">
              На главную
            </Link>
          </p>
        </CardBody>
      </Card>
    </div>
  );
}
