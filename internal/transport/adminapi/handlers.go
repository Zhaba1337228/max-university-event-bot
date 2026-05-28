package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// --- AUTH ---

// handleAuthExchange — POST /api/auth/exchange { "t": "<magic-jwt>" }.
// Парсит magic, выдаёт session-cookie, возвращает 204.
func (s *Server) handleAuthExchange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"t"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_body", "Невалидный JSON"))
		return
	}
	if body.Token == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_token", "Поле t обязательно"))
		return
	}

	magic, err := s.auth.VerifyMagic(body.Token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("invalid_magic", "Магическая ссылка недействительна или истекла"))
		return
	}

	session, err := s.auth.IssueSession(r.Context(), magic.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, errResp("access_denied", "Доступ запрещён"))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // на проде только HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.auth.SessionTTL().Seconds()),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleAuthLogout — POST /api/auth/logout. Очищает cookie.
func (s *Server) handleAuthLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleAuthMe — GET /api/auth/me. Возвращает текущего пользователя.
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":   c.UserID,
			"role": string(c.Role),
		},
	})
}

// --- EVENTS ---

// handleListEvents — GET /api/events?status=open&limit=50&offset=0.
// На MVP возвращает только open-события. status=mine — события организатора.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	q := r.URL.Query()
	statusF := q.Get("status")

	if statusF == "mine" {
		evs, err := s.deps.Events.ListByOrganizer(r.Context(), c.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"events": eventsToDTO(evs),
			"total":  len(evs),
		})
		return
	}

	limit := parseIntDefault(q.Get("limit"), 50, 1, 100)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1000000)
	items, total, err := s.deps.Events.ListOpen(r.Context(), limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		dto := eventToDTO(it.Event)
		dto["free_seats"] = it.FreeSeats
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out, "total": total})
}

// handleGetEvent — GET /api/events/:id. Возвращает событие + статистику.
func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}
	ev, err := s.deps.Events.Get(r.Context(), id)
	if err != nil || ev == nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "Событие не найдено"))
		return
	}
	stats, _ := s.deps.Events.Stats(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{
		"event": eventToDTO(ev),
		"stats": statsToDTO(stats),
	})
}

