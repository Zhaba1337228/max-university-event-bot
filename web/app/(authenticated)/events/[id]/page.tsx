"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { EventDetail, canCheckin, canEditEvent } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";
import { useMe } from "@/components/me-context";

export default function EventDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const me = useMe();
  const id = Number(params.id);
  const [data, setData] = useState<EventDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  async function load() {
    setErr(null);
    try {
      const d = await api.get<EventDetail>(`/api/events/${id}`);
      setData(d);
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось загрузить",
      );
    }
  }

  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) {
      router.replace("/events");
      return;
    }
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function toggleStatus(target: "close" | "open") {
    if (!data) return;
    setBusy(true);
    setToast(null);
    try {
      await api.post(`/api/events/${id}/${target}`);
      setToast(target === "close" ? "Регистрация закрыта." : "Регистрация открыта.");
      await load();
    } catch (e) {
      setToast(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось обновить",
      );
    } finally {
      setBusy(false);
    }
  }

  if (err) return <p className="text-danger">{err}</p>;
  if (!data) return <p className="text-subtle">Загрузка…</p>;

  const ev = data.event;
  const st = data.stats;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link href="/events" className="text-sm text-subtle no-underline hover:text-text">
            ← К списку
          </Link>
          <h1 className="mt-1 break-words text-2xl font-semibold sm:text-3xl">{ev.title}</h1>
        </div>
        <Badge className={statusBadge(ev.status) + " self-start"}>{statusLabel(ev.status)}</Badge>
      </div>

      {toast && <p className="text-subtle">{toast}</p>}

      <Card>
        <CardHeader>
          <CardTitle>Информация</CardTitle>
        </CardHeader>
        <CardBody>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm md:grid-cols-2">
            <Row label="Начало" value={fmtDate(ev.starts_at)} />
            {ev.ends_at && <Row label="Окончание" value={fmtDate(ev.ends_at)} />}
            <Row label="Где" value={ev.location || "—"} />
            <Row label="Формат" value={ev.format} />
            <Row label="Вместимость" value={String(ev.capacity)} />
            {ev.tags && ev.tags.length > 0 && (
              <Row label="Теги" value={ev.tags.join(", ")} />
            )}
          </dl>
          {ev.description && (
            <p className="mt-4 whitespace-pre-line text-sm text-text/90">{ev.description}</p>
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Статистика</CardTitle>
        </CardHeader>
        <CardBody>
          {!st ? (
            <p className="text-subtle">Нет данных</p>
          ) : (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3 lg:grid-cols-7">
              <StatBox label="Зарегистр." value={st.registered} />
              <StatBox label="Свободно" value={st.free_seats} />
              <StatBox label="Лист ожид." value={st.waitlist} />
              <StatBox label="Отменили" value={st.cancelled} />
              <StatBox label="Пришли" value={st.attended} highlight />
              <StatBox label="Не пришли" value={st.no_show} />
              <StatBox label="Вместим." value={st.capacity} />
            </div>
          )}
          {st?.top_interests && Object.keys(st.top_interests).length > 0 && (
            <div className="mt-4">
              <div className="mb-1 text-xs text-subtle">Топ интересов</div>
              <div className="flex flex-wrap gap-2">
                {Object.entries(st.top_interests)
                  .sort((a, b) => b[1] - a[1])
                  .slice(0, 8)
                  .map(([k, v]) => (
                    <Badge key={k} className="bg-muted text-text border border-border">
                      {k} · {v}
                    </Badge>
                  ))}
              </div>
            </div>
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Действия</CardTitle>
        </CardHeader>
        <CardBody className="flex flex-wrap gap-2">
          <Link href={`/events/${id}/participants`}>
            <Button variant="secondary">Участники</Button>
          </Link>
          <Link href={`/events/${id}/broadcast`}>
            <Button variant="secondary">Рассылка</Button>
          </Link>
          {canEditEvent(me.user.role, me.user.id, ev.created_by) && (
            <Link href={`/events/${id}/edit`}>
              <Button variant="secondary">Редактировать</Button>
            </Link>
          )}
          {ev.status === "open" ? (
            <Button variant="danger" disabled={busy} onClick={() => toggleStatus("close")}>
              Закрыть регистрацию
            </Button>
          ) : (
            <Button variant="primary" disabled={busy} onClick={() => toggleStatus("open")}>
              Открыть регистрацию
            </Button>
          )}
          {/* Кнопка сканера видна только тем, кто имеет право на check-in (staff/admin).
              Для организатора она скрыта — QR-кодами гостей он не занимается. */}
          {canCheckin(me.user.role) && (
            <Link href={`/checkin?event=${id}`}>
              <Button variant="secondary">Check-in (камера)</Button>
            </Link>
          )}
        </CardBody>
      </Card>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <>
      <dt className="text-subtle">{label}</dt>
      <dd className="text-text">{value}</dd>
    </>
  );
}

function StatBox({ label, value, highlight }: { label: string; value: number; highlight?: boolean }) {
  return (
    <div
      className={
        "rounded-md border px-3 py-2 " +
        (highlight
          ? "border-success/30 bg-success/10"
          : "border-border bg-muted/40")
      }
    >
      <div className="text-xs text-subtle">{label}</div>
      <div className={"mt-0.5 text-lg font-semibold " + (highlight ? "text-success" : "")}>
        {value}
      </div>
    </div>
  );
}
