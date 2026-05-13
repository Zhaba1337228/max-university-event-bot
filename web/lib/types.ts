// DTO-типы, отзеркаленные с adminapi/handlers.go.
// Менять синхронно с бэком.

export type Role = "applicant" | "organizer" | "admin";

export type Me = {
  user: { id: number; role: Role };
};

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
  registered_at?: string;
  checkin_at?: string;
  interest_program?: string;
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
