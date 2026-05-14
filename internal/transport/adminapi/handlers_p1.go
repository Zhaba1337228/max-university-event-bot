package adminapi

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// --- AUDIT LOG ---

// handleEventAuditLog — GET /api/events/:id/actions?limit=50.
// Доступ: organizer-owner или admin.
func (s *Server) handleEventAuditLog(w http.ResponseWriter, r *http.Request) {
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

	limit := parseIntDefault(r.URL.Query().Get("limit"), 50, 1, 200)
	logs, err := s.deps.ActionLogs.ListByEvent(r.Context(), id, limit)
	if err != nil {
		s.log.Error("list event actions failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	items := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		items = append(items, actionLogToDTO(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func actionLogToDTO(l *domain.ActionLog) map[string]any {
	dto := map[string]any{
		"id":         l.ID,
		"action":     string(l.Action),
		"created_at": l.CreatedAt.UTC().Format(time.RFC3339),
	}
	if l.ActorUserID != nil {
		dto["actor_user_id"] = *l.ActorUserID
	}
	if l.TargetUserID != nil {
		dto["target_user_id"] = *l.TargetUserID
	}
	if l.RegistrationID != nil {
		dto["registration_id"] = *l.RegistrationID
	}
	if l.EventID != nil {
		dto["event_id"] = *l.EventID
	}
	if len(l.Payload) > 0 && string(l.Payload) != "{}" {
		// Payload — json.RawMessage; отдаём как уже распарсенное.
		var parsed any
		if err := json.Unmarshal(l.Payload, &parsed); err == nil {
			dto["payload"] = parsed
		}
	}
	return dto
}

// --- CSV EXPORT ---

// handleExportParticipantsCSV — GET /api/events/:id/participants.csv.
// Возвращает text/csv с UTF-8 BOM. Включает ВСЕ статусы (registered, waitlist,
// attended, no_show, cancelled). Audit_log пишет факт экспорта (compliance).
func (s *Server) handleExportParticipantsCSV(w http.ResponseWriter, r *http.Request) {
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

	regs, err := s.deps.RegsRepo.ListByEventAllStatuses(r.Context(), s.deps.DB, id, 10000, 0)
	if err != nil {
		s.log.Error("csv: list failed", "err", err, "event_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}

	// Заголовки до записи в тело, чтобы браузер сразу скачал файл.
	filename := fmt.Sprintf("participants_event_%d.csv", id)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	// UTF-8 BOM для Excel.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{
		"id", "user_id", "status", "full_name", "contact",
		"interest_program", "registered_at", "cancelled_at",
		"checkin_at", "checkin_by", "source",
	})
	for _, reg := range regs {
		interest := ""
		if reg.InterestProgram != nil {
			interest = *reg.InterestProgram
		}
		registeredAt := ""
		if reg.RegisteredAt != nil {
			registeredAt = reg.RegisteredAt.UTC().Format(time.RFC3339)
		}
		cancelledAt := ""
		if reg.CancelledAt != nil {
			cancelledAt = reg.CancelledAt.UTC().Format(time.RFC3339)
		}
		checkinAt := ""
		if reg.CheckinAt != nil {
			checkinAt = reg.CheckinAt.UTC().Format(time.RFC3339)
		}
		checkinBy := ""
		if reg.CheckinBy != nil {
			checkinBy = strconv.FormatInt(*reg.CheckinBy, 10)
		}
		_ = cw.Write([]string{
			strconv.FormatInt(reg.ID, 10),
			strconv.FormatInt(reg.UserID, 10),
			string(reg.Status),
			reg.FullNameSnapshot,
			reg.ContactSnapshot,
			interest,
			registeredAt,
			cancelledAt,
			checkinAt,
			checkinBy,
			reg.Source,
		})
	}

	// Audit-лог факта экспорта (compliance). Ошибку не возвращаем — CSV уже отдан.
	actor := c.UserID
	evID := id
	payload := []byte(fmt.Sprintf(`{"rows":%d}`, len(regs)))
	_ = s.deps.ActionLogsRepo.Append(r.Context(), s.deps.DB, &domain.ActionLog{
		ActorUserID: &actor,
		EventID:     &evID,
		Action:      domain.ActionParticipantsExported,
		Payload:     payload,
	})
}

// --- MANUAL MARK ---

// handleManualMark — POST /api/events/:id/registrations/:regID/mark
// { "status": "attended" | "no_show" }.
// Доступ: organizer-owner / admin (staff обходится без этого endpoint'а,
// у него есть страница check-in с QR-сканером).
func (s *Server) handleManualMark(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	eventID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || eventID <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id события"))
		return
	}
	regID, err := strconv.ParseInt(chi.URLParam(r, "regID"), 10, 64)
	if err != nil || regID <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_reg_id", "Некорректный id записи"))
		return
	}
	if !ownsEvent(r, s, c.UserID, c.Role, eventID) {
		writeJSON(w, http.StatusForbidden, errResp("forbidden", "Нет доступа"))
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_body", "Невалидный JSON"))
		return
	}
	st := domain.RegistrationStatus(strings.TrimSpace(body.Status))
	reg, err := s.deps.Attendance.ManualMark(r.Context(), c.UserID, eventID, regID, st)
	switch {
	case errors.Is(err, service.ErrManualMarkInvalidStatus):
		writeJSON(w, http.StatusBadRequest, errResp("bad_status", "Допустимы только attended или no_show"))
		return
	case errors.Is(err, service.ErrRegistrationNotFound):
		writeJSON(w, http.StatusNotFound, errResp("reg_not_found", "Регистрация не найдена"))
		return
	case errors.Is(err, service.ErrRegNotForEvent):
		writeJSON(w, http.StatusBadRequest, errResp("reg_event_mismatch", "Регистрация не принадлежит этому событию"))
		return
	case errors.Is(err, service.ErrRegNotActive):
		writeJSON(w, http.StatusConflict, errResp("reg_cancelled", "Эту регистрацию нельзя отметить — она отменена"))
		return
	case err != nil:
		s.log.Error("manual mark failed", "err", err, "event_id", eventID, "reg_id", regID)
		writeJSON(w, http.StatusInternalServerError, errResp("internal", "Внутренняя ошибка"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registration": registrationToDTO(reg, false),
	})
}

// --- USERS (admin only) ---

// handleListUsers — GET /api/users?role=&query=&limit=&offset=.
// Доступ: admin (middleware).
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	roleFilter := domain.Role(strings.TrimSpace(q.Get("role")))
	if roleFilter != "" && !isValidRoleParam(roleFilter) {
		writeJSON(w, http.StatusBadRequest, errResp("bad_role", "Неверная роль"))
		return
	}
	limit := parseIntDefault(q.Get("limit"), 50, 1, 200)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1000000)
	search := strings.TrimSpace(q.Get("query"))

	users, total, err := s.deps.Users.List(r.Context(), roleFilter, search, limit, offset)
	if err != nil {
		s.log.Error("list users failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp("db", "Ошибка чтения"))
		return
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, userToDTO(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

// handleSetUserRole — PATCH /api/users/:id/role { "role": "organizer" }.
// Доступ: admin (middleware).
func (s *Server) handleSetUserRole(w http.ResponseWriter, r *http.Request) {
	c, _ := claimsFromContext(r.Context())
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || targetID <= 0 {
		writeJSON(w, http.StatusBadRequest, errResp("bad_id", "Некорректный id пользователя"))
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("bad_body", "Невалидный JSON"))
		return
	}
	role := domain.Role(strings.TrimSpace(body.Role))
	u, err := s.deps.Users.SetRole(r.Context(), c.UserID, targetID, role)
	switch {
	case errors.Is(err, service.ErrUserInvalidRole):
		writeJSON(w, http.StatusBadRequest, errResp("bad_role", "Неверная роль (допустимо: applicant, organizer, staff, admin)"))
		return
	case errors.Is(err, service.ErrUserCannotChangeSelf):
		writeJSON(w, http.StatusBadRequest, errResp("self_change", "Нельзя менять собственную роль"))
		return
	case errors.Is(err, service.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, errResp("user_not_found", "Пользователь не найден"))
		return
	case err != nil:
		s.log.Error("set user role failed", "err", err, "target_id", targetID)
		writeJSON(w, http.StatusInternalServerError, errResp("internal", "Внутренняя ошибка"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToDTO(u)})
}

func isValidRoleParam(r domain.Role) bool {
	switch r {
	case domain.RoleApplicant, domain.RoleOrganizer, domain.RoleStaff, domain.RoleAdmin:
		return true
	}
	return false
}

func userToDTO(u *domain.User) map[string]any {
	if u == nil {
		return nil
	}
	dto := map[string]any{
		"id":          u.ID,
		"max_user_id": u.MaxUserID,
		"role":        string(u.Role),
		"created_at":  u.CreatedAt.UTC().Format(time.RFC3339),
	}
	if u.FullName != nil {
		dto["full_name"] = *u.FullName
	}
	// Контакты только в маске (admin может прочитать в БД, но в API не нужны без явного интента).
	if u.Phone != nil {
		dto["phone_masked"] = maskContactDTO(*u.Phone)
	}
	if u.Email != nil {
		dto["email_masked"] = maskContactDTO(*u.Email)
	}
	if u.ConsentAt != nil {
		dto["consent_at"] = u.ConsentAt.UTC().Format(time.RFC3339)
	}
	return dto
}
