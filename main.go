package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rafidkotta/go_websocket_chat/config"
	"github.com/rafidkotta/go_websocket_chat/routes"
	"log"
)

func main() {
	app := fiber.New()
	config.LoadEnv()
	app.Use("/ws", func(c *fiber.Ctx) error {
		if c.Get("host") == "localhost:3000" {
			c.Locals("Host", "Localhost:3000")
			return c.Next()
		}
		return c.Status(403).SendString("Request origin not allowed")
	})
	// Create chat routes
	routes.ChatRoutes(app)
	// Start server
	log.Println("App version : ", config.Version)
	config.StartServer(app)
}
