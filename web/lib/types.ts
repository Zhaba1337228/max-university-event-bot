// DTO-типы, отзеркаленные с adminapi/handlers.go.
// Менять синхронно с бэком.

export type Role = "applicant" | "organizer" | "staff" | "admin";

export type Me = {
  user: { id: number; role: Role };
};

// roleLabel — короткая русская подпись роли для UI.
export function roleLabel(r: Role): string {
  switch (r) {
    case "admin":
      return "Администратор";
    case "organizer":
      return "Организатор";
    case "staff":
      return "Волонтёр (check-in)";
    default:
      return r;
  }
}

// canCheckin — может ли пользователь делать check-in (сканировать QR).
// Организатор НЕ может — он создаёт события, но не сканирует гостей.
export function canCheckin(r: Role): boolean {
  return r === "staff" || r === "admin";
}

// canManageEvents — может ли пользователь видеть дашборд и список мероприятий.
// Staff НЕ может — у него только сканер.
export function canManageEvents(r: Role): boolean {
  return r === "organizer" || r === "admin";
}

export type EventStatus = "draft" | "open" | "closed" | "cancelled" | "completed";

export type EventDTO = {
  id: number;
  title: string;
  description: string;
  starts_at: string;
  ends_at?: string;
  location: string;
  format: string;
  capacity: number;
  status: EventStatus;
  tags: string[] | null;
  free_seats?: number;
  short_summary?: string;
  created_by?: number;
  late_cancel_allowed?: boolean;
};

export type EventInput = {
  title: string;
  description: string;
  starts_at: string; // RFC3339
  ends_at?: string; // RFC3339
  location: string;
  format: "offline" | "online" | "hybrid";
  capacity: number;
  status?: "open" | "closed"; // только для update
  tags: string[];
};

// canEditEvent — пользователь может редактировать мероприятие, если он
// admin или владелец (created_by совпадает с me.user.id).
export function canEditEvent(role: Role, meID: number, createdBy?: number): boolean {
  if (role === "admin") return true;
  if (role !== "organizer") return false;
  return typeof createdBy === "number" && createdBy === meID;
}

// canManageRegistrations — может ли пользователь управлять записями
// (смотреть участников, ставить вручную attended/no_show).
// Для конкретного события нужна дополнительная проверка canEditEvent.
export function canManageRegistrations(role: Role): boolean {
  return role === "admin" || role === "organizer";
}

// canBroadcast — может ли пользователь делать рассылку участникам.
// Для конкретного события нужна дополнительная проверка canEditEvent.
export function canBroadcast(role: Role): boolean {
  return role === "admin" || role === "organizer";
}

// canViewAudit — может ли пользователь читать журнал действий.
export function canViewAudit(role: Role): boolean {
  return role === "admin" || role === "organizer";
}

// canUnmaskPII — может ли пользователь раскрыть полные ФИО/контакты участника.
// Только admin — у организатора данные маскируются для защиты ПДн.
export function canUnmaskPII(role: Role): boolean {
  return role === "admin";
}

// canManageUsers — может ли пользователь управлять ролями других пользователей.
export function canManageUsers(role: Role): boolean {
  return role === "admin";
}

// RBAC матрица ответственностей по ролям (используется в /roles странице).
export type RoleCapability = {
  key: string;
  label: string;
  admin: boolean;
  organizer: boolean | "own"; // "own" = только для своих мероприятий
  staff: boolean;
  applicant: boolean;
};

