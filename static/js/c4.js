colors = ["red", "yellow", "blue", "pink", "orange", "black"];

window.onload = function () {
    // update link box with current page link
    document.getElementById("link-text").innerHTML = window.location;

    // make play again button play again
    document.getElementById("playAgainButton").addEventListener("click", function () {
        $('#playAgainModal').modal('hide');
        playAgain();
    });

    var options = { backdrop: "static", keyboard: false }
    $('#playerNameModal').modal(options);

    document.getElementById("playerNameInputButton").addEventListener("click", function () {
        var playerName = document.getElementById("playerNameInput").value.trim()
        if (playerName != "") {
            $('#playerNameModal').modal('hide');
            connectToGame(playerName)
        } else {
            document.getElementById("playerNameModalMessage").innerHTML = "<strong>Please enter a valid username.</strong>"
        }
    })
}

function connectToGame(playerName) {
    // create websocket connection
    ws = new WebSocket("wss://" + window.location.host + "/ws?gameid=" + window.location.pathname.substr(1) + "&name=" + playerName);

    ws.onmessage = function (event) {
        var msg = JSON.parse(event.data);

        // check message is not emtpy
        if (msg.hasOwnProperty("message")) {
            // update message box
            document.getElementById("message-box").innerText = msg.message;

            // set global variables
            playerIndex = msg.playerIndex;
            playerTurn = msg.playerTurn;
            grid = msg.game.grid;
            isOver = msg.game.isOver;
            turn = msg.game.turn;

            drawBoard(grid);

            var players = msg.game.players
            drawPlayerList(players)

            // prompt to play again after game ends
            if (isOver && playerIndex != -1) {
                $('#playAgainModal').modal();
            }
        }
    }

    // shows an error if the websocket is terminated unexpectedly 
    ws.onclose = function (event) {
        if (!isOver) {
            document.getElementById("message-box").innerHTML = "Error: connection has been terminated.";
        }
    }

    // shows an error if there is a websocket error
    ws.onerror = function (event) {
        document.getElementById("message-box").innerHTML = "Error: connection has been terminated.";
    }
}

function playAgain() {
    var playerMove = { placement: -1, playAgain: true };
    ws.send(JSON.stringify(playerMove));
}

// makes a move at the selected slot if valid.
function selectSlot(x, y) {
    if (playerTurn) {
        if (isValidMove(x, y)) {
            p = (x + (y * grid[0].length));
            var playerMove = { placement: p };
            ws.send(JSON.stringify(playerMove));

            return true;
        }
    }
    return false;
}

// returns true if the move is valid
function isValidMove(x, y) {

    // out of bounds
    if (y >= grid.length || x >= grid[0].length) {
        return false;
    }

    // slot is not empty
    if (grid[y][x] != -1) {
        return false;
    }

    // slot below is empty
    if (y < grid.length - 1) {
        if (grid[y + 1][x] == -1) {
            return false;
        }
    }

    return true;
}

// draws the game board
function drawBoard(grid) {
    boardContainer = document.getElementById("board-container");
    board = document.createElement("div");

    var row = document.createElement("div");
    row.classList.add("row");
    row.classList.add("no-gutters");

    board.appendChild(row);

    for (i = 0; i < grid.length; i++) {

        for (j = 0; j < grid[0].length; j++) {
            var col = document.createElement("div");
            col.className = "col m-2";

            var circle = document.createElement("div");
            circle.classList.add("circle");

            if (grid[i][j] == -1) {
                circle.classList.add("grey");
                circle.classList.add("hover-" + colors[playerIndex]);
            } else {
                circle.classList.add(colors[grid[i][j]]);
            }

            circle.x = j;
            circle.y = i;

            circle.addEventListener("click", function () {
                if (selectSlot(this.x, this.y)) {
                    this.classList.remove("grey");
                    this.classList.add(colors[playerIndex]);
                }
            });

            col.appendChild(circle);
            row.appendChild(col);
        }

        // add row seperator but not after last row
        if (i != grid.length - 1) {
            var rowSeperator = document.createElement("div");
            rowSeperator.classList.add("w-100");
            row.appendChild(rowSeperator);
        }
    }

    boardContainer.innerHTML = "";
    boardContainer.appendChild(board);
}

function drawPlayerList(players) {
    var playerListContainer = document.getElementById("player-list-container")
    var playerList = document.createElement("ul");
    playerList.classList.add("list-group");

    for (var i = 0; i < players.length; i++) {
        var listItem = document.createElement("li");
        listItem.classList.add("list-group-item");
        listItem.innerText = players[i].name;
        if (i == turn % players.length) {
            listItem.classList.add("active");
        }
        playerList.appendChild(listItem);
    }
    playerListContainer.innerHTML = "";
    playerListContainer.appendChild(playerList);
}