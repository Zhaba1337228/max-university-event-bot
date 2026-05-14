"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { api, HttpError } from "@/lib/api";
import {
  CheckinResp,
  CheckinErrorBody,
  EventDTO,
  Registration,
  Participant,
} from "@/lib/types";
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
  | { kind: "err"; status: number; body: CheckinErrorBody | null; fallback: string };

// Состояния доступа к камере. Изначально idle — камера НЕ запрашивается
// автоматически, чтобы браузер не показывал permission prompt без явного
// действия пользователя.
type CamState =
  | "idle" // ничего не делали, ждём клик
  | "checking" // querying permission API
  | "requesting" // вызвали getUserMedia, ждём ответ юзера
  | "granted" // ok, можно монтировать <Scanner/>
  | "denied" // юзер отказал или политика блокирует
  | "unsupported"; // navigator.mediaDevices не доступен (старый браузер / http://)

export default function CheckinPage() {
  const [camState, setCamState] = useState<CamState>("idle");
  const [manual, setManual] = useState("");
  const [result, setResult] = useState<Result | null>(null);
  const [busy, setBusy] = useState(false);
  // anti-flood: один и тот же QR в пределах 3 секунд игнорируем
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
      .catch(() => {
        // Браузер не поддерживает 'camera' в Permissions — нормально, ждём клик.
      });
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
      // eslint-disable-next-line no-console
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
      if (e instanceof HttpError) {
        // body может быть как простой { error, message }, так и расширенной
        // CheckinErrorBody (на 409: отменена / waitlist / no_show / окно закрыто).
        const body = (e.body as unknown as CheckinErrorBody | null) ?? null;
        const fallback = body?.message ?? "Не удалось проверить QR";
        setResult({ kind: "err", status: e.status, body, fallback });
      } else {
        setResult({
          kind: "err",
          status: 0,
          body: null,
          fallback: "Сетевая ошибка",
        });
      }
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
          Доступно только волонтёрам на входе (роль{" "}
          <span className="font-medium text-text">staff</span>) и администраторам.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Сканер</CardTitle>
        </CardHeader>
        <CardBody>
          <div className="relative mx-auto aspect-square w-full max-w-md overflow-hidden rounded-md border border-border bg-black">
            {camState === "granted" ? (
              <Scanner
                onScan={(codes) => {
                  if (codes && codes.length > 0 && codes[0].rawValue) {
                    submit(codes[0].rawValue);
                  }
                }}
                onError={() => {
                  // Тихо: ошибки декодирования — нормально.
                }}
                constraints={{ facingMode: "environment" }}
                styles={{
                  container: { width: "100%", height: "100%" },
                  video: {
                    width: "100%",
                    height: "100%",
                    objectFit: "cover",
                  },
                }}
              />
            ) : (
              <CameraPlaceholder
                state={camState}
                onRequest={requestCamera}
              />
            )}
          </div>

          <div className="mt-3 flex flex-wrap gap-2">
            {camState === "granted" ? (
              <Button variant="secondary" onClick={stopCamera}>
                Выключить камеру
              </Button>
            ) : camState === "denied" ? (
              <Button onClick={requestCamera}>Повторить запрос</Button>
            ) : camState === "unsupported" ? null : (
              <Button onClick={requestCamera} disabled={camState === "requesting"}>
                {camState === "requesting" ? "Запрашиваем…" : "Включить камеру"}
              </Button>
            )}
            {result && (
              <Button variant="ghost" onClick={() => setResult(null)}>
                Очистить результат
              </Button>
            )}
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
              placeholder="MAXUEB1.eyJlIj..."
              value={manual}
              onChange={(e) => setManual(e.target.value)}
              inputMode="text"
              autoCapitalize="off"
              autoCorrect="off"
              spellCheck={false}
            />
            <Button
              type="submit"
              disabled={busy || !manual.trim()}
              className="sm:w-auto"
            >
              Отметить
            </Button>
          </form>
          <p className="mt-2 text-xs text-subtle">
            Принимаем оба формата: новый{" "}
            <span className="font-mono">MAXUEB1.&lt;base64&gt;</span>{" "}
            (зашифрованный) и старый{" "}
            <span className="font-mono">MAXUEB:&lt;event&gt;:&lt;code&gt;</span>.
          </p>
        </CardBody>
      </Card>

      {result && <ResultCard result={result} />}
    </div>
  );
}

// --- ResultCard и подкарты ------------------------------------------------

