"use client";

import { FormEvent, useState } from "react";
import { EventDTO, EventInput } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

// EventForm — переиспользуемая форма создания и редактирования мероприятия.
//
// mode="create" — все поля пустые / дефолтные, status скрыт (всегда open).
// mode="edit" — поля заполнены из initial, доступно переключение open/closed.
//
// Валидация на клиенте лёгкая (title непустой, дата начала задана) — основная
// проверка на сервере. На submit зовём onSubmit; родитель решает, что
// делать (POST или PATCH).

type Props = {
  mode: "create" | "edit";
  initial?: EventDTO;
  onSubmit: (input: EventInput) => Promise<void>;
  submitting: boolean;
  error: string | null;
};

// toLocalInputValue конвертирует RFC3339 (UTC, например "2026-05-13T21:42:00Z")
// в значение для <input type="datetime-local"> в локальном TZ браузера
// (формат "YYYY-MM-DDTHH:MM" без TZ-суффикса).
function toLocalInputValue(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  // используем компоненты в локальном TZ
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    d.getFullYear() +
    "-" +
    pad(d.getMonth() + 1) +
    "-" +
    pad(d.getDate()) +
    "T" +
    pad(d.getHours()) +
    ":" +
    pad(d.getMinutes())
  );
}

// fromLocalInputValue: "YYYY-MM-DDTHH:MM" → RFC3339 в UTC.
function fromLocalInputValue(s: string): string {
  if (!s) return "";
  const d = new Date(s); // браузер считает строку без TZ как локальную
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString();
}

