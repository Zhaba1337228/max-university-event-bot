package messages

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/fsm"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// =============================================================================
// Главное меню и помощь
// =============================================================================

// Welcome — приветствие при /start. Если знаем имя пользователя — обращаемся.
// Содержит дисклеймер, обязательный по правилам платформы.
func Welcome(name string) string {
	disclaimer := joinLines(
		"Сервис разработан командой хакатона университета и не является",
		"официальной функцией платформы MAX.",
		"",
	)
	if name == "" {
		return disclaimer + joinLines(
			"Здравствуйте! Я помогу записаться на мероприятие университета.",
			"",
			"Что хотите сделать?",
		)
	}
	return disclaimer + fmt.Sprintf(
		"Здравствуйте, %s! Я помогу записаться на мероприятие университета.\n\nЧто хотите сделать?",
		name,
	)
}

// Help — текст команды /help.
func Help() string {
	return joinLines(
		"Сервис разработан командой хакатона университета. Не является официальной функцией MAX.",
		"",
		"Я умею:",
		"- показать список мероприятий и записать вас;",
		"- показать вашу запись и статус;",
		"- отменить запись;",
		"- подобрать мероприятие по вашему интересу (с помощью ИИ);",
		"- напомнить о мероприятии накануне и за час до начала.",
		"",
		"Команды:",
		"/start - вернуться в главное меню",
		"/help - помощь",
		"/forget_me - удалить все мои данные",
	)
}

// MainMenuPrompt — небольшая подсказка над кнопками главного меню.
func MainMenuPrompt() string {
	return "Главное меню. Выберите действие."
}

// =============================================================================
// 152-ФЗ: согласие и удаление данных
// =============================================================================

// ConsentAsk — текст запроса согласия на обработку ПДн.
// Версия документа фиксируется в users.consent_policy_ver.
func ConsentAsk(policyVer string) string {
	return joinLines(
		"Чтобы записать вас на мероприятие, необходимо согласие на обработку персональных данных.",
		"",
		"Что мы храним: ФИО — только чтобы оформить запись на мероприятие и подтвердить участие.",
		"",
		"Срок хранения: до 1 года или до отзыва согласия.",
		"",
		"Вы можете в любой момент удалить все свои данные командой /forget_me.",
		"",
		"Версия документа: "+policyVer,
	)
}

// ConsentDeclined — отказ.
func ConsentDeclined() string {
	return "Без согласия запись невозможна. Если передумаете - вернитесь к карточке мероприятия и нажмите «Записаться»."
}

// ConsentRecorded — успешная фиксация согласия.
func ConsentRecorded() string {
	return "Согласие сохранено. Продолжаем запись."
}

// ForgetMeAsk — подтверждение удаления данных.
func ForgetMeAsk() string {
	return "Удалить все ваши данные? Будут удалены: профиль, согласие, все ваши записи и история действий. Это действие необратимо."
}

// ForgetMeDone — после удаления.
func ForgetMeDone() string {
	return "Все ваши данные удалены. Если решите вернуться - просто отправьте /start."
}

// ForgetMeCancelled — пользователь отказался от удаления.
func ForgetMeCancelled() string {
	return "Удаление отменено. Ваши данные сохранены."
}

// =============================================================================
// Список мероприятий и карточка
// =============================================================================

// EventListEmpty — пусто.
func EventListEmpty() string {
	return "Пока нет открытых мероприятий. Загляните чуть позже."
}

// EventListHeader — заголовок над списком.
func EventListHeader() string {
	return "Доступные мероприятия:"
}

// EventListItem — одна строка в текстовом списке (если в кнопках не помещается весь заголовок).
// Содержит название, дату, формат и длительность (если известна).
func EventListItem(idx int, e *domain.Event) string {
	dur := ""
	if e.EndsAt != nil {
		d := e.EndsAt.Sub(e.StartsAt)
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if h > 0 && m > 0 {
			dur = fmt.Sprintf(", %d ч %d мин", h, m)
		} else if h > 0 {
			dur = fmt.Sprintf(", %d ч", h)
		} else {
			dur = fmt.Sprintf(", %d мин", m)
		}
	}
	return fmt.Sprintf("%d. %s — %s (%s%s)", idx+1, e.Title, FormatDateTime(e.StartsAt), HumanFormat(e.Format), dur)
}

