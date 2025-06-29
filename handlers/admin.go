package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"telegram-bot/config"
	"telegram-bot/database"
	"telegram-bot/localization"
	"telegram-bot/models"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AdminHandler struct {
	bot      *tgbotapi.BotAPI
	db       *database.Database
	config   *config.Config
	loc      *localization.Localization
	sessions map[int64]*models.UserSession
}

func NewAdminHandler(bot *tgbotapi.BotAPI, db *database.Database, cfg *config.Config, loc *localization.Localization) *AdminHandler {
	return &AdminHandler{
		bot:      bot,
		db:       db,
		config:   cfg,
		loc:      loc,
		sessions: make(map[int64]*models.UserSession),
	}
}

func (h *AdminHandler) HandleAdminCommand(update tgbotapi.Update) {
	userID := update.Message.From.ID

	if !h.config.IsAdmin(userID) {
		user, _ := h.db.GetUser(userID)
		lang := "ru"
		if user != nil {
			lang = user.Language
		}
		text := h.loc.Get(lang, "not_admin")
		msg := tgbotapi.NewMessage(userID, text)
		h.bot.Send(msg)
		return
	}

	h.sendAdminMenu(userID)
}

func (h *AdminHandler) sendAdminMenu(userID int64) {
	user, _ := h.db.GetUser(userID)
	lang := "ru"
	if user != nil {
		lang = user.Language
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_user_count"), "admin_user_count"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_stats"), "admin_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_db_download"), "admin_db_download"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_mass_message"), "admin_mass_message"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_change_balance"), "admin_change_balance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_back_to_user_menu"), "main_menu"),
		),
	)

	text := h.loc.Get(lang, "admin_activated")
	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *AdminHandler) HandleAdminCallback(query *tgbotapi.CallbackQuery) {
	userID := query.From.ID

	if !h.config.IsAdmin(userID) {
		return
	}

	user, _ := h.db.GetUser(userID)
	lang := "ru"
	if user != nil {
		lang = user.Language
	}

	switch query.Data {
	case "admin_user_count":
		h.handleUserCount(query, lang)
	case "admin_stats":
		h.handleStats(query, lang)
	case "admin_db_download":
		h.handleDBDownload(query, lang)
	case "admin_mass_message":
		h.handleMassMessageStart(query, lang)
	case "admin_change_balance":
		h.handleChangeBalanceStart(query, lang)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	h.bot.Request(callback)
}

func (h *AdminHandler) handleUserCount(query *tgbotapi.CallbackQuery, lang string) {
	stats, err := h.db.GetStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		return
	}

	text := fmt.Sprintf("%s\nВсего пользователей: %d",
		h.loc.Get(lang, "btn_user_count"), stats.Total)

	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)
}

func (h *AdminHandler) handleStats(query *tgbotapi.CallbackQuery, lang string) {
	stats, err := h.db.GetStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		return
	}

	title := h.loc.Get(lang, "stats_title")
	text := h.loc.Get(lang, "stats_text", stats.Total, stats.Day, stats.Week, stats.Month)

	fullText := fmt.Sprintf("<b>%s</b>\n\n%s", title, text)
	msg := tgbotapi.NewMessage(query.From.ID, fullText)
	msg.ParseMode = tgbotapi.ModeHTML
	h.bot.Send(msg)
}

func (h *AdminHandler) handleDBDownload(query *tgbotapi.CallbackQuery, lang string) {
	users, err := h.db.ExportAllUsers()
	if err != nil {
		log.Printf("Error exporting users: %v", err)
		return
	}

	if len(users) == 0 {
		text := "База данных пуста."
		msg := tgbotapi.NewMessage(query.From.ID, text)
		h.bot.Send(msg)
		return
	}

	// Преобразуем в JSON
	jsonData, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}

	// Создаем файл
	fileName := fmt.Sprintf("database_%s.json", time.Now().Format("2006-01-02"))
	fileBytes := tgbotapi.FileBytes{
		Name:  fileName,
		Bytes: jsonData,
	}

	doc := tgbotapi.NewDocument(query.From.ID, fileBytes)
	doc.Caption = h.loc.Get(lang, "db_caption")
	h.bot.Send(doc)
}

func (h *AdminHandler) handleMassMessageStart(query *tgbotapi.CallbackQuery, lang string) {
	h.sessions[query.From.ID] = &models.UserSession{
		State: "awaiting_broadcast_message",
	}

	text := h.loc.Get(lang, "broadcast_prompt")
	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)
}

func (h *AdminHandler) handleChangeBalanceStart(query *tgbotapi.CallbackQuery, lang string) {
	h.sessions[query.From.ID] = &models.UserSession{
		State: "awaiting_balance_user_id",
	}

	text := h.loc.Get(lang, "balance_prompt_id")
	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)
}

