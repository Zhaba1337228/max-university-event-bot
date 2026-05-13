"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { ParticipantsResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { fmtDate } from "@/lib/format";

const PAGE_SIZE = 50;

export default function ParticipantsPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const [data, setData] = useState<ParticipantsResp | null>(null);
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

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
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось загрузить",
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (Number.isFinite(id) && id > 0) load(0, "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setOffset(0);
    load(0, q);
  }

  const total = data?.total ?? 0;
  const items = data?.items ?? [];
  const hasNext = offset + items.length < total;

  return (
    <div className="space-y-6">
      <div>
        <Link href={`/events/${id}`} className="text-sm text-subtle no-underline hover:text-text">
          ← К мероприятию
        </Link>
        <h1 className="mt-1 text-2xl font-semibold">Участники</h1>
      </div>

      {err && <p className="text-danger">{err}</p>}

      <Card>
        <CardHeader>
          <CardTitle>
            Всего: {total}
            {total > 0 && (
              <span className="ml-2 text-sm text-subtle">
                · показано {items.length}
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardBody>
          <form onSubmit={onSearch} className="mb-4 flex gap-2">
            <Input
              placeholder="Поиск по ФИО или контакту"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            <Button type="submit" variant="primary">
              Найти
            </Button>
          </form>

          {loading ? (
            <p className="text-subtle">Загрузка…</p>
          ) : items.length === 0 ? (
            <p className="text-subtle">Никого не найдено.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-border text-subtle">
                    <th className="py-2 pr-3 font-medium">ФИО</th>
                    <th className="py-2 pr-3 font-medium">Контакт</th>
                    <th className="py-2 pr-3 font-medium">Интерес</th>
                    <th className="py-2 pr-3 font-medium">Записан</th>
                    <th className="py-2 pr-3 font-medium">Check-in</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((r) => (
                    <tr key={r.id} className="border-b border-border/60 last:border-b-0">
                      <td className="py-2 pr-3">{r.full_name_masked}</td>
                      <td className="py-2 pr-3 font-mono text-xs">{r.contact_masked}</td>
                      <td className="py-2 pr-3">{r.interest_program || "—"}</td>
                      <td className="py-2 pr-3">{fmtDate(r.registered_at)}</td>
                      <td className="py-2 pr-3">{r.checkin_at ? fmtDate(r.checkin_at) : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="mt-4 flex items-center justify-between">
            <Button
              variant="secondary"
              disabled={offset === 0 || loading}
              onClick={() => {
                const o = Math.max(0, offset - PAGE_SIZE);
                setOffset(o);
                load(o, q);
              }}
            >
              ← Назад
            </Button>
            <span className="text-xs text-subtle">
              {offset + 1}–{offset + items.length} из {total}
            </span>
            <Button
              variant="secondary"
              disabled={!hasNext || loading}
              onClick={() => {
                const o = offset + PAGE_SIZE;
                setOffset(o);
                load(o, q);
              }}
            >
              Вперёд →
            </Button>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
