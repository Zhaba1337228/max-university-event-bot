"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { BroadcastResp, EventDetail, canEditEvent, roleLabel } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Toast } from "@/components/ui/toast";
import { useMe } from "@/components/me-context";
import {
  IconArrowLeft,
  IconBroadcast,
  IconAlertCircle,
  IconLock,
  IconUsers,
  IconCheckCircle,
} from "@/components/ui/icons";

const MAX_LEN = 4000;

function AccessDenied({ id }: { id: number }) {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center page-enter">
      <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-border bg-surface shadow-elevated">
        <IconLock size={28} className="text-danger" />
      </div>
      <div>
        <h2 className="text-xl font-bold text-text">Доступ запрещён</h2>
        <p className="mt-1 text-sm text-subtle max-w-sm mx-auto">
          Рассылка доступна только создателю мероприятия или администратору.
          Как участник или волонтёр вы не можете отправлять сообщения.
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

export default function BroadcastPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);
  const me = useMe();

  const [eventDetail, setEventDetail] = useState<EventDetail | null>(null);
  const [eventErr, setEventErr] = useState<number | null>(null);

  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState<number | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) return;
    api.get<EventDetail>(`/api/events/${id}`)
      .then((d) => setEventDetail(d))
      .catch((e) => {
        if (e instanceof HttpError) setEventErr(e.status);
        else setEventErr(500);
      });
  }, [id]);

  const canEdit =
    eventDetail != null &&
    canEditEvent(me.user.role, me.user.id, eventDetail.event.created_by);

  async function onSend() {
    const t = text.trim();
    if (!t) { setErr("Текст не может быть пустым"); return; }
    if (t.length > MAX_LEN) { setErr(`Текст длиннее ${MAX_LEN} символов`); return; }
    const regCount = eventDetail?.stats?.registered ?? 0;
    if (!window.confirm(
      `Отправить сообщение ${regCount > 0 ? `${regCount} ` : ""}зарегистрированным участникам?`
    )) return;
    setBusy(true);
    setErr(null);
    setSent(null);
    try {
      const res = await api.post<BroadcastResp>(`/api/events/${id}/broadcast`, { text: t });
      setSent(res.sent);
      setText("");
    } catch (e) {
      setErr(e instanceof HttpError && e.body?.message ? e.body.message : "Не удалось отправить");
    } finally {
      setBusy(false);
    }
  }

  // Access denied
  if (eventErr === 403 || (eventDetail && !canEdit)) {
    return <AccessDenied id={id} />;
  }

  const remaining = MAX_LEN - text.length;
  const ev = eventDetail?.event;
  const regCount = eventDetail?.stats?.registered ?? null;

  return (
    <div className="space-y-6 page-enter">
      {/* Breadcrumb */}
      <div>
        <Link
          href={`/events/${id}`}
          className="inline-flex items-center gap-1.5 text-sm text-subtle no-underline hover:text-text transition-colors"
        >
          <IconArrowLeft size={14} />
          {ev?.title ?? "К мероприятию"}
        </Link>
        <h1 className="mt-2 text-2xl font-bold">Рассылка</h1>
        <p className="mt-0.5 text-sm text-subtle">
          Отправить сообщение всем зарегистрированным участникам
        </p>
      </div>

      {/* Sender role badge */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-subtle">Отправитель:</span>
        <Badge className={
          me.user.role === "admin"
            ? "bg-accent/15 text-accent border border-accent/25"
            : "bg-blue-500/15 text-blue-400 border border-blue-400/25"
        }>
          {roleLabel(me.user.role)}
        </Badge>
      </div>

      {/* Warning banner */}
      {regCount !== null && (
        <div className="flex items-start gap-3 rounded-xl border border-warn/30 bg-warn/8 p-4">
          <IconAlertCircle size={20} className="mt-0.5 shrink-0 text-warn" />
          <div className="text-sm">
            <span className="font-medium text-text">Внимание</span>
            <p className="mt-0.5 text-subtle">
              Это сообщение получат{" "}
              <span className="font-semibold text-text">{regCount}</span>{" "}
              зарегистрированных участников{ev ? ` мероприятия «${ev.title}»` : ""}.
              Рассылка необратима.
            </p>
          </div>
        </div>
      )}

      {/* Form */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <IconBroadcast size={16} className="text-subtle" />
            Сообщение участникам
          </CardTitle>
          {regCount !== null && (
            <div className="flex items-center gap-1.5 text-xs text-subtle">
              <IconUsers size={12} />
              {regCount} получателей
            </div>
          )}
        </CardHeader>
        <CardBody className="space-y-3">
          <Textarea
            placeholder="Например: «Напоминаем, что вход с 17:30. Возьмите студенческий.»"
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={8}
          />
          <div className="flex items-center justify-between text-xs">
            <span className={remaining < 200 ? (remaining < 0 ? "text-danger" : "text-warn") : "text-subtle"}>
              Осталось символов: {remaining}
            </span>
            <span className="text-subtle">Markdown не поддерживается</span>
          </div>

          {err && <Toast message={err} kind="error" onDismiss={() => setErr(null)} />}
          {sent !== null && (
            <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success">
              <IconCheckCircle size={18} />
              Доставлено: <span className="font-bold">{sent}</span> участникам
            </div>
          )}

          <div className="flex gap-2 pt-1">
            <Button
              onClick={onSend}
              disabled={busy || !text.trim() || remaining < 0}
              className="gap-1.5"
            >
              <IconBroadcast size={14} />
              {busy ? "Отправляем…" : "Отправить"}
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