// EventCard — карточка одного мероприятия.
func EventCard(e *domain.Event, freeSeats int, activeReg *domain.Registration) string {
	summary := e.Description
	if e.ShortSummary != nil && *e.ShortSummary != "" {
		summary = *e.ShortSummary
	}
	timeStr := FormatDateTime(e.StartsAt)
	if e.EndsAt != nil {
		timeStr += " — " + FormatTime(*e.EndsAt)
	}
	seatsLine := fmt.Sprintf("Свободно мест: %d из %d", freeSeats, e.Capacity)
	if freeSeats == 0 {
		seatsLine = "Мест нет"
	}
	lines := []string{
		e.Title,
		"",
		"Когда: " + timeStr,
		"Где: " + e.Location,
		"Формат: " + HumanFormat(e.Format),
		seatsLine,
	}
	if activeReg != nil {
		statusLine := "Статус: вы уже записаны"
		if activeReg.Status == domain.RegStatusWaitlist {
			statusLine = "Статус: вы в листе ожидания"
		}
		lines = append(lines, statusLine)
	}
	lines = append(lines, "", summary)
	return joinLines(lines...)
}

// EventDetails — расширенная карточка по кнопке «Подробнее».
func EventDetails(e *domain.Event) string {
	timeStr := FormatDateTime(e.StartsAt)
	if e.EndsAt != nil {
		timeStr += " — " + FormatTime(*e.EndsAt)
	}
	cancelPolicy := "до начала мероприятия"
	if e.LateCancelAllowed {
		cancelPolicy = "в любое время (в т.ч. после начала)"
	}
	lines := []string{
		e.Title,
		"",
		"Когда: " + timeStr,
		"Где: " + e.Location,
		"Формат: " + HumanFormat(e.Format),
		fmt.Sprintf("Мест всего: %d", e.Capacity),
		"",
		"Описание:",
		e.Description,
		"",
		"Условия отмены: " + cancelPolicy,
	}
	return joinLines(lines...)
}

// EventNotAvailable — карточка с несуществующим/закрытым id.
func EventNotAvailable() string {
	return "Мероприятие недоступно. Возможно, оно было удалено или регистрация закрыта."
}

// EventClosedNow — регистрация закрыта в момент подтверждения.
func EventClosedNow() string {
	return "К сожалению, регистрация на это мероприятие уже закрыта."
}

// =============================================================================
// FSM регистрации
// =============================================================================

// AskFullName — шаг reg_full_name.
func AskFullName() string {
	return "Введите ваше ФИО полностью (например: Иванов Иван Иванович)."
}

// AskFullNameWithSaved — шаг reg_full_name когда ФИО уже сохранено.
func AskFullNameWithSaved(name string) string {
	return joinLines(
		"Используем ваше сохранённое ФИО: «"+name+"».",
		"",
		"Какое направление вам интересно?",
	)
}

// InvalidFullName — валидация не прошла.
func InvalidFullName() string {
	return "Похоже на неполное ФИО. Пожалуйста, отправьте фамилию, имя и отчество одним сообщением."
}

// AskInterest — шаг reg_interest.
func AskInterest() string {
	return joinLines(
		"Какое направление вам интересно?",
		"",
		"Например: «Прикладная информатика», «Программная инженерия», «Информационная безопасность».",
		"",
		"Можно написать просто темой - бот учтёт это в подборе.",
	)
}

// RegConfirmation — финальный экран перед подтверждением.
func RegConfirmation(e *domain.Event, ctxFSM fsm.UserFSMContext) string {
	return joinLines(
		"Проверьте данные:",
		"",
		"Мероприятие: "+e.Title,
		"Когда: "+FormatDateTime(e.StartsAt),
		"Где: "+e.Location,
		"Формат: "+HumanFormat(e.Format),
		"",
		"ФИО: "+ctxFSM.DraftFullName,
		"Направление: "+ctxFSM.DraftInterest,
		"",
		"Запись можно будет отменить до начала мероприятия — место вернётся в пул.",
		"",
		"Всё верно?",
	)
}