func (h *AdminHandler) HandleMessage(update tgbotapi.Update) {
	userID := update.Message.From.ID

	if !h.config.IsAdmin(userID) {
		return
	}

	// Проверяем команду отмены
	if update.Message.Text == "/cancel" {
		h.handleCancel(userID)
		return
	}

	session, exists := h.sessions[userID]
	if !exists {
		return
	}

	user, _ := h.db.GetUser(userID)
	lang := "ru"
	if user != nil {
		lang = user.Language
	}

	switch session.State {
	case "awaiting_broadcast_message":
		h.handleBroadcastMessage(update.Message, lang)
	case "awaiting_balance_user_id":
		h.handleBalanceUserID(update.Message, lang, session)
	case "awaiting_balance_amount":
		h.handleBalanceAmount(update.Message, lang, session)
	}
}

func (h *AdminHandler) handleBroadcastMessage(message *tgbotapi.Message, lang string) {
	userIDs, err := h.db.GetAllUserIDs()
	if err != nil {
		log.Printf("Error getting user IDs: %v", err)
		return
	}

	// Уведомляем о начале рассылки
	text := h.loc.Get(lang, "broadcast_sending", len(userIDs))
	msg := tgbotapi.NewMessage(message.From.ID, text)
	h.bot.Send(msg)

	successCount := 0
	failCount := 0

	// Рассылаем сообщение
	for _, userID := range userIDs {
		copyMsg := tgbotapi.NewMessage(userID, message.Text)
		if message.Photo != nil && len(message.Photo) > 0 {
			// Если это фото
			photo := tgbotapi.NewPhoto(userID, tgbotapi.FileID(message.Photo[len(message.Photo)-1].FileID))
			photo.Caption = message.Caption
			if _, err := h.bot.Send(photo); err != nil {
				failCount++
				continue
			}
		} else {
			// Обычное текстовое сообщение
			if _, err := h.bot.Send(copyMsg); err != nil {
				failCount++
				continue
			}
		}
		successCount++
	}

	// Отчет о рассылке
	reportText := h.loc.Get(lang, "broadcast_complete", successCount, failCount)
	reportMsg := tgbotapi.NewMessage(message.From.ID, reportText)
	h.bot.Send(reportMsg)

	// Очищаем сессию
	delete(h.sessions, message.From.ID)
}

func (h *AdminHandler) handleBalanceUserID(message *tgbotapi.Message, lang string, session *models.UserSession) {
	userID, err := strconv.ParseInt(message.Text, 10, 64)
	if err != nil {
		text := h.loc.Get(lang, "balance_user_not_found", message.Text)
		msg := tgbotapi.NewMessage(message.From.ID, text)
		h.bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь
	targetUser, err := h.db.GetUser(userID)
	if err != nil || targetUser == nil {
		text := h.loc.Get(lang, "balance_user_not_found", userID)
		msg := tgbotapi.NewMessage(message.From.ID, text)
		h.bot.Send(msg)
		return
	}

	// Сохраняем ID пользователя и переходим к следующему состоянию
	session.AwaitingBalanceUserID = userID
	session.State = "awaiting_balance_amount"

	text := h.loc.Get(lang, "balance_prompt_amount", userID, targetUser.Balance)
	msg := tgbotapi.NewMessage(message.From.ID, text)
	h.bot.Send(msg)
}

func (h *AdminHandler) handleBalanceAmount(message *tgbotapi.Message, lang string, session *models.UserSession) {
	amount, err := strconv.ParseFloat(message.Text, 64)
	if err != nil {
		text := h.loc.Get(lang, "balance_invalid_amount")
		msg := tgbotapi.NewMessage(message.From.ID, text)
		h.bot.Send(msg)
		return
	}

	userID := session.AwaitingBalanceUserID

	// Обновляем баланс
	if err := h.db.UpdateUserBalance(userID, amount); err != nil {
		log.Printf("Error updating balance for user %d: %v", userID, err)
		return
	}

	text := h.loc.Get(lang, "balance_update_success", userID, amount)
	msg := tgbotapi.NewMessage(message.From.ID, text)
	h.bot.Send(msg)

	// Очищаем сессию
	delete(h.sessions, message.From.ID)
}

func (h *AdminHandler) handleCancel(userID int64) {
	user, _ := h.db.GetUser(userID)
	lang := "ru"
	if user != nil {
		lang = user.Language
	}

	text := h.loc.Get(lang, "cancel_operation")
	msg := tgbotapi.NewMessage(userID, text)
	h.bot.Send(msg)

	// Очищаем сессию
	delete(h.sessions, userID)
}
