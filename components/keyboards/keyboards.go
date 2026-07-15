package keyboards

import (
	tele "gopkg.in/telebot.v4"
)

var menu = &tele.ReplyMarkup{ResizeKeyboard: true}
var BtnStarCard = menu.Text("⭐ Прогноз по звёздам")
var BtnNotalCard = menu.Text("🔮 Нотальная карта")
var BtnAnotherBuy = menu.Text("🃏 Совет от Таро")

func CreateBuyKeyboard() *tele.ReplyMarkup {
	menu.Reply(
		menu.Row(BtnStarCard),
		menu.Row(BtnNotalCard),
		menu.Row(BtnAnotherBuy),
	)
	return menu
}

func CreateMainMenu(webAppURL string) *tele.ReplyMarkup {
	mainMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	webAppBtn := mainMenu.WebApp("🔮 Открыть Таро", &tele.WebApp{URL: webAppURL})
	mainMenu.Reply(
		mainMenu.Row(webAppBtn),
	)
	return mainMenu
}
