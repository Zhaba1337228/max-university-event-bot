"use client";

import Link from "next/link";
import { ROLE_CAPABILITIES, roleLabel } from "@/lib/types";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { useMe } from "@/components/me-context";
import {
  IconShield,
  IconCalendar,
  IconQr,
  IconUser,
  IconCheck,
  IconX,
} from "@/components/ui/icons";

// Role definitions for the overview cards
const ROLE_OVERVIEW = [
  {
    role: "admin" as const,
    icon: <IconShield size={22} />,
    color: "border-l-accent bg-gradient-to-r from-accent/8 to-transparent",
    iconBg: "bg-accent/15 text-accent",
    label: "Администратор",
    desc: "Полный доступ ко всем функциям. Управляет ролями пользователей, видит все мероприятия, может раскрывать ПДн.",
    examples: ["Создаёт и редактирует любые мероприятия", "Управляет ролями: organizer, staff", "Видит полные ФИО и контакты участников", "Делает QR check-in и рассылки"],
  },
  {
    role: "organizer" as const,
    icon: <IconCalendar size={22} />,
    color: "border-l-blue-400 bg-gradient-to-r from-blue-500/8 to-transparent",
    iconBg: "bg-blue-500/15 text-blue-400",
    label: "Организатор",
    desc: "Создаёт мероприятия и управляет только теми, которые создал сам. Данные участников видит в замаскированном виде и может выдавать роль волонтёра.",
    examples: ["Создаёт свои мероприятия", "Редактирует/закрывает только свои", "Видит и отмечает участников своих событий", "Выдаёт и снимает роль волонтёра (staff)"],
  },
  {
    role: "staff" as const,
    icon: <IconQr size={22} />,
    color: "border-l-emerald-400 bg-gradient-to-r from-emerald-500/8 to-transparent",
    iconBg: "bg-emerald-500/15 text-emerald-400",
    label: "Волонтёр (check-in)",
    desc: "Стоит на входе и сканирует QR-коды гостей. Не имеет доступа к управлению мероприятиями.",
    examples: ["QR-сканер check-in на входе", "Ручной ввод QR-кода", "Поиск записи по коду участника"],
  },
  {
    role: "applicant" as const,
    icon: <IconUser size={22} />,
    color: "border-l-border bg-gradient-to-r from-muted/40 to-transparent",
    iconBg: "bg-muted text-subtle",
    label: "Абитуриент",
    desc: "Взаимодействует только с ботом в мессенджере MAX. Нет доступа к веб-панели.",
    examples: ["Регистрируется на мероприятия через бота", "Получает уведомления", "Отменяет запись через бота"],
  },
];

// Column cell
function CapCell({ value }: { value: boolean | "own" }) {
  if (value === "own") {
    return (
      <span className="inline-flex items-center justify-center gap-1 text-xs font-medium text-blue-400">
        <IconCheck size={12} />
        своё
      </span>
    );
  }
  if (value === true) {
    return (
      <span className="flex items-center justify-center">
        <IconCheck size={16} className="text-success" />
      </span>
    );
  }
  return (
    <span className="flex items-center justify-center">
      <IconX size={14} className="text-border" />
    </span>
  );
}

