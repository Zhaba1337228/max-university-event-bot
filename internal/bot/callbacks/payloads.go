// Package callbacks определяет формат payload'ов inline-кнопок MAX-бота.
//
// Формат: "group:action:arg1:arg2:..." (разделитель ":"),
// длина payload'а ≤ 1024 байт согласно лимиту MAX Bot API.
//
// Парсер симметричен конструкторам — переиспользуем константы и хелперы,
// чтобы не возникало рассинхрона между «отправили» и «получили». Каждый
// конструктор покрыт табличным тестом в payloads_test.go.
package callbacks

import (
	"strconv"
	"strings"
)

// Group — логическая группа payload'а. Маршрутизация Dispatcher идёт по Group.
const (
	GroupMain      = "main"
	GroupEvent     = "ev"
	GroupReg       = "reg"
	GroupMy        = "my"
	GroupCancel    = "cancel"
	GroupAI        = "ai"
	GroupWaitlist  = "wl"
	GroupOrg       = "org"
	GroupOrgList   = "orglist"
	GroupOrgNotif  = "orgnotif"
	GroupOrgClose  = "orgclose"
	GroupOrgCancel = "orgcancel"
	GroupAdmin     = "admin"
	GroupBack      = "back"
)

const (
	sep = ":"
)

// Payload — распарсенный callback. Args — позиционные аргументы как строки;
// для типизированного доступа есть ArgInt64.
type Payload struct {
	Raw    string
	Group  string
	Action string
	Args   []string
}

// Parse разбирает строку payload. Пустой / односложный вход даст пустые
// поля без ошибки — handler в этом случае уйдёт в fallback.
func Parse(raw string) Payload {
	p := Payload{Raw: raw}
	if raw == "" {
		return p
	}
	parts := strings.SplitN(raw, sep, 3)
	if len(parts) > 0 {
		p.Group = parts[0]
	}
	if len(parts) > 1 {
		p.Action = parts[1]
	}
	if len(parts) > 2 && parts[2] != "" {
		p.Args = strings.Split(parts[2], sep)
	}
	return p
}

