package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const gameWidth = 7
const gameHeight = 6

type game struct {
	Players []*websocket.Conn          `json:"-"`
	Grid    [gameHeight][gameWidth]int `json:"grid"`
	Turn    int                        `json:"turn"`
	GameID  string                     `json:"gameId"`
}

type info struct {
	Game        game   `json:"game"`
	Message     string `json:"message"`
	PlayerTurn  bool   `json:"playerTurn"`
	PlayerIndex int    `json:"playerIndex"`
}

type playerMove struct {
	Placement int `json:"placement"`
}

var upgrader = websocket.Upgrader{}
var games = make(map[string]*game)

func main() {
	http.HandleFunc("/", gameHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/ws", wsHandler)

	log.Println("Starting on port 8292")
	http.ListenAndServe(":8292", nil)
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Path[1:]
	if len(gameID) != 0 {
		http.ServeFile(w, r, "html/game.html")
	} else {
		http.ServeFile(w, r, "html/home.html")
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	args := r.URL.Query()
	gameID := args.Get("gameid")
	if gameID == "" {
		w.WriteHeader(http.StatusBadRequest)
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("failed to upgrade connection")
	}

	// if game found, player joins it
	g, ok := games[gameID]
	if ok {
		if len(g.Players) < 2 {
			g.registerPlayer(ws)
			return
		} else {
			msg := "Game Full."
			ws.WriteJSON(msg)
			ws.Close()
			w.WriteHeader(http.StatusBadRequest)
		}
	} else {
		// if game not found then it is created
		g = newGame(gameID)
		games[gameID] = g

		g.registerPlayer(ws)
		go g.playGame()
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
}

// initializes a game
func (g *game) playGame() {
	g.Turn = 0

	// wait for players
	for len(g.Players) < 2 {
		log.Println("waiting for players, current players: " + strconv.Itoa(len(g.Players)))
		for i, player := range g.Players {

			msg := info{*g, "Waiting for Players...", false, i}
			player.WriteJSON(msg)
		}
		time.Sleep(1000 * time.Millisecond)
	}

	for g.getWinner() == -1 {
		playerIndex := g.Turn % 2
		currentPlayer := g.Players[playerIndex]
		opponentIndex := (g.Turn + 1) % 2
		opponent := g.Players[opponentIndex]

		// notify player its their turn.
		msg := info{*g, "Your Turn.", true, playerIndex}
		err := currentPlayer.WriteJSON(msg)
		if err != nil {
			g.forfeit(playerIndex)
			return
		}

		// notify other player its their opponents turn
		msg = info{*g, "Opponents Turn.", false, opponentIndex}
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

		if !g.isValidMove(move) {
			g.forfeit(playerIndex)
			return
		}

		// do move

		// next turn
		g.Turn++
	}

	winnerIndex := g.getWinner()
	winner := g.Players[winnerIndex]
	loserIndex := (winnerIndex + 1) % 2
	loser := g.Players[loserIndex]

	msg := info{*g, "You Won!", false, winnerIndex}
	winner.WriteJSON(msg)

	msg = info{*g, "You lose.", false, loserIndex}
	loser.WriteJSON(msg)

	for _, player := range g.Players {
		player.Close()
	}

	delete(games, g.GameID)
}

// Players[playerIndex] loses the game and opponent is notified
func (g *game) forfeit(playerIndex int) {
	loser := g.Players[playerIndex]
	msg := info{*g, "Error, you have forfeit the game.", false, playerIndex}
	loser.WriteJSON(msg)
	loser.Close()

	opponentIndex := (playerIndex + 1) % 2
	opponent := g.Players[opponentIndex]
	msg = info{*g, "Opponent has forfeit.", false, opponentIndex}
	opponent.WriteJSON(msg)
	opponent.Close()

	delete(games, g.GameID)
}

// isValidMove returns true if the move is valid
func (g game) isValidMove(move playerMove) bool {
	return true
}

// getWinner returns the winner of the game, -1 if not yet won
func (g game) getWinner() int {
	return -1
}

// registerPlayer adds a player to a game
func (g *game) registerPlayer(c *websocket.Conn) {
	g.Players = append(g.Players, c)
}

// newGame creates a new game object with the specified gameID
func newGame(gameID string) *game {
	var players []*websocket.Conn
	var grid [gameHeight][gameWidth]int
	turn := 0

	// fill grid with -1 (empty)
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			grid[i][j] = -1
		}
	}

	return &game{players, grid, turn, gameID}
}