// RegSuccess — после успешной записи (status=registered).
// attendanceCode — уникальный код записи для идентификации участника.
func RegSuccess(e *domain.Event, attendanceCode string) string {
	codeStr := ""
	if attendanceCode != "" {
		codeStr = "\nКод записи: " + domain.ShortAttendanceCode(attendanceCode)
	}
	return joinLines(
		"Вы записаны на мероприятие.",
		"",
		"Мероприятие: "+e.Title,
		"Когда: "+FormatDateTime(e.StartsAt),
		"Где: "+e.Location,
		"Статус: запись подтверждена."+codeStr,
		"",
		"QR-код приглашения будет отправлен отдельным сообщением — покажите его на входе.",
		"За день до мероприятия и за час до начала придёт напоминание.",
	)
}

// AlreadyRegistered — попытка повторной записи на то же событие.
func AlreadyRegistered() string {
	return "Вы уже записаны на это мероприятие. Откройте «Моя запись», чтобы увидеть детали."
}

// RegCancelledDraft — пользователь отменил процесс записи.
func RegCancelledDraft() string {
	return "Запись не сохранена."
}

// =============================================================================
// Waitlist
// =============================================================================

// WaitlistConfirmed — пользователь встал в лист ожидания (мест нет).
func WaitlistConfirmed(pos int) string {
	return fmt.Sprintf(
		"Свободных мест сейчас нет, но вы добавлены в лист ожидания.\n"+
			"Ваше место в очереди: %d.\n\n"+
			"Если освободится место, я сразу напишу с предложением подтвердить участие.",
		pos,
	)
}

// WaitlistPromotedAsk — освободилось место, спрашиваем подтверждение.
func WaitlistPromotedAsk(e *domain.Event) string {
	return fmt.Sprintf(
		"Освободилось место на мероприятии «%s» (%s).\n\nХотите подтвердить участие?",
		e.Title, FormatDateTime(e.StartsAt),
	)
}

// WaitlistPromotedConfirmed — пользователь подтвердил из waitlist.
func WaitlistPromotedConfirmed(e *domain.Event) string {
	return "Отлично! Вы записаны на «" + e.Title + "»."
}

// WaitlistPromotedDeclined — отказался.
func WaitlistPromotedDeclined() string {
	return "Понятно. Запись из листа ожидания не подтверждена. Если передумаете - откройте карточку мероприятия."
}

// =============================================================================
// «Моя запись» и отмена
// =============================================================================

// MyRegistration — карточка активной записи.
func MyRegistration(e *domain.Event, r *domain.Registration) string {
	lines := []string{
		"Ваша запись:",
		"",
		"Мероприятие: " + e.Title,
		"Когда: " + FormatDateTime(e.StartsAt),
		"Где: " + e.Location,
		"Статус: " + HumanStatus(r.Status),
	}
	if r.Status == domain.RegStatusWaitlist && r.WaitlistPosition != nil {
		lines = append(lines, fmt.Sprintf("Место в очереди: %d", *r.WaitlistPosition))
	}
	if r.AttendanceCode != nil && *r.AttendanceCode != "" {
		lines = append(lines, "Код записи: "+domain.ShortAttendanceCode(*r.AttendanceCode))
	}
	if r.InterestProgram != nil && *r.InterestProgram != "" {
		lines = append(lines, "Направление: "+*r.InterestProgram)
	}
	return joinLines(lines...)
}

// MyRegistrationsList — несколько записей одним сообщением.
func MyRegistrationsList(items []struct {
	Event *domain.Event
	Reg   *domain.Registration
}) string {
	if len(items) == 0 {
		return MyRegistrationEmpty()
	}
	parts := make([]string, 0, len(items))
	for i, it := range items {
		parts = append(parts, fmt.Sprintf("%d. %s (%s) - %s",
			i+1, it.Event.Title, FormatDateTime(it.Event.StartsAt), HumanStatus(it.Reg.Status)))
	}
	return joinLines(append([]string{"Ваши записи:", ""}, parts...)...)
}

