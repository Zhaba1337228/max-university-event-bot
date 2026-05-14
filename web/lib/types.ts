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
};

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
  created_at?: string;
  registered_at?: string;
  cancelled_at?: string;
  checkin_at?: string;
  checkin_by?: number;
  waitlist_position?: number;
  interest_program?: string;
};

// Participant — detailed user info for the check-in result card.
// Returned by /api/checkin success + by `registration_*` 409 conflict bodies.
export type Participant = {
  id: number;
  max_user_id: number;
  role: Role;
  full_name?: string;
  phone?: string;
  email?: string;
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

// CheckinResp — успешный 200 ответ. Поля participant/scanner добавлены в PR
// «detailed check-in card»: волонтёру нужны реальные телефон/email и кто
// именно отметил (для повторных сканов).
export type CheckinResp = {
  already_done: boolean;
  registration: Registration;
  event: EventDTO;
  participant?: Participant;
  scanner?: Participant;
};

// CheckinErrorBody — 409 Conflict с подробной карточкой, если регистрация нашлась,
// но её нельзя отметить (отменена / waitlist / no_show / окно закрыто).
export type CheckinErrorBody = {
  error: string;
  message: string;
  registration?: Registration;
  event?: EventDTO;
  participant?: Participant;
};

export type BroadcastResp = {
  sent: number;
};
