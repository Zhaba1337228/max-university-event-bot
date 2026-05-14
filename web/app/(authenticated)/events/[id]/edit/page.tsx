"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import {
  EventDetail,
  EventDTO,
  EventInput,
  canEditEvent,
} from "@/lib/types";
import { useMe } from "@/components/me-context";
import { EventForm } from "../../_components/EventForm";

export default function EditEventPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const me = useMe();
  const id = Number(params.id);

  const [event, setEvent] = useState<EventDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!Number.isFinite(id) || id <= 0) {
      router.replace("/events");
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const d = await api.get<EventDetail>(`/api/events/${id}`);
        if (!cancelled) {
          setEvent(d.event);
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
  }, [id, router]);

  async function handleSubmit(input: EventInput) {
    setSubmitting(true);
    setErr(null);
    try {
      await api.patch<{ event: EventDTO }>(`/api/events/${id}`, input);
      router.replace(`/events/${id}`);
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось сохранить",
      );
      setSubmitting(false);
    }
  }

  if (loading) return <p className="text-subtle">Загрузка…</p>;
  if (err && !event) return <p className="text-danger">{err}</p>;
  if (!event) return <p className="text-danger">Мероприятие не найдено</p>;

  if (!canEditEvent(me.user.role, me.user.id, event.created_by)) {
    return (
      <div className="space-y-3">
        <h1 className="text-2xl font-semibold">Нет доступа</h1>
        <p className="text-subtle">
          Редактировать мероприятие может только его владелец или администратор.
        </p>
        <Link
          href={`/events/${id}`}
          className="text-accent no-underline hover:underline"
        >
          ← К мероприятию
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <Link
          href={`/events/${id}`}
          className="text-sm text-subtle no-underline hover:text-text"
        >
          ← К мероприятию
        </Link>
        <h1 className="mt-1 text-2xl font-semibold sm:text-3xl">
          Редактирование: {event.title}
        </h1>
      </div>

      <EventForm
        mode="edit"
        initial={event}
        onSubmit={handleSubmit}
        submitting={submitting}
        error={err}
      />
    </div>
  );
}