// MyRegistrationEmpty — активных записей нет.
func MyRegistrationEmpty() string {
	return "У вас нет активных записей. Хотите выбрать мероприятие?"
}

// CancelAsk — двухшаговое подтверждение отмены.
func CancelAsk(e *domain.Event) string {
	return fmt.Sprintf(
		"Вы действительно хотите отменить запись?\n\nМероприятие: %s\nКогда: %s",
		e.Title, FormatDateTime(e.StartsAt),
	)
}

// CancelDone — успешная отмена пользователем.
func CancelDone() string {
	return "Запись отменена. Если планы изменятся - вы сможете записаться снова, если останутся места."
}

// CancelLateForbidden — попытка отмены после начала мероприятия, когда это запрещено.
func CancelLateForbidden() string {
	return "Мероприятие уже началось. Отмена записи после старта не предусмотрена правилами этого мероприятия."
}

// CancelLate — поздняя отмена разрешена правилами мероприятия.
func CancelLate() string {
	return "Запись отменена (поздняя отмена). Мероприятие уже началось, поэтому место может не вернуться в пул."
}

// =============================================================================
// QR check-in
// =============================================================================

// QRCaption — подпись к PNG с QR-кодом, который бот шлёт после записи.
func QRCaption(e *domain.Event, attendanceCode string) string {
	lines := []string{
		"Ваш QR-код для прохода на мероприятие.",
		"",
		"Мероприятие: " + e.Title,
		"Когда: " + FormatDateTime(e.StartsAt),
		"Где: " + e.Location,
	}
	if attendanceCode != "" {
		lines = append(lines, "Код записи: "+domain.ShortAttendanceCode(attendanceCode))
	}
	lines = append(lines,
		"",
		"Покажите этот QR-код организатору на входе.",
		"Если потеряете - используйте кнопку «Показать мой QR» в разделе «Моя запись».",
	)
	return joinLines(lines...)
}

// QRNotAvailable — попытка получить QR, когда нет активной записи.
func QRNotAvailable() string {
	return "QR-код доступен только при активной записи. Откройте «Моя запись» для проверки статуса."
}

// =============================================================================
// История действий
// =============================================================================

// HistoryEmpty — список пуст.
func HistoryEmpty() string {
	return "История пуста."
}

// HistoryHeader — заголовок списка.
func HistoryHeader() string {
	return "Ваши последние действия:"
}

// HistoryLine — одна строка истории.
func HistoryLine(log *domain.ActionLog) string {
	return fmt.Sprintf("%s - %s", FormatDateTime(log.CreatedAt), humanAction(log.Action))
}

func humanAction(a domain.ActionType) string {
	switch a {
	case domain.ActionRegistrationCreated:
		return "запись создана"
	case domain.ActionRegistrationCancelledUser:
		return "вы отменили запись"
	case domain.ActionRegistrationCancelledOrg:
		return "запись отменена организатором"
	case domain.ActionWaitlistAdded:
		return "добавлены в лист ожидания"
	case domain.ActionWaitlistPromoted:
		return "переведены из листа ожидания"
	case domain.ActionNotificationSent:
		return "получено уведомление"
	case domain.ActionConsentGranted:
		return "согласие на обработку ПДн получено"
	case domain.ActionForgetMe:
		return "удаление данных"
	case domain.ActionCheckinScanned:
		return "вход на мероприятие подтверждён"
	}
	return string(a)
}

// =============================================================================
// Напоминания
// =============================================================================

// ReminderText — текст напоминания за 24 ч / 1 ч.
func ReminderText(e *domain.Event, hoursBefore int) string {
	when := FormatDateTime(e.StartsAt)
	header := "Напоминание о мероприятии"
	if hoursBefore == 1 {
		header = "Через час начнётся мероприятие"
	} else if hoursBefore == 24 {
		header = "Завтра мероприятие, на которое вы записаны"
	}
	return joinLines(
		header,
		"",
		"Мероприятие: "+e.Title,
		"Когда: "+when,
		"Где: "+e.Location,
		"",
		"Не забудьте показать QR-код на входе.",
	)
}