function ResultCard({ result }: { result: Result }) {
  if (result.kind === "ok") {
    return (
      <DetailedCard
        tone={result.data.already_done ? "warn" : "ok"}
        title={result.data.already_done ? "Уже отмечен" : "Готово"}
        subtitle={
          result.data.already_done
            ? "Этот участник уже был отмечен ранее. Данные ниже — для сверки."
            : "Участник отмечен на мероприятии. Можно пропускать."
        }
        registration={result.data.registration}
        event={result.data.event}
        participant={result.data.participant}
        scanner={result.data.scanner}
      />
    );
  }
  // Ошибка. Если бэк прислал подробную карточку (409 + registration/event) —
  // показываем её красным, чтобы волонтёр видел кого и куда «не пускают».
  const body = result.body;
  const hasDetail = !!(body?.registration || body?.event || body?.participant);
  return (
    <DetailedCard
      tone="err"
      title={errorTitle(body?.error, result.status)}
      subtitle={result.fallback}
      registration={hasDetail ? body?.registration : undefined}
      event={hasDetail ? body?.event : undefined}
      participant={hasDetail ? body?.participant : undefined}
    />
  );
}

function errorTitle(code: string | undefined, status: number): string {
  switch (code) {
    case "registration_cancelled":
      return "Регистрация отменена";
    case "registration_waitlist":
      return "В листе ожидания";
    case "registration_no_show":
      return "no_show";
    case "window_closed":
      return "Окно check-in закрыто";
    case "qr_tampered":
      return "QR подделан";
    case "qr_expired":
      return "QR истёк";
    case "bad_qr":
      return "Некорректный QR";
    case "not_registered":
      return "Регистрация не найдена";
    case "event_not_found":
      return "Событие не найдено";
    case "forbidden":
    case "role_forbidden":
      return "Нет прав";
    default:
      return status === 0 ? "Сетевая ошибка" : "Ошибка";
  }
}

function DetailedCard({
  tone,
  title,
  subtitle,
  registration,
  event,
  participant,
  scanner,
}: {
  tone: "ok" | "warn" | "err";
  title: string;
  subtitle: string;
  registration?: Registration;
  event?: EventDTO;
  participant?: Participant;
  scanner?: Participant;
}) {
  const toneBar =
    tone === "ok"
      ? "bg-success"
      : tone === "warn"
        ? "bg-warn"
        : "bg-danger";
  const toneText =
    tone === "ok"
      ? "text-success"
      : tone === "warn"
        ? "text-warn"
        : "text-danger";
  return (
    <Card>
      <div className={`h-1.5 w-full ${toneBar}`} />
      <CardHeader>
        <CardTitle className={toneText}>{title}</CardTitle>
        <p className="mt-1 text-sm text-subtle">{subtitle}</p>
      </CardHeader>
      <CardBody>
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {event && <EventBlock event={event} />}
          {(participant || registration) && (
            <ParticipantBlock
              participant={participant}
              registration={registration}
            />
          )}
          {registration && (
            <RegistrationBlock registration={registration} scanner={scanner} />
          )}
        </div>
      </CardBody>
    </Card>
  );
}

function EventBlock({ event }: { event: EventDTO }) {
  return (
    <Section title="Мероприятие">
      <Row label="Название" value={event.title} />
      <Row label="Начало" value={fmtDate(event.starts_at)} />
      {event.ends_at && <Row label="Окончание" value={fmtDate(event.ends_at)} />}
      {event.location && <Row label="Место" value={event.location} />}
      <Row label="Формат" value={fmtFormat(event.format)} />
      <Row label="Статус" value={fmtEventStatus(event.status)} />
    </Section>
  );
}

function ParticipantBlock({
  participant,
  registration,
}: {
  participant?: Participant;
  registration?: Registration;
}) {
  const name =
    participant?.full_name ||
    registration?.full_name ||
    registration?.full_name_masked ||
    "—";
  return (
    <Section title="Участник">
      <Row label="ФИО" value={name} mono={false} strong />
      {participant?.phone && <Row label="Телефон" value={participant.phone} mono />}
      {participant?.email && <Row label="Email" value={participant.email} mono />}
      {/* Если профиль обновлён после записи — показываем оба варианта. */}
      {registration?.contact &&
        registration.contact !== participant?.phone &&
        registration.contact !== participant?.email && (
          <Row
            label="Контакт при записи"
            value={registration.contact}
            mono
          />
        )}
      {registration?.interest_program && (
        <Row label="Программа интереса" value={registration.interest_program} />
      )}
      {participant?.max_user_id !== undefined && (
        <Row label="MAX user id" value={String(participant.max_user_id)} mono />
      )}
    </Section>
  );
}

