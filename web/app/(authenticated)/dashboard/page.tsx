"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, HttpError } from "@/lib/api";
import { DashboardResp, EventDTO, ListEventsResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardResp | null>(null);
  const [my, setMy] = useState<EventDTO[]>([]);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [s, evs] = await Promise.all([
          api.get<DashboardResp>("/api/dashboard"),
          api.get<ListEventsResp>("/api/events?status=mine"),
        ]);
        if (!cancelled) {
          setStats(s);
          setMy(evs.events || []);
        }
      } catch (e) {
        if (!cancelled) {
          setErr(
            e instanceof HttpError && e.body?.message
              ? e.body.message
              : "Не удалось загрузить данные",
          );
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Дашборд</h1>
      {err && <p className="text-danger">{err}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="Мои мероприятия" value={stats?.total_events ?? "—"} />
        <StatCard label="Зарегистрировано" value={stats?.total_registered ?? "—"} />
        <StatCard label="Предстоящие" value={stats?.upcoming_events ?? "—"} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Мои мероприятия</CardTitle>
        </CardHeader>
        <CardBody>
          {my.length === 0 ? (
            <p className="text-subtle">Пока нет мероприятий. Создайте их через бота (раздел /organizer).</p>
          ) : (
            <ul className="divide-y divide-border">
              {my.map((e) => (
                <li key={e.id} className="flex items-center justify-between py-3">
                  <div>
                    <Link
                      href={`/events/${e.id}`}
                      className="font-medium text-text no-underline hover:text-accent"
                    >
                      {e.title}
                    </Link>
                    <div className="mt-0.5 text-xs text-subtle">
                      {fmtDate(e.starts_at)} · {e.location}
                    </div>
                  </div>
                  <Badge className={statusBadge(e.status)}>{statusLabel(e.status)}</Badge>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: number | string }) {
  return (
    <Card>
      <div className="text-sm text-subtle">{label}</div>
      <div className="mt-1 text-3xl font-semibold">{value}</div>
    </Card>
  );
}
