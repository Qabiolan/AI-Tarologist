package chatgpt

import (
	configReader "bot/config"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type ChatCompletion struct {
	ID       string   `json:"id"`
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Object   string   `json:"object"`
	Created  int64    `json:"created"`
	Choices  []Choice `json:"choices"`
}

type Choice struct {
	Logprobs           interface{} `json:"logprobs"`
	FinishReason       string      `json:"finish_reason"`
	NativeFinishReason string      `json:"native_finish_reason"`
	Index              int         `json:"index"`
	Message            Message     `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var systemPrompt = `Ты — опытный таролог и астролог с 10-летним стажем. Твоя мудрость основана на глубоком знании карт таро, натальных карт, астрологии и эзотерики.

Твои правила:
1. Всегда отвечай в роли таролога с 10-летним стажем
2. Используй мистический, но понятный язык
3. Давай конкретные советы, основанные на символике карт
4. Если спрашивают про таро/астрологию — отвечай подробно и профессионально
5. Если спрашивают про обычные вещи — отвечай с точки зрения мудрости таролога
6. Всегда добавляй элементы эзотерики и мистики в ответы
7. Будь добрым, но честным — говори правду, даже если она непростая
8. Используй эмодзи для создания атмосферы: 🔮 ✨ 🌙 ⭐ 🃏 🌌 💫
9. Отвечай на русском языке
10. Объём ответа — 2-5 предложений, если не просят подробнее

Примеры:
- Если спрашивают "Как дела?" — ответь как таролог, возможно с пророчеством или советом
- Если спрашивают "Что такое таро?" — объясни профессионально
- Если спрашивают "Сowell ли мне быть программистом?" — дай совет через карты
- Если спрашивают про погоду — ответь с точки зрения мистики и энергий`

func RequestOpenAi(message string) string {
	api_key := configReader.Readconfig().APIKEY
	client := &http.Client{}
	var stringData = fmt.Sprintf(`{
  "model": "tencent/hy3:free",
  "messages": [
    {
      "role": "system",
      "content": "%s"
    },
    {
      "role": "user",
      "content": "%s"
    }
  ]
}`, systemPrompt, message)
	var data = strings.NewReader(stringData)
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", data)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return "Ошибка при создании запроса. Попробуйте позже."
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api_key)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return "Ошибка при обращении к API. Попробуйте позже."
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return "Ошибка при чтении ответа. Попробуйте позже."
	}
	var chatCompletion ChatCompletion
	log.Println(string(bodyText))
	if err := json.Unmarshal([]byte(bodyText), &chatCompletion); err != nil {
		log.Printf("Error parsing response: %v", err)
		return "Ошибка при обработке ответа. Попробуйте позже."
	}
	log.Println(chatCompletion)
	choises := chatCompletion.Choices
	if len(choises) > 0 {
		content := chatCompletion.Choices[0].Message.Content
		return content
	} else {
		return "Произошла какая-то ошибка и ответ не был сгенерирован"
	}
}

func RequestOpenAiWithContext(message string, userInfo string) string {
	api_key := configReader.Readconfig().APIKEY
	client := &http.Client{}

	contextMsg := fmt.Sprintf("Информация о пользователе: %s\n\nВопрос пользователя: %s", userInfo, message)

	var stringData = fmt.Sprintf(`{
  "model": "tencent/hy3:free",
  "messages": [
    {
      "role": "system",
      "content": "%s"
    },
    {
      "role": "user",
      "content": "%s"
    }
  ]
}`, systemPrompt, contextMsg)
	var data = strings.NewReader(stringData)
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", data)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return "Ошибка при создании запроса. Попробуйте позже."
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api_key)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return "Ошибка при обращении к API. Попробуйте позже."
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return "Ошибка при чтении ответа. Попробуйте позже."
	}
	var chatCompletion ChatCompletion
	if err := json.Unmarshal([]byte(bodyText), &chatCompletion); err != nil {
		log.Printf("Error parsing response: %v", err)
		return "Ошибка при обработке ответа. Попробуйте позже."
	}
	choises := chatCompletion.Choices
	if len(choises) > 0 {
		content := chatCompletion.Choices[0].Message.Content
		return content
	} else {
		return "Произошла какая-то ошибка и ответ не был сгенерирован"
	}
}
