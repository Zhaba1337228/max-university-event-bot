"use client";

import { useCallback, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { api, HttpError } from "@/lib/api";
import { CheckinResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// QR-scanner — только в браузере (требует navigator.mediaDevices).
const Scanner = dynamic(
  () => import("@yudiel/react-qr-scanner").then((m) => m.Scanner),
  { ssr: false },
);

type Result =
  | { kind: "ok"; data: CheckinResp }
  | { kind: "err"; message: string };

export default function CheckinPage() {
  const [scanning, setScanning] = useState(true);
  const [manual, setManual] = useState("");
  const [result, setResult] = useState<Result | null>(null);
  const [busy, setBusy] = useState(false);
  // anti-flood: один и тот же QR в пределах 3 секунд игнорируем
  const lastScanRef = useRef<{ code: string; at: number } | null>(null);

  const submit = useCallback(async (qr: string) => {
    const trimmed = qr.trim();
    if (!trimmed) return;
    const now = Date.now();
    if (
      lastScanRef.current &&
      lastScanRef.current.code === trimmed &&
      now - lastScanRef.current.at < 3000
    ) {
      return;
    }
    lastScanRef.current = { code: trimmed, at: now };

    setBusy(true);
    try {
      const data = await api.post<CheckinResp>("/api/checkin", { qr: trimmed });
      setResult({ kind: "ok", data });
    } catch (e) {
      const msg =
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось проверить QR";
      setResult({ kind: "err", message: msg });
    } finally {
      setBusy(false);
    }
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold sm:text-3xl">Check-in</h1>
        <p className="mt-1 text-sm text-subtle">
          Наведите камеру на QR-код участника или введите его вручную.
          Доступно только волонтёрам на входе (роль <span className="font-medium text-text">staff</span>) и администраторам.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Сканер</CardTitle>
        </CardHeader>
        <CardBody>
          {scanning ? (
            <div className="overflow-hidden rounded-md border border-border bg-black">
              <Scanner
                onScan={(codes) => {
                  if (codes && codes.length > 0 && codes[0].rawValue) {
                    submit(codes[0].rawValue);
                  }
                }}
                onError={() => {
                  // Тихо: ошибки декодирования это нормально.
                }}
                constraints={{ facingMode: "environment" }}
                styles={{ container: { width: "100%" } }}
              />
            </div>
          ) : (
            <p className="text-subtle">Сканер выключен.</p>
          )}
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              variant="secondary"
              onClick={() => {
                setScanning((v) => !v);
                setResult(null);
              }}
            >
              {scanning ? "Выключить камеру" : "Включить камеру"}
            </Button>
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Ввести вручную</CardTitle>
        </CardHeader>
        <CardBody>
          <form
            className="flex flex-col gap-2 sm:flex-row"
            onSubmit={(e) => {
              e.preventDefault();
              submit(manual);
            }}
          >
            <Input
              placeholder="MAXUEB:42:abcd1234..."
              value={manual}
              onChange={(e) => setManual(e.target.value)}
              inputMode="text"
              autoCapitalize="off"
              autoCorrect="off"
              spellCheck={false}
            />
            <Button type="submit" disabled={busy || !manual.trim()} className="sm:w-auto">
              Отметить
            </Button>
          </form>
          <p className="mt-2 text-xs text-subtle">
            Формат: <span className="font-mono">MAXUEB:&lt;event_id&gt;:&lt;code&gt;</span>
          </p>
        </CardBody>
      </Card>

      {result && (
        <Card>
          <CardHeader>
            <CardTitle>
              {result.kind === "ok"
                ? result.data.already_done
                  ? "Уже отмечен"
                  : "Готово"
                : "Ошибка"}
            </CardTitle>
          </CardHeader>
          <CardBody>
            {result.kind === "ok" ? (
              <div className="space-y-1 text-sm">
                <div>
                  <span className="text-subtle">Мероприятие:</span>{" "}
                  {result.data.event.title}
                </div>
                <div>
                  <span className="text-subtle">Участник:</span>{" "}
                  {result.data.registration.full_name || result.data.registration.full_name_masked}
                </div>
                {result.data.registration.contact && (
                  <div>
                    <span className="text-subtle">Контакт:</span>{" "}
                    <span className="font-mono">
                      {result.data.registration.contact}
                    </span>
                  </div>
                )}
                {result.data.registration.checkin_at && (
                  <div className="text-subtle">
                    Отметка:{" "}
                    {new Date(result.data.registration.checkin_at).toLocaleString("ru-RU")}
                  </div>
                )}
              </div>
            ) : (
              <p className="text-danger">{result.message}</p>
            )}
          </CardBody>
        </Card>
      )}
    </div>
  );
}
