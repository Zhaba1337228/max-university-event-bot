"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import {
  ParticipantsResp,
  Registration,
  EventDetail,
  canEditEvent,
  canUnmaskPII,
} from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Toast } from "@/components/ui/toast";
import { fmtDate } from "@/lib/format";
import { useMe } from "@/components/me-context";
import {
  IconArrowLeft,
  IconUsers,
  IconSearch,
  IconCheck,
  IconX,
  IconEye,
  IconArrowRight,
  IconLock,
  IconActivity,
} from "@/components/ui/icons";

const PAGE_SIZE = 50;

// Status badge colours
function regStatusBadge(status: string): string {
  switch (status) {
    case "registered":
      return "bg-success/15 text-success border border-success/30";
    case "attended":
      return "bg-accent/15 text-accent border border-accent/30";
    case "waitlist":
      return "bg-warn/15 text-warn border border-warn/30";
    case "cancelled":
    case "late_cancel":
      return "bg-danger/15 text-danger border border-danger/30";
    case "no_show":
      return "bg-muted text-subtle border border-border";
    default:
      return "bg-muted text-subtle border border-border";
  }
}

function regStatusLabel(status: string): string {
  switch (status) {
    case "registered":   return "Зарегистрирован";
    case "attended":     return "Пришёл";
    case "waitlist":     return "Лист ожид.";
    case "cancelled":    return "Отменил";
    case "late_cancel":  return "Поздняя отмена";
    case "no_show":      return "Не пришёл";
    default:             return status;
  }
}

// Left border colour for mobile cards
function regBorderClass(status: string): string {
  switch (status) {
    case "registered":                  return "border-l-success/60";
    case "attended":                    return "border-l-accent/60";
    case "waitlist":                    return "border-l-warn/60";
    case "cancelled": case "late_cancel": return "border-l-danger/60";
    default:                            return "border-l-border";
  }
}

// Access-denied placeholder
function AccessDenied({ id }: { id: number }) {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center page-enter">
      <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-border bg-surface shadow-elevated">
        <IconLock size={28} className="text-danger" />
      </div>
      <div>
        <h2 className="text-xl font-bold text-text">Доступ запрещён</h2>
        <p className="mt-1 text-sm text-subtle">
          Управление участниками доступно только создателю мероприятия или администратору.
        </p>
      </div>
      <Link href={`/events/${id}`}>
        <Button variant="secondary" className="gap-1.5">
          <IconArrowLeft size={14} />
          К мероприятию
        </Button>
      </Link>
    </div>
  );
}