// handleListParticipants — GET /api/events/:id/participants?limit=50&offset=0&q=.
func (s *Server) handleListParticipants(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}

	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 50, 1, 200)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1000000)
	searchQ := strings.TrimSpace(q.Get("q"))

	regs, err := s.deps.RegsRepo.ListByEvent(r.Context(), s.deps.DB, id, domain.RegStatusRegistered, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	total, _ := s.deps.RegsRepo.CountByEvent(r.Context(), s.deps.DB, id, domain.RegStatusRegistered)

	items := make([]map[string]any, 0, len(regs))
	for _, reg := range regs {
		if searchQ != "" && !strings.Contains(strings.ToLower(reg.FullNameSnapshot), strings.ToLower(searchQ)) &&
			!strings.Contains(strings.ToLower(reg.ContactSnapshot), strings.ToLower(searchQ)) {
			continue
		}
		items = append(items, registrationToDTO(reg, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

// eventInputDTO — JSON-схема для POST /api/events и PATCH /api/events/{id}.
// Принимаем только то, что разрешено редактировать админкой. cancelled/finished
// — внутренние статусы, через эту форму проставить нельзя.
type eventInputDTO struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	StartsAt          string   `json:"starts_at"`         // RFC3339
	EndsAt            string   `json:"ends_at,omitempty"` // RFC3339, optional
	Location          string   `json:"location"`
	Format            string   `json:"format"` // offline|online|hybrid
	Capacity          int      `json:"capacity"`
	Status            string   `json:"status,omitempty"` // open|closed (только для update)
	Tags              []string `json:"tags"`
	LateCancelAllowed bool     `json:"late_cancel_allowed,omitempty"` // TZ §5
}

// parseEventInput валидирует и переводит DTO в service.EventInput.
// Все «слабые» ошибки (формат даты, неизвестный format) дают 400 с
// полем error="bad_input"; бизнес-валидация — внутри service.Event.
func parseEventInput(body io.ReadCloser, allowStatus bool) (service.EventInput, error) {
	var dto eventInputDTO
	if err := json.NewDecoder(body).Decode(&dto); err != nil {
		return service.EventInput{}, fmt.Errorf("bad_body: %w", err)
	}
	in := service.EventInput{
		Title:             dto.Title,
		Description:       dto.Description,
		Location:          dto.Location,
		Format:            domain.EventFormat(dto.Format),
		Capacity:          dto.Capacity,
		Tags:              dto.Tags,
		LateCancelAllowed: dto.LateCancelAllowed,
	}
	startsAt, err := time.Parse(time.RFC3339, dto.StartsAt)
	if err != nil {
		return service.EventInput{}, fmt.Errorf("bad_starts_at: %w", err)
	}
	in.StartsAt = startsAt
	if strings.TrimSpace(dto.EndsAt) != "" {
		endsAt, err := time.Parse(time.RFC3339, dto.EndsAt)
		if err != nil {
			return service.EventInput{}, fmt.Errorf("bad_ends_at: %w", err)
		}
		in.EndsAt = &endsAt
	}
	if allowStatus && dto.Status != "" {
		in.Status = domain.EventStatus(dto.Status)
	}
	return in, nil
}

// handleEventCreate — POST /api/events.
//
// Доступ: только organizer и admin. Создаёт мероприятие с created_by = текущий
// пользователь. Возвращает 201 и {"event": {...}}.
func (s *Server) handleEventCreate(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	if c.Role != domain.RoleOrganizer && c.Role != domain.RoleAdmin {
		writeJSON(w, http.StatusForbidden, errResp("role_forbidden", "Создавать мероприятия могут только организатор и админ"))
		return
	}
	in, err := parseEventInput(http.MaxBytesReader(w, r.Body, 16*1024), false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_input", "Некорректный JSON: "+err.Error()))
		return
	}
	ev, err := s.deps.Events.Create(r.Context(), c.UserID, in)
	if err != nil {
		writeServiceErr(w, err, s.log, "event create")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"event": eventToDTO(ev)})
}

// handleEventUpdate — PATCH /api/events/:id.
//
// Доступ: organizer-владелец события или admin. Меняет все поля, кроме
// служебных (CreatedBy, CreatedAt). Поле status принимается только open|closed.
func (s *Server) handleEventUpdate(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}
	in, err := parseEventInput(http.MaxBytesReader(w, r.Body, 16*1024), true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_input", "Некорректный JSON: "+err.Error()))
		return
	}
	ev, err := s.deps.Events.Update(r.Context(), c.UserID, id, in)
	if err != nil {
		writeServiceErr(w, err, s.log, "event update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": eventToDTO(ev)})
}

// writeServiceErr — единая маппинг доменных ошибок event admin в HTTP.
func writeServiceErr(w http.ResponseWriter, err error, log *slog.Logger, op string) {
	switch {
	case errors.Is(err, service.ErrEventInvalidTitle):
		writeJSON(w, http.StatusBadRequest, errResp("bad_title", "Название пустое или длиннее 255 символов"))
	case errors.Is(err, service.ErrEventInvalidDescription):
		writeJSON(w, http.StatusBadRequest, errResp("bad_description", "Описание слишком длинное (макс 16000 символов)"))
	case errors.Is(err, service.ErrEventInvalidDates):
		writeJSON(w, http.StatusBadRequest, errResp("bad_dates", "Дата начала в прошлом или окончание не позже начала"))
	case errors.Is(err, service.ErrEventInvalidCapacity):
		writeJSON(w, http.StatusBadRequest, errResp("bad_capacity", "Вместимость должна быть от 1 до 100000"))
	case errors.Is(err, service.ErrEventInvalidFormat):
		writeJSON(w, http.StatusBadRequest, errResp("bad_format", "Формат: offline / online / hybrid"))
	case errors.Is(err, service.ErrEventInvalidStatus):
		writeJSON(w, http.StatusBadRequest, errResp("bad_status", "Статус: open или closed"))
	case errors.Is(err, service.ErrEventTooManyTags):
		writeJSON(w, http.StatusBadRequest, errResp("bad_tags", "До 20 тегов, каждый <= 50 символов"))
	case errors.Is(err, service.ErrEventNotFound):
		writeJSON(w, http.StatusNotFound, errResp("not_found", "Событие не найдено"))
	default:
		log.Error(op+" failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Внутренняя ошибка"))
	}
}

