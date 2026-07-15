package config

import (
	"log"
	"os"

	env "github.com/joho/godotenv"
)

type Config struct {
	APIURL         string
	APIKEY         string
	BOTTOKEN       string
	REDIS_ADDR     string
	REDIS_PASSWORD string
}

func init() {
	if err := env.Load(); err != nil {
		log.Println(err)
	}
}
func Readconfig() Config {
	api_url, _ := os.LookupEnv("APIURL")
	api_key, _ := os.LookupEnv("APIKEY")
	bot_token, _ := os.LookupEnv("TELEGRAMBOTTOKEN")
	redisAddr, _ := os.LookupEnv("REDIS-ADDR")
	redisPassword, _ := os.LookupEnv("REDIS-PASSWORD")
	return Config{
		APIURL:         api_url,
		APIKEY:         api_key,
		BOTTOKEN:       bot_token,
		REDIS_ADDR:     redisAddr,
		REDIS_PASSWORD: redisPassword,
	}
}
