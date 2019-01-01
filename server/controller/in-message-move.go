package controller

import (
	"encoding/json"
	"fmt"
	"github.com/team142/chessfor4/io/ws"
	"github.com/team142/chessfor4/model"
	"github.com/team142/chessfor4/model/messages"
	"log"
)

func handleInMessageMove(server *model.Server, client *ws.Client, msg []byte) {
	foundGame, game := server.FindGameByClient(client)
	if !foundGame {
		log.Println(fmt.Sprintf("Error finding game"))
		return
	}

	player, _ := game.FindPlayerByClient(client)

	var message messages.MessageMove
	if err := json.Unmarshal(msg, &message); err != nil {
		log.Println(fmt.Sprintf("Error unmarshaling, %s", err))
		return
	}

	log.Println(">> Moving ")
	err := game.Move(player, message)
	if err != nil {
		log.Println(fmt.Sprintf("Error trying to move, %s", err))
		//TODO: send error message to players
		return
	}
	game.ShareState()

}