// handleEventClose — POST /api/events/:id/close.
func (s *Server) handleEventClose(w http.ResponseWriter, r *http.Request) {
	s.handleEventUpdateStatus(w, r, domain.EventStatusClosed)
}

// handleEventOpen — POST /api/events/:id/open.
func (s *Server) handleEventOpen(w http.ResponseWriter, r *http.Request) {
	s.handleEventUpdateStatus(w, r, domain.EventStatusOpen)
}

// handleEventCancel — POST /api/events/:id/cancel.
// Отменяет мероприятие и рассылает уведомление всем зарегистрированным участникам.
// Доступ: organizer-owner или admin.
func (s *Server) handleEventCancel(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}

	ev, err := s.deps.Events.Get(r.Context(), id)
	if err != nil || ev == nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "Событие не найдено"))
		return
	}
	if ev.Status == domain.EventStatusCancelled {
		writeJSON(w, http.StatusConflict, errResp("already_cancelled", "Мероприятие уже отменено"))
		return
	}

	if err := s.deps.EventsRepo.UpdateStatus(r.Context(), s.deps.DB, id, domain.EventStatusCancelled); err != nil {
		s.log.Error("cancel event failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка обновления"))
		return
	}

	// Рассылка участникам.
	sent := 0
	if s.deps.Notification != nil {
		text := fmt.Sprintf("Мероприятие «%s» было отменено организаторами. Приносим извинения за неудобства.", ev.Title)
		sent, _ = s.deps.Notification.SendBroadcast(r.Context(), id, text)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event":          eventToDTO(ev),
		"notified_count": sent,
	})
}

func (s *Server) handleEventUpdateStatus(w http.ResponseWriter, r *http.Request, status domain.EventStatus) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}
	if err := s.deps.EventsRepo.UpdateStatus(r.Context(), s.deps.DB, id, status); err != nil {
		s.log.Error("update status failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка обновления"))
		return
	}
	ev, _ := s.deps.Events.Get(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{"event": eventToDTO(ev)})
}

