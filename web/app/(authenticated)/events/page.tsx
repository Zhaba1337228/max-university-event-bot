"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, HttpError } from "@/lib/api";
import { EventDTO, EventStatus, ListEventsResp, canManageEvents } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Toast } from "@/components/ui/toast";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";
import { useMe } from "@/components/me-context";
import {
  IconCalendar,
  IconLocation,
  IconUsers,
  IconPlus,
  IconSearch,
  IconChevronRight,
  IconClock,
  IconTag,
} from "@/components/ui/icons";

type Tab = "mine" | "open";

// Format label
const formatLabel: Record<string, string> = {
  offline: "Офлайн",
  online: "Онлайн",
  hybrid: "Гибрид",
};

// Format badge color
const formatColor: Record<string, string> = {
  offline: "bg-blue-500/15 text-blue-400 border border-blue-400/25",
  online: "bg-emerald-500/15 text-emerald-400 border border-emerald-400/25",
  hybrid: "bg-warn/15 text-warn border border-warn/25",
};

function EventCard({ e }: { e: EventDTO }) {
  const filled = e.capacity - (e.free_seats ?? 0);
  const pct = e.capacity > 0 ? Math.round((filled / e.capacity) * 100) : 0;
  const isFull = pct >= 100;
  const isNearFull = pct >= 80;

  return (
    <Link href={`/events/${e.id}`} className="group block no-underline">
      <div className="flex h-full flex-col rounded-xl border border-border bg-surface shadow-card transition-all duration-200 hover:border-accent/40 hover:shadow-elevated hover:-translate-y-0.5">
        {/* Card top — status accent strip */}
        <div
          className={`h-1 w-full rounded-t-xl ${
            e.status === "open"
              ? "bg-gradient-to-r from-success/70 to-success/30"
              : e.status === "completed"
                ? "bg-gradient-to-r from-accent/70 to-accent/30"
                : e.status === "cancelled"
                  ? "bg-gradient-to-r from-danger/70 to-danger/30"
                  : "bg-gradient-to-r from-muted to-border"
          }`}
        />

        <div className="flex flex-1 flex-col gap-3 p-4">
          {/* Title + status */}
          <div className="flex items-start justify-between gap-2">
            <h3 className="flex-1 text-sm font-semibold leading-snug text-text group-hover:text-accent transition-colors line-clamp-2">
              {e.title}
            </h3>
            <Badge className={statusBadge(e.status) + " shrink-0 text-xs"}>
              {statusLabel(e.status)}
            </Badge>
          </div>

          {/* Meta info */}
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 text-xs text-subtle">
              <IconClock size={12} className="shrink-0" />
              <span className="truncate">{fmtDate(e.starts_at)}</span>
            </div>
            {e.location && (
              <div className="flex items-center gap-1.5 text-xs text-subtle">
                <IconLocation size={12} className="shrink-0" />
                <span className="truncate">{e.location}</span>
              </div>
            )}
          </div>

          {/* Tags */}
          {e.tags && e.tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {e.tags.slice(0, 3).map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center gap-0.5 rounded-full bg-muted/80 px-2 py-0.5 text-xs text-subtle"
                >
                  <IconTag size={9} />
                  {tag}
                </span>
              ))}
              {e.tags.length > 3 && (
                <span className="text-xs text-subtle">+{e.tags.length - 3}</span>
              )}
            </div>
          )}

          <div className="mt-auto pt-2">
            {/* Format badge */}
            {e.format && (
              <Badge className={`${formatColor[e.format] ?? "bg-muted text-subtle"} mb-2`}>
                {formatLabel[e.format] ?? e.format}
              </Badge>
            )}

            {/* Capacity progress */}
            {typeof e.free_seats === "number" && (
              <div>
                <div className="mb-1 flex items-center justify-between text-xs">
                  <span className="flex items-center gap-1 text-subtle">
                    <IconUsers size={11} />
                    Заполненность
                  </span>
                  <span
                    className={`tabular-nums font-medium ${
                      isFull ? "text-danger" : isNearFull ? "text-warn" : "text-subtle"
                    }`}
                  >
                    {filled}/{e.capacity}
                    {isFull && " · Нет мест"}
                  </span>
                </div>
                <Progress
                  value={pct}
                  colorClass={isFull ? "bg-danger" : isNearFull ? "bg-warn" : "bg-accent"}
                />
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between border-t border-border/60 px-4 py-2.5">
          <span className="text-xs text-subtle">Открыть →</span>
          <IconChevronRight size={14} className="text-border group-hover:text-subtle transition-colors" />
        </div>
      </div>
    </Link>
  );
}

