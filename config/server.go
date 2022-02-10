package config

import (
	"github.com/gofiber/fiber/v2"
	"log"
)

// StartServer func for starting a simple server.
func StartServer(a *fiber.App) {
	// Run server.
	if err := a.Listen(GetEnv(EnvServerURL, "localhost:3030")); err != nil {
		log.Printf("Oops... Server is not running! Reason: %v", err)
	}
}
