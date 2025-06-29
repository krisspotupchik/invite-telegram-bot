package main

import (
	"log"
	"strings"
	"telegram-bot/config"
	"telegram-bot/database"
	"telegram-bot/handlers"
	"telegram-bot/localization"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.Load()

	// Инициализируем локализацию
	loc := localization.New()

	// Подключаемся к базе данных
	db, err := database.New(cfg.DatabaseFile)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Создаем бота
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Создаем обработчики
	userHandler := handlers.NewUserHandler(bot, db, cfg, loc)
	adminHandler := handlers.NewAdminHandler(bot, db, cfg, loc)

	// Настраиваем получение обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Println("Bot started successfully! Waiting for messages...")

	// Основной цикл обработки сообщений
	for update := range updates {
		if update.Message != nil {
			// Обрабатываем команды
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					go userHandler.HandleStart(update)
				case "admin":
					go adminHandler.HandleAdminCommand(update)
				case "cancel":
					go userHandler.HandleMessage(update)
					go adminHandler.HandleMessage(update)
				}
			} else {
				// Обрабатываем обычные сообщения (для диалогов)
				go userHandler.HandleMessage(update)
				go adminHandler.HandleMessage(update)
			}
		} else if update.CallbackQuery != nil {
			// Обрабатываем нажатия на кнопки
			if strings.HasPrefix(update.CallbackQuery.Data, "lang_") {
				go userHandler.HandleLanguageSelection(update.CallbackQuery)
			} else if update.CallbackQuery.Data == "main_menu" {
				go userHandler.HandleUserCallback(update.CallbackQuery)
			} else if isUserCallback(update.CallbackQuery.Data) {
				go userHandler.HandleUserCallback(update.CallbackQuery)
			} else if isAdminCallback(update.CallbackQuery.Data) {
				go adminHandler.HandleAdminCallback(update.CallbackQuery)
			}
		}
	}
}

func isUserCallback(data string) bool {
	userCallbacks := []string{"user_balance", "user_withdraw", "user_gift", "user_referral"}
	for _, callback := range userCallbacks {
		if data == callback {
			return true
		}
	}
	return false
}

func isAdminCallback(data string) bool {
	adminCallbacks := []string{"admin_user_count", "admin_stats", "admin_db_download", "admin_mass_message", "admin_change_balance"}
	for _, callback := range adminCallbacks {
		if data == callback {
			return true
		}
	}
	return false
}
