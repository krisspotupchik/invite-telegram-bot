package handlers

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"telegram-bot/config"
	"telegram-bot/database"
	"telegram-bot/localization"
	"telegram-bot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UserHandler struct {
	bot      *tgbotapi.BotAPI
	db       *database.Database
	config   *config.Config
	loc      *localization.Localization
	sessions map[int64]*models.UserSession
}

func NewUserHandler(bot *tgbotapi.BotAPI, db *database.Database, cfg *config.Config, loc *localization.Localization) *UserHandler {
	return &UserHandler{
		bot:      bot,
		db:       db,
		config:   cfg,
		loc:      loc,
		sessions: make(map[int64]*models.UserSession),
	}
}

func (h *UserHandler) HandleStart(update tgbotapi.Update) {
	userID := update.Message.From.ID

	// Проверяем, есть ли пользователь в базе
	user, err := h.db.GetUser(userID)
	if err != nil {
		log.Printf("Error getting user %d: %v", userID, err)
		return
	}

	if user == nil {
		// Новый пользователь - создаем и показываем выбор языка
		var referredBy *int64
		if update.Message.CommandArguments() != "" {
			if referrerID, err := strconv.ParseInt(update.Message.CommandArguments(), 10, 64); err == nil {
				if referrerID != userID {
					// Проверяем, существует ли реферер
					if referrer, _ := h.db.GetUser(referrerID); referrer != nil {
						referredBy = &referrerID
					}
				}
			}
		}

		if err := h.db.CreateUser(userID, referredBy, h.config.RewardAmount); err != nil {
			log.Printf("Error creating user %d: %v", userID, err)
			return
		}

		// Уведомляем реферера
		if referredBy != nil {
			referrerUser, _ := h.db.GetUser(*referredBy)
			if referrerUser != nil {
				text := h.loc.Get(referrerUser.Language, "new_referral_notification", h.config.RewardAmount)
				msg := tgbotapi.NewMessage(*referredBy, text)
				h.bot.Send(msg)
			}
		}

		// Показываем выбор языка только для новых пользователей
		h.sendLanguageSelection(userID)
	} else {
		// Существующий пользователь - показываем профиль на его языке
		h.sendUserMenu(userID, user.Language)
	}
}

func (h *UserHandler) sendLanguageSelection(userID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("English 🇬🇧", "lang_en"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Русский 🇷🇺", "lang_ru"),
		),
	)

	text := h.loc.Get("ru", "welcome") + "\n\n" + h.loc.Get("en", "welcome")
	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

func (h *UserHandler) HandleLanguageSelection(query *tgbotapi.CallbackQuery) {
	userID := query.From.ID
	langCode := strings.TrimPrefix(query.Data, "lang_")

	// Сохраняем язык в базу
	if err := h.db.UpdateUserLanguage(userID, langCode); err != nil {
		log.Printf("Error updating language for user %d: %v", userID, err)
		return
	}

	// Показываем полный профиль пользователя
	h.showUserProfile(query, langCode)

	// Подтверждаем обработку callback
	callback := tgbotapi.NewCallback(query.ID, "")
	h.bot.Request(callback)
}

