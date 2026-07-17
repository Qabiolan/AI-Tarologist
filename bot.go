package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"bot/components/chatgpt"
	"bot/components/keyboards"
	"bot/components/tarot"
	message "bot/components/message"
	r "bot/components/redis"
	configReader "bot/config"

	tele "gopkg.in/telebot.v4"
)

type WebAppData struct {
	Service    string `json:"service"`
	Zodiac     string `json:"zodiac"`
	ZodiacIcon string `json:"zodiacIcon"`
}

type UserSession struct {
	State         string `json:"state"`
	Name          string `json:"name"`
	BirthDate     string `json:"birthDate"`
	BirthTime     string `json:"birthTime"`
	BirthPlace    string `json:"birthPlace"`
	InfoCollected bool   `json:"infoCollected"`
}

func saveSession(r *r.RedisClient, ctx context.Context, userID int, session *UserSession) {
	key := fmt.Sprintf("session_%d", userID)
	data, _ := json.Marshal(session)
	r.Setter(ctx, key, string(data), 24*30*time.Hour)
}

func loadSession(r *r.RedisClient, ctx context.Context, userID int) *UserSession {
	key := fmt.Sprintf("session_%d", userID)
	data, err := r.Getter(ctx, key)
	if err != nil || data == "" {
		return &UserSession{State: "none"}
	}
	session := &UserSession{}
	if err := json.Unmarshal([]byte(data), session); err != nil {
		return &UserSession{State: "none"}
	}
	return session
}

func main() {
	if err := tarot.LoadDeck(); err != nil {
		fmt.Printf("Warning: Could not load tarot deck: %v\n", err)
	}

	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		fs := http.FileServer(http.Dir("./miniapp"))
		http.Handle("/", fs)
		http.Handle("/tarot_cards/", http.StripPrefix("/tarot_cards/", http.FileServer(http.Dir("./tarot_cards"))))
		fmt.Printf("🌙 Mini App server running on port %s\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			fmt.Printf("Mini App server error: %v\n", err)
		}
	}()

	bot_token := configReader.Readconfig().BOTTOKEN
	pref := tele.Settings{
		Token:  bot_token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	RedisClient := r.NewClient()
	redisCtx := context.Background()

	bot, err := tele.NewBot(pref)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	webAppURL := os.Getenv("WEBAPP_URL")
	if webAppURL == "" {
		webAppURL = "https://ai-tarologist-pjck.onrender.com"
	}

	mainMenu := keyboards.CreateMainMenu(webAppURL)

	spreadMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnGeneral := spreadMenu.Text("🔮 Общая характеристика")
	btnDaily := spreadMenu.Text("☀️ Расклад на сегодня")
	btnWeekly := spreadMenu.Text("📅 Расклад на неделю")
	btnMonthly := spreadMenu.Text("📆 Расклад на месяц")
	btnYearly := spreadMenu.Text("🌟 Расклад на год")
	btnChat := spreadMenu.Text("💬 Свободный чат")
	spreadMenu.Reply(
		spreadMenu.Row(btnGeneral),
		spreadMenu.Row(btnDaily, btnWeekly),
		spreadMenu.Row(btnMonthly, btnYearly),
		spreadMenu.Row(btnChat),
	)

	bot.Handle("/start", func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		session := &UserSession{State: "wait_name"}
		saveSession(RedisClient, redisCtx, userID, session)
		return ctx.Send(message.StartText)
	})

	bot.Handle(tele.OnText, func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		text := ctx.Text()
		session := loadSession(RedisClient, redisCtx, userID)

		switch session.State {
		case "wait_name":
			session.Name = strings.TrimSpace(text)
			session.State = "wait_birth_date"
			saveSession(RedisClient, redisCtx, userID, session)
			return ctx.Send(fmt.Sprintf(message.AskBirthDate, session.Name))

		case "wait_birth_date":
			session.BirthDate = strings.TrimSpace(text)
			session.State = "wait_birth_time"
			saveSession(RedisClient, redisCtx, userID, session)
			return ctx.Send(fmt.Sprintf(message.AskBirthTime, session.Name))

		case "wait_birth_time":
			session.BirthTime = strings.TrimSpace(text)
			session.State = "wait_birth_place"
			saveSession(RedisClient, redisCtx, userID, session)
			return ctx.Send(fmt.Sprintf(message.AskBirthPlace, session.Name))

		case "wait_birth_place":
			session.BirthPlace = strings.TrimSpace(text)
			session.InfoCollected = true
			session.State = "ready"
			saveSession(RedisClient, redisCtx, userID, session)

			thankYou := fmt.Sprintf(message.ThankYou,
				session.Name,
				session.BirthDate,
				session.BirthTime,
				session.BirthPlace,
			)
			return ctx.Send(thankYou, spreadMenu)

		case "ready":
			return handleButton(ctx, session, text, spreadMenu, RedisClient, redisCtx, userID)

		case "free_chat":
			if isSpreadButton(text) {
				session.State = "ready"
				saveSession(RedisClient, redisCtx, userID, session)
				return handleButton(ctx, session, text, spreadMenu, RedisClient, redisCtx, userID)
			}
			return handleFreeChat(ctx, session, text, RedisClient, redisCtx)

		default:
			if session.InfoCollected {
				return handleFreeChat(ctx, session, text, RedisClient, redisCtx)
			}
			return ctx.Send("Нажми /start чтобы начать знакомство ✨", mainMenu)
		}
	})

	bot.Handle("/menu", func(ctx tele.Context) error {
		return ctx.Send("Выбери расклад:", spreadMenu)
	})

	bot.Handle(tele.OnWebApp, func(ctx tele.Context) error {
		msg := ctx.Message()
		if msg == nil || msg.WebAppData == nil {
			return ctx.Send("Не удалось получить данные.")
		}
		data := msg.WebAppData.Data
		if data == "" {
			return ctx.Send("Не удалось получить данные.")
		}

		var webAppData WebAppData
		if err := json.Unmarshal([]byte(data), &webAppData); err != nil {
			return ctx.Send("Ошибка обработки данных.")
		}

		userID := int(ctx.Sender().ID)
		session := loadSession(RedisClient, redisCtx, userID)

		if !session.InfoCollected {
			return ctx.Send("Сначала пройди знакомство! Отправь /start")
		}

		var spreadType string
		switch webAppData.Service {
		case "stars":
			spreadType = "general"
		case "natal":
			spreadType = "daily"
		case "advice":
			spreadType = "weekly"
		default:
			return ctx.Send("Неизвестная услуга.")
		}

		return handleSpread(ctx, session, spreadType, RedisClient, redisCtx, userID)
	})

	fmt.Println("🤖 AI Таролог бот запущен!")
	bot.Start()
}