// =============================================================================
// Организаторская часть
// =============================================================================

// NotifDisabledDone — уведомления по записи отключены.
func NotifDisabledDone() string {
	return "Уведомления по этому мероприятию отключены. Включить обратно можно через «Моя запись»."
}

// NotifEnabledDone — уведомления по записи включены.
func NotifEnabledDone() string {
	return "Уведомления по этому мероприятию включены."
}

// OrgSearchCodeAsk — приглашение ввести код записи для поиска.
func OrgSearchCodeAsk() string {
	return "Введите код записи участника (например: a1b2c3d4)."
}

// OrgSearchCodeNotFound — код не найден.
func OrgSearchCodeNotFound() string {
	return "Участник с таким кодом записи не найден."
}

// OrgSearchCodeResult — найденная запись.
func OrgSearchCodeResult(code, fullName, status string) string {
	return joinLines(
		"Результат поиска:",
		"",
		"Код: "+code,
		"Участник: "+fullName,
		"Статус: "+status,
	)
}

// OrganizerNoAccess — попытка зайти без роли organizer/admin.
func OrganizerNoAccess() string {
	return "Раздел доступен только организаторам."
}

// OrganizerMenu — приветствие в /organizer.
func OrganizerMenu() string {
	return "Меню организатора. Выберите действие."
}

// OrganizerNoEvents — у организатора нет своих мероприятий.
func OrganizerNoEvents() string {
	return "Нет мероприятий, которыми вы управляете."
}

// OrganizerStats — текстовая сводка по событию.
func OrganizerStats(e *domain.Event, s *domain.EventStats) string {
	top := make([]string, 0, len(s.TopInterests))
	// детерминированный порядок: по убыванию count, при равных — по имени
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(s.TopInterests))
	for k, v := range s.TopInterests {
		pairs = append(pairs, kv{k, v})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	for _, p := range pairs {
		top = append(top, fmt.Sprintf("- %s: %d", p.k, p.v))
	}
	if len(top) == 0 {
		top = append(top, "- пока нет данных")
	}

	return joinLines(
		e.Title,
		"",
		fmt.Sprintf("Всего мест: %d", s.Capacity),
		fmt.Sprintf("Записано: %d", s.Registered),
		fmt.Sprintf("Свободно: %d", s.FreeSeats),
		fmt.Sprintf("В листе ожидания: %d", s.Waitlist),
		fmt.Sprintf("Отменили запись: %d", s.Cancelled),
		fmt.Sprintf("Посетили: %d", s.Attended),
		"",
		"Топ интересов:",
		strings.Join(top, "\n"),
	)
}

// OrganizerAskNotifText — приглашение ввести текст рассылки.
func OrganizerAskNotifText() string {
	return "Напишите текст уведомления для участников. Длина до 4000 символов."
}

// OrganizerNotifPreview — предпросмотр перед отправкой.
func OrganizerNotifPreview(text string, recipients int) string {
	return joinLines(
		"Предпросмотр уведомления:",
		"",
		text,
		"",
		fmt.Sprintf("Отправить %d участникам?", recipients),
	)
}

// OrganizerNotifSent — успешная рассылка.
func OrganizerNotifSent(n int) string {
	return fmt.Sprintf("Отправлено %d сообщений.", n)
}

// OrganizerNotifCancelled — отмена черновика рассылки.
func OrganizerNotifCancelled() string {
	return "Рассылка отменена. Текст не отправлен."
}

// OrganizerCloseAsk — двухшаговое подтверждение закрытия регистрации.
func OrganizerCloseAsk(e *domain.Event) string {
	return joinLines(
		"Закрыть регистрацию на «"+e.Title+"»?",
		"",
		"После закрытия новые записи приниматься не будут. Лист ожидания тоже становится недоступным.",
	)
}

// OrganizerClosed — успешное закрытие.
func OrganizerClosed() string {
	return "Регистрация закрыта."
}

// OrganizerOpened — обратное действие.
func OrganizerOpened() string {
	return "Регистрация снова открыта."
}

