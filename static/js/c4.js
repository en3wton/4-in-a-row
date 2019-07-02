window.onload = function () {
    ws = new WebSocket("ws://" + window.location.host + "/ws?gameid=" + window.location.pathname.substr(1))

    ws.onmessage = function (event) {
        console.log(event)
        var msg = JSON.parse(event.data);

        // update message box
        document.getElementById("message-box").innerHTML = msg.message;

        // set global variables
        playerIndex = msg.playerIndex;
        playerTurn = msg.playerTurn;
        grid = msg.game.grid;

        drawBoard(msg.game.grid);
    }

    ws.onopen = function () {
        console.log("socket opened");
    }
}

// makes a move at the selected slot if valid.
function selectSlot(x, y) {
    if (playerTurn) {
        if (isValidMove(x, y)) {
            p = (x + (y * grid[0].length));
            var playerMove = {placement:p}
            ws.send(JSON.stringify(playerMove))

            return true;
        }
    }
    return false
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
            return false
        }
    }

    return true
}

// draws the game board
function drawBoard(grid) {
    boardContainer = document.getElementById("board-container");
    board = document.createElement("div");

    var row = document.createElement("div");
    row.classList.add("row");

    board.appendChild(row);

    for (i = 0; i < grid.length; i++) {

        for (j = 0; j < grid[0].length; j++) {
            var col = document.createElement("div");

            col.className = "col m-1 circle";

            if (grid[i][j] == 0) {
                col.classList.add("red");
            } else if (grid[i][j] == 1) {
                col.classList.add("yellow");
            } else {
                col.classList.add("grey");

                if (playerIndex == 0) {
                    col.classList.add("hover-red");
                } else {
                    col.classList.add("hover-yellow");
                }
            }

            col.x = j;
            col.y = i;

            col.addEventListener("click", function () {
                if (selectSlot(this.x, this.y)) {
                    this.classList.remove("grey")
                    if (playerIndex == 0) {
                        this.classList.add("red")
                    } else {
                        this.classList.add("yellow")
                    }
                }
            });

            row.appendChild(col);
        }

        // add row seperator but not after last row
        if (i != grid.length - 1) {
            var rowSeperator = document.createElement("div");
            rowSeperator.classList.add("w-100");
            row.appendChild(rowSeperator);
        }
    }

    boardContainer.innerHTML = ""
    boardContainer.appendChild(board);
}