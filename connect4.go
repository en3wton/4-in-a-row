package row

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const gameWidth = 7
const gameHeight = 6

type game struct {
	Players []*websocket.Conn          `json:"-"`
	Grid    [gameWidth][gameHeight]int `json:"grid"`
	Turn    int                        `json:"turn"`
	GameID  string                     `json:"gameId`
}

type info struct {
	Game       game   `json:"game"`
	Message    string `json:"message"`
	PlayerTurn bool   `json:"playerTurn"`
}

type playerMove struct {
	Placement int `json:"Placement"`
}

var upgrader = websocket.Upgrader{}
var games = make(map[string]game)

func main() {

}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Path

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("failed to upgrade connection")
	}

	// if game found, player joins it
	g, ok := games[gameID]
	if ok && len(g.Players) < 2 {
		g.registerPlayer(ws)
		return
	}

	// 404 if game not found
	w.WriteHeader(http.StatusNotFound)
}

// initializes a game
func (g game) playGame() {
	g.Turn = 0

	// wait for players
	for len(g.Players) < 2 {
		for _, player := range g.Players {
			msg := info{g, "Waiting for Players", false}
			player.WriteJSON(msg)
		}
		time.Sleep(100 * time.Millisecond)
	}

	for g.getWinner() == -1 {
		playerIndex := g.Turn % 2
		currentPlayer := g.Players[playerIndex]
		opponentIndex := (g.Turn + 1) % 2
		opponent := g.Players[opponentIndex]

		// notify player its their turn.
		msg := info{g, "Your Turn.", true}
		err := currentPlayer.WriteJSON(msg)
		if err != nil {
			g.forfeit(playerIndex)
			return
		}

		// notify other player its their opponents turn
		msg = info{g, "Opponents Turn.", false}
		err = opponent.WriteJSON(msg)
		if err != nil {
			g.forfeit(opponentIndex)
		}

		var move playerMove
		err = currentPlayer.ReadJSON(&move)
		if err != nil {
			g.forfeit(playerIndex)
			return
		}

		if !g.isValidMove() {
			g.forfeit(playerIndex)
			return
		}

		// next turn
		g.Turn++
	}

	winnerIndex := g.getWinner()
	winner := g.Players[winnerIndex]
	loser := g.Players[(winnerIndex+1)%2]

	msg := info{g, "You Won!", false}
	winner.WriteJSON(msg)

	msg = info{g, "You lose.", false}
	loser.WriteJSON(msg)

	for _, player := range g.Players {
		player.Close()
	}

	delete(games, g.GameID)
}

// Players[playerIndex] loses the game and opponent is notified
func (g game) forfeit(playerIndex int) {
	loser := g.Players[playerIndex]
	msg := info{g, "Error, you have forfeit the game.", false}
	loser.WriteJSON(msg)
	loser.Close()

	opponent := g.Players[(playerIndex+1)%2]
	msg = info{g, "Opponent has forfeit.", false}
	opponent.WriteJSON(msg)
	opponent.Close()

	delete(games, g.GameID)
}

// test if move is valid
func (g game) isValidMove(move playerMove) bool {
	return false
}

// test if game has been won
func (g game) getWinner() int {
	return -1
}

// adds a player to a game
func (g game) registerPlayer(c *websocket.Conn) {
	g.Players = append(g.Players, c)
}