func (h *UserHandler) showUserProfile(query *tgbotapi.CallbackQuery, lang string) {
	userID := query.From.ID
	
	// Получаем свежие данные пользователя
	user, err := h.db.GetUser(userID)
	if err != nil || user == nil {
		log.Printf("Error getting user data for profile: %v", err)
		return
	}

	// Формируем реферальную ссылку
	referralLink := fmt.Sprintf("https://t.me/%s?start=%d", h.bot.Self.UserName, user.UserID)
	
	// Количество рефералов
	referralCount := len(user.Referrals)
	
	// Формируем текст профиля (убрали дату регистрации)
	var profileText string
	if lang == "ru" {
		profileText = fmt.Sprintf(
			"🤑 <b>Дарите подарки и зарабатывайте</b>\n\n"+
			"Отправляйте друзьям подарки и получайте криптовалюту.\n"+
			"Вывод станет доступен при накоплении %.2f USDT на балансе.\n\n"+
			"<b>Ваша реферальная ссылка:</b>\n"+
			"<code>%s</code>\n\n"+
			"У вас <b>%d подтвержденных рефералов</b>\n\n"+
			"<b>Ваш баланс:</b> %.2f USDT (%.2f$)\n\n"+
			"<b>Ваш ID:</b> <code>%d</code>",
			h.config.MinWithdrawalAmount,
			referralLink,
			referralCount,
			user.Balance,
			user.Balance,
			user.UserID,
		)
	} else {
		profileText = fmt.Sprintf(
			"🤑 <b>Give gifts and earn</b>\n\n"+
			"Send gifts to friends and get cryptocurrency.\n"+
			"Withdrawal will be available when accumulating %.2f USDT on balance.\n\n"+
			"<b>Your referral link:</b>\n"+
			"<code>%s</code>\n\n"+
			"You have <b>%d confirmed referrals</b>\n\n"+
			"<b>Your balance:</b> %.2f USDT (%.2f$)\n\n"+
			"<b>Your ID:</b> <code>%d</code>",
			h.config.MinWithdrawalAmount,
			referralLink,
			referralCount,
			user.Balance,
			user.Balance,
			user.UserID,
		)
	}

	// Создаем клавиатуру
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_balance"), "user_balance"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_withdraw"), "user_withdraw"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_gift"), "user_gift"),
		),
	)

	// Редактируем сообщение
	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, profileText)
	edit.ReplyMarkup = &keyboard
	edit.ParseMode = tgbotapi.ModeHTML
	h.bot.Send(edit)
}

func (h *UserHandler) sendUserMenu(userID int64, lang string) {
	user, err := h.db.GetUser(userID)
	if err != nil || user == nil {
		log.Printf("Error getting user data for menu: %v", err)
		return
	}

	// Формируем реферальную ссылку
	referralLink := fmt.Sprintf("https://t.me/%s?start=%d", h.bot.Self.UserName, user.UserID)
	
	// Количество рефералов
	referralCount := len(user.Referrals)
	
	// Формируем текст профиля (убрали дату регистрации)
	var profileText string
	if lang == "ru" {
		profileText = fmt.Sprintf(
			"🤑 <b>Дарите подарки и зарабатывайте</b>\n\n"+
			"Отправляйте друзьям подарки и получайте криптовалюту.\n"+
			"Вывод станет доступен при накоплении %.2f USDT на балансе.\n\n"+
			"<b>Ваша реферальная ссылка:</b>\n"+
			"<code>%s</code>\n\n"+
			"У вас <b>%d подтвержденных рефералов</b>\n\n"+
			"<b>Ваш баланс:</b> %.2f USDT (%.2f$)\n\n"+
			"<b>Ваш ID:</b> <code>%d</code>",
			h.config.MinWithdrawalAmount,
			referralLink,
			referralCount,
			user.Balance,
			user.Balance,
			user.UserID,
		)
	} else {
		profileText = fmt.Sprintf(
			"🤑 <b>Give gifts and earn</b>\n\n"+
			"Send gifts to friends and get cryptocurrency.\n"+
			"Withdrawal will be available when accumulating %.2f USDT on balance.\n\n"+
			"<b>Your referral link:</b>\n"+
			"<code>%s</code>\n\n"+
			"You have <b>%d confirmed referrals</b>\n\n"+
			"<b>Your balance:</b> %.2f USDT (%.2f$)\n\n"+
			"<b>Your ID:</b> <code>%d</code>",
			h.config.MinWithdrawalAmount,
			referralLink,
			referralCount,
			user.Balance,
			user.Balance,
			user.UserID,
		)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_balance"), "user_balance"),
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_withdraw"), "user_withdraw"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.loc.Get(lang, "btn_gift"), "user_gift"),
		),
	)

	msg := tgbotapi.NewMessage(userID, profileText)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = tgbotapi.ModeHTML
	h.bot.Send(msg)
}

