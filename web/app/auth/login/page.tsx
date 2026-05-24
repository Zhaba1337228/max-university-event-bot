"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Toast } from "@/components/ui/toast";
import { IconLock, IconArrowRight } from "@/components/ui/icons";

export default function LoginPage() {
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
    <div className="flex min-h-screen items-center justify-center bg-bg px-4">
      <div className="w-full max-w-sm">
        {/* Logo + title */}
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-accent to-accentHover shadow-elevated">
            <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-text">MAX Admin</h1>
          <p className="mt-1 text-sm text-subtle">Панель управления мероприятиями</p>
        </div>

        {/* Card */}
        <div className="rounded-2xl border border-border bg-surface p-6 shadow-elevated">
          {/* How to get token */}
          <div className="mb-5 rounded-lg border border-accent/20 bg-accent/8 p-3.5">
            <div className="flex items-start gap-2.5">
              <IconLock size={16} className="mt-0.5 shrink-0 text-accent" />
              <div className="text-sm">
                <p className="font-medium text-text">Как войти</p>
                <p className="mt-0.5 text-subtle">
                  Отправьте{" "}
                  <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-text">
                    /admin_login
                  </code>{" "}
                  боту в MAX — он пришлёт ссылку-токен.
                </p>
              </div>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <div>
              <label className="mb-1.5 block text-xs font-medium text-subtle">
                Токен авторизации
              </label>
              <Input
                placeholder="Вставьте токен из бота..."
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoFocus
                autoComplete="off"
                spellCheck={false}
                className="font-mono text-xs"
              />
            </div>

            {error && (
              <Toast message={error} kind="error" onDismiss={() => setError(null)} />
            )}

            <Button
              type="submit"
              disabled={busy || !token.trim()}
              size="lg"
              className="w-full justify-center gap-2 mt-1"
            >
              {busy ? (
                "Входим…"
              ) : (
                <>
                  Войти
                  <IconArrowRight size={16} />
                </>
              )}
            </Button>
          </form>

          <p className="mt-4 text-center text-xs text-subtle">
            Токен действует <span className="text-text">5 минут</span>. Если истёк — повторите{" "}
            <code className="font-mono">/admin_login</code>.
          </p>
        </div>
      </div>
    </div>
  );
}
