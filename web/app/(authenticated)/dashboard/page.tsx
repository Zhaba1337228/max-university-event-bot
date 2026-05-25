"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, HttpError } from "@/lib/api";
import { DashboardResp, EventDTO, ListEventsResp, roleLabel, canManageEvents, canManageUsers } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { SkeletonStatRow } from "@/components/ui/skeleton";
import { Toast } from "@/components/ui/toast";
import { fmtDate, statusBadge, statusLabel } from "@/lib/format";
import { useMe } from "@/components/me-context";
import {
  IconCalendar,
  IconUsers,
  IconTrendingUp,
  IconClock,
  IconPlus,
  IconChevronRight,
  IconBarChart,
  IconQr,
} from "@/components/ui/icons";

// Stat card with icon, gradient accent bar, trend line
function StatCard({
  label,
  value,
  icon,
  accent = "accent",
  sub,
}: {
  label: string;
  value: number | string;
  icon: React.ReactNode;
  accent?: "accent" | "blue" | "emerald" | "warn";
  sub?: string;
}) {
  const accentMap = {
    accent: {
      border: "border-l-accent",
      iconBg: "bg-accent/15 text-accent",
      glow: "shadow-[0_0_0_1px_rgba(124,92,255,0.08)]",
    },
    blue: {
      border: "border-l-blue-400",
      iconBg: "bg-blue-500/15 text-blue-400",
      glow: "shadow-[0_0_0_1px_rgba(59,130,246,0.08)]",
    },
    emerald: {
      border: "border-l-emerald-400",
      iconBg: "bg-emerald-500/15 text-emerald-400",
      glow: "shadow-[0_0_0_1px_rgba(16,185,129,0.08)]",
    },
    warn: {
      border: "border-l-warn",
      iconBg: "bg-warn/15 text-warn",
      glow: "shadow-[0_0_0_1px_rgba(245,158,11,0.08)]",
    },
  };
  const c = accentMap[accent];
  return (
    <div
      className={`rounded-xl border border-border border-l-2 ${c.border} bg-surface p-4 shadow-card ${c.glow} flex flex-col gap-3`}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-subtle">{label}</span>
        <span className={`flex h-8 w-8 items-center justify-center rounded-lg ${c.iconBg}`}>
          {icon}
        </span>
      </div>
      <div>
        <div className="text-3xl font-bold tabular-nums leading-none text-text">{value}</div>
        {sub && <div className="mt-1 text-xs text-subtle">{sub}</div>}
      </div>
    </div>
  );
}

// Event row in dashboard list
function EventRow({ e }: { e: EventDTO }) {
  const pct = e.capacity > 0 ? Math.round(((e.capacity - (e.free_seats ?? e.capacity)) / e.capacity) * 100) : 0;
  return (
    <li className="group">
      <Link
        href={`/events/${e.id}`}
        className="flex items-center gap-3 rounded-lg px-3 py-2.5 no-underline transition-colors hover:bg-muted/60"
      >
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border bg-muted/60">
          <IconCalendar size={16} className="text-subtle" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium text-text group-hover:text-accent transition-colors">
              {e.title}
            </span>
            <Badge className={statusBadge(e.status) + " shrink-0 text-xs"}>
              {statusLabel(e.status)}
            </Badge>
          </div>
          <div className="mt-1 flex items-center gap-3">
            <span className="text-xs text-subtle">{fmtDate(e.starts_at)}</span>
            {typeof e.free_seats === "number" && (
              <div className="flex flex-1 items-center gap-2 max-w-[120px]">
                <Progress
                  value={pct}
                  size="sm"
                  colorClass={pct >= 90 ? "bg-danger" : pct >= 60 ? "bg-warn" : "bg-accent"}
                />
                <span className="shrink-0 text-xs tabular-nums text-subtle">
                  {e.capacity - (e.free_seats ?? 0)}/{e.capacity}
                </span>
              </div>
            )}
          </div>
        </div>
        <IconChevronRight size={14} className="shrink-0 text-border group-hover:text-subtle transition-colors" />
      </Link>
    </li>
  );
}

