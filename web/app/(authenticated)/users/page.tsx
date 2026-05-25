"use client";

import { useEffect, useMemo, useState } from "react";
import { api, HttpError } from "@/lib/api";
import { Role, UserListItem, UserListResp, roleLabel } from "@/lib/types";
import { useMe } from "@/components/me-context";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Toast } from "@/components/ui/toast";
import { fmtDate } from "@/lib/format";
import {
  IconUsers,
  IconSearch,
  IconShield,
  IconUser,
  IconArrowLeft,
  IconArrowRight,
} from "@/components/ui/icons";

const ADMIN_ROLES: Role[] = ["applicant", "organizer", "staff", "admin"];
const ORGANIZER_ROLES: Role[] = ["applicant", "staff"];
const PAGE_SIZE = 50;

// Role badge styles
const roleBadge: Record<Role, string> = {
  admin: "bg-accent/15 text-accent border border-accent/25",
  organizer: "bg-blue-500/15 text-blue-400 border border-blue-400/25",
  staff: "bg-emerald-500/15 text-emerald-400 border border-emerald-400/25",
  applicant: "bg-muted text-subtle border border-border",
};

function RolePill({ role }: { role: Role }) {
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${roleBadge[role]}`}>
      {role === "admin" && <IconShield size={10} />}
      {roleLabel(role) || role}
    </span>
  );
}

export default function UsersPage() {
  const me = useMe();
  const [data, setData] = useState<UserListResp | null>(null);
  const [roleFilter, setRoleFilter] = useState<"" | Role>("");
  const [query, setQuery] = useState("");
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [savingID, setSavingID] = useState<number | null>(null);

  const managerRole = me.user.role;
  const availableRoles = managerRole === "organizer" ? ORGANIZER_ROLES : ADMIN_ROLES;

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

  const canEditUser = useMemo(
    () => (u: UserListItem) => {
      if (isMyself(u.id)) return false;
      if (managerRole === "admin") return true;
      return u.role === "applicant" || u.role === "staff";
    },
    [isMyself, managerRole],
  );

  const roleHint = useMemo(
    () => (u: UserListItem) => {
      if (isMyself(u.id)) return "(вы сами — смена недоступна)";
      if (managerRole === "organizer" && !canEditUser(u)) {
        return "(эту роль меняет только администратор)";
      }
      return null;
    },
    [canEditUser, isMyself, managerRole],
  );

  const roleOptions = useMemo(
    () => (u: UserListItem) => {
      if (canEditUser(u)) return availableRoles;
      return [u.role];
    },
    [availableRoles, canEditUser],
  );

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setOffset(0);
    load(0, roleFilter, query);
  }

  return (
    <div className="space-y-6 page-enter">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Пользователи</h1>
          <p className="mt-1 text-sm text-subtle">
            {managerRole === "admin"
              ? "Управление ролями. Изменения сразу попадают в audit log."
              : "Назначение и снятие роли волонтёра. Организатор может переводить applicant ↔ staff; изменения попадают в audit log."}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="rounded-lg border border-border bg-muted/60 px-3 py-1.5">
            <span className="text-xs text-subtle">Всего: </span>
            <span className="text-sm font-semibold text-text">{total}</span>
          </div>
        </div>
      </div>

      {err && <Toast message={err} kind="error" onDismiss={() => setErr(null)} />}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <IconUsers size={16} className="text-subtle" />
            Список пользователей
          </CardTitle>
        </CardHeader>
        <CardBody>
          {/* Search / filters */}
          <form
            onSubmit={onSearch}
            className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center"
          >
            <div className="relative flex-1">
              <IconSearch size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-subtle" />
              <Input
                placeholder="Поиск по MAX ID, ФИО, телефону или email"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="pl-8"
              />
            </div>
            <select
              value={roleFilter}
              onChange={(e) => {
                const v = e.target.value as "" | Role;
                setRoleFilter(v);
                setOffset(0);
                load(0, v, query);
              }}
              className="rounded-md border border-border bg-muted/70 px-3 py-2 text-sm text-text focus:outline-none focus:ring-1 focus:ring-accent/30 sm:w-44"
            >
              <option value="">Все роли</option>
              {availableRoles.map((r) => (
                <option key={r} value={r}>
                  {roleLabel(r) || r}
                </option>
              ))}
            </select>
            <Button type="submit" className="gap-1.5 sm:w-auto">
              <IconSearch size={13} />
              Найти
            </Button>
          </form>

          {/* Role filter pills */}
          <div className="mb-4 flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => { setRoleFilter(""); setOffset(0); load(0, "", query); }}
              className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                roleFilter === "" ? "bg-accent text-white" : "bg-muted text-subtle hover:text-text"
              }`}
            >
              Все
            </button>
            {availableRoles.map((r) => (
              <button
                key={r}
                type="button"
                onClick={() => { setRoleFilter(r); setOffset(0); load(0, r, query); }}
                className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                  roleFilter === r
                    ? roleBadge[r].replace("border ", "")
                    : "bg-muted text-subtle hover:text-text"
                }`}
              >
                {roleLabel(r) || r}
              </button>
            ))}
          </div>

          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3, 4, 5].map((i) => (
                <div key={i} className="h-14 rounded-lg bg-muted/50 animate-pulse" />
              ))}
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted/60">
                <IconUser size={24} className="text-subtle" />
              </div>
              <p className="text-subtle">Никого не найдено.</p>
            </div>
          ) : (
            <>
              {/* Mobile cards */}
              <ul className="space-y-2 md:hidden">
                {items.map((u) => (
                  <li
                    key={u.id}
                    className="rounded-lg border border-border bg-muted/30 p-3 transition-colors hover:bg-muted/50"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-subtle">
                            {(u.full_name || `M${u.max_user_id}`).slice(0, 1).toUpperCase()}
                          </div>
                          <div className="min-w-0">
                            <div className="truncate text-sm font-medium text-text">
                              {u.full_name || `MAX#${u.max_user_id}`}
                            </div>
                            <div className="font-mono text-xs text-subtle truncate">
                              {u.phone_masked || u.email_masked || "—"}
                            </div>
                          </div>
                        </div>
                      </div>
                      <RolePill role={u.role} />
                    </div>
                    <div className="mt-3">
                      <RoleSelect
                        value={u.role}
                        options={roleOptions(u)}
                        disabled={savingID === u.id || !canEditUser(u)}
                        onChange={(r) => setUserRole(u, r)}
                      />
                      {roleHint(u) && (
                        <p className="mt-1 text-xs text-subtle">{roleHint(u)}</p>
                      )}
                    </div>
                    <div className="mt-2 text-xs text-subtle">
                      c {fmtDate(u.created_at)} · ID: {u.max_user_id}
                    </div>
                  </li>
                ))}
              </ul>

              {/* Desktop table */}
              <div className="hidden overflow-x-auto md:block">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-border">
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Пользователь</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Контакт</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">Роль</th>
                      <th className="pb-2 pr-3 text-xs font-medium uppercase tracking-wide text-subtle">MAX ID</th>
                      <th className="pb-2 text-xs font-medium uppercase tracking-wide text-subtle">Создан</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((u) => (
                      <tr
                        key={u.id}
                        className={`border-b border-border/40 last:border-b-0 transition-colors ${
                          isMyself(u.id) ? "bg-accent/5" : "hover:bg-muted/30"
                        }`}
                      >
                        <td className="py-2.5 pr-3">
                          <div className="flex items-center gap-2">
                            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-subtle">
                              {(u.full_name || `M${u.max_user_id}`).slice(0, 1).toUpperCase()}
                            </div>
                            <div>
                              <div className="font-medium text-text">
                                {u.full_name || "—"}
                              </div>
                              {isMyself(u.id) && (
                                <div className="text-xs text-accent">вы</div>
                              )}
                            </div>
                          </div>
                        </td>
                        <td className="py-2.5 pr-3 font-mono text-xs text-subtle">
                          {u.phone_masked || u.email_masked || "—"}
                        </td>
                        <td className="py-2.5 pr-3">
                          <div className="flex flex-col gap-1">
                            <RoleSelect
                              value={u.role}
                              options={roleOptions(u)}
                              disabled={savingID === u.id || !canEditUser(u)}
                              onChange={(r) => setUserRole(u, r)}
                            />
                            {roleHint(u) && (
                              <span className="text-[11px] text-subtle">{roleHint(u)}</span>
                            )}
                          </div>
                        </td>
                        <td className="py-2.5 pr-3 font-mono text-xs text-subtle">
                          {u.max_user_id}
                        </td>
                        <td className="py-2.5 text-xs text-subtle">
                          {fmtDate(u.created_at)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {/* Pagination */}
          <div className="mt-5 flex items-center justify-between border-t border-border/60 pt-4">
            <Button
              variant="secondary"
              disabled={offset === 0 || loading}
              onClick={() => {
                const o = Math.max(0, offset - PAGE_SIZE);
                setOffset(o);
                load(o, roleFilter, query);
              }}
              className="gap-1.5"
            >
              <IconArrowLeft size={14} />
              Назад
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
              className="gap-1.5"
            >
              Вперёд
              <IconArrowRight size={14} />
            </Button>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}

function RoleSelect({
  value,
  options,
  disabled,
  onChange,
}: {
  value: Role;
  options: Role[];
  disabled: boolean;
  onChange: (r: Role) => void;
}) {
  return (
    <select
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as Role)}
      className={`rounded-md border px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-accent/30 transition-colors ${
        disabled
          ? "cursor-not-allowed border-border bg-muted/30 text-subtle opacity-60"
          : `border-border bg-muted/70 text-text hover:bg-muted ${roleBadge[value]}`
      }`}
    >
      {options.map((r) => (
        <option key={r} value={r}>
          {roleLabel(r) || r}
        </option>
      ))}
    </select>
  );
}
