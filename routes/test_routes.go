package routes

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"log"
)

func TestRoutes(app *fiber.App) {
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		fmt.Println(c.Locals("Host")) // "Localhost:3000"
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			message := string(msg)
			if message == "hello" {
				err = c.WriteMessage(mt, []byte("hai"))
				if err != nil {
					log.Println("write:", err)
					break
				}
			}  else {
				log.Printf("recv: %s", msg)
				err = c.WriteMessage(mt, msg)
				if err != nil {
					log.Println("write:", err)
					break
				}
			}
		}
	}))
}
