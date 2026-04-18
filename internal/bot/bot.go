package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"service-status-page/internal/config"
	"service-status-page/internal/store"
)

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
		_, err := b.bot.Send(&tele.User{ID: adminID}, text)
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
	b.bot.Handle("/status", b.adminOnly(b.handleStatus))
	b.bot.Handle("/announce", b.adminOnly(b.handleAnnounce))
	b.bot.Handle("/resolve", b.adminOnly(b.handleResolve))
	b.bot.Handle("/list", b.adminOnly(b.handleList))
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

func (b *Bot) handleStatus(c tele.Context) error {
	state, message, err := ParseStatusCommand(c.Message().Payload)
	if err != nil {
		return c.Send(err.Error())
	}
	if _, err := b.store.SetStatus(state, message, adminName(c.Sender())); err != nil {
		return c.Send("Не удалось сохранить статус.")
	}
	return c.Send("Статус обновлен.")
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

func (b *Bot) handleResolve(c tele.Context) error {
	message := strings.TrimSpace(c.Message().Payload)
	if message == "" {
		message = "Проблема устранена, сервис работает штатно"
	}
	if _, err := b.store.Resolve(message, adminName(c.Sender())); err != nil {
		return c.Send("Не удалось сохранить статус.")
	}
	return c.Send("Статус переведен в ok.")
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

func ParseStatusCommand(payload string) (store.StatusState, string, error) {
	fields := strings.Fields(strings.TrimSpace(payload))
	if len(fields) < 2 {
		return "", "", fmt.Errorf("Использование: /status ok|maintenance|incident текст статуса")
	}

	state := store.StatusState(fields[0])
	switch state {
	case store.StatusOK, store.StatusMaintenance, store.StatusIncident:
	default:
		return "", "", fmt.Errorf("Неизвестный статус: %s", fields[0])
	}

	message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(payload), fields[0]))
	if message == "" {
		return "", "", fmt.Errorf("Текст статуса не должен быть пустым")
	}
	return state, message, nil
}

func helpText() string {
	return strings.Join([]string{
		"/status ok|maintenance|incident текст",
		"/announce текст объявления",
		"/resolve текст",
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
