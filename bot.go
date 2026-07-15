package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func main() {
	// Start Mini App HTTP server
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

	if err := RedisClient.Setter(redisCtx, "State", "default", 10*time.Minute); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// WebApp URL - will be set to the Render domain
	webAppURL := os.Getenv("WEBAPP_URL")
	if webAppURL == "" {
		webAppURL = "https://pushup-impart-unwrapped.ngrok-free.dev"
	}

	mainMenu := keyboards.CreateMainMenu(webAppURL)
	menu := keyboards.CreateBuyKeyboard()

	bot.Handle("/start", func(ctx tele.Context) error {
		if err := RedisClient.SetNewUser(redisCtx, int(ctx.Sender().ID), ctx.Sender().Username, false, " ", nil, 24*30*time.Hour); err != nil {
			panic(err)
		}
		if err := RedisClient.Setter(redisCtx, "State", "infoWait", 5*time.Minute); err != nil {
			panic(err)
		}
		return ctx.Send(message.StartText, mainMenu)
	})

	bot.Handle(tele.OnText, func(ctx tele.Context) error {
		state, err := RedisClient.Getter(redisCtx, "State")
		if err != nil {
			panic(err)
		}
		if state == "infoWait" {
			userInfo := ctx.Text()
			if err := RedisClient.UpdateFieldUser(redisCtx, int(ctx.Sender().ID), "info", userInfo, 24*30*time.Hour); err != nil {
				panic(err)
			}
			return ctx.Send("✨ Спасибо за информацию! Теперь нажми кнопку «Открыть Таро» внизу, чтобы получить свой расклад.", mainMenu)
		}
		return ctx.Send("Нажми кнопку «Открыть Таро» внизу, чтобы начать расклад ✨", mainMenu)
	})

	bot.Handle("/menu", func(ctx tele.Context) error {
		return ctx.Send("Выбери услугу:", menu)
	})

	bot.Handle(&keyboards.BtnStarCard, func(ctx tele.Context) error {
		user, err := RedisClient.ReadUser(redisCtx, int(ctx.Sender().ID))
		if err != nil || user.Info == " " || user.Info == "" {
			return ctx.Send("Сначала расскажи о себе! Отправь /start и поделись информацией.")
		}
		ctx.Send("⭐ Составляю прогноз по звёздам, ожидайте...")
		text := fmt.Sprintf(message.StarRequest, user.Info)
		resp := chatgpt.RequestOpenAi(text)
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

	bot.Handle(&keyboards.BtnNotalCard, func(ctx tele.Context) error {
		user, err := RedisClient.ReadUser(redisCtx, int(ctx.Sender().ID))
		if err != nil || user.Info == " " || user.Info == "" {
			return ctx.Send("Сначала расскажи о себе! Отправь /start и поделись информацией.")
		}
		ctx.Send("🔮 Составляю нотальную карту, ожидайте...")
		text := fmt.Sprintf(message.NotalMap, user.Info)
		resp := chatgpt.RequestOpenAi(text)
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

	bot.Handle(&keyboards.BtnAnotherBuy, func(ctx tele.Context) error {
		user, err := RedisClient.ReadUser(redisCtx, int(ctx.Sender().ID))
		if err != nil || user.Info == " " || user.Info == "" {
			return ctx.Send("Сначала расскажи о себе! Отправь /start и поделись информацией.")
		}
		ctx.Send("🃏 Вытягиваю карту и составляю совет, ожидайте...")
		text := fmt.Sprintf(message.TaroAdvice, user.Info)
		resp := chatgpt.RequestOpenAi(text)
		return ctx.Send(resp)
	})

	// Обработка данных из Mini App
	bot.Handle(tele.OnWebApp, func(ctx tele.Context) error {
		msg := ctx.Message()
		if msg == nil || msg.WebAppData == nil {
			return ctx.Send("Не удалось получить данные. Попробуйте ещё раз.")
		}
		data := msg.WebAppData.Data
		if data == "" {
			return ctx.Send("Не удалось получить данные. Попробуйте ещё раз.")
		}

		var webAppData WebAppData
		if err := json.Unmarshal([]byte(data), &webAppData); err != nil {
			return ctx.Send("Ошибка обработки данных. Попробуйте ещё раз.")
		}

		user, err := RedisClient.ReadUser(redisCtx, int(ctx.Sender().ID))
		if err != nil || user.Info == " " || user.Info == "" {
			return ctx.Send("Сначала расскажи о себе! Отправь /start и поделись информацией.")
		}

		var prompt string
		var title string

		switch webAppData.Service {
		case "stars":
			prompt = fmt.Sprintf(message.StarRequest, user.Info)
			title = fmt.Sprintf("⭐ Звёздный прогноз для %s %s", webAppData.ZodiacIcon, webAppData.Zodiac)
		case "natal":
			prompt = fmt.Sprintf(message.NotalMap, user.Info)
			title = fmt.Sprintf("🔮 Нотальная карта для %s %s", webAppData.ZodiacIcon, webAppData.Zodiac)
		case "advice":
			prompt = fmt.Sprintf(message.TaroAdvice, user.Info)
			title = fmt.Sprintf("🃏 Совет от Таро для %s %s", webAppData.ZodiacIcon, webAppData.Zodiac)
		default:
			return ctx.Send("Неизвестная услуга. Попробуйте ещё раз.")
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
		return ctx.Send("🌟 Обращайтесь ещё! Для нового расклада нажмите «Открыть Таро».")
	})

	bot.Handle("/testPay", func(ctx tele.Context) error {
		user, err := RedisClient.ReadUser(redisCtx, int(ctx.Sender().ID))
		if err != nil {
			panic(err)
		}
		text := fmt.Sprintf(message.TaroAdvice, user.Info)
		resp := chatgpt.RequestOpenAi(text)
		return ctx.Send(resp)
	})

	fmt.Println("🤖 AI Таролог бот запущен!")
	bot.Start()
}
