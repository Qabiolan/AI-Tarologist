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
	Name        string
	FullName    string
	BirthDate   string
	BirthTime   string
	BirthPlace  string
	InfoCollected bool
}

// Хранилище данных пользователей (в памяти для простоты)
var userStorage = make(map[int]*UserData)

func getUserData(userID int) *UserData {
	if _, ok := userStorage[userID]; !ok {
		userStorage[userID] = &UserData{}
	}
	return userStorage[userID]
}

func main() {
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		fs := http.FileServer(http.Dir("./miniapp"))
		http.Handle("/", fs)
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
		webAppURL = "https://pushup-impart-unwrapped.ngrok-free.dev"
	}

	mainMenu := keyboards.CreateMainMenu(webAppURL)

	// Клавиатура для выбора расклада
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

	// Состояния диалога
	const (
		StateNone         = "none"
		StateWaitName     = "wait_name"
		StateWaitFullName = "wait_full_name"
		StateWaitBirthDate = "wait_birth_date"
		StateWaitBirthTime = "wait_birth_time"
		StateWaitBirthPlace = "wait_birth_place"
		StateReady        = "ready"
	)

	bot.Handle("/start", func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		userData := getUserData(userID)
		*userData = UserData{} // Reset

		if err := RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitName, 30*time.Minute); err != nil {
			fmt.Println(err)
		}

		return ctx.Send(message.StartText)
	})

	bot.Handle(tele.OnText, func(ctx tele.Context) error {
		userID := int(ctx.Sender().ID)
		text := ctx.Text()

		// Получаем состояние
		state, _ := RedisClient.Getter(redisCtx, fmt.Sprintf("state_%d", userID))
		if state == "" {
			state = StateNone
		}

		userData := getUserData(userID)

		switch state {
		case StateWaitName:
			userData.Name = strings.TrimSpace(text)
			RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateWaitFullName, 30*time.Minute)
			return ctx.Send(fmt.Sprintf(message.AskFullName, userData.Name))

		case StateWaitFullName:
			userData.FullName = strings.TrimSpace(text)
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

			// Сохраняем полную информацию в Redis
			fullInfo := fmt.Sprintf("Имя: %s, Полное имя: %s, Дата рождения: %s, Время рождения: %s, Место рождения: %s",
				userData.Name, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
			RedisClient.UpdateFieldUser(redisCtx, userID, "info", fullInfo, 24*30*time.Hour)

			thankYou := fmt.Sprintf(message.ThankYou,
				userData.Name,
				userData.FullName,
				userData.BirthDate,
				userData.BirthTime,
				userData.BirthPlace,
			)
			return ctx.Send(thankYou, spreadMenu)

		case StateReady:
			// Обработка выбора расклада или свободный чат
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
				// Свободный чат даже в режиме Ready
				return handleFreeChat(ctx, userData, text, RedisClient, redisCtx, userID)
			}

		case "free_chat":
			if text == "🔮 Общая характеристика" || text == "☀️ Расклад на сегодня" || text == "📅 Расклад на неделю" || text == "📆 Расклад на месяц" || text == "🌟 Расклад на год" {
				RedisClient.Setter(redisCtx, fmt.Sprintf("state_%d", userID), StateReady, 30*time.Minute)
				return handleSpread(ctx, userData, strings.TrimLeft(strings.TrimRight(text, " "), " "), RedisClient, redisCtx, userID)
			}
			return handleFreeChat(ctx, userData, text, RedisClient, redisCtx, userID)

		default:
			// Если данные уже собраны — свободный чат
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

		var prompt string
		var title string

		switch webAppData.Service {
		case "stars":
			prompt = fmt.Sprintf(message.StarsRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
			title = fmt.Sprintf("🔮 Общая характеристика для %s", userData.Name)
		case "natal":
			prompt = fmt.Sprintf(message.DailyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
			title = fmt.Sprintf("☀️ Расклад на сегодня для %s", userData.Name)
		case "advice":
			prompt = fmt.Sprintf(message.WeeklyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
			title = fmt.Sprintf("📅 Расклад на неделю для %s", userData.Name)
		default:
			return ctx.Send("Неизвестная услуга.")
		}

		ctx.Send(title)
		ctx.Send("Карты раскладываются... ✨")

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
	})

	fmt.Println("🤖 AI Таролог бот запущен!")
	bot.Start()
}

func handleSpread(ctx tele.Context, userData *UserData, spreadType string, RedisClient *r.RedisClient, redisCtx context.Context, userID int) error {
	if !userData.InfoCollected {
		return ctx.Send("Сначала пройди знакомство! Отправь /start")
	}

	ctx.Send("🔮 Раскладываю карты, ожидайте...")

	var prompt string
	var title string

	switch spreadType {
	case "general":
		prompt = fmt.Sprintf(message.StarsRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		title = fmt.Sprintf("🔮 Общая характеристика для %s", userData.Name)
	case "daily":
		prompt = fmt.Sprintf(message.DailyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		title = fmt.Sprintf("☀️ Расклад на сегодня для %s", userData.Name)
	case "weekly":
		prompt = fmt.Sprintf(message.WeeklyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		title = fmt.Sprintf("📅 Расклад на неделю для %s", userData.Name)
	case "monthly":
		prompt = fmt.Sprintf(message.MonthlyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
		title = fmt.Sprintf("📆 Расклад на месяц для %s", userData.Name)
	case "yearly":
		prompt = fmt.Sprintf(message.YearlyRequest, userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)
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
		userData.FullName, userData.BirthDate, userData.BirthTime, userData.BirthPlace)

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
