"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { BroadcastResp } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";

const MAX_LEN = 4000;

export default function BroadcastPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ sent: number } | null>(null);
  const [err, setErr] = useState<string | null>(null);

  async function onSend() {
    const t = text.trim();
    if (!t) {
      setErr("Текст не может быть пустым");
      return;
    }
    if (t.length > MAX_LEN) {
      setErr(`Текст длиннее ${MAX_LEN} символов`);
      return;
    }
    if (!confirm("Отправить сообщение всем зарегистрированным участникам?")) return;
    setBusy(true);
    setErr(null);
    setResult(null);
    try {
      const res = await api.post<BroadcastResp>(`/api/events/${id}/broadcast`, { text: t });
      setResult({ sent: res.sent });
      setText("");
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось отправить",
      );
    } finally {
      setBusy(false);
    }
  }

  const remaining = MAX_LEN - text.length;

  return (
    <div className="space-y-6">
      <div>
        <Link href={`/events/${id}`} className="text-sm text-subtle no-underline hover:text-text">
          ← К мероприятию
        </Link>
        <h1 className="mt-1 text-2xl font-semibold">Рассылка</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Сообщение участникам</CardTitle>
        </CardHeader>
        <CardBody className="space-y-3">
          <Textarea
            placeholder="Например: «Напоминаем, что вход с 17:30. Возьмите студенческий.»"
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={8}
          />
          <div className="flex items-center justify-between text-xs">
            <span className={remaining < 0 ? "text-danger" : "text-subtle"}>
              Осталось: {remaining}
            </span>
            <span className="text-subtle">
              Лимит {MAX_LEN}. Markdown не поддерживается.
            </span>
          </div>
          {err && <p className="text-sm text-danger">{err}</p>}
          {result && (
            <p className="text-sm text-success">
              Доставлено: {result.sent}.
            </p>
          )}
          <div className="flex gap-2 pt-2">
            <Button onClick={onSend} disabled={busy || !text.trim()}>
              Отправить
            </Button>
            <Link href={`/events/${id}`}>
              <Button variant="secondary">Отмена</Button>
            </Link>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
