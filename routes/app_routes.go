package routes

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"log"
	"strings"
	"time"
)

type client struct {
	connection *websocket.Conn
	username   string
	userID     string
} // Add more data To this type if needed

type Chat struct {
	Message string `json:"message"`
	From    string `json:"from"`
	To      string `json:"to"`
	Type    string `json:"type"`
	Time    string `json:"time"`
}

type Message struct {
	Event   string      `json:"event"`
	Payload interface{} `json:"payload"`
}

type RoomList struct {
	Rooms []string `json:"rooms"`
}

type WelcomeMessage struct {
	Rooms  []GroupInfo `json:"groups"`
	People []People    `json:"people"`
}

type People struct {
	Id       string `json:"id"`
	Username string `json:"username"`
}

type GroupInfo struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type Alert struct {
	Message string `json:"message"`
}

type Group struct {
	Name         string   `json:"name"`
	Id           string   `json:"id"`
	Participants []string `json:"participants"`
}

type joinGroupRequest struct {
	GroupId string `json:"name"`
	Client  client `json:"client"`
}

var clients = make(map[client]client) // Note: although large maps with pointer-like types (e.g. strings) as keys are slow, using pointers themselves as keys is acceptable and fast
var groups = make(map[string]Group)
var register = make(chan client)
var broadcast = make(chan Chat)
var unregister = make(chan client)
var createGroup = make(chan Group)
var joinGroup = make(chan joinGroupRequest)
var leaveGroup = make(chan joinGroupRequest)

func ChatRoutes(app *fiber.App) {
	app.Use(CheckUser)
	go runHub()
	app.Get("/chat/:username", websocket.New(func(c *websocket.Conn) {
		client := createClient(c)
		defer func() {
			unregister <- client
			c.Close()
		}()

		// Register the client

		register <- client

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return // Calls the deferred function, i.e. closes the connection on error
			}
			log.Println("msg : ", string(message), err)
			if messageType == websocket.TextMessage {
				socketEvent := Message{}
				err = json.Unmarshal(message, &socketEvent)
				if err != nil {
					log.Println("json error : #message , error:", err)
				}
				// Broadcast the received Chat
				handleSocketEvents(client, socketEvent)
			} else {
				log.Println("websocket Chat received of type", messageType)
			}
		}
	}))
}

func handleSocketEvents(client client, socketEvent Message) {
	switch socketEvent.Event {
	case "message":
		message := Chat{}
		b, err := json.Marshal(socketEvent.Payload)
		if err != nil {
			log.Println("json encode error:", err)
		}
		err = json.Unmarshal(b, &message)
		if err != nil {
			log.Println("json decode error:", err)
		}
		message.Time = time.Now().Format(time.RFC3339)
		message.Type = "text"
		message.From = "user/" + client.username
		broadcast <- message
	case "create_group":
		group := Group{}
		b, err := json.Marshal(socketEvent.Payload)
		if err != nil {
			log.Println("json encode error:", err)
		}
		err = json.Unmarshal(b, &group)
		if err != nil {
			log.Println("json decode error:", err)
		}
		group.Id = uuid.New().String()
		group.Participants = append(group.Participants, client.username)
		createGroup <- group
	case "join_group":
		join := joinGroupRequest{}
		b, err := json.Marshal(socketEvent.Payload)
		if err != nil {
			log.Println("json encode error:", err)
		}
		err = json.Unmarshal(b, &join)
		if err != nil {
			log.Println("json decode error:", err)
		}
		join.Client = client
		joinGroup <- join
	case "leave_group":
		join := joinGroupRequest{}
		b, err := json.Marshal(socketEvent.Payload)
		if err != nil {
			log.Println("json encode error:", err)
		}
		err = json.Unmarshal(b, &join)
		if err != nil {
			log.Println("json decode error:", err)
		}
		join.Client = client
		leaveGroup <- join
	}
}

func CheckUser(c *fiber.Ctx) error {
	path := strings.Split(c.Path(), "chat/")
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
	if !checkUsername(username) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": true,
			"msg":   "Username already taken",
		})
	}
	return c.Next()
}

func createClient(c *websocket.Conn) client {
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
			sendMessageToUser(connection, welcomeMessage(connection))

		case message := <-broadcast:
			_, receivers := getReceivers(message.To)
			// Send the Chat To all clients
			sendChat(receivers, message)

		case connection := <-unregister:
			// Remove the client from the hub
			delete(clients, connection)

		case group := <-createGroup:
			// Create group
			oldGroup := getGroup(group.Name)
			if oldGroup != nil {
				message := Chat{
					Message: "group already exists",
					From:    "server",
					To:      "user/" + group.Participants[0],
					Type:    "text",
					Time:    time.Now().Format(time.RFC3339),
				}
				_, receivers := getReceivers(message.To)
				sendChat(receivers, message)
				return
			}
			groups[group.Name] = group
			message := Chat{
				Message: "Group created",
				From:    "server",
				To:      "user/" + group.Participants[0],
				Type:    "text",
				Time:    time.Now().Format(time.RFC3339),
			}
			_, receivers := getReceivers(message.To)
			sendChat(receivers, message)

		case join := <-joinGroup:
			// Join group
			joinAGroup(join)

		case leave := <-leaveGroup:
			// Leave group
			leaveAGroup(leave)
		}
	}
}

