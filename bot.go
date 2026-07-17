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

type UserData struct {
	Name         string
	BirthDate    string
	BirthTime    string
	BirthPlace   string
	InfoCollected bool
}

var userStorage = make(map[int]*UserData)

func getUserData(userID int) *UserData {
	if _, ok := userStorage[userID]; !ok {
		userStorage[userID] = &UserData{}
	}
	return userStorage[userID]
}

func main() {
	// Load tarot deck
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

		// Serve tarot card images
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

	const (
		StateNone          = "none"
		StateWaitName      = "wait_name"
		StateWaitBirthDate = "wait_birth_date"
		StateWaitBirthTime = "wait_birth_time"
		StateWaitBirthPlace = "wait_birth_place"
		StateReady         = "ready"
	)

	bot.Handle("/start", func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		userData := getUserData(userID)
		*userData = UserData{}

		if err := RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitName, 30*time.Minute); err != nil {
			fmt.Println(err)
		}

		return ctx.Send(message.StartText)
	})

	bot.Handle(tele.OnText, func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		text := ctx.Text()

		state, _ := RedisClient.Getter(redisCtx, fmt.Sprintf("state_%d", userID))
		if state == "" {
			state = StateNone
		}

		userData := getUserData(userID)

		switch state {
		case StateWaitName:
			userData.Name = strings.TrimSpace(text)
			RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitBirthDate, 30*time.Minute)
			return ctx.Send(fmt.Sprintf(message.AskBirthDate, userData.Name))

		case StateWaitBirthDate:
			userData.BirthDate = strings.TrimSpace(text)
			RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitBirthTime, 30*time.Minute)
			return ctx.Send(fmt.Sprintf(message.AskBirthTime, userData.Name))

		case StateWaitBirthTime:
			userData.BirthTime = strings.TrimSpace(text)
			RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitBirthPlace, 30*time.Minute)
			return ctx.Send(fmt.Sprintf(message.AskBirthPlace, userData.Name))

		case StateWaitBirthPlace:
			userData.BirthPlace = strings.TrimSpace(text)
			userData.InfoCollected = true
			RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)

			fullInfo := fmt.Sprintf("Имя: %s, Дата рождения: %s, Время рождения: %s, Место рождения: %s",
				userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
			RedisClient.UpdateFieldUser(redisCtx, userID, "info", fullInfo, 24*30*time.Hour)

			thankYou := fmt.Sprintf(message.ThankYou,
				userData.Name,
				userData.BirthDate,
				userData.BirthTime,
				userData.BirthPlace,
			)
			return ctx.Send(thankYou, spreadMenu)

		case StateReady:
			if text == "🔮 Общая характеристика" {
				return handleSpread(ctx, userData, "general", RedisClient, redisCtx, userID)
			} else if text == "☀️ Расклад на сегодня" {
				return handleSpread(ctx, userData, "daily", RedisClient, redisCtx, userID)
			} else if text == "📅 Расклад на неделю" {
				return handleSpread(ctx, userData, "weekly", RedisClient, redisCtx, userID)
			} else if text == "📆 Расклад на месяц" {
				return handleSpread(ctx, userData, "monthly", RedisClient, redisCtx, userID)
			} else if text == "🌟 Расклад на год" {
				return handleSpread(ctx, userData, "yearly", RedisClient, redisCtx, userID)
			} else if text == "💬 Свободный чат" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), "free_chat", 30*time.Minute)
				return ctx.Send("💬 Режим свободного чата активирован!\nЗадавай любые вопросы — я отвечу как опытный таролог.\n\nДля возврата к раскладам нажми /start", spreadMenu)
			} else {
				return handleFreeChat(ctx, userData, text, RedisClient, redisCtx, userID)
			}

		case "free_chat":
			if text == "🔮 Общая характеристика" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, "general", RedisClient, redisCtx, userID)
			} else if text == "☀️ Расклад на сегодня" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, "daily", RedisClient, redisCtx, userID)
			} else if text == "📅 Расклад на неделю" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, "weekly", RedisClient, redisCtx, userID)
			} else if text == "📆 Расклад на месяц" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, "monthly", RedisClient, redisCtx, userID)
			} else if text == "🌟 Расклад на год" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, "yearly", RedisClient, redisCtx, userID)
			}
			return handleFreeChat(ctx, userData, text, RedisClient, redisCtx, userID)

		default:
			if userData.InfoCollected {
				return handleFreeChat(ctx, userData, text, RedisClient, redisCtx, userID)
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
		userData := getUserData(userID)

		if !userData.InfoCollected {
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

		return handleSpread(ctx, userData, spreadType, RedisClient, redisCtx, userID)
	})

	fmt.Println("🤖 AI Таролог бот запущен!")
	bot.Start()
}

func handleSpread(ctx tele.Context, userData *UserData, spreadType string, RedisClient *r.RedisClient, redisCtx context.Context, userID int) error {
	if !userData.InfoCollected {
		return ctx.Send("Сначала пройди знакомство! Отправь /start")
	}

	// Determine how many cards to draw
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

	// Draw random cards with keys for image lookup
	cards, cardKeys := tarot.GetRandomCardsWithKeys(cardCount)

	// Send card images
	ctx.Send("🔮 Раскладываю карты...")
	for i, card := range cards {
		imagePath := tarot.GetCardImagePathByKey(cardKeys[i])
		if imagePath != "" {
			// Send image with card name
			photo := &tele.Photo{
				File:    tele.FromDisk(imagePath),
				Caption: fmt.Sprintf("✨ %s (%s)", card.Name, card.Number),
			}
			ctx.Send(photo)
			time.Sleep(500 * time.Millisecond) // Small delay for better UX
		}
	}

	// Generate prompt with cards
	cardsText := tarot.FormatCardsForPrompt(cards)
	var prompt string
	var title string

	switch spreadType {
	case "general":
		prompt = fmt.Sprintf(message.StarsRequest, userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("🔮 Общая характеристика для %s", userData.Name)
	case "daily":
		prompt = fmt.Sprintf(message.DailyRequest, userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("☀️ Расклад на сегодня для %s", userData.Name)
	case "weekly":
		prompt = fmt.Sprintf(message.WeeklyRequest, userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("📅 Расклад на неделю для %s", userData.Name)
	case "monthly":
		prompt = fmt.Sprintf(message.MonthlyRequest, userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("📆 Расклад на месяц для %s", userData.Name)
	case "yearly":
		prompt = fmt.Sprintf(message.YearlyRequest, userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		prompt += "\n\n" + cardsText + "\n\nУчитывай выпавшие карты в своём толковании."
		title = fmt.Sprintf("🌟 Расклад на год для %s", userData.Name)
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

func handleFreeChat(ctx tele.Context, userData *UserData, text string, RedisClient *r.RedisClient, redisCtx context.Context, userID int) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	ctx.Send("🔮 Думаю над ответом...")

	userInfo := fmt.Sprintf("Имя: %s, Дата рождения: %s, Время рождения: %s, Место рождения: %s",
		userData.Name, userData.BirthDate, userData.BirthTime, userData.BirthPlace)

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