export default function ParticipantsPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const me = useMe();

  // Event meta (needed for ownership check + title)
  const [eventDetail, setEventDetail] = useState<EventDetail | null>(null);
  const [eventErr, setEventErr] = useState<number | null>(null); // HTTP status

  // Participants
  const [data, setData] = useState<ParticipantsResp | null>(null);
  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  // Actions
  const [marking, setMarking] = useState<Record<number, boolean>>({});
  const [unmasking, setUnmasking] = useState<Record<number, boolean>>({});
  const [unmasked, setUnmasked] = useState<Record<number, { full_name: string; contact: string }>>({});

  // Load event first (ownership check)
  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) return;
    api.get<EventDetail>(`/api/events/${id}`)
      .then((d) => setEventDetail(d))
      .catch((e) => {
        if (e instanceof HttpError) setEventErr(e.status);
        else setEventErr(500);
      });
  }, [id]);

  // Derived: can this user manage this event?
  const canEdit =
    eventDetail != null &&
    canEditEvent(me.user.role, me.user.id, eventDetail.event.created_by);

  // Load participants
  async function load(o = offset, query = q) {
    setErr(null);
    setLoading(true);
    try {
      const usp = new URLSearchParams();
      usp.set("limit", String(PAGE_SIZE));
      usp.set("offset", String(o));
      if (query.trim()) usp.set("q", query.trim());
      const d = await api.get<ParticipantsResp>(
        `/api/events/${id}/participants?${usp.toString()}`,
      );
      setData(d);
    } catch (e) {
      if (e instanceof HttpError && e.status === 403) {
        setEventErr(403);
      } else {
        setErr(
          e instanceof HttpError && e.body?.message
            ? e.body.message
            : "Не удалось загрузить",
        );
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (eventDetail && canEdit) load(0, "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [eventDetail, canEdit]);

  async function mark(regID: number, status: "attended" | "no_show") {
    setMarking((m) => ({ ...m, [regID]: true }));
    try {
      const resp = await api.post<{ registration: Registration }>(
        `/api/events/${id}/registrations/${regID}/mark`,
        { status },
      );
      setData((prev) =>
        prev
          ? { ...prev, items: prev.items.map((r) => r.id === regID ? { ...r, ...resp.registration } : r) }
          : prev,
      );
    } catch (e) {
      setErr(e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось отметить");
    } finally {
      setMarking((m) => { const n = { ...m }; delete n[regID]; return n; });
    }
  }

  async function unmask(regID: number) {
    if (unmasked[regID]) return;
    setUnmasking((m) => ({ ...m, [regID]: true }));
    try {
      const resp = await api.post<{ full_name: string; contact: string }>(
        `/api/events/${id}/registrations/${regID}/unmask`,
      );
      setUnmasked((m) => ({ ...m, [regID]: resp }));
    } catch (e) {
      setErr(e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось раскрыть ПДн");
    } finally {
      setUnmasking((m) => { const n = { ...m }; delete n[regID]; return n; });
    }
  }

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setOffset(0);
    load(0, q);
  }

  // Show access denied if event loaded and user has no rights
  if (eventErr === 403 || (eventDetail && !canEdit && me.user.role !== "admin")) {
    return <AccessDenied id={id} />;
  }

  const total = data?.total ?? 0;
  const items = data?.items ?? [];
  const hasNext = offset + items.length < total;
  const canPII = canUnmaskPII(me.user.role);

  // Status filter pills
  const STATUS_FILTERS = [
    { value: "", label: "Все" },
    { value: "registered", label: "Записан" },
    { value: "attended", label: "Пришёл" },
    { value: "waitlist", label: "Ожидание" },
    { value: "cancelled", label: "Отменил" },
    { value: "no_show", label: "Не пришёл" },
  ];

  const filteredItems = statusFilter
    ? items.filter((r) => r.status === statusFilter)
    : items;

  const eventTitle = eventDetail?.event.title;

  return (
    <div className="space-y-6 page-enter">
      {/* Header */}
      <div>
        <Link
          href={`/events/${id}`}
          className="inline-flex items-center gap-1.5 text-sm text-subtle no-underline hover:text-text transition-colors"
        >
          <IconArrowLeft size={14} />
          {eventTitle ? eventTitle : "К мероприятию"}
        </Link>
        <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Участники</h1>
            <p className="mt-0.5 text-sm text-subtle">
              Всего: <span className="font-medium text-text">{total}</span>
              {loading && " · Загрузка…"}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={`/events/${id}/audit`}>
              <Button variant="secondary" className="gap-1.5">
                <IconActivity size={14} />
                Журнал
              </Button>
            </Link>
            {canEdit && (
              <a href={`/api/events/${id}/participants.csv`} download>
                <Button variant="primary" className="gap-1.5">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" />
                  </svg>
                  CSV
                </Button>
              </a>
            )}
          </div>
        </div>
      </div>

      {err && <Toast message={err} kind="error" onDismiss={() => setErr(null)} />}

      {/* Filters */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <form onSubmit={onSearch} className="flex flex-1 gap-2">
          <div className="relative flex-1">
            <IconSearch size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-subtle" />
            <Input
              placeholder="Поиск по ФИО или контакту"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              className="pl-8"
            />
          </div>
          <Button type="submit" variant="secondary" className="shrink-0">
            Найти
          </Button>
        </form>
        {/* Status filter pills */}
        <div className="flex flex-wrap gap-1">
          {STATUS_FILTERS.map((f) => (
            <button
              key={f.value}
              type="button"
              onClick={() => setStatusFilter(f.value)}
              className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                statusFilter === f.value
                  ? "bg-accent text-white"
                  : "bg-muted text-subtle hover:text-text"
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <IconUsers size={16} className="text-subtle" />
            {statusFilter
              ? `${filteredItems.length} из ${total}`
              : `${total} участников`}
          </CardTitle>
          {canPII && (
            <span className="rounded-full bg-accent/15 px-2.5 py-0.5 text-xs font-medium text-accent">
              Режим администратора · ПДн доступны
            </span>
          )}
        </CardHeader>
        <CardBody>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="h-14 rounded-lg bg-muted/50 animate-pulse" />
              ))}
            </div>
          ) : filteredItems.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <IconUsers size={28} className="text-subtle" />
              <p className="text-subtle">Участников не найдено</p>
            </div>
          ) : (
            <>
              {/* Mobile cards */}
              <ul className="space-y-2 md:hidden">
                {filteredItems.map((r) => {
                  const u = unmasked[r.id];
                  return (
                    <li
                      key={r.id}
                      className={`rounded-lg border-l-2 border border-border bg-muted/30 p-3 ${regBorderClass(r.status)}`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0 flex-1">
                          <div className="font-medium text-text">
                            {u ? u.full_name : r.full_name_masked}
                          </div>
                          <div className="mt-0.5 font-mono text-xs text-subtle">
                            {u ? u.contact : r.contact_masked}
                          </div>
                        </div>
                        <Badge className={regStatusBadge(r.status) + " shrink-0 text-xs"}>
                          {regStatusLabel(r.status)}
                        </Badge>
                      </div>
                      {r.interest_program && (
                        <div className="mt-1 text-xs text-subtle">
                          Интерес: <span className="text-text">{r.interest_program}</span>
                        </div>
                      )}
                      <div className="mt-1.5 text-xs text-subtle">
                        Записан: {fmtDate(r.registered_at)}
                        {r.checkin_at && (
                          <span className="ml-2 text-success">· пришёл {fmtDate(r.checkin_at)}</span>
                        )}
                      </div>
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {canEdit && (
                          <>
                            <Button
                              type="button"
                              variant="primary"
                              size="md"
                              disabled={!!marking[r.id] || r.status === "attended"}
                              onClick={() => mark(r.id, "attended")}
                              className="gap-1"
                            >
                              <IconCheck size={13} />
                              Пришёл
                            </Button>
                            <Button
                              type="button"
                              variant="secondary"
                              size="md"
                              disabled={!!marking[r.id] || r.status === "no_show" || r.status.startsWith("cancelled")}
                              onClick={() => mark(r.id, "no_show")}
                              className="gap-1"
                            >
                              <IconX size={13} />
                              Не пришёл
                            </Button>
                          </>
                        )}
                        {canPII && !u && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="md"
                            disabled={!!unmasking[r.id]}
                            onClick={() => unmask(r.id)}
                            className="gap-1 border border-accent/30 text-accent"
                          >
                            <IconEye size={13} />
                            ПДн
                          </Button>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ul>

              {/* Desktop table */}
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-border">
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">ФИО</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Контакт</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Статус</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Интерес</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Записан</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Check-in</th>
                      {(canEdit || canPII) && (
                        <th className="pb-2 text-xs font-medium uppercase tracking-wide text-subtle">Действия</th>
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {filteredItems.map((r) => {
                      const u = unmasked[r.id];
                      return (
                        <tr
                          key={r.id}
                          className="border-b border-border/40 last:border-b-0 hover:bg-muted/20 transition-colors"
                        >
                          <td className="py-2.5 pr-3 font-medium text-text">
                            {u ? u.full_name : r.full_name_masked}
                            {u && (
                              <span className="ml-1.5 rounded bg-accent/15 px-1 py-0.5 text-xs text-accent">ПДн</span>
                            )}
                          </td>
                          <td className="py-2.5 pr-3 font-mono text-xs text-subtle">
                            {u ? u.contact : r.contact_masked}
                          </td>
                          <td className="py-2.5 pr-3">
                            <Badge className={regStatusBadge(r.status) + " text-xs"}>
                              {regStatusLabel(r.status)}
                            </Badge>
                          </td>
                          <td className="py-2.5 pr-3 text-xs text-subtle">
                            {r.interest_program || "—"}
                          </td>
                          <td className="py-2.5 pr-3 text-xs text-subtle">{fmtDate(r.registered_at)}</td>
                          <td className={`py-2.5 pr-3 text-xs ${r.checkin_at ? "text-success" : "text-subtle"}`}>
                            {r.checkin_at ? fmtDate(r.checkin_at) : "—"}
                          </td>
                          {(canEdit || canPII) && (
                            <td className="py-2.5">
                              <div className="flex gap-1.5">
                                {canEdit && (
                                  <>
                                    <Button
                                      type="button"
                                      variant="primary"
                                      size="md"
                                      disabled={!!marking[r.id] || r.status === "attended"}
                                      onClick={() => mark(r.id, "attended")}
                                      className="gap-1 px-2"
                                    >
                                      <IconCheck size={12} />
                                      Пришёл
                                    </Button>
                                    <Button
                                      type="button"
                                      variant="secondary"
                                      size="md"
                                      disabled={!!marking[r.id] || r.status === "no_show" || r.status.startsWith("cancelled")}
                                      onClick={() => mark(r.id, "no_show")}
                                      className="gap-1 px-2"
                                    >
                                      <IconX size={12} />
                                      Нет
                                    </Button>
                                  </>
                                )}
                                {canPII && !u && (
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="md"
                                    disabled={!!unmasking[r.id]}
                                    onClick={() => unmask(r.id)}
                                    className="gap-1 border border-accent/30 px-2 text-accent"
                                  >
                                    <IconEye size={12} />
                                    ПДн
                                  </Button>
                                )}
                              </div>
                            </td>
                          )}
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {/* Pagination */}
          <div className="mt-5 flex items-center justify-between border-t border-border/60 pt-4">
            <Button
              variant="secondary"
              disabled={offset === 0 || loading}
              onClick={() => {
                const o = Math.max(0, offset - PAGE_SIZE);
                setOffset(o);
                load(o, q);
              }}
              className="gap-1.5"
            >
              <IconArrowLeft size={14} />
              Назад
            </Button>
            <span className="text-xs text-subtle">
              {items.length === 0 ? "0" : `${offset + 1}–${offset + items.length}`} из {total}
            </span>
            <Button
              variant="secondary"
              disabled={!hasNext || loading}
              onClick={() => {
                const o = offset + PAGE_SIZE;
                setOffset(o);
                load(o, q);
              }}
              className="gap-1.5"
            >
              Вперёд
              <IconArrowRight size={14} />
            </Button>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