func checkUsername(username string) bool {
	for user := range clients {
		if user.username == username {
			return false
		}
	}
	return true
}

func getGroup(name string) *Group {
	for _, grp := range groups {
		if grp.Name == name {
			return &grp
		}
	}
	return nil
}

func checkIsParticipant(group Group, client client) bool {
	for _, part := range group.Participants {
		if client.username == part {
			return true
		}
	}
	return false
}

func getGroupClients(to Group) []client {
	var groupClients []client
	participants := to.Participants
	for _, part := range participants {
		client := getClient(part)
		if client != nil {
			groupClients = append(groupClients, *client)
		}
	}
	return groupClients
}

func getClient(username string) *client {
	for client := range clients {
		if client.username == username {
			return &client
		}
	}
	return nil
}

func leaveAGroup(join joinGroupRequest) {
	group := getGroup(join.GroupId)
	if group == nil {
		sendMessageToUser(join.Client, Message{
			Event:   "alert",
			Payload: Alert{Message: "No group found to leave"},
		})
		return
	}
	if !checkIsParticipant(*group, join.Client) {
		sendMessageToUser(join.Client, Message{
			Event:   "alert",
			Payload: Alert{Message: "Not a member of group"},
		})
		return
	}
	group.Participants = RemoveParticipant(group.Participants, join.Client.username)
	groups[join.GroupId] = *group
	chat := Chat{
		Message: join.Client.username + " left group",
		From:    join.Client.username,
		To:      "/group/" + group.Name,
		Type:    "text",
		Time:    time.Now().Format(time.RFC3339),
	}
	_, receivers := getReceivers(chat.To)
	sendChat(receivers, chat)
}

func findPosition(users []string, user string) int {
	for i, str := range users {
		if str == user {
			return i
		}
	}
	return -1
}

func RemoveParticipant(participants []string, user string) []string {
	index := findPosition(participants, user)
	if index >= 0 {
		ret := make([]string, 0)
		ret = append(ret, participants[:index]...)
		return append(ret, participants[index+1:]...)
	} else {
		return participants
	}
}

func joinAGroup(join joinGroupRequest) {
	group := getGroup(join.GroupId)
	if group == nil {
		sendMessageToUser(join.Client, Message{
			Event:   "alert",
			Payload: Alert{Message: "No group found to join"},
		})
		return
	}
	if checkIsParticipant(*group, join.Client) {
		sendMessageToUser(join.Client, Message{
			Event:   "alert",
			Payload: Alert{Message: "Already a member of group"},
		})
		return
	}
	group.Participants = append(group.Participants, join.Client.username)
	groups[group.Name] = *group
	chat := Chat{
		Message: join.Client.username + " joined this group",
		From:    join.Client.username,
		To:      "/group/" + group.Name,
		Type:    "text",
		Time:    time.Now().Format(time.RFC3339),
	}
	_, receivers := getReceivers(chat.To)
	sendChat(receivers, chat)
}

func sendChat(receivers []client, message Chat) {
	for _, receiver := range receivers {
		if receiver.username != message.From {
			sendMessageToUser(receiver, Message{
				Event:   "chat",
				Payload: message,
			})
		}
	}
}

func sendMessageToUser(receiver client, message Message) {
	b, err := json.Marshal(message)
	if err != nil {
		log.Println("json encode error:", err)
	}
	if err := receiver.connection.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("write error:", err)
		receiver.connection.WriteMessage(websocket.CloseMessage, []byte{})
		receiver.connection.Close()
		delete(clients, receiver)
	}
}

func groupsList() []GroupInfo {
	var groupsList []GroupInfo
	for _, group := range groups {
		groupsList = append(groupsList, GroupInfo{
			Name: group.Name,
			Id:   group.Id,
		})
	}
	return groupsList
}

func getUsers(current client) []People {
	var people []People
	for i := range clients {
		if i.username != current.username {
			people = append(people, People{
				Id:       i.userID,
				Username: i.username,
			})
		}
	}
	return people
}

func welcomeMessage(user client) Message {
	welcome := WelcomeMessage{
		Rooms:  groupsList(),
		People: getUsers(user),
	}
	return Message{
		Event:   "welcome",
		Payload: welcome,
	}
}

func getReceivers(to string) (string, []client) {
	var Clients []client
	if strings.Contains(to, "group/") {
		groupId := strings.Split(to, "group/")[1]
		group := getGroup(groupId)
		if group == nil {
			return "Group", Clients
		}
		Clients = append(Clients, getGroupClients(*group)...)
		return "Group", Clients
	} else if strings.Contains(to, "user/") {
		username := strings.Split(to, "user/")[1]
		c := getClient(username)
		if c == nil {
			return "user", Clients
		}
		Clients = append(Clients, *c)
		return "user", Clients
	}
	return "default", Clients
}