func (h *UserHandler) HandleUserCallback(query *tgbotapi.CallbackQuery) {
	userID := query.From.ID
	user, err := h.db.GetUser(userID)
	if err != nil || user == nil {
		return
	}

	switch query.Data {
	case "user_balance":
		h.handleBalance(query, user)
	case "user_withdraw":
		h.handleWithdrawStart(query, user)
	case "user_gift":
		h.handleGift(query, user)
	case "main_menu":
		h.showUserProfile(query, user.Language)
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	h.bot.Request(callback)
}

func (h *UserHandler) handleBalance(query *tgbotapi.CallbackQuery, user *models.User) {
	// Получаем обновленные данные пользователя
	freshUser, err := h.db.GetUser(user.UserID)
	if err != nil {
		log.Printf("Error getting fresh user data: %v", err)
		return
	}

	text := h.loc.Get(user.Language, "balance_display", freshUser.Balance)
	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)

	// Также обновляем главное меню с новыми данными
	h.showUserProfile(query, user.Language)
}

func (h *UserHandler) handleWithdrawStart(query *tgbotapi.CallbackQuery, user *models.User) {
	if user.Balance < h.config.MinWithdrawalAmount {
		text := h.loc.Get(user.Language, "withdraw_insufficient_funds",
			h.config.MinWithdrawalAmount, user.Balance)
		msg := tgbotapi.NewMessage(query.From.ID, text)
		h.bot.Send(msg)
		return
	}

	// Устанавливаем состояние ожидания кошелька
	h.sessions[user.UserID] = &models.UserSession{
		State:                "awaiting_wallet",
		AwaitingWalletAmount: user.Balance,
	}

	text := h.loc.Get(user.Language, "withdraw_prompt", user.Balance)
	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)
}

func (h *UserHandler) handleGift(query *tgbotapi.CallbackQuery, user *models.User) {
	text := h.loc.Get(user.Language, "gift_not_implemented")
	msg := tgbotapi.NewMessage(query.From.ID, text)
	h.bot.Send(msg)
}

func (h *UserHandler) HandleMessage(update tgbotapi.Update) {
	userID := update.Message.From.ID

	// Проверяем команду отмены
	if update.Message.Text == "/cancel" {
		h.handleCancel(userID)
		return
	}

	session, exists := h.sessions[userID]
	if !exists {
		return
	}

	user, err := h.db.GetUser(userID)
	if err != nil || user == nil {
		return
	}

	switch session.State {
	case "awaiting_wallet":
		h.handleWalletAddress(update.Message, user, session)
	}
}

func (h *UserHandler) handleWalletAddress(message *tgbotapi.Message, user *models.User, session *models.UserSession) {
	walletAddress := message.Text

	// Проверяем формат TRC20 кошелька
	trc20Regex := regexp.MustCompile(`^T[a-zA-Z0-9]{33}$`)
	if !trc20Regex.MatchString(walletAddress) {
		text := h.loc.Get(user.Language, "withdraw_invalid_wallet")
		msg := tgbotapi.NewMessage(message.From.ID, text)
		h.bot.Send(msg)
		return
	}

	amount := session.AwaitingWalletAmount

	// Уведомляем пользователя об успехе
	text := h.loc.Get(user.Language, "withdraw_success_user", amount, walletAddress)
	msg := tgbotapi.NewMessage(message.From.ID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	h.bot.Send(msg)

	// Уведомляем админов
	adminText := h.loc.Get("ru", "admin_withdrawal_notification",
		user.UserID, amount, walletAddress)
	for _, adminID := range h.config.AdminUserIDs {
		adminMsg := tgbotapi.NewMessage(adminID, adminText)
		adminMsg.ParseMode = tgbotapi.ModeHTML
		h.bot.Send(adminMsg)
	}

	// Обнуляем баланс
	h.db.UpdateUserBalance(user.UserID, 0.0)

	// Очищаем сессию
	delete(h.sessions, user.UserID)
}

func (h *UserHandler) handleCancel(userID int64) {
	user, err := h.db.GetUser(userID)
	if err != nil || user == nil {
		return
	}

	text := h.loc.Get(user.Language, "cancel_operation")
	msg := tgbotapi.NewMessage(userID, text)
	h.bot.Send(msg)

	// Очищаем сессию
	delete(h.sessions, userID)
}