export const ROLE_CAPABILITIES: RoleCapability[] = [
  { key: "dashboard",       label: "Дашборд и статистика",           admin: true,  organizer: true,  staff: false, applicant: false },
  { key: "events_list",     label: "Список мероприятий",              admin: true,  organizer: true,  staff: false, applicant: false },
  { key: "events_create",   label: "Создание мероприятий",            admin: true,  organizer: true,  staff: false, applicant: false },
  { key: "events_edit",     label: "Редактирование мероприятий",      admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "events_toggle",   label: "Открыть / закрыть регистрацию",   admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "participants",    label: "Просмотр участников",             admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "mark_attended",   label: "Ручная отметка посещения",        admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "broadcast",       label: "Рассылка уведомлений",            admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "export_csv",      label: "Экспорт CSV участников",          admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "audit_log",       label: "Журнал действий",                 admin: true,  organizer: "own", staff: false, applicant: false },
  { key: "unmask_pii",      label: "Раскрытие ФИО / контактов (ПДн)", admin: true,  organizer: false, staff: false, applicant: false },
  { key: "checkin_qr",      label: "QR-сканер check-in на входе",     admin: true,  organizer: false, staff: true,  applicant: false },
  { key: "manage_users",    label: "Управление ролями пользователей",  admin: true,  organizer: false, staff: false, applicant: false },
];

export type EventStats = {
  capacity: number;
  registered: number;
  cancelled: number;
  waitlist: number;
  attended: number;
  no_show: number;
  free_seats: number;
  top_interests: Record<string, number> | null;
};

export type EventDetail = {
  event: EventDTO;
  stats: EventStats | null;
};

export type Registration = {
  id: number;
  user_id: number;
  event_id: number;
  status: string;
  full_name_masked: string;
  full_name?: string;
  contact_masked: string;
  contact?: string;
  source: string;
  registered_at?: string;
  checkin_at?: string;
  interest_program?: string;
  attendance_code?: string;
};

export type LookupByCodeResp = {
  registration: Registration;
  event: EventDTO;
};

export type ListEventsResp = {
  events: EventDTO[];
  total: number;
};

export type ParticipantsResp = {
  items: Registration[];
  total: number;
};

export type DashboardResp = {
  total_events: number;
  total_registered: number;
  upcoming_events: number;
};

export type CheckinResp = {
  already_done: boolean;
  registration: Registration;
  event: EventDTO;
};

export type BroadcastResp = {
  sent: number;
};

// --- P1: Audit log / Manual mark / Users ---

export type AuditLogEntry = {
  id: number;
  action: string;
  created_at: string;
  actor_user_id?: number;
  target_user_id?: number;
  registration_id?: number;
  event_id?: number;
  payload?: Record<string, unknown>;
};

export type AuditLogResp = {
  items: AuditLogEntry[];
  total: number;
};

// actionLabel — короткая русская подпись типа действия для audit log.
export function actionLabel(a: string): string {
  switch (a) {
    case "registration_created":
      return "Регистрация создана";
    case "registration_cancelled_by_user":
      return "Отменено участником";
    case "registration_cancelled_by_organizer":
      return "Отменено организатором";
    case "waitlist_added":
      return "Добавлен в waitlist";
    case "waitlist_promoted":
      return "Перенесён из waitlist";
    case "notification_sent":
      return "Уведомление отправлено";
    case "event_closed":
      return "Событие закрыто";
    case "event_opened":
      return "Событие открыто";
    case "event_created":
      return "Событие создано";
    case "event_updated":
      return "Событие изменено";
    case "ai_recommendation_shown":
      return "AI-подбор показан";
    case "ai_notification_rewritten":
      return "AI улучшил уведомление";
    case "ai_summary_generated":
      return "AI: сводка сгенерирована";
    case "admin_login":
      return "Вход в админку";
    case "admin_logout":
      return "Выход из админки";
    case "marked_attended_manually":
      return "Отмечен вручную (пришёл)";
    case "marked_no_show_manually":
      return "Отмечен вручную (не пришёл)";
    case "user_role_changed":
      return "Роль пользователя изменена";
    case "participants_exported_csv":
      return "Экспортирован CSV";
    case "pii_unmasked":
      return "Раскрыты ПДн";
    case "checkin_scanned":
      return "QR-скан check-in";
    case "consent_granted":
      return "Согласие выдано";
    case "forget_me":
      return "Удалён по «забыть»";
    default:
      return a;
  }
}

export type UserListItem = {
  id: number;
  max_user_id: number;
  role: Role;
  full_name?: string;
  phone_masked?: string;
  email_masked?: string;
  consent_at?: string;
  created_at: string;
};

export type UserListResp = {
  items: UserListItem[];
  total: number;
};
