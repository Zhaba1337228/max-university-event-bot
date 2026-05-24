"use client";

import Link from "next/link";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { useMe } from "@/components/me-context";
import { roleLabel } from "@/lib/types";
import { IconLock, IconArrowLeft, IconShield } from "@/components/ui/icons";

// /forbidden?reason=... — дружелюбная страница 403 для случаев,
// когда роль пользователя не позволяет открыть запрошенный раздел.
export default function ForbiddenPage() {
  return (
    <Suspense fallback={<p className="text-subtle">Загрузка…</p>}>
      <Body />
    </Suspense>
  );
}

function Body() {
  const sp = useSearchParams();
  const me = useMe();
  const reason = sp.get("reason");

  let icon = <IconLock size={40} className="text-danger" />;
  let title = "Нет доступа";
  let message =
    "У вашей роли нет прав на эту страницу. Обратитесь к администратору, если считаете это ошибкой.";
  let back = { href: "/dashboard", label: "На дашборд" };

  if (reason === "checkin_organizer") {
    icon = <IconShield size={40} className="text-warn" />;
    title = "Check-in — только для волонтёров";
    message =
      "Сканирование QR-кодов гостей выполняют сотрудники с ролью «Волонтёр (staff)». " +
      "Как организатор вы создаёте мероприятия и работаете с участниками, но не сканируете гостей — " +
      "это разделение нужно, чтобы у входа стоял отдельный человек со сканером.";
    back = { href: "/dashboard", label: "К моим мероприятиям" };
  } else if (reason === "users_admin") {
    icon = <IconShield size={40} className="text-accent" />;
    title = "Только для администраторов";
    message =
      "Управление пользователями и ролями доступно только администраторам системы.";
    back = { href: "/dashboard", label: "На дашборд" };
  }

  return (
    <div className="flex min-h-[60vh] items-center justify-center page-enter">
      <div className="mx-auto max-w-md text-center">
        {/* Icon */}
        <div className="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-2xl border border-border bg-surface shadow-elevated">
          {icon}
        </div>

        {/* Title */}
        <h1 className="text-2xl font-bold text-text">{title}</h1>

        {/* Message */}
        <p className="mt-3 text-sm leading-relaxed text-subtle">{message}</p>

        {/* Role badge */}
        <div className="mt-4 inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-3 py-1.5 text-xs text-subtle">
          <span>Ваша роль:</span>
          <span className="font-medium text-text">{roleLabel(me.user.role)}</span>
        </div>

        {/* CTA */}
        <div className="mt-6">
          <Link href={back.href}>
            <Button className="gap-1.5">
              <IconArrowLeft size={14} />
              {back.label}
            </Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
