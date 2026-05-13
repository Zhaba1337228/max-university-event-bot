"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, HttpError } from "@/lib/api";
import { EventDTO, ListEventsResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";

type Tab = "mine" | "open";

export default function EventsPage() {
  const [tab, setTab] = useState<Tab>("mine");
  const [items, setItems] = useState<EventDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErr(null);
    (async () => {
      try {
        const data = await api.get<ListEventsResp>(
          tab === "mine" ? "/api/events?status=mine" : "/api/events?limit=50",
        );
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
    return () => {
      cancelled = true;
    };
  }, [tab]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Мероприятия</h1>
        <div className="flex gap-2">
          <Button
            variant={tab === "mine" ? "primary" : "secondary"}
            onClick={() => setTab("mine")}
          >
            Мои
          </Button>
          <Button
            variant={tab === "open" ? "primary" : "secondary"}
            onClick={() => setTab("open")}
          >
            Все открытые
          </Button>
        </div>
      </div>

      {err && <p className="text-danger">{err}</p>}

      <Card>
        <CardHeader>
          <CardTitle>{tab === "mine" ? "Мои мероприятия" : "Открытые мероприятия"}</CardTitle>
        </CardHeader>
        <CardBody>
          {loading ? (
            <p className="text-subtle">Загрузка…</p>
          ) : items.length === 0 ? (
            <p className="text-subtle">Ничего не найдено.</p>
          ) : (
            <ul className="divide-y divide-border">
              {items.map((e) => (
                <li key={e.id} className="flex items-center justify-between gap-3 py-3">
                  <div className="min-w-0 flex-1">
                    <Link
                      href={`/events/${e.id}`}
                      className="block truncate font-medium text-text no-underline hover:text-accent"
                    >
                      {e.title}
                    </Link>
                    <div className="mt-0.5 truncate text-xs text-subtle">
                      {fmtDate(e.starts_at)} · {e.location || "—"} · {e.format}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-3">
                    {typeof e.free_seats === "number" && (
                      <span className="text-xs text-subtle">
                        {e.free_seats}/{e.capacity}
                      </span>
                    )}
                    <Badge className={statusBadge(e.status)}>{statusLabel(e.status)}</Badge>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