// OrganizerCancelAsk — подтверждение отмены мероприятия.
func OrganizerCancelAsk(e *domain.Event) string {
	return joinLines(
		"Отменить мероприятие «"+e.Title+"»?",
		"",
		"Все записанные участники получат уведомление об отмене.",
		"Это действие необратимо.",
	)
}

// OrganizerEventCancelled — мероприятие успешно отменено.
func OrganizerEventCancelled(sent int) string {
	return fmt.Sprintf("Мероприятие отменено. Уведомления отправлены %d участникам.", sent)
}

// EventCancelledByOrg — сообщение участникам при отмене мероприятия.
func EventCancelledByOrg(title string) string {
	return joinLines(
		"Мероприятие «"+title+"» было отменено организаторами.",
		"",
		"Приносим извинения за неудобства. Вы можете выбрать другое мероприятие в главном меню.",
	)
}

// =============================================================================
// Magic-link для входа в админку (День 13)
// =============================================================================

// AdminLoginLink — текст сообщения с inline-кнопкой magic-link.
func AdminLoginLink() string {
	return joinLines(
		"Ссылка для входа в веб-админку. Действует 5 минут, открывается только в одном браузере.",
		"",
		"Если не запрашивали - просто проигнорируйте сообщение.",
	)
}

// AdminLoginNoAccess — не organizer и не admin.
func AdminLoginNoAccess() string {
	return "Веб-админка доступна только организаторам и администраторам."
}

// =============================================================================
// AI
// =============================================================================

// AIAskInterest — приглашение ввести интерес для AI-подбора.
func AIAskInterest() string {
	return "Опишите, что вам интересно (одной фразой). Я подберу подходящее мероприятие."
}

// AIUnavailable — ИИ упал, fallback в обычный список.
func AIUnavailable() string {
	return "Подбор временно недоступен. Покажу обычный список мероприятий."
}

// AIWarningNote — короткая плашка о том, что ИИ может ошибаться.
func AIWarningNote() string {
	return "Важно: ИИ может ошибаться. Проверьте детали перед использованием."
}

// AIRecommendation — обёртка над текстом, полученным от AI.
func AIRecommendation(text string) string {
	return joinLines(
		"Подобрал для вас:",
		"",
		text,
		"",
		AIWarningNote(),
	)
}

// AIRecommendationPage — заголовок страницы AI-рекомендаций.
func AIRecommendationPage(page, totalPages int, text string) string {
	header := fmt.Sprintf("Рекомендации (%d из %d):", page, totalPages)
	return joinLines(
		header,
		"",
		text,
		"",
		AIWarningNote(),
	)
}

// AIOrganizerSummary — форматирует AI-сводку для организатора.
func AIOrganizerSummary(title, summary string) string {
	return joinLines(
		"AI-сводка по «"+title+"»:",
		"",
		summary,
		"",
		AIWarningNote(),
	)
}

// AIOrganizerNotifPreview — форматирует AI-улучшенный текст рассылки.
func AIOrganizerNotifPreview(text string, recipients int) string {
	return joinLines(
		"Текст улучшен через ИИ:",
		"",
		OrganizerNotifPreview(text, recipients),
		"",
		AIWarningNote(),
	)
}

// =============================================================================
// Системные / fallback
// =============================================================================

// FallbackUnknown — не поняли команду / неожиданный текст.
func FallbackUnknown() string {
	return "Не понял запрос. Нажмите /start, чтобы вернуться в главное меню, или /help для подсказки."
}

// ErrorTryLater — общая ошибка, реальную причину не раскрываем пользователю.
func ErrorTryLater() string {
	return "Что-то пошло не так. Попробуйте ещё раз через минуту."
}

// RateLimited — пользователь шлёт слишком быстро.
func RateLimited() string {
	return "Слишком много запросов. Подождите несколько секунд."
}

// joinLines склеивает строки через \n. Не использует joinNonEmpty,
// потому что нам важны пустые строки-разделители абзацев.
func joinLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

// _ — placeholder, чтобы импорт time не казался линтеру неиспользуемым,
// если случайно из ru.go все обращения уйдут.
var _ = time.Time{}
