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
	Logprobs           interface{} `json:"logprobs"` // может быть nil или структура, если потребуется
	FinishReason       string      `json:"finish_reason"`
	NativeFinishReason string      `json:"native_finish_reason"`
	Index              int         `json:"index"`
	Message            Message     `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func RequestOpenAi(message string) string {
	api_key := configReader.Readconfig().APIKEY
	client := &http.Client{}
	var stringData = fmt.Sprintf(`{
  "model": "google/gemma-4-26b-a4b-it:free",
  "messages": [
    {
      "role": "user",
	  "content": "%s",
    }
  ]
}`, message)
	var data = strings.NewReader(stringData)
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", data)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api_key)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var chatCompletion ChatCompletion
	log.Println(string(bodyText))
	if err := json.Unmarshal([]byte(bodyText), &chatCompletion); err != nil {
		panic(err)
	}
	log.Println(chatCompletion)
	choises := chatCompletion.Choices
	if len(choises) > 0 {
		content := chatCompletion.Choices[0].Message.Content
		return content
	} else {
		return "Произошла какая то ошибка и ответ не был сгенерирован"
	}
}
