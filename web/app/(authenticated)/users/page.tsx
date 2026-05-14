"use client";

import { useEffect, useMemo, useState } from "react";
import { api, HttpError } from "@/lib/api";
import { Role, UserListItem, UserListResp, roleLabel } from "@/lib/types";
import { useMe } from "@/components/me-context";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { fmtDate } from "@/lib/format";

const ROLES: Role[] = ["applicant", "organizer", "staff", "admin"];
const PAGE_SIZE = 50;

export default function UsersPage() {
  const me = useMe();
  const [data, setData] = useState<UserListResp | null>(null);
  const [roleFilter, setRoleFilter] = useState<"" | Role>("");
  const [query, setQuery] = useState("");
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [savingID, setSavingID] = useState<number | null>(null);

  async function load(o = offset, r = roleFilter, q = query) {
    setLoading(true);
    setErr(null);
    try {
      const usp = new URLSearchParams();
      usp.set("limit", String(PAGE_SIZE));
      usp.set("offset", String(o));
      if (r) usp.set("role", r);
      if (q.trim()) usp.set("query", q.trim());
      const d = await api.get<UserListResp>(`/api/users?${usp.toString()}`);
      setData(d);
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось загрузить",
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load(0, "", "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function setUserRole(u: UserListItem, newRole: Role) {
    if (u.role === newRole) return;
    setSavingID(u.id);
    setErr(null);
    try {
      const resp = await api.patch<{ user: UserListItem }>(
        `/api/users/${u.id}/role`,
        { role: newRole },
      );
      setData((prev) =>
        prev
          ? {
              ...prev,
              items: prev.items.map((x) =>
                x.id === u.id ? { ...x, ...resp.user } : x,
              ),
            }
          : prev,
      );
    } catch (e) {
      setErr(
        e instanceof HttpError && e.body?.message
          ? e.body.message
          : "Не удалось сменить роль",
      );
    } finally {
      setSavingID(null);
    }
  }

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const hasNext = offset + items.length < total;

  const isMyself = useMemo(
    () => (uid: number) => me?.user.id === uid,
    [me],
  );

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setOffset(0);
    load(0, roleFilter, query);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Пользователи</h1>
        <p className="mt-1 text-sm text-subtle">
          Управление ролями. Доступно только администраторам. Изменения попадают
          в audit log.
        </p>
      </div>

      {err && <p className="text-danger">{err}</p>}

      <Card>
        <CardHeader>
          <CardTitle>
            Всего: {total}
            {total > 0 && (
              <span className="ml-2 text-sm text-subtle">
                · показано {items.length}
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardBody>
          <form
            onSubmit={onSearch}
            className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center"
          >
            <Input
              placeholder="Поиск по ФИО, телефону или email"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="sm:flex-1"
            />
            <select
              value={roleFilter}
              onChange={(e) => {
                const v = e.target.value as "" | Role;
                setRoleFilter(v);
                setOffset(0);
                load(0, v, query);
              }}
              className="rounded-md border border-border bg-muted/70 px-3 py-2.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-accent/50 sm:w-48"
            >
              <option value="">Все роли</option>
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {roleLabel(r) || r}
                </option>
              ))}
            </select>
            <Button type="submit" variant="primary" className="sm:w-auto">
              Найти
            </Button>
          </form>

          {loading ? (
            <p className="text-subtle">Загрузка…</p>
          ) : items.length === 0 ? (
            <p className="text-subtle">Никого не найдено.</p>
          ) : (
            <>
              {/* Mobile cards */}
              <ul className="space-y-2 md:hidden">
                {items.map((u) => (
                  <li
                    key={u.id}
                    className="rounded-lg border border-border bg-muted/40 p-3"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="font-medium text-text">
                          {u.full_name || `MAX#${u.max_user_id}`}
                        </div>
                        <div className="mt-0.5 font-mono text-xs text-subtle">
                          {u.phone_masked || u.email_masked || "—"}
                        </div>
                        <div className="mt-1 text-xs text-subtle">
                          c {fmtDate(u.created_at)}
                        </div>
                      </div>
                    </div>
                    <div className="mt-3">
                      <RoleSelect
                        value={u.role}
                        disabled={savingID === u.id || isMyself(u.id)}
                        onChange={(r) => setUserRole(u, r)}
                      />
                      {isMyself(u.id) && (
                        <p className="mt-1 text-xs text-subtle">
                          (вы сами; смена недоступна)
                        </p>
                      )}
                    </div>
                  </li>
                ))}
              </ul>

              {/* Desktop table */}
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-border text-subtle">
                      <th className="py-2 pr-3 font-medium">ID</th>
                      <th className="py-2 pr-3 font-medium">ФИО</th>
                      <th className="py-2 pr-3 font-medium">Контакт</th>
                      <th className="py-2 pr-3 font-medium">Роль</th>
                      <th className="py-2 pr-3 font-medium">MAX id</th>
                      <th className="py-2 font-medium">Создан</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((u) => (
                      <tr
                        key={u.id}
                        className="border-b border-border/60 last:border-b-0"
                      >
                        <td className="py-2 pr-3 font-mono text-xs">{u.id}</td>
                        <td className="py-2 pr-3">{u.full_name || "—"}</td>
                        <td className="py-2 pr-3 font-mono text-xs">
                          {u.phone_masked || u.email_masked || "—"}
                        </td>
                        <td className="py-2 pr-3">
                          <RoleSelect
                            value={u.role}
                            disabled={savingID === u.id || isMyself(u.id)}
                            onChange={(r) => setUserRole(u, r)}
                          />
                          {isMyself(u.id) && (
                            <div className="mt-0.5 text-xs text-subtle">
                              (вы сами)
                            </div>
                          )}
                        </td>
                        <td className="py-2 pr-3 font-mono text-xs">
                          {u.max_user_id}
                        </td>
                        <td className="py-2 text-xs text-subtle">
                          {fmtDate(u.created_at)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          <div className="mt-4 flex items-center justify-between">
            <Button
              variant="secondary"
              disabled={offset === 0 || loading}
              onClick={() => {
                const o = Math.max(0, offset - PAGE_SIZE);
                setOffset(o);
                load(o, roleFilter, query);
              }}
            >
              ← Назад
            </Button>
            <span className="text-xs text-subtle">
              {items.length === 0
                ? "0"
                : `${offset + 1}–${offset + items.length}`}{" "}
              из {total}
            </span>
            <Button
              variant="secondary"
              disabled={!hasNext || loading}
              onClick={() => {
                const o = offset + PAGE_SIZE;
                setOffset(o);
                load(o, roleFilter, query);
              }}
            >
              Вперёд →
            </Button>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}

function RoleSelect({
  value,
  disabled,
  onChange,
}: {
  value: Role;
  disabled: boolean;
  onChange: (r: Role) => void;
}) {
  return (
    <select
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as Role)}
      className="rounded-md border border-border bg-muted/70 px-2.5 py-1.5 text-sm text-text focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:opacity-50"
    >
      {ROLES.map((r) => (
        <option key={r} value={r}>
          {roleLabel(r) || r}
        </option>
      ))}
    </select>
  );
}
