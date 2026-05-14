"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { AuditLogResp, actionLabel } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { fmtDate } from "@/lib/format";

export default function AuditLogPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const [data, setData] = useState<AuditLogResp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) return;
    (async () => {
      setLoading(true);
      try {
        const d = await api.get<AuditLogResp>(
          `/api/events/${id}/actions?limit=200`,
        );
        setData(d);
      } catch (e) {
        setErr(
          e instanceof HttpError && e.body?.message
            ? e.body.message
            : "Не удалось загрузить журнал",
        );
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const items = data?.items ?? [];

  return (
    <div className="space-y-6">
      <div>
        <Link
          href={`/events/${id}`}
          className="text-sm text-subtle no-underline hover:text-text"
        >
          ← К мероприятию
        </Link>
        <h1 className="mt-1 text-2xl font-semibold">Журнал действий</h1>
        <p className="mt-1 text-sm text-subtle">
          Immutable audit log по этому мероприятию (последние 200 событий).
        </p>
      </div>

      {err && <p className="text-danger">{err}</p>}

      <Card>
        <CardHeader>
          <CardTitle>Всего записей: {items.length}</CardTitle>
        </CardHeader>
        <CardBody>
          {loading ? (
            <p className="text-subtle">Загрузка…</p>
          ) : items.length === 0 ? (
            <p className="text-subtle">Журнал пуст.</p>
          ) : (
            <ul className="divide-y divide-border">
              {items.map((row) => (
                <li key={row.id} className="py-3">
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between">
                    <div className="font-medium text-text">
                      {actionLabel(row.action)}
                    </div>
                    <div className="text-xs text-subtle">
                      {fmtDate(row.created_at)}
                    </div>
                  </div>
                  <div className="mt-1 text-xs text-subtle">
                    {row.actor_user_id ? (
                      <>
                        Актор: <span className="text-text">#{row.actor_user_id}</span>
                      </>
                    ) : (
                      <span>Система</span>
                    )}
                    {row.target_user_id ? (
                      <>
                        {" · "}Цель: <span className="text-text">#{row.target_user_id}</span>
                      </>
                    ) : null}
                    {row.registration_id ? (
                      <>
                        {" · "}reg #{row.registration_id}
                      </>
                    ) : null}
                  </div>
                  {row.payload && Object.keys(row.payload).length > 0 && (
                    <pre className="mt-2 max-w-full overflow-x-auto rounded-md bg-muted/40 p-2 text-xs text-text">
                      {JSON.stringify(row.payload, null, 2)}
                    </pre>
                  )}
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
