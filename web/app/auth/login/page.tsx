import Link from "next/link";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";

export default function LoginInfoPage() {
  return (
    <div className="container max-w-md py-16">
      <Card>
        <CardHeader>
          <CardTitle>Вход в админку</CardTitle>
        </CardHeader>
        <CardBody>
          <p className="mb-4">
            Откройте бота в MAX, отправьте команду <code className="rounded bg-muted px-1.5 py-0.5">/admin_login</code> и нажмите
            «Войти в админку». Ссылка действует 5 минут.
          </p>
          <p className="text-subtle">
            Если ссылка не сработала — повторите команду в боте, чтобы получить
            свежий токен.
          </p>
          <p className="mt-6">
            <Link href="/">На главную</Link>
          </p>
        </CardBody>
      </Card>
    </div>
  );
}