// handleBroadcast — POST /api/events/:id/broadcast { "text": "..." }.
func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_body", "Невалидный JSON"))
		return
	}
	body.Text = strings.TrimSpace(body.Text)
	if body.Text == "" || len(body.Text) > 4000 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_text", "Текст пустой или длиннее 4000 символов"))
		return
	}

	sent, err := s.deps.Notification.SendBroadcast(r.Context(), id, body.Text)
	if err != nil {
		s.log.Error("broadcast failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("send_failed", "Не удалось отправить"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent})
}

// --- REGISTRATION LOOKUP ---

// handleLookupByCode — GET /api/registrations/by-code?code=<attendance_code>.
// Позволяет организатору и staff быстро найти запись по коду (без сканирования QR).
// Не отмечает участника пришедшим — только возвращает данные.
func (s *Server) handleLookupByCode(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_code", "Параметр code обязателен"))
		return
	}

	reg, err := s.deps.RegsRepo.GetByCode(r.Context(), s.deps.DB, code)
	if err != nil {
		s.log.Error("lookup by code failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	if reg == nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "Запись с таким кодом не найдена"))
		return
	}

	// Подгружаем событие для контекста.
	ev, _ := s.deps.Events.Get(r.Context(), reg.EventID)

	writeJSON(w, http.StatusOK, map[string]any{
		"registration": registrationToDTO(reg, false),
		"event":        eventToDTO(ev),
	})
}

// --- CHECK-IN (День 15) ---

// handleCheckin — POST /api/checkin { "qr": "MAXUEB:<eventID>:<code>" }.
//
// AttendanceService.CheckIn принимает scannerMaxUserID, потому что внутри
// он зовёт Role.RequireStaff. У нас в JWT — local user_id, поэтому
// делаем один доп. lookup через UsersRepo.GetByID → MaxUserID.
// Organizer-овнер события НЕ имеет права сканировать QR — вернётся 403.
func (s *Server) handleCheckin(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())

	var body struct {
		QR string `json:"qr"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_body", "Невалидный JSON"))
		return
	}
	body.QR = strings.TrimSpace(body.QR)
	if body.QR == "" {
		writeJSON(w, http.StatusBadRequest, errResp("missing_qr", "qr обязателен"))
		return
	}

	usr, err := s.deps.UsersRepo.GetByID(r.Context(), s.deps.DB, c.UserID)
	if err != nil || usr == nil {
		writeJSON(w, http.StatusInternalServerError, errResp("lookup", "Не удалось определить пользователя"))
		return
	}

	// Pre-check по роли из JWT: организатор НЕ приходит сюда (и НЕ сканерит гостей).
	// Staff/admin — пускаем. Applicant в admin API не попадёт (auth жёстко режет).
	if c.Role != domain.RoleStaff && c.Role != domain.RoleAdmin {
		writeJSON(w, http.StatusForbidden, errResp("role_forbidden", "Эта страница доступна только волонтёрам на входе (staff)"))
		return
	}

	res, err := s.deps.Attendance.CheckIn(r.Context(), usr.MaxUserID, body.QR)
	switch {
	case errors.Is(err, service.ErrQRInvalidPrefix), errors.Is(err, service.ErrQRInvalidFormat):
		writeJSON(w, http.StatusBadRequest, errResp("bad_qr", "Некорректный QR-код"))
		return
	case errors.Is(err, service.ErrQRTampered):
		writeJSON(w, http.StatusBadRequest, errResp("qr_tampered", "QR подделан или сгенерирован чужим сервером"))
		return
	case errors.Is(err, service.ErrQRExpired):
		writeJSON(w, http.StatusBadRequest, errResp("qr_expired", "Срок действия QR истёк"))
		return
	case errors.Is(err, service.ErrNotRegistered):
		writeJSON(w, http.StatusNotFound, errResp("not_registered", "Регистрация не найдена или неактивна"))
		return
	case errors.Is(err, service.ErrEventNotFound):
		writeJSON(w, http.StatusNotFound, errResp("event_not_found", "Событие не найдено"))
		return
	case errors.Is(err, service.ErrNotStaff), errors.Is(err, service.ErrNotOrganizer), errors.Is(err, service.ErrNotEventOwner):
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет прав на check-in"))
		return
	case errors.Is(err, service.ErrCheckinWindowClosed):
		writeJSON(w, http.StatusConflict, errResp("window_closed", "Окно check-in закрыто"))
		return
	case err != nil:
		s.log.Error("checkin failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp("internal", "Внутренняя ошибка"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"already_done": res.AlreadyDone,
		"registration": registrationToDTO(res.Registration, true),
		"event":        eventToDTO(res.Event),
	})
}

// --- DASHBOARD ---

// handleDashboard — GET /api/dashboard. Сводка по своим мероприятиям.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	evs, err := s.deps.Events.ListByOrganizer(r.Context(), c.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	totalEvents := len(evs)
	totalRegistered := 0
	upcoming := 0
	now := time.Now()
	for _, e := range evs {
		st, _ := s.deps.Events.Stats(r.Context(), e.ID)
		if st != nil {
			totalRegistered += st.Registered
		}
		if e.StartsAt.After(now) {
			upcoming++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_events":     totalEvents,
		"total_registered": totalRegistered,
		"upcoming_events":  upcoming,
	})
}

// handleEventDelete — DELETE /api/events/:id.
// Доступ: только владелец или admin.
func (s *Server) handleEventDelete(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, id) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}
	if err := s.deps.EventsRepo.Delete(r.Context(), s.deps.DB, id); err != nil {
		s.log.Error("delete event failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка удаления"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// ownsEvent — проверка ownership через прямой Get + сравнение CreatedBy.
// Avoid дёргать Role.RequireEventOwner с max_user_id=0 — в JWT у нас local id.
func ownsEvent(r *http.Request, s *Server, localUserID int64, role domain.Role, eventID int64) bool {
	if role == domain.RoleAdmin {
		return true
	}
	ev, err := s.deps.Events.Get(r.Context(), eventID)
	if err != nil || ev == nil {
		return false
	}
	return ev.CreatedBy != nil && *ev.CreatedBy == localUserID
}

func parseIntDefault(s string, def, lo, hi int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func eventsToDTO(evs []*domain.Event) []map[string]any {
	out := make([]map[string]any, 0, len(evs))
	for _, e := range evs {
		out = append(out, eventToDTO(e))
	}
	return out
}

func eventToDTO(e *domain.Event) map[string]any {
	if e == nil {
		return nil
	}
	dto := map[string]any{
		"id":                  e.ID,
		"title":               e.Title,
		"description":         e.Description,
		"starts_at":           e.StartsAt.UTC().Format(time.RFC3339),
		"location":            e.Location,
		"format":              string(e.Format),
		"capacity":            e.Capacity,
		"status":              string(e.Status),
		"tags":                e.Tags,
		"late_cancel_allowed": e.LateCancelAllowed,
	}
	if e.EndsAt != nil {
		dto["ends_at"] = e.EndsAt.UTC().Format(time.RFC3339)
	}
	if e.ShortSummary != nil {
		dto["short_summary"] = *e.ShortSummary
	}
	if e.CreatedBy != nil {
		dto["created_by"] = *e.CreatedBy
	}
	return dto
}

func statsToDTO(s *domain.EventStats) map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"capacity":      s.Capacity,
		"registered":    s.Registered,
		"cancelled":     s.Cancelled,
		"waitlist":      s.Waitlist,
		"attended":      s.Attended,
		"no_show":       s.NoShow,
		"free_seats":    s.FreeSeats,
		"top_interests": s.TopInterests,
	}
}

// registrationToDTO — DTO записи. unmask=true показывает контакт целиком
// (только для check-in результата); по умолчанию маскируем.
func registrationToDTO(r *domain.Registration, unmask bool) map[string]any {
	dto := map[string]any{
		"id":               r.ID,
		"user_id":          r.UserID,
		"event_id":         r.EventID,
		"status":           string(r.Status),
		"full_name_masked": maskFullName(r.FullNameSnapshot),
		"contact_masked":   maskContactDTO(r.ContactSnapshot),
		"source":           r.Source,
	}
	if r.RegisteredAt != nil {
		dto["registered_at"] = r.RegisteredAt.UTC().Format(time.RFC3339)
	}
	if r.CheckinAt != nil {
		dto["checkin_at"] = r.CheckinAt.UTC().Format(time.RFC3339)
	}
	if r.InterestProgram != nil {
		dto["interest_program"] = *r.InterestProgram
	}
	if unmask {
		dto["full_name"] = r.FullNameSnapshot
		dto["contact"] = r.ContactSnapshot
	}
	return dto
}

func maskFullName(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "***"
	}
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if i == 0 || len(p) == 0 {
			out = append(out, p)
			continue
		}
		out = append(out, string([]rune(p)[0])+".")
	}
	return strings.Join(out, " ")
}

func maskContactDTO(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "***"
	}
	if i := strings.Index(s, "@"); i > 0 {
		head := s[:i]
		if len(head) > 2 {
			head = head[:2] + "***"
		}
		return head + s[i:]
	}
	if len(s) >= 4 {
		return s[:2] + "***" + s[len(s)-2:]
	}
	return "***"
}
