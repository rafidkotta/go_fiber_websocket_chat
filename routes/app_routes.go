package routes

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"log"
	"strings"
)

type client struct{
	connection *websocket.Conn
	username   string
	userID     string
} // Add more data To this type if needed

type Message struct {
	Message string `json:"message"`
	From    string `json:"from"`
	To      string `json:"to"`
}

var clients = make(map[client]client) // Note: although large maps with pointer-like types (e.g. strings) as keys are slow, using pointers themselves as keys is acceptable and fast
var register = make(chan client)
var broadcast = make(chan Message)
var unregister = make(chan client)

func ChatRoutes(app *fiber.App) {
	app.Use(CheckUser)
	go runHub()
	app.Get("/chat/:username", websocket.New(func(c *websocket.Conn) {
		client := getClient(c)
		defer func() {
			unregister <- client
			c.Close()
		}()

		// Register the client

		register <- client

		for {
			messageType, message, err := c.ReadMessage()
			log.Println("msg : ",string(message), err)
			data := Message{}
			err = json.Unmarshal(message,&data)
			if err != nil {
				log.Println("json parse error:", err)
			}
			log.Println("json :",data.Message)
			log.Println("*************************************")
			data.From = client.username
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return // Calls the deferred function, i.e. closes the connection on error
			}

			if messageType == websocket.TextMessage {
				// Broadcast the received Message
				broadcast <- data
			} else {
				log.Println("websocket Message received of type", messageType)
			}
		}
	}))
}

func CheckUser (c *fiber.Ctx) error {
	path := strings.Split(c.Path(),"chat/")
	if len(path) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": true,
			"msg":   "Username not provided",
		})
	}
	username := path[1]
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": true,
			"msg":   "Username not provided",
		})
	}
	if !checkUsername(username){
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": true,
			"msg":   "Username already taken",
		})
	}
	return c.Next()
}

func getClient (c *websocket.Conn) client {
	username := c.Params("username")
	return client{
		connection: c,
		username:   username,
		userID:     uuid.New().String(),
	}
}

func runHub() {
	for {
		select {
		case connection := <-register:
			clients[connection] = client{}
			log.Println("connection registered")

		case message := <-broadcast:
			//log.Println("Message received:", message)

			// Send the Message To all clients
			for connection := range clients {
				if connection.username != message.From {
					if message.To != "all" && connection.username == message.To {
						if err := connection.connection.WriteMessage(websocket.TextMessage, []byte(message.Message)); err != nil {
							log.Println("write error:", err)
							connection.connection.WriteMessage(websocket.CloseMessage, []byte{})
							connection.connection.Close()
							delete(clients, connection)
						}
					}  else if message.To == "all" {
						if err := connection.connection.WriteMessage(websocket.TextMessage, []byte(message.Message)); err != nil {
							log.Println("write error:", err)
							connection.connection.WriteMessage(websocket.CloseMessage, []byte{})
							connection.connection.Close()
							delete(clients, connection)
						}
					}
				}
			}

		case connection := <-unregister:
			// Remove the client from the hub
			delete(clients, connection)

			log.Println("connection unregistered")
		}
	}
}

func checkUsername(username string) bool {
	for user := range clients{
		if user.username == username {
			return false
		}
	}
	return true
}
