"use client";

import { useCallback, useEffect, useRef, useState } from "react";
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
  // Поток, который мы поднимаем для запроса разрешения. После клика
  // «Включить камеру» оставляем его живым и отдаём контроль библиотеке,
  // которая внутри сама зовёт getUserMedia ещё раз — браузер второй раз
  // не запрашивает, потому что разрешение уже выдано на ориджин.
  const probeStreamRef = useRef<MediaStream | null>(null);

  // Если браузер поддерживает Permissions API — пытаемся узнать заранее,
  // не запрашивая (тип "camera" — экспериментальный, но в Chrome работает).
  useEffect(() => {
    if (typeof navigator === "undefined") return;
    if (!navigator.mediaDevices?.getUserMedia) {
      setCamState("unsupported");
      return;
    }
    // Permissions API опциональный. Если есть — узнаём текущий статус.
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
        // 'prompt' оставляем idle — пусть пользователь нажмёт кнопку.
      })
      .catch(() => {
        // Браузер не поддерживает 'camera' в Permissions — нормально, ждём клик.
      });
    return () => {
      probeStreamRef.current?.getTracks().forEach((t) => t.stop());
      probeStreamRef.current = null;
    };
  }, []);

  // requestCamera — основная точка входа. Запрашивает у пользователя
  // разрешение через getUserMedia. Если ok — сворачивает свой пробный
  // поток (библиотека откроет свой) и переводит в granted.
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
      // Сразу гасим пробный поток — он нам не нужен, его задача была
      // показать пользователю prompt и получить permission на origin.
      stream.getTracks().forEach((t) => t.stop());
      setCamState("granted");
    } catch (err) {
      // NotAllowedError | PermissionDeniedError → юзер отказал
      // NotFoundError → нет устройства
      // SecurityError → http без localhost
      setCamState("denied");
      // не сбрасываем результат предыдущих check-in'ов
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
          Доступно только волонтёрам на входе (роль{" "}
          <span className="font-medium text-text">staff</span>) и администраторам.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Сканер</CardTitle>
        </CardHeader>
        <CardBody>
          {/* Контейнер с фиксированным соотношением сторон — без него SVG-оверлей
              со скобками-уголками рисуется поверх схлопнутого <video> и видна
              только верхняя часть рамки. */}
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
                  {result.data.registration.full_name ||
                    result.data.registration.full_name_masked}
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
                    {new Date(
                      result.data.registration.checkin_at,
                    ).toLocaleString("ru-RU")}
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