export function EventForm({ mode, initial, onSubmit, submitting, error }: Props) {
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [startsAt, setStartsAt] = useState(toLocalInputValue(initial?.starts_at));
  const [endsAt, setEndsAt] = useState(toLocalInputValue(initial?.ends_at));
  const [location, setLocation] = useState(initial?.location ?? "");
  const [format, setFormat] = useState<"offline" | "online" | "hybrid">(
    (initial?.format as "offline" | "online" | "hybrid") ?? "offline",
  );
  const [capacity, setCapacity] = useState<string>(
    initial?.capacity ? String(initial.capacity) : "50",
  );
  const [status, setStatus] = useState<"open" | "closed">(
    initial?.status === "closed" ? "closed" : "open",
  );
  const [tagsText, setTagsText] = useState((initial?.tags ?? []).join(", "));
  const [clientErr, setClientErr] = useState<string | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiTagsLoading, setAiTagsLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setClientErr(null);

    const t = title.trim();
    if (!t) {
      setClientErr("Название обязательно");
      return;
    }
    if (!startsAt) {
      setClientErr("Дата и время начала обязательны");
      return;
    }
    const cap = Number(capacity);
    if (!Number.isInteger(cap) || cap <= 0) {
      setClientErr("Вместимость должна быть положительным целым числом");
      return;
    }
    const startISO = fromLocalInputValue(startsAt);
    if (!startISO) {
      setClientErr("Некорректная дата начала");
      return;
    }
    let endISO: string | undefined;
    if (endsAt) {
      endISO = fromLocalInputValue(endsAt);
      if (!endISO) {
        setClientErr("Некорректная дата окончания");
        return;
      }
      if (new Date(endISO).getTime() <= new Date(startISO).getTime()) {
        setClientErr("Окончание должно быть позже начала");
        return;
      }
    }

    const tags = tagsText
      .split(",")
      .map((x) => x.trim())
      .filter((x) => x.length > 0);

    const input: EventInput = {
      title: t,
      description: description.trim(),
      starts_at: startISO,
      ends_at: endISO,
      location: location.trim(),
      format,
      capacity: cap,
      tags,
    };
    if (mode === "edit") {
      input.status = status;
    }
    await onSubmit(input);
  }

  async function handleAIAnnounce() {
    if (!title.trim()) { setClientErr("Введите название для генерации описания"); return; }
    setAiLoading(true);
    setClientErr(null);
    try {
      const res = await api.post<{ description: string; short_summary: string }>("/api/ai/announce", {
        title: title.trim(),
        date: startsAt ? new Date(startsAt).toLocaleDateString("ru-RU", { day: "2-digit", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" }) : "",
        location: location.trim(),
        format: format,
        hint: description.trim(),
      });
      if (res.description) setDescription(res.description);
    } catch {
      setClientErr("AI временно недоступен");
    } finally {
      setAiLoading(false);
    }
  }

  async function handleAITags() {
    if (!title.trim()) { setClientErr("Введите название для подбора тегов"); return; }
    setAiTagsLoading(true);
    setClientErr(null);
    try {
      const res = await api.post<{ tags: string[] }>("/api/ai/tags", {
        title: title.trim(),
        description: description.trim(),
      });
      if (res.tags?.length) setTagsText(res.tags.join(", "));
    } catch {
      setClientErr("AI временно недоступен");
    } finally {
      setAiTagsLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Основное</CardTitle>
        </CardHeader>
        <CardBody className="space-y-3">
          <Field label="Название" required>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="День открытых дверей 2026"
              maxLength={255}
            />
          </Field>
          <Field label="Описание">
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              placeholder="Полное описание мероприятия (видно абитуриентам в боте)"
              maxLength={16000}
            />
            <Button
              type="button"
              variant="secondary"
              disabled={aiLoading}
              onClick={handleAIAnnounce}
              className="mt-1.5 gap-1.5 text-xs"
            >
              {aiLoading ? "Генерирую…" : "✨ Сгенерировать описание"}
            </Button>
          </Field>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Когда и где</CardTitle>
        </CardHeader>
        <CardBody className="space-y-3">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="Начало" required>
              <Input
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
              />
            </Field>
            <Field label="Окончание (опционально)">
              <Input
                type="datetime-local"
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
              />
            </Field>
          </div>
          <Field label="Место">
            <Input
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="Главный корпус, ауд. 301"
              maxLength={512}
            />
          </Field>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="Формат">
              <select
                value={format}
                onChange={(e) => setFormat(e.target.value as "offline" | "online" | "hybrid")}
                className="w-full rounded-md border border-border bg-muted/70 px-3 py-2.5 text-sm text-text focus:border-accent/40 focus:outline-none focus:ring-2 focus:ring-accent/50"
              >
                <option value="offline">Очно</option>
                <option value="online">Онлайн</option>
                <option value="hybrid">Гибрид</option>
              </select>
            </Field>
            <Field label="Вместимость" required>
              <Input
                type="number"
                min={1}
                max={100000}
                value={capacity}
                onChange={(e) => setCapacity(e.target.value)}
              />
            </Field>
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Дополнительно</CardTitle>
        </CardHeader>
        <CardBody className="space-y-3">
          <Field label="Теги (через запятую)">
            <Input
              value={tagsText}
              onChange={(e) => setTagsText(e.target.value)}
              placeholder="ит, дизайн, открытые-двери"
            />
            <Button
              type="button"
              variant="secondary"
              disabled={aiTagsLoading}
              onClick={handleAITags}
              className="mt-1.5 gap-1.5 text-xs"
            >
              {aiTagsLoading ? "Подбираю…" : "✨ Предложить теги"}
            </Button>
          </Field>
          {mode === "edit" && (
            <Field label="Статус">
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value as "open" | "closed")}
                className="w-full rounded-md border border-border bg-muted/70 px-3 py-2.5 text-sm text-text focus:border-accent/40 focus:outline-none focus:ring-2 focus:ring-accent/50"
              >
                <option value="open">Открыто (приём заявок идёт)</option>
                <option value="closed">Закрыто (приём остановлен)</option>
              </select>
            </Field>
          )}
        </CardBody>
      </Card>

      {(clientErr || error) && (
        <p className="text-sm text-danger">{clientErr || error}</p>
      )}

      <div className="flex gap-2">
        <Button type="submit" disabled={submitting}>
          {submitting ? "Сохраняем…" : mode === "create" ? "Создать" : "Сохранить"}
        </Button>
      </div>
    </form>
  );
}

function Field({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm text-subtle">
        {label}
        {required && <span className="ml-0.5 text-danger">*</span>}
      </span>
      {children}
    </label>
  );
}
