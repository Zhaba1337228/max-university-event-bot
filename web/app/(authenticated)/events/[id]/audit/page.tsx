"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { AuditLogEntry, AuditLogResp, actionLabel } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { fmtDate } from "@/lib/format";
import { IconArrowLeft, IconActivity, IconUsers, IconCalendar, IconBell, IconCheck } from "@/components/ui/icons";

// Action category for filtering + coloring
type ActionCat = "all" | "registration" | "mark" | "notification" | "event";

function categorize(action: string): ActionCat {
  if (action.includes("registration") || action.includes("waitlist") || action.includes("forget") || action.includes("consent")) return "registration";
  if (action.includes("attended") || action.includes("no_show") || action.includes("checkin")) return "mark";
  if (action.includes("notification") || action.includes("broadcast")) return "notification";
  if (action.startsWith("event_") || action.includes("admin_login") || action.includes("role_changed") || action.includes("csv")) return "event";
  return "registration";
}

function actionDotClass(action: string): string {
  const cat = categorize(action);
  switch (cat) {
    case "mark":         return "bg-accent border-accent/30";
    case "notification": return "bg-blue-400 border-blue-400/30";
    case "event":        return "bg-warn border-warn/30";
    default:
      if (action.includes("cancelled") || action.includes("cancel")) return "bg-danger border-danger/30";
      if (action.includes("created") || action.includes("promoted") || action.includes("checkin")) return "bg-success border-success/30";
      return "bg-subtle border-border";
  }
}

function actionLabelColor(action: string): string {
  if (action.includes("cancelled") || action.includes("cancel")) return "text-danger";
  if (action.includes("created") || action.includes("promoted") || action === "checkin_scanned") return "text-success";
  if (action.includes("attended") || action.includes("no_show")) return "text-accent";
  if (action.includes("notification") || action.includes("broadcast")) return "text-blue-400";
  if (action.startsWith("event_")) return "text-warn";
  return "text-text";
}

function ActionIcon({ action }: { action: string }) {
  const cat = categorize(action);
  const cls = "w-3.5 h-3.5";
  if (cat === "mark") return <IconCheck className={cls} />;
  if (cat === "notification") return <IconBell className={cls} />;
  if (cat === "event") return <IconCalendar className={cls} />;
  if (action.includes("cancelled")) return <span className={cls + " inline-block text-center leading-none"}>✕</span>;
  return <IconUsers className={cls} />;
}

const CATEGORY_FILTERS: { value: ActionCat; label: string }[] = [
  { value: "all", label: "Все" },
  { value: "registration", label: "Регистрации" },
  { value: "mark", label: "Отметки" },
  { value: "notification", label: "Уведомления" },
  { value: "event", label: "Событие" },
];

