"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, HttpError } from "@/lib/api";
import { EventInput, canManageEvents } from "@/lib/types";
import { useMe } from "@/components/me-context";
import { EventForm } from "../_components/EventForm";

export default function CreateEventPage() {
  const router = useRouter();
  const me = useMe();
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Защита от прямого захода staff/applicant в URL: страница доступна
  // только organizer и admin (canManageEvents). У RBAC основная защита на
  // бэке (POST /api/events отвечает 403), здесь — UX-страховка.
  if (!canManageEvents(me.user.role)) {
    return (
      <div className="space-y-3">
        <h1 className="text-2xl font-semibold">Нет доступа</h1>
        <p className="text-subtle">
          Создавать мероприятия могут только организатор и админ.
        </p>
        <Link href="/events" className="text-accent no-underline hover:underline">
          ← К списку мероприятий
        </Link>
      </div>
    );
  }

  async function handleSubmit(input: EventInput) {
    setSubmitting(true);
    setErr(null);
    try {
      const resp = await api.post<{ event: { id: number } }>("/api/events", input);
      router.replace(`/events/${resp.event.id}`);
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось создать",
      );
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <Link href="/events" className="text-sm text-subtle no-underline hover:text-text">
          ← К списку мероприятий
        </Link>
        <h1 className="mt-1 text-2xl font-semibold sm:text-3xl">Новое мероприятие</h1>
      </div>

      <EventForm
        mode="create"
        onSubmit={handleSubmit}
        submitting={submitting}
        error={err}
      />
    </div>
  );
}
