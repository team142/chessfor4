package model

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/team142/chessfor4/io/ws"
	"log"
)

const (
	maxSupportedBoards = 2
)

//CreateGame starts a game with a player
func CreateGame(creator *Player) *Game {
	game := &Game{
		ID:       uuid.NewV4().String(),
		Players:  make(map[int]*Player),
		Boards:   maxSupportedBoards,
		Title:    fmt.Sprintf("%s's game", creator.Profile.Nick),
		commands: make(chan func(*Game), 256),
		stop:     make(chan bool),
	}
	game.Players[1] = creator
	game.Owner = creator
	creator.SetTeamColorAndBoard(1, game.Boards)
	go game.run()
	return game
}

//ListOfGames describes a list of games on the server
type ListOfGames struct {
	Games []map[string]string `json:"games"`
}

//Game describes a game on the server
type Game struct {
	ID                 string          `json:"id"`
	Started            bool            `json:"started"`
	Title              string          `json:"title"`
	Owner              *Player         `json:"owner"`
	Players            map[int]*Player `json:"players"`
	Boards             int             `json:"boards"`
	CanStartBeforeFull bool            `json:"canStartBeforeFull"`
	commands           chan func(*Game)
	stop               chan bool
}

func (game *Game) DoWork(f func(*Game)) {
	game.commands <- f
}

func (game *Game) run() {
	for {
		select {
		case command := <-game.commands:
			command(game)
		case <-game.stop:
			close(game.commands)
			close(game.stop)
			log.Println("Stopping game runner")
			return
		}
	}
}

func (game *Game) Stop() {
	game.stop <- true
}

//JoinGame gets a player into a game
func (game *Game) JoinGame(player *Player) bool {
	found, spot := game.findSpot()
	if !found {
		return false
	}
	player.SetTeamColorAndBoard(spot, game.Boards)
	game.Players[spot] = player
	return true
}

func (game *Game) findSpot() (found bool, spot int) {
	if len(game.Players) >= game.MaxPlayers() {
		return false, 0
	}
	for s := 1; s <= game.MaxPlayers(); s++ {
		if game.Players[s] == nil {
			return true, s
		}
	}
	return false, 0
}

func (game *Game) findPiece(pieceID string) (found bool, piece *Piece, player *Player) {
	for _, player := range game.Players {
		piece, found := player.GetPieceByID(pieceID)
		if found {
			return true, piece, player
		}
	}
	found = false
	return
}

//StartGame starts the game for all players
func (game *Game) StartGame() {
	ok, msg := game.IsReadyToStart()
	if !ok {
		reply := CreateMessageError("Failed to start game", msg)
		b, _ := json.Marshal(reply)
		game.Owner.Profile.Client.Send <- b
		return
	}

	game.SetupBoards()
	game.Started = true
	game.ShareState()

}

//Announce announces something to all players
func (game *Game) Announce(b []byte) {
	for _, player := range game.Players {
		player.Profile.Client.Send <- b
	}
}

//MaxPlayers determines the max number of players
func (game *Game) MaxPlayers() int {
	return game.Boards * 2
}

//PlayerByClient for easy access
func (game *Game) PlayerByClient(client *ws.Client) (result *Player, seat int, found bool) {
	for seat, p := range game.Players {
		if p.Profile.Client == client {
			return p, seat, true
		}
	}
	return
}

//ShareState tells all players what is going on
func (game *Game) ShareState() {
	reply := CreateMessageShareState(game)
	b, _ := json.Marshal(reply)
	for _, player := range game.Players {
		player.Profile.Client.Send <- b
	}

}

//SetupBoards initializes all boards in the game
func (game *Game) SetupBoards() {
	for _, player := range game.Players {
		player.SetupBoard()
	}

}

//IsReadyToStart checks that the game can start
func (game *Game) IsReadyToStart() (ok bool, message string) {
	if game.CanStartBeforeFull {
		ok = true
		return
	}
	ok = game.MaxPlayers() == len(game.Players)
	if !ok {
		message = "Not enough players"
	}
	return
}

//Move moves a piece
func (game *Game) Move(client *ws.Client, message MessageMove) (didMove bool) {
	log.Println(">> Moving ")
	player, _, _ := game.PlayerByClient(client)
	pieceFound, piece, piecePlayer := game.findPiece(message.PieceID)

	if piece.isEqual(message) {
		log.Println("No move")
		return
	}

	defer game.ShareState()

	if player != piecePlayer {
		log.Println("Player does not own piece, " + message.PieceID)
		return
	}

	if !player.MyTurn {
		log.Println("Not my turn!, " + message.PieceID)
		return
	}

	if !pieceFound {
		log.Println("Piece not found, " + message.PieceID)
		return
	}

	//Check for bad state
	if message.Cache == false && message.Board == 0 {
		log.Println("Piece must be on board or in cache, not neither")
		return
	}

	if !piece.CanMoveLikeThat(player, message) {
		return
	}

	/*
		TODO: do other checks
	*/
	//if !player.OwnsPiece(move.PieceID) {
	//	err = fmt.Errorf("player doesnt not own piece: %s", move.PieceID)
	//}

	didMove = true
	piece.Move(message)
	return
}

func (game *Game) ChangeSeat(client *ws.Client, seat int) {
	if game.Players[seat] != nil {
		msg := CreateMessageError("Failed to move seats", "Seat taken")
		b, _ := json.Marshal(msg)
		client.Send <- b
		return
	}
	currentSeat := 0
	var currentPlayer *Player
	for seatN, playerN := range game.Players {
		if playerN.Profile.Client == client {
			currentSeat = seatN
			currentPlayer = playerN
			break
		}
	}
	if currentSeat == 0 {
		log.Fatal("Could not find client in game.. dying")
	}
	delete(game.Players, currentSeat)
	game.Players[seat] = currentPlayer
	currentPlayer.SetTeamColorAndBoard(seat, game.Boards)
	game.ShareState()

}

func (game *Game) RemoveClient(client *ws.Client) {
	pid := 0
	for seat, player := range game.Players {
		if player.Profile.Client == client {
			pid = seat
			break
		}
	}
	if pid != 0 {
		log.Println(">> Removing player with seat ", pid)
		delete(game.Players, pid)
	} else {
		log.Println(">> Could not find player to remove from game")
	}

	fmt.Println("There are now how many players ", len(game.Players))

	if game.Owner.Profile.Client == client {
		game.Owner = nil
		for a := range game.Players {
			log.Println(">> New owner is ", game.Players[a].Profile.Nick)
			game.Owner = game.Players[a]
			break
		}
		if game.Owner == nil && len(game.Players) > 0 {
			log.Println("Error: did not assign new owner.")
		}

	}
	game.ShareState()

}

func (game *Game) changeMoveFrom(client *ws.Client) {
	var newPlayer *Player
	player, seat, _ := game.PlayerByClient(client)
	if seat <= game.Boards {
		newPlayer = game.Players[seat+2]
	} else {
		newPlayer = game.Players[seat-2]
	}
	player.MyTurn = false
	newPlayer.MyTurn = true

}
