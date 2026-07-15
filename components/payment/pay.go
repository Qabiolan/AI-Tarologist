package payment

import (
	tele "gopkg.in/telebot.v4"
)

func CreatePayInvoice(
	ctx tele.Context,
	title string,
	description string,
	amount int64,
) *tele.Invoice {
	invoice := &tele.Invoice{
		Title:       title,
		Description: description,
		Payload:     "unique_payment",
		Token:       "",
		Currency:    "XTR",
		Prices: []tele.Price{
			{
				Label:  "Оплата запроса",
				Amount: int(amount),
			},
		},
	}
	return invoice
}
