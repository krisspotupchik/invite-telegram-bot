package models

import "time"

type User struct {
	UserID     int64     `json:"user_id"`
	Balance    float64   `json:"balance"`
	ReferredBy *int64    `json:"referred_by"`
	JoinDate   time.Time `json:"join_date"`
	Language   string    `json:"language"`
	Referrals  []int64   `json:"referrals"`
}

type UserSession struct {
	State                 string
	AwaitingWalletAmount  float64
	AwaitingBalanceUserID int64
}

type Stats struct {
	Total int `json:"total"`
	Day   int `json:"day"`
	Week  int `json:"week"`
	Month int `json:"month"`
}