// ArgInt64 безопасно достаёт i-й аргумент как int64.
// Если аргумента нет или он невалидный — возвращает 0; handler должен
// сам проверить результат на бизнес-уровне.
func (p Payload) ArgInt64(i int) int64 {
	if i < 0 || i >= len(p.Args) {
		return 0
	}
	v, err := strconv.ParseInt(p.Args[i], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// ArgString безопасно достаёт i-й аргумент как строку.
func (p Payload) ArgString(i int) string {
	if i < 0 || i >= len(p.Args) {
		return ""
	}
	return p.Args[i]
}

// build собирает payload «group:action[:args...]». Используется конструкторами.
func build(group, action string, args ...string) string {
	if len(args) == 0 {
		return group + sep + action
	}
	return group + sep + action + sep + strings.Join(args, sep)
}

// --- Main menu / навигация ---

// MainMenu возвращает payload главного меню.
func MainMenu() string { return build(GroupMain, "menu") }

// BackTo возвращает payload «назад в состояние state».
func BackTo(state string) string { return build(GroupBack, state) }

// --- События ---

// EventListPage — открыть страницу списка с offset.
func EventListPage(offset int) string { return build(GroupEvent, "list", itoa(offset)) }

// EventShow — открыть карточку события.
func EventShow(id int64) string { return build(GroupEvent, "show", i64(id)) }

// EventDetails — показать расширенную информацию (кнопка «Подробнее»).
func EventDetails(id int64) string { return build(GroupEvent, "details", i64(id)) }

// EventFilterSet — применить фильтр по формату (offline/online/hybrid/"" = все).
func EventFilterSet(format string) string { return build(GroupEvent, "filter", format) }

// EventFiltersOpen — открыть экран выбора фильтра.
func EventFiltersOpen() string { return build(GroupEvent, "filters_open") }

// EventFilterTime — фильтр по времени (today/week/"" = любое).
func EventFilterTime(period string) string { return build(GroupEvent, "filter_time", period) }

// EventFilterSeats — фильтр по наличию мест ("1" = только с местами, "" = все).
func EventFilterSeats(v string) string { return build(GroupEvent, "filter_seats", v) }

// EventFilterTag — фильтр по тегу (it/карьера/хакатон/поступление/"" = все).
func EventFilterTag(tag string) string { return build(GroupEvent, "filter_tag", tag) }

// EventFilterReset — сбросить все фильтры.
func EventFilterReset() string { return build(GroupEvent, "filter_reset") }

// --- Регистрация ---

// RegStart — начать запись на событие eventID.
func RegStart(eventID int64) string { return build(GroupReg, "start", i64(eventID)) }

// RegConsentYes — пользователь дал согласие на обработку ПДн.
func RegConsentYes() string { return build(GroupReg, "consent_yes") }

// RegConsentNo — пользователь отказался.
func RegConsentNo() string { return build(GroupReg, "consent_no") }

// RegConfirm — подтверждение регистрации в финальном шаге.
func RegConfirm() string { return build(GroupReg, "confirm") }

// RegEdit — пользователь хочет изменить введённые данные.
func RegEdit() string { return build(GroupReg, "edit") }

// RegCancelDraft — отменить незавершённую регистрацию.
func RegCancelDraft() string { return build(GroupReg, "cancel") }

// --- Моя запись / история ---

// MyShow — показать «мою запись».
func MyShow() string { return build(GroupMy, "show") }

// MyHistory — показать историю действий.
func MyHistory() string { return build(GroupMy, "history") }

// MyShowQR — повторно показать QR-код существующей регистрации (День 15).
func MyShowQR(regID int64) string { return build(GroupMy, "qr", i64(regID)) }

// MyToggleNotif — включить/выключить уведомления по конкретной записи.
func MyToggleNotif(regID int64) string { return build(GroupMy, "toggle_notif", i64(regID)) }

// ForgetMeAsk — двухшаговое подтверждение /forget_me.
func ForgetMeAsk() string { return build(GroupMy, "forget_ask") }

// ForgetMeYes — пользователь подтвердил удаление.
func ForgetMeYes() string { return build(GroupMy, "forget_yes") }

// ForgetMeNo — отменить /forget_me.
func ForgetMeNo() string { return build(GroupMy, "forget_no") }

// --- Отмена записи ---

// CancelAsk — спросить подтверждение отмены.
func CancelAsk(regID int64) string { return build(GroupCancel, "ask", i64(regID)) }

// CancelYes — подтвердить отмену.
func CancelYes(regID int64) string { return build(GroupCancel, "yes", i64(regID)) }

// CancelNo — отменить отмену :)
func CancelNo(regID int64) string { return build(GroupCancel, "no", i64(regID)) }

// --- AI ---

// AIPickStart — пригласить пользователя описать интерес.
func AIPickStart() string { return build(GroupAI, "pick") }

// AIPage — показать страницу AI-рекомендаций с offset.
func AIPage(offset int) string { return build(GroupAI, "page", itoa(offset)) }

// AIFAQStart — начать FAQ-диалог.
func AIFAQStart() string { return build(GroupAI, "faq") }

// --- Waitlist ---

// WaitlistJoin — встать в лист ожидания на eventID.
func WaitlistJoin(eventID int64) string { return build(GroupWaitlist, "join", i64(eventID)) }

// WaitlistPromoteYes — подтвердить запись из листа ожидания.
func WaitlistPromoteYes(regID int64) string { return build(GroupWaitlist, "yes", i64(regID)) }

// WaitlistPromoteNo — отказаться от подтверждения.
func WaitlistPromoteNo(regID int64) string { return build(GroupWaitlist, "no", i64(regID)) }

// --- Organizer ---

// OrgEntry — вход в организаторское меню.
func OrgEntry() string { return build(GroupOrg, "entry") }

// OrgStats — статистика по eventID.
func OrgStats(eventID int64) string { return build(GroupOrg, "stats", i64(eventID)) }

// OrgAISummary — AI-сводка по eventID (День 16).
func OrgAISummary(eventID int64) string { return build(GroupOrg, "ai_summary", i64(eventID)) }

// OrgListParticipants — постраничный список участников.
func OrgListParticipants(eventID int64, offset int) string {
	return build(GroupOrgList, "show", i64(eventID), itoa(offset))
}

// OrgListExport — экспорт участников в CSV.
func OrgListExport(eventID int64) string { return build(GroupOrgList, "csv", i64(eventID)) }

// OrgListSearchCode — начать поиск участника по коду записи.
func OrgListSearchCode(eventID int64) string { return build(GroupOrgList, "search_code", i64(eventID)) }

// OrgNotifStart — начало рассылки по eventID.
func OrgNotifStart(eventID int64) string { return build(GroupOrgNotif, "start", i64(eventID)) }

// OrgNotifAIRewrite — улучшить текст рассылки через AI.
func OrgNotifAIRewrite() string { return build(GroupOrgNotif, "ai") }

// OrgNotifSend — отправить рассылку.
func OrgNotifSend() string { return build(GroupOrgNotif, "send") }

// OrgNotifCancel — отменить черновик рассылки.
func OrgNotifCancel() string { return build(GroupOrgNotif, "cancel") }

// OrgCloseAsk — подтверждение закрытия регистрации.
func OrgCloseAsk(eventID int64) string { return build(GroupOrgClose, "ask", i64(eventID)) }

// OrgCloseYes — подтвердить закрытие.
func OrgCloseYes(eventID int64) string { return build(GroupOrgClose, "yes", i64(eventID)) }

// OrgOpenAsk — открыть регистрацию снова.
func OrgOpenAsk(eventID int64) string { return build(GroupOrgClose, "open_ask", i64(eventID)) }

// OrgOpenYes — подтвердить открытие.
func OrgOpenYes(eventID int64) string { return build(GroupOrgClose, "open_yes", i64(eventID)) }

// OrgCancelAsk — запросить подтверждение отмены мероприятия.
func OrgCancelAsk(eventID int64) string { return build(GroupOrgCancel, "ask", i64(eventID)) }

// OrgCancelYes — подтвердить отмену мероприятия.
func OrgCancelYes(eventID int64) string { return build(GroupOrgCancel, "yes", i64(eventID)) }

// --- Admin ---

// AdminPromote — назначить пользователя organizer'ом.
func AdminPromote(maxUserID int64) string { return build(GroupAdmin, "promote", i64(maxUserID)) }

// AdminDemote — снять права organizer.
func AdminDemote(maxUserID int64) string { return build(GroupAdmin, "demote", i64(maxUserID)) }

// --- internal helpers ---

func i64(v int64) string { return strconv.FormatInt(v, 10) }
func itoa(v int) string  { return strconv.Itoa(v) }