function RegistrationBlock({
  registration,
  scanner,
}: {
  registration: Registration;
  scanner?: Participant;
}) {
  return (
    <Section title="Регистрация">
      <Row
        label="Статус"
        value={fmtRegStatus(registration.status)}
        strong
      />
      {registration.registered_at && (
        <Row
          label="Зарегистрирован(а)"
          value={fmtDate(registration.registered_at)}
        />
      )}
      {!registration.registered_at && registration.created_at && (
        <Row label="Создана" value={fmtDate(registration.created_at)} />
      )}
      {registration.cancelled_at && (
        <Row label="Отменена" value={fmtDate(registration.cancelled_at)} />
      )}
      {registration.waitlist_position !== undefined && (
        <Row
          label="Позиция в листе ожидания"
          value={`#${registration.waitlist_position}`}
        />
      )}
      {registration.checkin_at && (
        <Row label="Отмечен" value={fmtDate(registration.checkin_at)} />
      )}
      {scanner && (
        <Row
          label="Отметил"
          value={
            scanner.full_name
              ? `${scanner.full_name} (${roleLabelShort(scanner.role)})`
              : roleLabelShort(scanner.role)
          }
        />
      )}
      <Row label="Источник" value={registration.source} mono />
    </Section>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-semibold uppercase tracking-wide text-subtle">
        {title}
      </div>
      <div className="space-y-1 text-sm">{children}</div>
    </div>
  );
}

function Row({
  label,
  value,
  mono = false,
  strong = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
  strong?: boolean;
}) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-2">
      <span className="min-w-[10rem] text-subtle">{label}:</span>
      <span
        className={`${mono ? "font-mono" : ""} ${strong ? "font-medium" : ""} break-all`}
      >
        {value}
      </span>
    </div>
  );
}

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString("ru-RU", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

function fmtFormat(f: string): string {
  switch (f) {
    case "offline":
      return "Офлайн";
    case "online":
      return "Онлайн";
    case "hybrid":
      return "Гибрид";
    default:
      return f;
  }
}

function fmtEventStatus(s: string): string {
  switch (s) {
    case "open":
      return "Открытая регистрация";
    case "closed":
      return "Регистрация закрыта";
    case "cancelled":
      return "Отменено";
    case "completed":
      return "Завершено";
    case "draft":
      return "Черновик";
    default:
      return s;
  }
}

function fmtRegStatus(s: string): string {
  switch (s) {
    case "registered":
      return "Зарегистрирован(а)";
    case "waitlist":
      return "Лист ожидания";
    case "cancelled_by_user":
      return "Отменена пользователем";
    case "cancelled_by_organizer":
      return "Отменена организатором";
    case "attended":
      return "Пришёл/пришла";
    case "no_show":
      return "Не явился (no_show)";
    default:
      return s;
  }
}

function roleLabelShort(r: string): string {
  switch (r) {
    case "admin":
      return "админ";
    case "organizer":
      return "организатор";
    case "staff":
      return "волонтёр";
    default:
      return r;
  }
}

// CameraPlaceholder — что показывается на месте <Scanner/>, когда мы ещё не
// получили доступ к камере. Отрисовываем 4 угла рамки (а не только верхние!)
// и понятный CTA в центре.
function CameraPlaceholder({
  state,
  onRequest,
}: {
  state: CamState;
  onRequest: () => void;
}) {
  return (
    <div className="absolute inset-0 flex items-center justify-center text-center">
      {/* 4 уголка рамки — чисто декоративные, чтобы было понятно, где появится поток */}
      <div className="pointer-events-none absolute inset-4">
        <span className="absolute left-0 top-0 h-6 w-6 border-l-2 border-t-2 border-accent" />
        <span className="absolute right-0 top-0 h-6 w-6 border-r-2 border-t-2 border-accent" />
        <span className="absolute bottom-0 left-0 h-6 w-6 border-b-2 border-l-2 border-accent" />
        <span className="absolute bottom-0 right-0 h-6 w-6 border-b-2 border-r-2 border-accent" />
      </div>
      <div className="relative max-w-[20rem] space-y-3 p-6 text-sm">
        {state === "unsupported" ? (
          <p className="text-subtle">
            Этот браузер не поддерживает доступ к камере. Попробуйте Chrome
            или Safari, либо введите код вручную.
          </p>
        ) : state === "denied" ? (
          <>
            <p className="text-danger">
              Доступ к камере запрещён. Откройте настройки сайта в браузере и
              разрешите камеру, затем нажмите «Повторить запрос».
            </p>
            <p className="text-xs text-subtle">
              В Chrome: значок 🔒 рядом с адресной строкой → «Камера: разрешить».
            </p>
          </>
        ) : state === "requesting" ? (
          <p className="text-subtle">
            Запрашиваем доступ к камере… подтвердите запрос в браузере.
          </p>
        ) : state === "checking" ? (
          <p className="text-subtle">Проверяем разрешения…</p>
        ) : (
          <>
            <p className="text-subtle">
              Камера выключена. Чтобы сканировать QR — нажмите кнопку ниже,
              браузер спросит разрешение.
            </p>
            <button
              type="button"
              onClick={onRequest}
              className="inline-flex items-center justify-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/90"
            >
              Включить камеру
            </button>
          </>
        )}
      </div>
    </div>
  );
}
