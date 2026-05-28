"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { EventDetail, canCheckin, canEditEvent } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Toast, ToastKind } from "@/components/ui/toast";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";
import { useMe } from "@/components/me-context";
import {
  IconArrowLeft,
  IconCalendar,
  IconLocation,
  IconUsers,
  IconClock,
  IconTag,
  IconEdit,
  IconBroadcast,
  IconList,
  IconQr,
  IconCheck,
  IconX,
  IconActivity,
  IconTrash,
} from "@/components/ui/icons";

const formatLabel: Record<string, string> = {
  offline: "Офлайн",
  online: "Онлайн",
  hybrid: "Гибрид",
};

export default function EventDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const me = useMe();
  const id = Number(params.id);
  const [data, setData] = useState<EventDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ msg: string; kind: ToastKind } | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

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

  async function deleteEvent() {
    setBusy(true);
    setToast(null);
    try {
      await api.delete(`/api/events/${id}`);
      router.replace("/events");
    } catch (e) {
      setToast({
        msg: e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось удалить",
        kind: "error",
      });
      setConfirmDelete(false);
    } finally {
      setBusy(false);
    }
  }

  async function toggleStatus(target: "close" | "open") {
    if (!data) return;
    setBusy(true);
    setToast(null);
    try {
      await api.post(`/api/events/${id}/${target}`);
      setToast({
        msg: target === "close" ? "Регистрация закрыта." : "Регистрация открыта.",
        kind: target === "close" ? "info" : "success",
      });
      await load();
    } catch (e) {
      setToast({
        msg: e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось обновить",
        kind: "error",
      });
    } finally {
      setBusy(false);
    }
  }

  if (err) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <Toast message={err} kind="error" />
        <Link href="/events">
          <Button variant="secondary" className="gap-1.5">
            <IconArrowLeft size={14} />
            К списку
          </Button>
        </Link>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-64 rounded-lg bg-muted animate-pulse" />
        <div className="h-44 rounded-xl bg-surface border border-border animate-pulse" />
        <div className="h-44 rounded-xl bg-surface border border-border animate-pulse" />
      </div>
    );
  }

  const ev = data.event;
  const st = data.stats;
  const filled = st ? Math.max(0, ev.capacity - st.free_seats) : 0;
  const pct = ev.capacity > 0 && st ? Math.round((filled / ev.capacity) * 100) : 0;
  const isFull = pct >= 100;
  const canEdit = canEditEvent(me.user.role, me.user.id, ev.created_by);

  return (
    <div className="space-y-6 page-enter">
      {/* Breadcrumb + title */}
      <div>
        <Link
          href="/events"
          className="inline-flex items-center gap-1.5 text-sm text-subtle no-underline hover:text-text transition-colors"
        >
          <IconArrowLeft size={14} />
          К мероприятиям
        </Link>
        <div className="mt-2 flex flex-wrap items-start justify-between gap-3">
          <h1 className="text-2xl font-bold leading-tight sm:text-3xl">{ev.title}</h1>
          <Badge className={statusBadge(ev.status) + " self-start text-sm px-3 py-1"}>
            {statusLabel(ev.status)}
          </Badge>
        </div>
      </div>

      {toast && (
        <Toast message={toast.msg} kind={toast.kind} onDismiss={() => setToast(null)} />
      )}

      <div className="grid gap-4 lg:grid-cols-3">
        {/* Left: info + description */}
        <div className="space-y-4 lg:col-span-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconCalendar size={16} className="text-subtle" />
                Информация
              </CardTitle>
              {ev.format && (
                <Badge
                  className={
                    ev.format === "offline"
                      ? "bg-blue-500/15 text-blue-400 border border-blue-400/25"
                      : ev.format === "online"
                        ? "bg-emerald-500/15 text-emerald-400 border border-emerald-400/25"
                        : "bg-warn/15 text-warn border border-warn/25"
                  }
                >
                  {formatLabel[ev.format] ?? ev.format}
                </Badge>
              )}
            </CardHeader>
            <CardBody>
              <div className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
                <InfoRow icon={<IconClock size={14} />} label="Начало" value={fmtDate(ev.starts_at)} />
                {ev.ends_at && (
                  <InfoRow icon={<IconClock size={14} />} label="Окончание" value={fmtDate(ev.ends_at)} />
                )}
                <InfoRow icon={<IconLocation size={14} />} label="Место" value={ev.location || "—"} />
                <InfoRow
                  icon={<IconUsers size={14} />}
                  label="Вместимость"
                  value={`${ev.capacity} чел.`}
                />
              </div>

              {/* Capacity bar */}
              {st && (
                <div className="mt-4 rounded-lg border border-border bg-muted/30 p-3">
                  <div className="mb-2 flex items-center justify-between text-sm">
                    <span className="font-medium text-text">Заполненность</span>
                    <span
                      className={`tabular-nums font-semibold ${
                        isFull ? "text-danger" : pct >= 80 ? "text-warn" : "text-accent"
                      }`}
                    >
                      {filled} / {ev.capacity}
                      {isFull && " — Мест нет"}
                    </span>
                  </div>
                  <Progress
                    value={pct}
                    size="md"
                    colorClass={isFull ? "bg-danger" : pct >= 80 ? "bg-warn" : "bg-accent"}
                    showLabel
                  />
                </div>
              )}

              {/* Tags */}
              {ev.tags && ev.tags.length > 0 && (
                <div className="mt-4 flex flex-wrap gap-1.5">
                  {ev.tags.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-1 rounded-full bg-muted/80 px-2.5 py-1 text-xs text-subtle"
                    >
                      <IconTag size={10} />
                      {tag}
                    </span>
                  ))}
                </div>
              )}

              {/* Description */}
              {ev.description && (
                <div className="mt-4 border-t border-border/60 pt-4">
                  <p className="whitespace-pre-line text-sm leading-relaxed text-text/90">
                    {ev.description}
                  </p>
                </div>
              )}
            </CardBody>
          </Card>

          {/* Stats card */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconActivity size={16} className="text-subtle" />
                Статистика участников
              </CardTitle>
            </CardHeader>
            <CardBody>
              {!st ? (
                <p className="py-4 text-center text-subtle">Данных пока нет</p>
              ) : (
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <StatBox label="Зарегистр." value={st.registered} color="blue" />
                  <StatBox label="Свободно" value={st.free_seats} color="subtle" />
                  <StatBox label="Лист ожид." value={st.waitlist} color="warn" />
                  <StatBox label="Отменили" value={st.cancelled} color="danger" />
                  <StatBox label="Пришли" value={st.attended} color="success" highlight />
                  <StatBox label="Не пришли" value={st.no_show} color="subtle" />
                  <StatBox label="Вместим." value={st.capacity} color="subtle" />
                </div>
              )}
              {st?.top_interests && Object.keys(st.top_interests).length > 0 && (
                <div className="mt-4">
                  <div className="mb-2 text-xs font-medium text-subtle">Топ интересов абитуриентов</div>
                  <div className="flex flex-wrap gap-1.5">
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
        </div>

        {/* Right: actions */}
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Управление</CardTitle>
            </CardHeader>
            <CardBody className="space-y-2">
              {/* Toggle registration — только владелец или admin */}
              {canEdit && ev.status === "open" && (
                <Button
                  variant="danger"
                  className="w-full justify-start gap-2"
                  disabled={busy}
                  onClick={() => toggleStatus("close")}
                >
                  <IconX size={15} />
                  Закрыть регистрацию
                </Button>
              )}
              {canEdit && ev.status !== "open" && ev.status !== "cancelled" && ev.status !== "completed" && (
                <Button
                  variant="primary"
                  className="w-full justify-start gap-2"
                  disabled={busy}
                  onClick={() => toggleStatus("open")}
                >
                  <IconCheck size={15} />
                  Открыть регистрацию
                </Button>
              )}

              {canEdit && <div className="h-px bg-border/60 my-1" />}

              {/* Участники и рассылка — только тем, кто управляет событием */}
              {canEdit && (
                <ActionLink
                  href={`/events/${id}/participants`}
                  icon={<IconList size={15} className="text-blue-400" />}
                  label="Участники"
                  desc="Список и ручная отметка"
                />
              )}
              {canEdit && (
                <ActionLink
                  href={`/events/${id}/broadcast`}
                  icon={<IconBroadcast size={15} className="text-accent" />}
                  label="Рассылка"
                  desc="Уведомить участников"
                />
              )}
              {canEdit && (
                <ActionLink
                  href={`/events/${id}/edit`}
                  icon={<IconEdit size={15} className="text-warn" />}
                  label="Редактировать"
                  desc="Изменить детали"
                />
              )}
              {canEdit && (
                <ActionLink
                  href={`/events/${id}/audit`}
                  icon={<IconActivity size={15} className="text-subtle" />}
                  label="Журнал действий"
                  desc="История изменений"
                />
              )}
              {/* Для не-владельцев — только просмотр (read-only) */}
              {!canEdit && (
                <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-xs text-subtle">
                  Это мероприятие принадлежит другому организатору. Управление недоступно.
                </div>
              )}
              {canCheckin(me.user.role) && (
                <ActionLink
                  href={`/checkin?event=${id}`}
                  icon={<IconQr size={15} className="text-emerald-400" />}
                  label="Check-in (камера)"
                  desc="Сканер QR-кодов гостей"
                />
              )}

              {canEdit && (
                <>
                  <div className="h-px bg-border/60 my-1" />
                  <a
                    href={`/api/events/${id}/participants.csv`}
                    download
                    className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5 no-underline transition-colors hover:bg-muted group"
                  >
                    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted">
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className="text-subtle">
                        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                        <polyline points="7 10 12 15 17 10" />
                        <line x1="12" y1="15" x2="12" y2="3" />
                      </svg>
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-xs font-medium text-text">Скачать CSV</div>
                      <div className="text-xs text-subtle">Список участников</div>
                    </div>
                  </a>
                </>
              )}
            </CardBody>
          </Card>

          {/* Delete event */}
          {canEdit && (
            <Card>
              <CardHeader>
                <CardTitle className="text-danger">Удаление</CardTitle>
              </CardHeader>
              <CardBody className="space-y-2">
                {!confirmDelete ? (
                  <Button
                    variant="danger"
                    className="w-full justify-start gap-2"
                    disabled={busy}
                    onClick={() => setConfirmDelete(true)}
                  >
                    <IconTrash size={15} />
                    Удалить мероприятие
                  </Button>
                ) : (
                  <div className="space-y-2">
                    <p className="text-xs text-danger">Удалить навсегда? Это действие необратимо.</p>
                    <div className="flex gap-2">
                      <Button
                        variant="danger"
                        className="flex-1 text-xs"
                        disabled={busy}
                        onClick={deleteEvent}
                      >
                        Да, удалить
                      </Button>
                      <Button
                        variant="secondary"
                        className="flex-1 text-xs"
                        disabled={busy}
                        onClick={() => setConfirmDelete(false)}
                      >
                        Отмена
                      </Button>
                    </div>
                  </div>
                )}
              </CardBody>
            </Card>
          )}

          {/* Event ID */}
          <div className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs text-subtle">
            ID мероприятия: <span className="font-mono font-medium text-text">{id}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function InfoRow({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 text-subtle">{icon}</span>
      <div>
        <div className="text-xs text-subtle">{label}</div>
        <div className="text-sm font-medium text-text">{value}</div>
      </div>
    </div>
  );
}

function StatBox({
  label,
  value,
  color,
  highlight,
}: {
  label: string;
  value: number;
  color: "blue" | "subtle" | "warn" | "danger" | "success" | "accent";
  highlight?: boolean;
}) {
  const colorMap = {
    blue: { text: "text-blue-400", bg: "bg-blue-500/8 border-blue-400/20" },
    subtle: { text: "text-subtle", bg: "bg-muted/40 border-border" },
    warn: { text: "text-warn", bg: "bg-warn/8 border-warn/20" },
    danger: { text: "text-danger", bg: "bg-danger/8 border-danger/20" },
    success: { text: "text-success", bg: "bg-success/8 border-success/20" },
    accent: { text: "text-accent", bg: "bg-accent/8 border-accent/20" },
  };
  const c = colorMap[color];
  return (
    <div
      className={`rounded-lg border px-3 py-2.5 ${c.bg} ${highlight ? "ring-1 ring-success/30" : ""}`}
    >
      <div className="text-xs text-subtle">{label}</div>
      <div className={`mt-0.5 text-xl font-bold tabular-nums ${c.text}`}>{value}</div>
    </div>
  );
}

function ActionLink({
  href,
  icon,
  label,
  desc,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  desc: string;
}) {
  return (
    <Link
      href={href}
      className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5 no-underline transition-colors hover:bg-muted group"
    >
      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-xs font-medium text-text group-hover:text-accent transition-colors">{label}</div>
        <div className="text-xs text-subtle">{desc}</div>
      </div>
    </Link>
  );
}
