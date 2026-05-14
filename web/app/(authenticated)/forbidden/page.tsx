"use client";

import Link from "next/link";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useMe } from "@/components/me-context";
import { roleLabel } from "@/lib/types";

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

  let title = "Нет доступа";
  let message =
    "У вашей роли нет прав на эту страницу. Обратитесь к администратору, если считаете это ошибкой.";
  let back = { href: "/dashboard", label: "На дашборд" };

  if (reason === "checkin_organizer") {
    title = "Check-in доступен только волонтёрам";
    message =
      "Сканирование QR-кодов гостей выполняют сотрудники с ролью «Волонтёр (staff)». " +
      "Как организатор вы создаёте мероприятия и работаете с участниками, но не сканируете гостей — " +
      "это разделение нужно, чтобы у входа стоял отдельный человек со сканером.";
    back = { href: "/dashboard", label: "К моим мероприятиям" };
  }

  return (
    <div className="mx-auto max-w-xl">
      <Card>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardBody className="space-y-3">
          <p className="text-text/90">{message}</p>
          <p className="text-xs text-subtle">
            Ваша роль: <span className="font-medium text-text">{roleLabel(me.user.role)}</span>
          </p>
          <div className="pt-2">
            <Link href={back.href}>
              <Button variant="primary">{back.label}</Button>
            </Link>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
