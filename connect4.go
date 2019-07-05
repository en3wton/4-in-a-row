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
	Players    []player                   `json:"players"`
	Grid       [gameHeight][gameWidth]int `json:"grid"`
	Turn       int                        `json:"turn"`
	GameID     string                     `json:"gameId"`
	NumPlayers int                        `json:"numPlayers"`
	IsOver     bool                       `json:"isOver"`
}

type player struct {
	Name   string          `json:"name"`
	Socket *websocket.Conn `json:"-"`
}

type info struct {
	Game        game   `json:"game"`
	Message     string `json:"message"`
	PlayerTurn  bool   `json:"playerTurn"`
	PlayerIndex int    `json:"playerIndex"`
}

type playerMove struct {
	Placement int  `json:"placement"`
	PlayAgain bool `json:"playAgain"`
}

var upgrader = websocket.Upgrader{}
var games = make(map[string]*game)

func main() {
	http.HandleFunc("/", gameHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/create", gameCreateHandler)
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
	args := r.URL.Query()
	numPlayersString := args.Get("players")

	var numPlayers int
	if numPlayersString != "" {
		var err error
		numPlayers, err = strconv.Atoi(numPlayersString)
		if err != nil {
			numPlayers = 2
		}
	} else {
		numPlayers = 2
	}

	h := sha1.New()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// create gameID, loop until it is unique.
	for {
		h.Write([]byte(strconv.Itoa(rnd.Int())))
		gameID := strings.ToLower(hex.EncodeToString(h.Sum(nil))[:6])

		_, exists := games[gameID]
		if !exists {
			g := newGame(gameID, numPlayers)
			games[gameID] = g
			go g.timeout()

			http.Redirect(w, r, "/"+gameID, 303)
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
		return
	}

	name := args.Get("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// if game found, player joins it
	g, ok := games[gameID]
	if ok {
		if len(g.Players) < g.NumPlayers {
			g.registerPlayer(ws, name)
			return
		} else {
			tmpGame := newGame("", -1)
			tmpGame.IsOver = true
			msg := info{*tmpGame, "Game Full.", false, -1}
			ws.WriteJSON(msg)
			ws.Close()
			return
		}
	} else {
		tmpGame := newGame("", -1)
		tmpGame.IsOver = true
		msg := info{*tmpGame, "Lobby does not exist.", false, -1}
		ws.WriteJSON(msg)
		ws.Close()
		return
	}
}

// playGame starts the game loop.
func (g *game) playGame() {
	for {
		playerIndex := g.Turn % g.NumPlayers
		currentPlayer := g.Players[playerIndex]

		// notify player its their turn.
		msg := info{*g, "Your Turn.", true, playerIndex}
		err := currentPlayer.Socket.WriteJSON(msg)
		if err != nil {
			g.forfeit(playerIndex)
			return
		}

		// notify other players its their opponents turn
		for i, player := range g.Players {
			if player != currentPlayer {
				msg = info{*g, currentPlayer.Name + "'s turn.", false, i}
				err = player.Socket.WriteJSON(msg)
				if err != nil {
					g.forfeit(i)
					return
				}
			}
		}

		var move playerMove
		err = currentPlayer.Socket.ReadJSON(&move)
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
			g.IsOver = true

			msg := info{*g, "You Win!", false, playerIndex}
			currentPlayer.Socket.WriteJSON(msg)

			for i, player := range g.Players {
				if player != currentPlayer {
					msg = info{*g, currentPlayer.Name + " wins.", false, i}
					player.Socket.WriteJSON(msg)
				}
			}

			g.playAgain()
			return
		}

		if g.boardIsFull() {
			g.IsOver = true

			// notify of draw
			for i, player := range g.Players {
				msg = info{*g, "Draw.", false, i}
				player.Socket.WriteJSON(msg)
			}

			g.playAgain()
			return
		}

		// next turn
		g.Turn++
	}
}

// playAgain handles each players play again prompt response.
func (g *game) playAgain() {
	delete(games, g.GameID)

	for _, p := range g.Players {
		go func(p player, gameID string) {
			var move playerMove
			p.Socket.ReadJSON(&move)
			if move.PlayAgain {
				game, exists := games[gameID]
				if exists {
					game.registerPlayer(p.Socket, p.Name)
				} else {
					game := newGame(gameID, g.NumPlayers)
					games[gameID] = game
					go g.timeout()

					game.registerPlayer(p.Socket, p.Name)
				}
			} else {
				p.Socket.Close()
			}
		}(p, g.GameID)
	}
}

// forfeit notifies player[playerIndex] that they have lost, and their opponnent that they have won
func (g *game) forfeit(playerIndex int) {
	g.IsOver = true
	loser := g.Players[playerIndex]
	msg := info{*g, "Error, You have been disconnected.", false, playerIndex}
	loser.Socket.WriteJSON(msg)
	loser.Socket.Close()

	for i, player := range g.Players {
		msg = info{*g, loser.Name + " has disconnected, game over.", false, i}
		player.Socket.WriteJSON(msg)
	}

	g.playAgain()
}

// isWinningMove returns true if the move at the specified coordinates resulted in a win for the player.
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
		player.Socket.Close()
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
func (g *game) registerPlayer(c *websocket.Conn, name string) {
	p := player{name, c}
	g.Players = append(g.Players, p)

	repeat := true
	for repeat {
		repeat = false

		numMissing := g.NumPlayers - len(g.Players)
		for i, player := range g.Players {
			msg := info{*g, "Waiting for " + strconv.Itoa(numMissing) + " more player(s) to join.", false, -1}
			err := player.Socket.WriteJSON(msg)
			if err != nil {
				// remove player that has left
				g.Players = append(g.Players[:i], g.Players[i+1:]...)
				repeat = true
			}
		}
	}

	if len(g.Players) == g.NumPlayers {
		go g.playGame()
	}
}

// timeout prevents the websockets from timing out.
func (g *game) timeout() {
	for !g.IsOver {
		time.Sleep(30 * time.Second)
		for i, player := range g.Players {
			err := player.Socket.WriteJSON("")
			if err != nil {
				g.Players = append(g.Players[:i], g.Players[i+1:]...)
			}
		}
		if len(g.Players) == 0 {
			g.endGame()
		}
	}
}

// newGame creates a new game object with the specified gameID
func newGame(gameID string, numPlayers int) *game {
	var players []player
	var grid [gameHeight][gameWidth]int
	turn := 0

	// fill grid with -1 (empty)
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			grid[i][j] = -1
		}
	}

	return &game{players, grid, turn, gameID, numPlayers, false}
}
