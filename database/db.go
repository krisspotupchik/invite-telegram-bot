package database

import (
	"database/sql"
	"strconv"
	"telegram-bot/models"
	"time"

	_ "modernc.org/sqlite" // Используем pure Go SQLite драйвер
)

// Остальной код остается без изменений
type Database struct {
	db *sql.DB
}

func New(dbFile string) (*Database, error) {
	db, err := sql.Open("sqlite", dbFile) // Изменили с "sqlite3" на "sqlite"
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id INTEGER PRIMARY KEY,
			balance REAL DEFAULT 0.0,
			referred_by INTEGER,
			join_date DATETIME,
			language TEXT DEFAULT 'ru'
		)`,
		`CREATE TABLE IF NOT EXISTS referrals (
			referrer_id INTEGER,
			referred_id INTEGER,
			date_added DATETIME,
			FOREIGN KEY (referrer_id) REFERENCES users (user_id),
			FOREIGN KEY (referred_id) REFERENCES users (user_id)
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) GetUser(userID int64) (*models.User, error) {
	var user models.User
	var referredBy sql.NullInt64
	var joinDate string

	query := `SELECT user_id, balance, referred_by, join_date, language FROM users WHERE user_id = ?`
	err := d.db.QueryRow(query, userID).Scan(&user.UserID, &user.Balance, &referredBy, &joinDate, &user.Language)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if referredBy.Valid {
		user.ReferredBy = &referredBy.Int64
	}

	user.JoinDate, _ = time.Parse("2006-01-02 15:04:05", joinDate)

	// Получаем рефералов
	referralQuery := `SELECT referred_id FROM referrals WHERE referrer_id = ?`
	rows, err := d.db.Query(referralQuery, userID)
	if err != nil {
		return &user, nil
	}
	defer rows.Close()

	for rows.Next() {
		var referralID int64
		if err := rows.Scan(&referralID); err == nil {
			user.Referrals = append(user.Referrals, referralID)
		}
	}

	return &user, nil
}

func (d *Database) CreateUser(userID int64, referredBy *int64, rewardAmount float64) error {
	joinDate := time.Now().Format("2006-01-02 15:04:05")

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO users (user_id, balance, referred_by, join_date, language) VALUES (?, ?, ?, ?, ?)`,
		userID, 0.0, referredBy, joinDate, "ru")
	if err != nil {
		return err
	}

	if referredBy != nil {
		// Добавляем запись о реферале
		_, err = tx.Exec(`INSERT INTO referrals (referrer_id, referred_id, date_added) VALUES (?, ?, ?)`,
			*referredBy, userID, joinDate)
		if err != nil {
			return err
		}

		// Начисляем бонус рефереру
		_, err = tx.Exec(`UPDATE users SET balance = balance + ? WHERE user_id = ?`, rewardAmount, *referredBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *Database) UpdateUserBalance(userID int64, newBalance float64) error {
	_, err := d.db.Exec(`UPDATE users SET balance = ? WHERE user_id = ?`, newBalance, userID)
	return err
}

func (d *Database) UpdateUserLanguage(userID int64, language string) error {
	_, err := d.db.Exec(`UPDATE users SET language = ? WHERE user_id = ?`, language, userID)
	return err
}

func (d *Database) GetAllUserIDs() ([]int64, error) {
	rows, err := d.db.Query(`SELECT user_id FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}

	return userIDs, nil
}

func (d *Database) GetStats() (*models.Stats, error) {
	var stats models.Stats

	// Общее количество пользователей
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.Total)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	weekAgo := now.Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	monthAgo := now.Add(-30 * 24 * time.Hour).Format("2006-01-02 15:04:05")

	// За последние 24 часа
	err = d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE join_date >= ?`, dayAgo).Scan(&stats.Day)
	if err != nil {
		return nil, err
	}

	// За последние 7 дней
	err = d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE join_date >= ?`, weekAgo).Scan(&stats.Week)
	if err != nil {
		return nil, err
	}

	// За последние 30 дней
	err = d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE join_date >= ?`, monthAgo).Scan(&stats.Month)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (d *Database) ExportAllUsers() (map[string]*models.User, error) {
	rows, err := d.db.Query(`SELECT user_id, balance, referred_by, join_date, language FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make(map[string]*models.User)

	for rows.Next() {
		var user models.User
		var referredBy sql.NullInt64
		var joinDate string

		err := rows.Scan(&user.UserID, &user.Balance, &referredBy, &joinDate, &user.Language)
		if err != nil {
			continue
		}

		if referredBy.Valid {
			user.ReferredBy = &referredBy.Int64
		}

		user.JoinDate, _ = time.Parse("2006-01-02 15:04:05", joinDate)

		// Получаем рефералов для каждого пользователя
		referralRows, err := d.db.Query(`SELECT referred_id FROM referrals WHERE referrer_id = ?`, user.UserID)
		if err == nil {
			for referralRows.Next() {
				var referralID int64
				if err := referralRows.Scan(&referralID); err == nil {
					user.Referrals = append(user.Referrals, referralID)
				}
	}
			referralRows.Close()
		}

		users[strconv.FormatInt(user.UserID, 10)] = &user
	}

	return users, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
