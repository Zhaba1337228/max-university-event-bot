"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { api, HttpError } from "@/lib/api";
import { CheckinResp, LookupByCodeResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Toast } from "@/components/ui/toast";
import {
  IconQr,
  IconCheckCircle,
  IconXCircle,
  IconAlertCircle,
  IconSearch,
  IconUser,
  IconCalendar,
  IconClock,
  IconX,
} from "@/components/ui/icons";

// QR-scanner — только в браузере (требует navigator.mediaDevices).
const Scanner = dynamic(
  () => import("@yudiel/react-qr-scanner").then((m) => m.Scanner),
  { ssr: false },
);

type Result =
  | { kind: "ok"; data: CheckinResp }
  | { kind: "err"; message: string };

type CamState =
  | "idle"
  | "checking"
  | "requesting"
  | "granted"
  | "denied"
  | "unsupported";

export default function CheckinPage() {
  const [camState, setCamState] = useState<CamState>("idle");
  const [manual, setManual] = useState("");
  const [result, setResult] = useState<Result | null>(null);
  const [busy, setBusy] = useState(false);
  const [shortCode, setShortCode] = useState("");
  const [lookupResult, setLookupResult] = useState<LookupByCodeResp | null>(null);
  const [lookupErr, setLookupErr] = useState<string | null>(null);
  const [lookupBusy, setLookupBusy] = useState(false);
  const lastScanRef = useRef<{ code: string; at: number } | null>(null);
  const probeStreamRef = useRef<MediaStream | null>(null);

  useEffect(() => {
    if (typeof navigator === "undefined") return;
    if (!navigator.mediaDevices?.getUserMedia) {
      setCamState("unsupported");
      return;
    }
    const perms = (
      navigator as Navigator & {
        permissions?: {
          query: (q: { name: string }) => Promise<{ state: string }>;
        };
      }
    ).permissions;
    if (!perms?.query) return;
    perms
      .query({ name: "camera" })
      .then((res) => {
        if (res.state === "granted") setCamState("granted");
        else if (res.state === "denied") setCamState("denied");
      })
      .catch(() => {});
    return () => {
      probeStreamRef.current?.getTracks().forEach((t) => t.stop());
      probeStreamRef.current = null;
    };
  }, []);

  const requestCamera = useCallback(async () => {
    if (typeof navigator === "undefined") return;
    if (!navigator.mediaDevices?.getUserMedia) {
      setCamState("unsupported");
      return;
    }
    setCamState("requesting");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: { ideal: "environment" } },
        audio: false,
      });
      stream.getTracks().forEach((t) => t.stop());
      setCamState("granted");
    } catch (err) {
      setCamState("denied");
      console.warn("[checkin] camera denied:", err);
    }
  }, []);

  const stopCamera = useCallback(() => {
    setCamState("idle");
  }, []);

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
    <div className="space-y-6 page-enter">
      <div>
        <h1 className="text-2xl font-bold sm:text-3xl">Check-in</h1>
        <p className="mt-1 text-sm text-subtle">
          Сканируйте QR-коды участников или ищите по коду записи
        </p>
      </div>

      {/* Result banner — shown prominently at top when there's a result */}
      {result && (
        <CheckinResultBanner result={result} onClear={() => setResult(null)} />
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        {/* Scanner card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconQr size={16} className="text-subtle" />
              Сканер QR
            </CardTitle>
          </CardHeader>
          <CardBody>
            <div className="relative mx-auto aspect-square w-full max-w-sm overflow-hidden rounded-xl border border-border bg-black">
              {camState === "granted" ? (
                <Scanner
                  onScan={(codes) => {
                    if (codes && codes.length > 0 && codes[0].rawValue) {
                      submit(codes[0].rawValue);
                    }
                  }}
                  onError={() => {}}
                  constraints={{ facingMode: "environment" }}
                  styles={{
                    container: { width: "100%", height: "100%" },
                    video: { width: "100%", height: "100%", objectFit: "cover" },
                  }}
                />
              ) : (
                <CameraPlaceholder state={camState} onRequest={requestCamera} />
              )}
            </div>

            <div className="mt-3 flex flex-wrap gap-2">
              {camState === "granted" ? (
                <Button variant="secondary" onClick={stopCamera} className="gap-1.5">
                  <IconX size={14} />
                  Выключить камеру
                </Button>
              ) : camState === "denied" ? (
                <Button onClick={requestCamera} className="gap-1.5">
                  Повторить запрос
                </Button>
              ) : camState === "unsupported" ? null : (
                <Button onClick={requestCamera} disabled={camState === "requesting"} className="gap-1.5">
                  <IconQr size={14} />
                  {camState === "requesting" ? "Запрашиваем…" : "Включить камеру"}
                </Button>
              )}
            </div>
          </CardBody>
        </Card>

        {/* Manual + lookup */}
        <div className="space-y-4">
          {/* Manual input */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconSearch size={16} className="text-subtle" />
                Ввести QR вручную
              </CardTitle>
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
                  placeholder="MAXUEB1.eyJlIj..."
                  value={manual}
                  onChange={(e) => setManual(e.target.value)}
                  inputMode="text"
                  autoCapitalize="off"
                  autoCorrect="off"
                  spellCheck={false}
                />
                <Button type="submit" disabled={busy || !manual.trim()} className="gap-1.5 sm:w-auto">
                  {busy ? "…" : "Отметить"}
                </Button>
              </form>
              <p className="mt-2 text-xs text-subtle">
                Форматы:{" "}
                <span className="font-mono text-text/70">MAXUEB1.&lt;base64&gt;</span>{" "}
                или{" "}
                <span className="font-mono text-text/70">MAXUEB:&lt;event&gt;:&lt;code&gt;</span>
              </p>
            </CardBody>
          </Card>

          {/* Lookup by short code */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconSearch size={16} className="text-subtle" />
                Найти по коду записи
              </CardTitle>
            </CardHeader>
            <CardBody>
              <p className="mb-3 text-sm text-subtle">
                Введите короткий код{" "}
                <span className="font-mono text-text/70">a1b2c3d4</span> — бот показывает его участнику после регистрации. Без отметки «пришёл».
              </p>
              <form
                className="flex flex-col gap-2 sm:flex-row"
                onSubmit={(e) => {
                  e.preventDefault();
                  const c = shortCode.trim();
                  if (!c) return;
                  setLookupBusy(true);
                  setLookupErr(null);
                  setLookupResult(null);
                  api
                    .get<LookupByCodeResp>(`/api/registrations/by-code?code=${encodeURIComponent(c)}`)
                    .then((d) => setLookupResult(d))
                    .catch((err) => {
                      setLookupErr(
                        err instanceof HttpError && err.status === 404
                          ? "Запись с таким кодом не найдена."
                          : err instanceof HttpError && err.body?.message
                            ? err.body.message
                            : "Ошибка поиска",
                      );
                    })
                    .finally(() => setLookupBusy(false));
                }}
              >
                <Input
                  placeholder="a1b2c3d4..."
                  value={shortCode}
                  onChange={(e) => setShortCode(e.target.value)}
                  autoCapitalize="off"
                  autoCorrect="off"
                  spellCheck={false}
                />
                <Button
                  type="submit"
                  disabled={lookupBusy || !shortCode.trim()}
                  className="sm:w-auto"
                >
                  {lookupBusy ? "Ищем…" : "Найти"}
                </Button>
              </form>

              {lookupErr && (
                <Toast message={lookupErr} kind="error" onDismiss={() => setLookupErr(null)} />
              )}

              {lookupResult && (
                <LookupResultCard data={lookupResult} />
              )}
            </CardBody>
          </Card>
        </div>
      </div>
    </div>
  );
}

// ─── Big result banner ──────────────────────────────────────────────

function CheckinResultBanner({ result, onClear }: { result: Result; onClear: () => void }) {
  if (result.kind === "err") {
    return (
      <div className="animate-in slide-in-from-top-2 duration-300 relative flex items-start gap-4 rounded-xl border border-danger/40 bg-danger/10 p-5">
        <IconXCircle size={36} className="shrink-0 text-danger" />
        <div className="flex-1">
          <div className="text-lg font-bold text-danger">Ошибка check-in</div>
          <div className="mt-0.5 text-sm text-danger/80">{result.message}</div>
        </div>
        <button
          type="button"
          onClick={onClear}
          className="shrink-0 rounded-md p-1 text-danger/60 hover:text-danger transition-colors"
          aria-label="Закрыть"
        >
          <IconX size={16} />
        </button>
      </div>
    );
  }

  const r = result.data.registration;
  const e = result.data.event;
  const alreadyDone = result.data.already_done;

  return (
    <div
      className={`animate-in slide-in-from-top-2 duration-300 relative rounded-xl border p-5 ${
        alreadyDone
          ? "border-warn/40 bg-warn/10"
          : "border-success/40 bg-success/10"
      }`}
    >
      <button
        type="button"
        onClick={onClear}
        className="absolute right-4 top-4 rounded-md p-1 text-subtle hover:text-text transition-colors"
        aria-label="Закрыть"
      >
        <IconX size={16} />
      </button>

      <div className="flex items-start gap-4">
        {alreadyDone ? (
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full border-2 border-warn/40 bg-warn/15">
            <IconAlertCircle size={24} className="text-warn" />
          </div>
        ) : (
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full border-2 border-success/40 bg-success/15">
            <IconCheckCircle size={24} className="text-success" />
          </div>
        )}

        <div className="flex-1 min-w-0 pr-6">
          <div className={`text-lg font-bold ${alreadyDone ? "text-warn" : "text-success"}`}>
            {alreadyDone ? "Уже отмечен ранее" : "Check-in успешен!"}
          </div>

          <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
            <InfoItem
              icon={<IconUser size={13} />}
              label="Участник"
              value={r.full_name || r.full_name_masked}
            />
            <InfoItem
              icon={<IconCalendar size={13} />}
              label="Мероприятие"
              value={e.title}
            />
            {r.checkin_at && (
              <InfoItem
                icon={<IconClock size={13} />}
                label={alreadyDone ? "Отмечен ранее" : "Отмечен в"}
                value={new Date(r.checkin_at).toLocaleString("ru-RU")}
              />
            )}
            {r.contact && (
              <InfoItem icon={<IconUser size={13} />} label="Контакт" value={r.contact} mono />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function LookupResultCard({ data }: { data: LookupByCodeResp }) {
  const r = data.registration;
  const e = data.event;

  const statusColors: Record<string, string> = {
    registered: "text-success",
    attended: "text-accent",
    cancelled: "text-danger",
    waitlist: "text-warn",
    no_show: "text-subtle",
  };

  return (
    <div className="mt-3 rounded-lg border border-border bg-muted/30 p-3 space-y-2 text-sm animate-in fade-in duration-200">
      <div className="flex items-center justify-between">
        <span className="font-medium text-text">{r.full_name_masked}</span>
        <span className={`font-medium ${statusColors[r.status] ?? "text-subtle"}`}>
          {r.status}
        </span>
      </div>
      <div className="text-xs text-subtle">{e.title}</div>
      {r.registered_at && (
        <div className="text-xs text-subtle">
          Зарегистрирован:{" "}
          {new Date(r.registered_at).toLocaleString("ru-RU")}
        </div>
      )}
      {r.checkin_at && (
        <div className="text-xs text-success">
          Отмечен: {new Date(r.checkin_at).toLocaleString("ru-RU")}
        </div>
      )}
    </div>
  );
}

function InfoItem({
  icon,
  label,
  value,
  mono,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-start gap-1.5">
      <span className="mt-0.5 text-subtle">{icon}</span>
      <div>
        <div className="text-xs text-subtle">{label}</div>
        <div className={`text-sm font-medium text-text ${mono ? "font-mono" : ""}`}>{value}</div>
      </div>
    </div>
  );
}

// ─── Camera placeholder ──────────────────────────────────────────────

function CameraPlaceholder({
  state,
  onRequest,
}: {
  state: CamState;
  onRequest: () => void;
}) {
  return (
    <div className="absolute inset-0 flex items-center justify-center text-center">
      {/* Decorative corners */}
      <div className="pointer-events-none absolute inset-4">
        <span className="absolute left-0 top-0 h-7 w-7 border-l-2 border-t-2 border-accent" />
        <span className="absolute right-0 top-0 h-7 w-7 border-r-2 border-t-2 border-accent" />
        <span className="absolute bottom-0 left-0 h-7 w-7 border-b-2 border-l-2 border-accent" />
        <span className="absolute bottom-0 right-0 h-7 w-7 border-b-2 border-r-2 border-accent" />
      </div>

      <div className="relative max-w-[20rem] space-y-4 p-6">
        {state === "unsupported" ? (
          <>
            <IconAlertCircle size={36} className="mx-auto text-warn" />
            <p className="text-sm text-subtle">
              Этот браузер не поддерживает камеру. Попробуйте Chrome или Safari, либо введите код вручную.
            </p>
          </>
        ) : state === "denied" ? (
          <>
            <IconXCircle size={36} className="mx-auto text-danger" />
            <p className="text-sm text-danger">
              Доступ к камере запрещён. Разрешите камеру в настройках сайта, затем нажмите «Повторить».
            </p>
            <p className="text-xs text-subtle">
              Chrome: 🔒 в адресной строке → «Камера: разрешить»
            </p>
          </>
        ) : state === "requesting" ? (
          <>
            <div className="mx-auto h-9 w-9 animate-pulse rounded-full bg-accent/30" />
            <p className="text-sm text-subtle">Запрашиваем доступ… подтвердите в браузере.</p>
          </>
        ) : state === "checking" ? (
          <p className="text-sm text-subtle">Проверяем разрешения…</p>
        ) : (
          <>
            <IconQr size={40} className="mx-auto text-accent/60" />
            <p className="text-sm text-subtle">
              Нажмите кнопку ниже, чтобы запустить сканер QR-кодов.
            </p>
            <button
              type="button"
              onClick={onRequest}
              className="inline-flex items-center gap-1.5 justify-center rounded-lg bg-accent px-4 py-2.5 text-sm font-medium text-white shadow-card hover:bg-accentHover transition-colors"
            >
              <IconQr size={14} />
              Включить камеру
            </button>
          </>
        )}
      </div>
    </div>
  );
}
