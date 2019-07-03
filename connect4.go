package main

import (
	"crypto/sha1"
	"encoding/hex"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
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
	http.HandleFunc("/create/", gameCreateHandler)
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

// gameCreateHandler creates a game and redirects the user to it.
func gameCreateHandler(w http.ResponseWriter, r *http.Request) {
	h := sha1.New()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// create gameID, loop until it is unique.
	for {
		h.Write([]byte(strconv.Itoa(rnd.Int())))
		gameID := strings.ToLower(hex.EncodeToString(h.Sum(nil))[:6])

		_, exists := games[gameID]
		if !exists {
			g := newGame(gameID)
			games[gameID] = g
			go g.timeout()

			http.Redirect(w, r, "/"+gameID, 302)
			return
		}
	}
}

// wsHandler handles websocket connections and puts them in the right game.
func wsHandler(w http.ResponseWriter, r *http.Request) {
	args := r.URL.Query()
	gameID := args.Get("gameid")
	if gameID == "" {
		w.WriteHeader(http.StatusBadRequest)
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// if game found, player joins it
	g, ok := games[gameID]
	if ok {
		if len(g.Players) < 2 {
			g.registerPlayer(ws)
			return
		} else {
			tmpGame := newGame("")
			msg := info{*tmpGame, "Game Full.", false, 0}
			ws.WriteJSON(msg)
			ws.Close()
			return
		}
	} else {
		tmpGame := newGame("")
		msg := info{*tmpGame, "Lobby does not exist.", false, 0}
		ws.WriteJSON(msg)
		ws.Close()
		return
	}
}

// playGame starts the game loop.
func (g *game) playGame() {
	for {
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
			return
		}

		var move playerMove
		err = currentPlayer.ReadJSON(&move)
		if err != nil {
			g.forfeit(playerIndex)
			return
		}

		x, y := move.toCoordinates()
		if !g.isValidMove(x, y) {
			// no cheating
			g.forfeit(playerIndex)
			return
		}

		// execute move
		g.Grid[y][x] = playerIndex

		// check for game over
		if g.isWinningMove(x, y) {
			msg := info{*g, "You Win!", false, playerIndex}
			currentPlayer.WriteJSON(msg)

			msg = info{*g, "You Lose.", false, opponentIndex}
			opponent.WriteJSON(msg)

			g.endGame()
			return
		}

		if g.boardIsFull() {
			// notify of draw
			msg := info{*g, "Draw.", false, -1}
			currentPlayer.WriteJSON(msg)
			opponent.WriteJSON(msg)

			g.endGame()
			return
		}

		// next turn
		g.Turn++
	}
}

// forfeit notifies player[playerIndex] that they have lost, and their opponnent that they have won
func (g *game) forfeit(playerIndex int) {
	loser := g.Players[playerIndex]
	msg := info{*g, "Error, You have been disconnected.", false, playerIndex}
	loser.WriteJSON(msg)
	loser.Close()

	opponentIndex := (playerIndex + 1) % 2
	opponent := g.Players[opponentIndex]
	msg = info{*g, "Opponent has disconnected.", false, opponentIndex}
	opponent.WriteJSON(msg)
	opponent.Close()

	delete(games, g.GameID)
}

func (g *game) isWinningMove(x int, y int) bool {
	playerIndex := g.Grid[y][x]
	var consecutive int

	// check horizontal
	consecutive = 0
	for i := 0; i < len(g.Grid[0]); i++ {
		if g.Grid[y][i] == playerIndex {
			consecutive++
			if consecutive == 4 {
				return true
			}
		} else {
			consecutive = 0
		}
	}

	// check vertical
	consecutive = 0
	for i := 0; i < len(g.Grid); i++ {
		if g.Grid[i][x] == playerIndex {
			consecutive++
			if consecutive == 4 {
				return true
			}
		} else {
			consecutive = 0
		}
	}

	// check diagonal top-left to bottom-right
	consecutive = 0

	tmpX := x
	tmpY := y
	for tmpX > 0 && tmpY > 0 {
		tmpX--
		tmpY--
	}

	for tmpX < len(g.Grid[0]) && tmpY < len(g.Grid) {
		if g.Grid[tmpY][tmpX] == playerIndex {
			consecutive++
			if consecutive == 4 {
				return true
			}
		} else {
			consecutive = 0
		}
		tmpX++
		tmpY++
	}

	// check diagonal bottom-left to top-right
	consecutive = 0

	tmpX = x
	tmpY = y
	for tmpX < len(g.Grid[0])-1 && tmpY > 0 {
		tmpX++
		tmpY--
	}

	for tmpX >= 0 && tmpY < len(g.Grid) {
		if g.Grid[tmpY][tmpX] == playerIndex {
			consecutive++
			if consecutive == 4 {
				return true
			}
		} else {
			consecutive = 0
		}
		tmpX--
		tmpY++
	}

	return false
}

// boardIsFull checks if the board is full
func (g *game) boardIsFull() bool {
	for i := 0; i < len(g.Grid); i++ {
		for j := 0; j < len(g.Grid[0]); j++ {
			if g.Grid[i][j] == -1 {
				return false
			}
		}
	}
	return true
}

// endGame disconnects players and removes the game from the map.
func (g *game) endGame() {
	for _, player := range g.Players {
		player.Close()
	}
	delete(games, g.GameID)
}

// isValidMove returns true if the move is valid
func (g *game) isValidMove(x int, y int) bool {

	// out of bounds
	if y >= len(g.Grid) || x >= len(g.Grid[0]) {
		return false
	}

	// slot is not empty
	if g.Grid[y][x] != -1 {
		return false
	}

	// slot below is empty
	if y < len(g.Grid)-1 {
		if g.Grid[y+1][x] == -1 {
			return false
		}
	}

	return true
}

// toCoordinates converts a slot index into a set of coordinates on the grid.
func (m playerMove) toCoordinates() (x int, y int) {
	x = m.Placement % gameWidth
	y = m.Placement / gameWidth

	return
}

// registerPlayer adds a player to a game
func (g *game) registerPlayer(c *websocket.Conn) {
	g.Players = append(g.Players, c)

	if len(g.Players) == 2 {
		go g.playGame()
	} else {
		for i, player := range g.Players {
			msg := info{*g, "Waiting for an opponent.", false, i}
			err := player.WriteJSON(msg)
			if err != nil {
				g.forfeit(i)
			}
		}
	}

}

// timeout deletes the game if no one joins within 30 seconds.
// if the game has not started after 5 minutes the game is deleted.
func (g *game) timeout() {
	time.Sleep(30 * time.Second)
	if len(g.Players) == 0 {
		g.endGame()
	} else if len(g.Players) < 2 {
		time.Sleep(5 * time.Minute)
		if len(g.Players) < 2 {
			for _, player := range g.Players {
				msg := info{*g, "Lobby has timed out.", false, -1}
				player.WriteJSON(msg)
			}
			g.endGame()
		}
	}
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
