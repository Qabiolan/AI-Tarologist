package models

import (
	"encoding/json"
	"errors"
	"strings"
)

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	IsPremium bool   `json:"isPremium"`
	Info      string `json:"info"`
	// ?* Data format: " { paymentDate : paymentID}"
	PaymentHistory map[string]string `json:"paymentHistory" default:""`
}

func (u User) MarshalBinary() ([]byte, error) {
	return json.Marshal(u)
}

func NewUser(ID int, Username string, isPremium bool, Info string, PaymentHistory map[string]string) (User, error) {
	// Валидация ID
	if ID == 0 {
		return User{}, errors.New("id не может быть равен нулю")
	}

	// Валидация Username
	if strings.TrimSpace(Username) == "" {
		return User{}, errors.New("username не может быть пустым")
	}

	// Валидация длины Username
	if len(Username) > 50 {
		return User{}, errors.New("username не может быть длиннее 50 символов")
	}

	// Валидация PaymentHistory
	if PaymentHistory != nil {
		// Проверяем, что все ключи и значения в PaymentHistory не пустые
		for key, value := range PaymentHistory {
			if strings.TrimSpace(key) == "" {
				return User{}, errors.New("ключи в paymentHistory не могут быть пустыми")
			}
			if strings.TrimSpace(value) == "" {
				return User{}, errors.New("значения в paymentHistory не могут быть пустыми")
			}
		}

		return User{
			ID:             ID,
			Username:       strings.TrimSpace(Username),
			IsPremium:      isPremium,
			Info:           strings.TrimSpace(Info),
			PaymentHistory: PaymentHistory,
		}, nil
	}

	return User{
		ID:        ID,
		Username:  strings.TrimSpace(Username),
		IsPremium: isPremium,
		Info:      strings.TrimSpace(Info),
	}, nil
}
