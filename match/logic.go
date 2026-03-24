// Package match contains pure game logic for Tic-Tac-Toe.
package match

// winLines defines all possible winning combinations on a 3×3 board.
var winLines = [][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // columns
	{0, 4, 8}, {2, 4, 6}, // diagonals
}

// CheckWinner returns the winning mark ("X" or "O") if there is one, or "" if no winner yet.
func CheckWinner(board [9]string) string {
	for _, line := range winLines {
		a, b, c := line[0], line[1], line[2]
		if board[a] != "" && board[a] == board[b] && board[a] == board[c] {
			return board[a]
		}
	}
	return ""
}

// IsDraw returns true if all cells are filled with no winner.
func IsDraw(board [9]string) bool {
	for _, cell := range board {
		if cell == "" {
			return false
		}
	}
	return true
}

// IsValidMove checks if a position (0–8) is valid and the cell is empty.
func IsValidMove(board [9]string, position int) bool {
	return position >= 0 && position <= 8 && board[position] == ""
}
