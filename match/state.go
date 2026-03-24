// Package match defines the game state types used by the Nakama match handler.
package match

// ModuleName is the Nakama match module id (must match RegisterMatch in main).
const ModuleName = "nebula_strike"

// OpCode defines the message types for client-server communication.
const (
	OpCodeMove        int64 = 1 // Client → Server: player makes a move
	OpCodeStateUpdate int64 = 2 // Server → Client: full game state update
	OpCodeGameOver    int64 = 3 // Server → Client: game result
)

// State holds all in-memory data for a single active match.
type State struct {
	Board          [9]string              `json:"board"`
	Marks          map[string]string      `json:"marks"`       // userId → "X" | "O"
	CurrentTurn    string                 `json:"currentTurn"` // "X" or "O"
	Players        map[string]*PlayerInfo `json:"players"`
	Score          map[string]int         `json:"score"`  // "X" → 0, "O" → 0
	Winner         string                 `json:"winner"` // "" | userId | "draw"
	GameOver       bool                   `json:"gameOver"`
	Mode           string                 `json:"mode"`         // "classic" | "timed"
	TurnDeadline   int64                  `json:"turnDeadline"` // unix ms for timed mode
	MoveCount      int                    `json:"moveCount"`
	MatchStartTime int64                  `json:"matchStartTime"` // unix ms
	Round          int                    `json:"round"`
	PlayerCount    int                    `json:"playerCount"`
}

// PlayerInfo stores identity and mark assignment for a match participant.
type PlayerInfo struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Mark     string `json:"mark"`
}

// MoveMessage is the payload sent by the client when making a move.
type MoveMessage struct {
	Position int `json:"position"`
}

// StateMessage is the payload broadcast to clients on each state update.
type StateMessage struct {
	Board        [9]string              `json:"board"`
	CurrentTurn  string                 `json:"currentTurn"`
	Players      map[string]*PlayerInfo `json:"players"`
	Marks        map[string]string      `json:"marks"`
	Score        map[string]int         `json:"score"`
	GameOver     bool                   `json:"gameOver"`
	Winner       string                 `json:"winner"`
	Mode         string                 `json:"mode"`
	TurnDeadline int64                  `json:"turnDeadline"`
	MoveCount    int                    `json:"moveCount"`
	Round        int                    `json:"round"`
	PlayerCount  int                    `json:"playerCount"`
}

// GameOverMessage is sent to clients when the game concludes.
type GameOverMessage struct {
	Winner     string    `json:"winner"`
	WinnerMark string    `json:"winnerMark"`
	WinnerName string    `json:"winnerName"`
	LoserName  string    `json:"loserName"`
	Reason     string    `json:"reason"` // "win" | "draw" | "timeout" | "opponent_disconnected"
	Board      [9]string `json:"board"`
	MoveCount  int       `json:"moveCount"`
	Duration   int64     `json:"duration"` // seconds
}