export default function RolesPage() {
  const me = useMe();

  return (
    <div className="space-y-8 page-enter">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold sm:text-3xl">Права доступа</h1>
        <p className="mt-1 text-sm text-subtle">
          Матрица разрешений по ролям. Администратор управляет всеми ролями, а организатор может выдавать и забирать только роль волонтёра.
          {(me.user.role === "admin" || me.user.role === "organizer") && (
            <> <Link href="/users" className="text-accent">Перейти к пользователям →</Link></>
          )}
        </p>
      </div>

      {/* Role overview cards */}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {ROLE_OVERVIEW.map((r) => {
          const isCurrent = me.user.role === r.role;
          return (
            <div
              key={r.role}
              className={`rounded-xl border border-border border-l-2 p-4 ${r.color} ${
                isCurrent ? "ring-1 ring-accent/30" : ""
              }`}
            >
              <div className="flex items-center gap-3">
                <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${r.iconBg}`}>
                  {r.icon}
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="font-semibold text-text">{r.label}</span>
                    {isCurrent && (
                      <span className="rounded-full bg-accent/15 px-1.5 py-0.5 text-xs text-accent">
                        вы
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <p className="mt-3 text-xs leading-relaxed text-subtle">{r.desc}</p>
              <ul className="mt-3 space-y-1">
                {r.examples.map((ex) => (
                  <li key={ex} className="flex items-start gap-1.5 text-xs text-text/70">
                    <span className="mt-0.5 shrink-0 text-border">›</span>
                    {ex}
                  </li>
                ))}
              </ul>
            </div>
          );
        })}
      </div>

      {/* Permission matrix table */}
      <Card>
        <CardHeader>
          <CardTitle>Матрица разрешений</CardTitle>
        </CardHeader>
        <CardBody>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="pb-3 pr-4 text-left text-xs font-medium uppercase tracking-wide text-subtle">
                    Действие
                  </th>
                  {ROLE_OVERVIEW.filter((r) => r.role !== "applicant").map((r) => (
                    <th
                      key={r.role}
                      className={`pb-3 px-3 text-center text-xs font-medium uppercase tracking-wide ${
                        me.user.role === r.role ? "text-accent" : "text-subtle"
                      }`}
                    >
                      {r.role === "admin" ? "Admin" : r.role === "organizer" ? "Org" : "Staff"}
                      {me.user.role === r.role && (
                        <span className="ml-1 text-accent">↑</span>
                      )}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {ROLE_CAPABILITIES.map((cap, idx) => (
                  <tr
                    key={cap.key}
                    className={`border-b border-border/40 last:border-b-0 ${
                      idx % 2 === 0 ? "" : "bg-muted/10"
                    }`}
                  >
                    <td className="py-2.5 pr-4 text-sm text-text">{cap.label}</td>
                    <td className="py-2.5 px-3 text-center">
                      <CapCell value={cap.admin} />
                    </td>
                    <td className="py-2.5 px-3 text-center">
                      <CapCell value={cap.organizer} />
                    </td>
                    <td className="py-2.5 px-3 text-center">
                      <CapCell value={cap.staff} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex flex-wrap gap-4 border-t border-border/60 pt-4 text-xs text-subtle">
            <span className="flex items-center gap-1.5">
              <IconCheck size={12} className="text-success" /> Разрешено
            </span>
            <span className="flex items-center gap-1.5">
              <IconCheck size={12} className="text-blue-400" /> своё — только для своих мероприятий
            </span>
            <span className="flex items-center gap-1.5">
              <IconX size={12} className="text-border" /> Запрещено
            </span>
          </div>
        </CardBody>
      </Card>

      {/* How to grant access */}
      {(me.user.role === "admin" || me.user.role === "organizer") && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <IconShield size={16} className="text-subtle" />
              {me.user.role === "admin" ? "Как выдать доступ" : "Как назначить волонтёра"}
            </CardTitle>
          </CardHeader>
          <CardBody className="space-y-3 text-sm text-subtle">
            <p>
              {me.user.role === "admin"
                ? <>Чтобы дать пользователю роль <span className="font-medium text-text">Организатора</span> или <span className="font-medium text-text">Волонтёра</span>:</>
                : <>Чтобы выдать пользователю роль <span className="font-medium text-text">Волонтёра</span>:</>}
            </p>
            <ol className="ml-4 list-decimal space-y-1.5">
              <li>
                Пользователь должен войти в бот MAX хотя бы один раз (это создаёт запись в системе).
              </li>
              <li>
                Перейдите в раздел{" "}
                <Link href="/users" className="text-accent font-medium">
                  Пользователи
                </Link>{" "}
                и найдите нужного по имени, телефону или MAX ID.
              </li>
              <li>
                В колонке «Роль» выберите нужную роль из выпадающего списка — она применяется сразу.
              </li>
              <li>
                Пользователь сможет войти в панель через бот командой{" "}
                <code className="rounded bg-muted px-1.5 py-0.5 font-mono">/admin_login</code>.
              </li>
            </ol>
            <div className="pt-2">
              <Link href="/users">
                <span className="inline-flex items-center gap-1.5 rounded-lg border border-accent/30 bg-accent/8 px-3 py-2 text-sm font-medium text-accent no-underline hover:bg-accent/15 transition-colors">
                  Перейти к управлению пользователями →
                </span>
              </Link>
            </div>
          </CardBody>
        </Card>
      )}
    </div>
  );
}