export default function AuditLogPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const [data, setData] = useState<AuditLogResp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [catFilter, setCatFilter] = useState<ActionCat>("all");
  const [expandedPayloads, setExpandedPayloads] = useState<Set<number>>(new Set());

  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) return;
    (async () => {
      setLoading(true);
      try {
        const d = await api.get<AuditLogResp>(`/api/events/${id}/actions?limit=200`);
        setData(d);
      } catch (e) {
        setErr(e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось загрузить журнал");
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const allItems = data?.items ?? [];
  const items: AuditLogEntry[] = catFilter === "all"
    ? allItems
    : allItems.filter((r) => categorize(r.action) === catFilter);

  function togglePayload(id: number) {
    setExpandedPayloads((s) => {
      const n = new Set(s);
      n.has(id) ? n.delete(id) : n.add(id);
      return n;
    });
  }

  return (
    <div className="space-y-6 page-enter">
      <div>
        <Link
          href={`/events/${id}`}
          className="inline-flex items-center gap-1.5 text-sm text-subtle no-underline hover:text-text transition-colors"
        >
          <IconArrowLeft size={14} />
          К мероприятию
        </Link>
        <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Журнал действий</h1>
            <p className="mt-0.5 text-sm text-subtle">
              Неизменяемый audit log · последние 200 событий
            </p>
          </div>
          <div className="rounded-lg border border-border bg-muted/60 px-3 py-1.5">
            <span className="text-xs text-subtle">Записей: </span>
            <span className="text-sm font-semibold text-text">{allItems.length}</span>
          </div>
        </div>
      </div>

      {err && <p className="text-sm text-danger">{err}</p>}

      {/* Category filter */}
      <div className="flex flex-wrap gap-1.5">
        {CATEGORY_FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            onClick={() => setCatFilter(f.value)}
            className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors ${
              catFilter === f.value
                ? "bg-accent text-white"
                : "bg-muted text-subtle hover:text-text"
            }`}
          >
            {f.label}
            {f.value !== "all" && (
              <span className="ml-1.5 opacity-60">
                {allItems.filter((r) => categorize(r.action) === f.value).length}
              </span>
            )}
          </button>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <IconActivity size={16} className="text-subtle" />
            {catFilter === "all" ? "Все действия" : CATEGORY_FILTERS.find((f) => f.value === catFilter)?.label}
          </CardTitle>
        </CardHeader>
        <CardBody>
          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="flex gap-3">
                  <div className="mt-1 h-3 w-3 shrink-0 rounded-full bg-muted animate-pulse" />
                  <div className="flex-1 space-y-1.5">
                    <div className="h-4 w-48 rounded bg-muted animate-pulse" />
                    <div className="h-3 w-32 rounded bg-muted/70 animate-pulse" />
                  </div>
                </div>
              ))}
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <IconActivity size={28} className="text-subtle" />
              <p className="text-subtle">Записей нет</p>
            </div>
          ) : (
            // Timeline
            <ol className="relative">
              {/* Vertical line */}
              <div className="absolute left-[5px] top-2 bottom-2 w-px bg-border/60" />

              {items.map((row, idx) => {
                const hasPayload = row.payload && Object.keys(row.payload).length > 0;
                const expanded = expandedPayloads.has(row.id);
                return (
                  <li key={row.id} className={`relative pl-7 ${idx < items.length - 1 ? "pb-5" : ""}`}>
                    {/* Dot */}
                    <span
                      className={`absolute left-0 top-[3px] flex h-3 w-3 items-center justify-center rounded-full border ${actionDotClass(row.action)}`}
                    />

                    {/* Content */}
                    <div className="group">
                      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
                        <span className={`text-sm font-semibold ${actionLabelColor(row.action)}`}>
                          {actionLabel(row.action)}
                        </span>
                        <span className="text-xs text-subtle tabular-nums">{fmtDate(row.created_at)}</span>
                      </div>

                      <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-subtle">
                        {row.actor_user_id ? (
                          <span>Актор: <span className="font-mono text-text/70">#{row.actor_user_id}</span></span>
                        ) : (
                          <span className="italic">Система</span>
                        )}
                        {row.target_user_id && (
                          <span>→ <span className="font-mono text-text/70">#{row.target_user_id}</span></span>
                        )}
                        {row.registration_id && (
                          <span className="rounded bg-muted/80 px-1.5 py-0.5 font-mono">
                            reg#{row.registration_id}
                          </span>
                        )}
                      </div>

                      {hasPayload && (
                        <div className="mt-1.5">
                          <button
                            type="button"
                            onClick={() => togglePayload(row.id)}
                            className="text-xs text-accent/70 hover:text-accent transition-colors"
                          >
                            {expanded ? "▾ скрыть данные" : "▸ показать данные"}
                          </button>
                          {expanded && (
                            <pre className="mt-1.5 max-w-full overflow-x-auto rounded-lg border border-border bg-muted/40 p-2.5 text-xs text-text/80">
                              {JSON.stringify(row.payload, null, 2)}
                            </pre>
                          )}
                        </div>
                      )}
                    </div>
                  </li>
                );
              })}
            </ol>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
