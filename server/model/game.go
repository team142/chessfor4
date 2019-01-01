package model

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/team142/chessfor4/io/ws"
	"github.com/team142/chessfor4/model/messages"
	"log"
)

const (
	MaxSupportedBoards = 2
)

func CreateGame(creator *Player) *Game {
	game := &Game{
		ID:      uuid.NewV4().String(),
		Players: make(map[int]*Player),
		Boards:  MaxSupportedBoards,
		Title:   fmt.Sprintf("%s's game", creator.Profile.Nick),
	}
	game.Players[1] = creator
	game.Owner = creator
	creator.SetTeamAndColor(1, game.Boards)
	return game
}

type ListOfGames struct {
	Games []map[string]string `json:"games"`
}

type Game struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Owner   *Player         `json:"-"`
	Players map[int]*Player `json:"players"`
	Boards  int             `json:"boards"`
}

func (game *Game) JoinGame(player *Player) bool {
	found, spot := game.findSpot()
	if !found {
		return false
	}
	player.SetTeamAndColor(spot, game.Boards)
	game.Players[spot] = player
	return true
}

func (game *Game) findSpot() (found bool, spot int) {
	if len(game.Players) >= game.GetMaxPlayers() {
		return false, 0
	}
	for s := 1; s <= game.GetMaxPlayers(); s++ {
		if game.Players[s] == nil {
			return true, s
		}
	}
	return false, 0
}
func (game *Game) Announce(b []byte) {
	for _, player := range game.Players {
		player.Profile.Client.Send <- b
	}
}

func (game *Game) GetMaxPlayers() int {
	return game.Boards * 2
}

func (game *Game) FindPlayerByClient(client *ws.Client) (result *Player, found bool) {
	for _, p := range game.Players {
		if p.Profile.Client == client {
			result, found = p, true
			return
		}
	}
	return
}

func (game *Game) ShareState() {
	b, err := json.Marshal(&game)
	if err != nil {
		log.Println(fmt.Sprintf("Error marshalling, %s", err))
	}
	for _, player := range game.Players {
		player.Profile.Client.Send <- b
	}

}

func (game *Game) SetupBoards() {
	for _, player := range game.Players {
		player.SetupBoard()
	}

}

func (game *Game) CanStart() bool {
	return game.GetMaxPlayers() == len(game.Players)
}

func (game *Game) Move(player *Player, move messages.MessageMove) (err error) {
	if !player.OwnsPiece(move.PieceID) {
		err = fmt.Errorf("player doesnt not own piece: %s", move.PieceID)
	}

	piece, found := player.GetPieceByID(move.PieceID)
	if !found {
		err = fmt.Errorf("could not find piece: %s", move.PieceID)
	}

	/*
		TODO: do other checks
	*/

	err = piece.TryMove(game, player.Color, move.FromX, move.FromY, move.ToX, move.ToY)
	return
}