const STATUS_FILTERS: { value: "" | EventStatus; label: string }[] = [
  { value: "", label: "Все" },
  { value: "open", label: "Открытые" },
  { value: "closed", label: "Закрытые" },
  { value: "draft", label: "Черновики" },
  { value: "completed", label: "Завершённые" },
  { value: "cancelled", label: "Отменённые" },
];

export default function EventsPage() {
  const me = useMe();
  const canCreate = canManageEvents(me.user.role);
  const [tab, setTab] = useState<Tab>("mine");
  const [statusFilter, setStatusFilter] = useState<"" | EventStatus>("");
  const [search, setSearch] = useState("");
  const [items, setItems] = useState<EventDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErr(null);
    (async () => {
      try {
        const usp = new URLSearchParams();
        usp.set("limit", "60");
        if (tab === "mine") usp.set("status", "mine");
        else if (statusFilter) usp.set("status", statusFilter);
        const data = await api.get<ListEventsResp>(`/api/events?${usp.toString()}`);
        if (!cancelled) {
          setItems(data.events || []);
          setLoading(false);
        }
      } catch (e) {
        if (!cancelled) {
          setErr(
            e instanceof HttpError && e.body?.message
              ? e.body.message
              : "Не удалось загрузить",
          );
          setLoading(false);
        }
      }
    })();
    return () => { cancelled = true; };
  }, [tab, statusFilter]);

  // Client-side search filter
  const filtered = search.trim()
    ? items.filter(
        (e) =>
          e.title.toLowerCase().includes(search.toLowerCase()) ||
          (e.location ?? "").toLowerCase().includes(search.toLowerCase()),
      )
    : items;

  return (
    <div className="space-y-6 page-enter">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold sm:text-3xl">Мероприятия</h1>
          <p className="mt-0.5 text-sm text-subtle">
            {filtered.length > 0 ? `${filtered.length} шт.` : "Список пуст"}
          </p>
        </div>
        {canCreate && (
          <Link href="/events/new">
            <Button className="gap-1.5">
              <IconPlus size={15} />
              Создать
            </Button>
          </Link>
        )}
      </div>

      {err && <Toast message={err} kind="error" onDismiss={() => setErr(null)} />}

      {/* Filters toolbar */}
      <Card className="p-3 sm:p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          {/* Tab switch */}
          <div className="inline-flex rounded-lg border border-border bg-muted/50 p-0.5">
            {(["mine", "open"] as Tab[]).map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => { setTab(t); setStatusFilter(""); }}
                className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                  tab === t
                    ? "bg-accent text-white shadow-card"
                    : "text-subtle hover:text-text"
                }`}
              >
                {t === "mine" ? "Мои" : "Все"}
              </button>
            ))}
          </div>

          {/* Search */}
          <div className="relative flex-1">
            <IconSearch size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-subtle" />
            <input
              type="search"
              placeholder="Поиск по названию или месту..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-md border border-border bg-muted/70 py-2 pl-8 pr-3 text-sm text-text placeholder:text-subtle/60 focus:border-accent/50 focus:outline-none focus:ring-1 focus:ring-accent/30"
            />
          </div>

          {/* Status filter (only for "all" tab) */}
          {tab === "open" && (
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
          )}
        </div>
      </Card>

      {/* Grid */}
      {loading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <div key={i} className="h-56 rounded-xl border border-border bg-surface animate-pulse" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-xl border border-dashed border-border py-16 text-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted/60">
            <IconCalendar size={28} className="text-subtle" />
          </div>
          <div>
            <p className="font-semibold text-text">Ничего не найдено</p>
            <p className="mt-1 text-sm text-subtle">
              {search ? "Попробуйте другой запрос" : "Мероприятий пока нет"}
            </p>
          </div>
          {canCreate && !search && (
            <Link href="/events/new">
              <Button variant="secondary" className="gap-1.5">
                <IconPlus size={14} />
                Создать первое
              </Button>
            </Link>
          )}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((e) => (
            <EventCard key={e.id} e={e} />
          ))}
        </div>
      )}
    </div>
  );
}