func isSpreadButton(text string) bool {
	buttons := []string{
		"🔮 Общая характеристика",
		"☀️ Расклад на сегодня",
		"📅 Расклад на неделю",
		"📆 Расклад на месяц",
		"🌟 Расклад на год",
	}
	for _, btn := range buttons {
		if text == btn {
			return true
		}
	}
	return false
}

func handleButton(ctx tele.Context, session *UserSession, text string, spreadMenu *tele.ReplyMarkup, RedisClient *r.RedisClient, redisCtx context.Context, userID int) error {
	switch text {
	case "🔮 Общая характеристика":
		return handleSpread(ctx, session, "general", RedisClient, redisCtx, userID)
	case "☀️ Расклад на сегодня":
		return handleSpread(ctx, session, "daily", RedisClient, redisCtx, userID)
	case "📅 Расклад на неделю":
		return handleSpread(ctx, session, "weekly", RedisClient, redisCtx, userID)
	case "📆 Расклад на месяц":
		return handleSpread(ctx, session, "monthly", RedisClient, redisCtx, userID)
	case "🌟 Расклад на год":
		return handleSpread(ctx, session, "yearly", RedisClient, redisCtx, userID)
	case "💬 Свободный чат":
		session.State = "free_chat"
		saveSession(RedisClient, redisCtx, userID, session)
		return ctx.Send("💬 Режим свободного чата активирован!\nЗадавай любые вопросы — я отвечу как опытный таролог.\n\nДля возврата к раскладам нажми /start", spreadMenu)
	default:
		return handleFreeChat(ctx, session, text, RedisClient, redisCtx)
	}
}

func handleSpread(ctx tele.Context, session *UserSession, spreadType string, RedisClient *r.RedisClient, redisCtx context.Context, userID int) error {
	if !session.InfoCollected {
		return ctx.Send("Сначала пройди знакомство! Отправь /start")
	}

	cardCount := 1
	switch spreadType {
	case "daily":
		cardCount = 3
	case "weekly":
		cardCount = 7
	case "monthly":
		cardCount = 4
	case "yearly":
		cardCount = 12
	case "general":
		cardCount = 5
	}

	cards, cardKeys := tarot.GetRandomCardsWithKeys(cardCount)

	ctx.Send("🔮 Раскладываю карты...")
	for i, card := range cards {
		imagePath := tarot.GetCardImagePathByKey(cardKeys[i])
		if imagePath != "" {
			photo := &tele.Photo{
				File:    tele.FromDisk(imagePath),
				Caption: fmt.Sprintf("✨ %s (%s)", card.Name, card.Number),
			}
			ctx.Send(photo)
			time.Sleep(500 * time.Millisecond)
		}
	}

	cardsText := tarot.FormatCardsForPrompt(cards)
	var prompt string
	var title string

	switch spreadType {
	case "general":
		prompt = fmt.Sprintf(message.StarsRequest, session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("🔮 Общая характеристика для %s", session.Name)
	case "daily":
		prompt = fmt.Sprintf(message.DailyRequest, session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("☀️ Расклад на сегодня для %s", session.Name)
	case "weekly":
		prompt = fmt.Sprintf(message.WeeklyRequest, session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("📅 Расклад на неделю для %s", session.Name)
	case "monthly":
		prompt = fmt.Sprintf(message.MonthlyRequest, session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("📆 Расклад на месяц для %s", session.Name)
	case "yearly":
		prompt = fmt.Sprintf(message.YearlyRequest, session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("🌟 Расклад на год для %s", session.Name)
	}

	ctx.Send(title)

	resp := chatgpt.RequestOpenAi(prompt)
	maxLen := 4096
	for i := 0; i < len(resp); i += maxLen {
		end := i + maxLen
		if end > len(resp) {
			end = len(resp)
		}
		ctx.Send(resp[i:end])
	}
	return ctx.Send("🌟 Обращайтесь ещё!")
}

func handleFreeChat(ctx tele.Context, session *UserSession, text string, RedisClient *r.RedisClient, redisCtx context.Context) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	ctx.Send("🔮 Думаю над ответом...")

	userInfo := fmt.Sprintf("Имя: %s, Дата рождения: %s, Время рождения: %s, Место рождения: %s",
		session.Name, session.BirthDate, session.BirthTime, session.BirthPlace)

	resp := chatgpt.RequestOpenAiWithContext(text, userInfo)
	maxLen := 4096
	for i := 0; i < len(resp); i += maxLen {
		end := i + maxLen
		if end > len(resp) {
			end = len(resp)
		}
		ctx.Send(resp[i:end])
	}
	return nil
}
