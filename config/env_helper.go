package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

func LoadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file  \n" + err.Error())
	}
}

func GetEnvValue(key string) string {
	value := os.Getenv(key)
	return value
}

func GetEnv(key string, defaultValue string) string {
	value := GetEnvValue(key)
	if value == "" {
		return defaultValue
	}
	return value
}
