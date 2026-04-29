package bot

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"service-status-page/internal/checks"
	"service-status-page/internal/config"
	"service-status-page/internal/store"
)

const (
	defaultMaintenanceStatusMessage = "Сервис временно на техническом обслуживании"
	defaultIncidentStatusMessage    = "В работе сервиса наблюдаются проблемы"
	defaultClearMessage             = "Объявление снято"
	adminChatSignature              = "Админ"
)

var noPreview = &tele.SendOptions{
	DisableWebPagePreview: true,
}

type Bot struct {
	bot      *tele.Bot
	store    *store.Store
	adminIDs map[int64]struct{}
	admins   []int64
}

func New(cfg config.Config, st *store.Store) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}

	b := &Bot{
		bot:      tb,
		store:    st,
		adminIDs: cfg.AdminIDs,
		admins:   cfg.AdminIDList,
	}
	b.registerHandlers()
	b.syncCommands()
	return b, nil
}

func (b *Bot) Start() {
	log.Print("telegram bot started")
	b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) NotifyReport(report store.Report) error {
	if len(b.admins) == 0 {
		return fmt.Errorf("no telegram admins configured")
	}

	text := formatReport(report)
	var lastErr error
	for _, adminID := range b.admins {
		_, err := b.bot.Send(&tele.User{ID: adminID}, text, noPreview)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (b *Bot) NotifyAvailabilityProblems(results []checks.Result) error {
	return b.sendToAdmins(formatAvailabilityProblems(results))
}

func (b *Bot) NotifyAvailabilityRecovered(results []checks.Result) error {
	return b.sendToAdmins(formatAvailabilityRecovered(results))
}

func (b *Bot) sendToAdmins(text string) error {
	if len(b.admins) == 0 {
		return fmt.Errorf("no telegram admins configured")
	}
	if text == "" {
		return nil
	}

	var lastErr error
	for _, adminID := range b.admins {
		_, err := b.bot.Send(&tele.User{ID: adminID}, text, noPreview)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (b *Bot) registerHandlers() {
	b.bot.Handle("/help", b.adminOnly(func(c tele.Context) error {
		return c.Send(helpText())
	}))
	b.bot.Handle("/maintenance", b.adminOnly(b.handleStatusAnnouncement(store.AnnouncementMaintenance, "/maintenance", defaultMaintenanceStatusMessage)))
	b.bot.Handle("/incident", b.adminOnly(b.handleStatusAnnouncement(store.AnnouncementIncident, "/incident", defaultIncidentStatusMessage)))
	b.bot.Handle("/announce", b.adminOnly(b.handleAnnounce))
	b.bot.Handle("/chat", b.adminOnly(b.handleChatMessage))
	b.bot.Handle("/info", b.adminOnly(b.handlePinnedInfo))
	b.bot.Handle("/clear", b.adminOnly(b.handleClear))
	b.bot.Handle("/clearinfo", b.adminOnly(b.handleClearPinnedInfo))
	b.bot.Handle("/delete_last", b.adminOnly(b.handleDeleteLatest))
	b.bot.Handle("/list", b.adminOnly(b.handleList))
}

func (b *Bot) syncCommands() {
	commands := []tele.Command{
		{Text: "announce", Description: "Опубликовать обычное объявление"},
		{Text: "chat", Description: "Отправить сообщение в чат"},
		{Text: "maintenance", Description: "Опубликовать объявление о работах"},
		{Text: "incident", Description: "Опубликовать объявление об инциденте"},
		{Text: "info", Description: "Обновить постоянный инфо-блок"},
		{Text: "clear", Description: "Снять активное объявление"},
		{Text: "clearinfo", Description: "Очистить постоянный инфо-блок"},
		{Text: "delete_last", Description: "Удалить последнее объявление"},
		{Text: "list", Description: "Показать последние объявления"},
		{Text: "help", Description: "Показать справку"},
	}

	for _, adminID := range b.admins {
		scope := tele.CommandScope{
			Type:   tele.CommandScopeChat,
			ChatID: adminID,
		}
		if err := b.bot.SetCommands(commands, scope); err != nil {
			log.Printf("failed to sync telegram commands for %d: %v", adminID, err)
		}
	}
}

func (b *Bot) adminOnly(next func(tele.Context) error) func(tele.Context) error {
	return func(c tele.Context) error {
		sender := c.Sender()
		if sender == nil || !IsAdmin(sender.ID, b.adminIDs) {
			return c.Send("Нет доступа.")
		}
		return next(c)
	}
}

func (b *Bot) handleStatusAnnouncement(kind store.AnnouncementKind, command string, defaultMessage ...string) func(tele.Context) error {
	return func(c tele.Context) error {
		message, err := ParseStatusMessage(c.Message().Payload, command, defaultMessage...)
		if err != nil {
			return c.Send(err.Error())
		}
		if _, err := b.publishStatusAnnouncement(kind, message, adminName(c.Sender())); err != nil {
			return c.Send("Не удалось сохранить объявление.")
		}
		return c.Send("Объявление опубликовано.")
	}
}

func (b *Bot) publishStatusAnnouncement(kind store.AnnouncementKind, message, createdBy string) (store.Announcement, error) {
	return b.store.AddAnnouncement(message, kind, createdBy)
}

func (b *Bot) handleAnnounce(c tele.Context) error {
	message := strings.TrimSpace(c.Message().Payload)
	if message == "" {
		return c.Send("Использование: /announce текст объявления")
	}
	if _, err := b.store.AddAnnouncement(message, store.AnnouncementInfo, adminName(c.Sender())); err != nil {
		return c.Send("Не удалось сохранить объявление.")
	}
	return c.Send("Объявление опубликовано.")
}

func (b *Bot) handleChatMessage(c tele.Context) error {
	message := strings.TrimSpace(c.Message().Payload)
	if message == "" {
		return c.Send("Использование: /chat текст сообщения")
	}
	if _, err := b.publishChatMessage(message); err != nil {
		return c.Send("Не удалось отправить сообщение.")
	}
	return c.Send("Сообщение отправлено в чат.")
}

func (b *Bot) publishChatMessage(message string) (store.Announcement, error) {
	return b.store.AddAnnouncement(message, store.AnnouncementAdminChat, adminChatSignature)
}

func (b *Bot) handlePinnedInfo(c tele.Context) error {
	message := strings.TrimSpace(c.Message().Payload)
	if message == "" {
		return c.Send("Использование: /info текст постоянного блока")
	}
	if _, err := b.store.SetPinnedInfo(message, adminName(c.Sender())); err != nil {
		return c.Send("Не удалось сохранить постоянный блок.")
	}
	return c.Send("Постоянный блок обновлен.")
}

func (b *Bot) handleClear(c tele.Context) error {
	message := strings.TrimSpace(c.Message().Payload)
	if message == "" {
		message = defaultClearMessage
	}
	if _, err := b.store.AddAnnouncement(message, store.AnnouncementCleared, adminName(c.Sender())); err != nil {
		return c.Send("Не удалось сохранить объявление.")
	}
	return c.Send("Активное объявление снято.")
}

func (b *Bot) handleClearPinnedInfo(c tele.Context) error {
	cleared, err := b.store.ClearPinnedInfo()
	if err != nil {
		return c.Send("Не удалось очистить постоянный блок.")
	}
	if !cleared {
		return c.Send("Постоянный блок уже пуст.")
	}
	return c.Send("Постоянный блок очищен.")
}

func (b *Bot) handleDeleteLatest(c tele.Context) error {
	ann, statusChanged, err := b.store.DeleteLatestAnnouncement()
	if errors.Is(err, store.ErrNoAnnouncements) {
		return c.Send("Объявлений пока нет.")
	}
	if err != nil {
		return c.Send("Не удалось удалить последнее объявление.")
	}

	lines := []string{
		"Последнее объявление удалено.",
		fmt.Sprintf("%s [%s]\n%s", ann.CreatedAt.Format("02.01 15:04"), ann.Kind, ann.Message),
	}
	if statusChanged {
		lines = append(lines, "Legacy-статус откатан на предыдущий.")
	}
	return c.Send(strings.Join(lines, "\n\n"))
}

func (b *Bot) handleList(c tele.Context) error {
	snap := b.store.Snapshot()
	if len(snap.Announcements) == 0 {
		return c.Send("Объявлений пока нет.")
	}

	limit := 10
	if len(snap.Announcements) < limit {
		limit = len(snap.Announcements)
	}
	lines := make([]string, 0, limit)
	for _, ann := range snap.Announcements[:limit] {
		lines = append(lines, fmt.Sprintf("%s [%s]\n%s", ann.CreatedAt.Format("02.01 15:04"), ann.Kind, ann.Message))
	}
	return c.Send(strings.Join(lines, "\n\n"))
}

func IsAdmin(id int64, admins map[int64]struct{}) bool {
	_, ok := admins[id]
	return ok
}

func ParseStatusMessage(payload, command string, defaultMessage ...string) (string, error) {
	message := strings.TrimSpace(payload)
	if message == "" {
		if len(defaultMessage) > 0 && strings.TrimSpace(defaultMessage[0]) != "" {
			return strings.TrimSpace(defaultMessage[0]), nil
		}
		return "", fmt.Errorf("Использование: %s текст объявления", command)
	}
	return message, nil
}

func helpText() string {
	return strings.Join([]string{
		"/announce текст объявления",
		"/chat текст сообщения",
		"/maintenance [текст объявления]",
		"/incident [текст объявления]",
		"/info текст постоянного блока",
		"/clear [текст записи]",
		"/clearinfo",
		"/delete_last",
		"/list",
		"/help",
	}, "\n")
}

func formatReport(report store.Report) string {
	parts := []string{
		"Новый баг-репорт",
		report.Message,
	}
	if report.Name != "" {
		parts = append(parts, "Имя: "+report.Name)
	}
	if report.Contact != "" {
		parts = append(parts, "Контакт: "+report.Contact)
	}
	return strings.Join(parts, "\n\n")
}

func formatAvailabilityProblems(results []checks.Result) string {
	failed := make([]checks.Result, 0, len(results))
	for _, result := range results {
		if result.State != checks.StateUp {
			failed = append(failed, result)
		}
	}
	if len(failed) == 0 {
		return ""
	}

	parts := []string{"Проблемы с доступностью сайтов"}
	for _, result := range failed {
		lines := []string{
			result.Name,
			result.URL,
			"Состояние: " + availabilityStateTitle(result),
		}
		if result.StatusCode > 0 {
			lines = append(lines, fmt.Sprintf("HTTP: %d", result.StatusCode))
		}
		if result.Error != "" {
			lines = append(lines, "Ошибка: "+result.Error)
		}
		if result.LatencyMs > 0 {
			lines = append(lines, fmt.Sprintf("Задержка: %d мс", result.LatencyMs))
		}
		if !result.CheckedAt.IsZero() {
			lines = append(lines, "Проверено: "+result.CheckedAt.Format("02.01 15:04 UTC"))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n\n")
}

func formatAvailabilityRecovered(results []checks.Result) string {
	if len(results) == 0 {
		return "Доступность сайтов восстановлена"
	}

	parts := []string{
		"Доступность сайтов восстановлена",
		fmt.Sprintf("Все проверки успешны: %d", len(results)),
	}

	checkedAt := latestCheckedAt(results)
	if !checkedAt.IsZero() {
		parts = append(parts, "Проверено: "+checkedAt.Format("02.01 15:04 UTC"))
	}

	return strings.Join(parts, "\n\n")
}

func latestCheckedAt(results []checks.Result) time.Time {
	var latest time.Time
	for _, result := range results {
		if result.CheckedAt.After(latest) {
			latest = result.CheckedAt
		}
	}
	return latest
}

func availabilityStateTitle(result checks.Result) string {
	switch result.State {
	case checks.StateHTTPError:
		return "HTTP-ошибка"
	case checks.StateDown:
		return "Недоступен"
	default:
		return result.State
	}
}

func adminName(user *tele.User) string {
	if user == nil {
		return "telegram"
	}
	if user.Username != "" {
		return "@" + user.Username
	}
	if strings.TrimSpace(user.FirstName+" "+user.LastName) != "" {
		return strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	return fmt.Sprintf("%d", user.ID)
}