export default function DashboardPage() {
  const me = useMe();
  const [stats, setStats] = useState<DashboardResp | null>(null);
  const [my, setMy] = useState<EventDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const canCreate = canManageEvents(me.user.role);

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
          setLoading(false);
        }
      } catch (e) {
        if (!cancelled) {
          setErr(
            e instanceof HttpError && e.body?.message
              ? e.body.message
              : "Не удалось загрузить данные",
          );
          setLoading(false);
        }
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const role = me.user.role;
  const isAdmin = role === "admin";

  // Greeting based on time of day
  const hour = new Date().getHours();
  const greeting = hour < 12 ? "Доброе утро" : hour < 18 ? "Добрый день" : "Добрый вечер";

  return (
    <div className="space-y-6 page-enter">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm text-subtle">{greeting} 👋</p>
          <h1 className="mt-0.5 text-2xl font-bold text-text sm:text-3xl">Дашборд</h1>
          <p className="mt-1 text-sm text-subtle">
            Вы вошли как{" "}
            <span className="font-medium text-text">{roleLabel(role)}</span>
          </p>
        </div>
        {canCreate && (
          <Link href="/events/new">
            <Button className="gap-1.5">
              <IconPlus size={15} />
              Создать мероприятие
            </Button>
          </Link>
        )}
      </div>

      {err && <Toast message={err} kind="error" onDismiss={() => setErr(null)} />}

      {/* Stats */}
      {loading ? (
        <SkeletonStatRow />
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 sm:gap-4">
          <StatCard
            label={isAdmin ? "Все мероприятия" : "Мои мероприятия"}
            value={stats?.total_events ?? 0}
            icon={<IconCalendar size={18} />}
            accent="accent"
          />
          <StatCard
            label="Зарегистрировано"
            value={stats?.total_registered ?? 0}
            icon={<IconUsers size={18} />}
            accent="blue"
            sub="всего участников"
          />
          <StatCard
            label="Предстоящие"
            value={stats?.upcoming_events ?? 0}
            icon={<IconClock size={18} />}
            accent="emerald"
            sub="запланировано"
          />
        </div>
      )}

      {/* Main content grid */}
      <div className="grid gap-4 lg:grid-cols-3">
        {/* Events list — 2/3 width on large screens */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconCalendar size={16} className="text-subtle" />
              {isAdmin ? "Все мои мероприятия" : "Мои мероприятия"}
            </CardTitle>
            {my.length > 0 && (
              <Link href="/events" className="text-xs text-accent no-underline hover:underline">
                Все →
              </Link>
            )}
          </CardHeader>
          <CardBody>
            {loading ? (
              <div className="space-y-2">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="h-14 rounded-lg bg-muted/50 animate-pulse" />
                ))}
              </div>
            ) : my.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-3 py-10 text-center">
                <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted/60">
                  <IconCalendar size={24} className="text-subtle" />
                </div>
                <div>
                  <p className="font-medium text-text">Нет мероприятий</p>
                  <p className="mt-0.5 text-sm text-subtle">
                    Создайте первое через бот или кнопку выше
                  </p>
                </div>
                {canCreate && (
                  <Link href="/events/new">
                    <Button variant="secondary" className="mt-1 gap-1.5">
                      <IconPlus size={14} />
                      Создать
                    </Button>
                  </Link>
                )}
              </div>
            ) : (
              <ul className="divide-y divide-border/50 -mx-1">
                {my.slice(0, 8).map((e) => (
                  <EventRow key={e.id} e={e} />
                ))}
              </ul>
            )}
          </CardBody>
        </Card>

        {/* Quick actions sidebar — 1/3 width on large screens */}
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <IconTrendingUp size={16} className="text-subtle" />
                Быстрые действия
              </CardTitle>
            </CardHeader>
            <CardBody className="space-y-2">
              {canCreate && (
                <QuickAction
                  href="/events/new"
                  icon={<IconPlus size={16} className="text-accent" />}
                  label="Создать мероприятие"
                  desc="Откроет форму создания"
                  accent
                />
              )}
              <QuickAction
                href="/events"
                icon={<IconCalendar size={16} className="text-blue-400" />}
                label="Все мероприятия"
                desc="Список и фильтры"
              />
              {canManageUsers(role) && (
                <>
                  <QuickAction
                    href="/users"
                    icon={<IconUsers size={16} className="text-emerald-400" />}
                    label={role === "organizer" ? "Волонтёры" : "Пользователи"}
                    desc={role === "organizer" ? "Выдать или снять роль staff" : "Управление ролями"}
                  />
                </>
              )}
              {isAdmin && (
                <QuickAction
                  href="/checkin"
                  icon={<IconQr size={16} className="text-warn" />}
                  label="Check-in"
                  desc="Сканер QR-кодов"
                />
              )}
            </CardBody>
          </Card>

          {/* Role info card */}
          <RoleInfoCard role={role} />
        </div>
      </div>
    </div>
  );
}

function QuickAction({
  href,
  icon,
  label,
  desc,
  accent,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  desc: string;
  accent?: boolean;
}) {
  return (
    <Link
      href={href}
      className={`flex items-center gap-3 rounded-lg border p-3 no-underline transition-all hover:shadow-card group ${
        accent
          ? "border-accent/30 bg-accent/5 hover:bg-accent/10"
          : "border-border bg-muted/30 hover:bg-muted"
      }`}
    >
      <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${accent ? "bg-accent/15" : "bg-muted"}`}>
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-text group-hover:text-accent transition-colors">{label}</div>
        <div className="text-xs text-subtle">{desc}</div>
      </div>
      <IconChevronRight size={14} className="shrink-0 text-border group-hover:text-subtle transition-colors" />
    </Link>
  );
}

function RoleInfoCard({ role }: { role: string }) {
  const roleInfo: Record<string, { title: string; desc: string; color: string; icon: React.ReactNode }> = {
    admin: {
      title: "Администратор",
      desc: "Полный доступ ко всем функциям системы",
      color: "border-accent/30 bg-accent/5",
      icon: <IconBarChart size={20} className="text-accent" />,
    },
    organizer: {
      title: "Организатор",
      desc: "Создание и управление мероприятиями",
      color: "border-blue-400/30 bg-blue-500/5",
      icon: <IconCalendar size={20} className="text-blue-400" />,
    },
    staff: {
      title: "Волонтёр",
      desc: "Check-in участников на входе",
      color: "border-emerald-400/30 bg-emerald-500/5",
      icon: <IconQr size={20} className="text-emerald-400" />,
    },
  };
  const info = roleInfo[role];
  if (!info) return null;
  return (
    <div className={`rounded-xl border p-4 ${info.color}`}>
      <div className="flex items-start gap-3">
        {info.icon}
        <div>
          <div className="text-sm font-semibold text-text">{info.title}</div>
          <div className="mt-0.5 text-xs text-subtle">{info.desc}</div>
        </div>
      </div>
    </div>
  );
}
