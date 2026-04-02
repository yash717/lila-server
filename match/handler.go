// Package match implements the Nakama authoritative match handler for Nebula Strike.
package match

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	tickRate      = 5     // ticks per second
	timedTurnMs   = 30000 // 30 seconds per turn in timed mode
	leaderboardID = "nebula_strike_global"
)

// Handler implements the runtime.Match interface for server-authoritative Tic-Tac-Toe.
type Handler struct{}

// NewHandler is the factory function registered with Nakama's match system.
func NewHandler(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
	return &Handler{}, nil
}

// MatchInit creates the initial pristine game state when a match is instantiated.
func (h *Handler) MatchInit(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	mode := "classic"
	if m, ok := params["mode"].(string); ok && m != "" {
		mode = m
	}

	host := ""
	if hn, ok := params["host"].(string); ok {
		host = hn
	}

	state := &State{
		Board:          [9]string{},
		Marks:          make(map[string]string),
		CurrentTurn:    "X",
		Players:        make(map[string]*PlayerInfo),
		Score:          map[string]int{"X": 0, "O": 0},
		Winner:         "",
		GameOver:       false,
		Mode:           mode,
		TurnDeadline:   0,
		MoveCount:      0,
		MatchStartTime: time.Now().UnixMilli(),
		Round:          1,
		PlayerCount:    0,
	}

	label, _ := json.Marshal(map[string]interface{}{
		"mode": mode,
		"open": true,
		"host": host,
	})

	logger.Info("Match created — mode: %s host: %s", mode, host)
	return state, tickRate, string(label)
}

// MatchJoinAttempt acts as a gatekeeper, rejecting connections if the match is full or over.
func (h *Handler) MatchJoinAttempt(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presence runtime.Presence, metadata map[string]string) (interface{}, bool, string) {
	gs := state.(*State)

	if len(gs.Players) >= 2 {
		return gs, false, "Match is full"
	}
	if gs.GameOver {
		return gs, false, "Match has ended"
	}

	return gs, true, ""
}

// MatchJoin assigns marks (X/O) to players and starts the game when 2 players are present.
func (h *Handler) MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	gs := state.(*State)

	for _, p := range presences {
		uid := p.GetUserId()

		// Assign mark: first joiner gets X, second gets O
		mark := "X"
		for _, existing := range gs.Marks {
			if existing == "X" {
				mark = "O"
				break
			}
		}
		gs.Marks[uid] = mark

		// Fetch username from account
		username := "Player"
		if account, err := nk.AccountGetId(ctx, uid); err == nil {
			if account.User.DisplayName != "" {
				username = account.User.DisplayName
			} else if account.User.Username != "" {
				username = account.User.Username
			}
		}

		gs.Players[uid] = &PlayerInfo{
			UserID:   uid,
			Username: username,
			Mark:     mark,
		}

		logger.Info("Player %s (%s) joined as %s", username, uid, mark)
	}

	gs.PlayerCount = len(gs.Players)

	// Start game when both players are present
	if len(gs.Players) == 2 {
		if gs.Mode == "timed" {
			gs.TurnDeadline = time.Now().UnixMilli() + timedTurnMs
		}

		// Mark match as full
		label, _ := json.Marshal(map[string]interface{}{
			"mode": gs.Mode,
			"open": false,
		})
		_ = dispatcher.MatchLabelUpdate(string(label))

		logger.Info("Game started — mode: %s", gs.Mode)
	}

	broadcastState(dispatcher, gs)
	return gs
}

// MatchLeave handles disconnections. If a player quits mid-game, the opponent wins.
func (h *Handler) MatchLeave(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	gs := state.(*State)

	for _, p := range presences {
		leavingUID := p.GetUserId()
		logger.Info("Player %s left", leavingUID)

		// If game is active and had 2 players, the opponent wins by forfeit
		if !gs.GameOver && len(gs.Players) == 2 {
			for uid := range gs.Players {
				if uid != leavingUID {
					gs.GameOver = true
					gs.Winner = uid

					result := GameOverMessage{
						Winner:     uid,
						WinnerMark: gs.Marks[uid],
						WinnerName: gs.Players[uid].Username,
						LoserName:  gs.Players[leavingUID].Username,
						Reason:     "opponent_disconnected",
						Board:      gs.Board,
						MoveCount:  gs.MoveCount,
						Duration:   (time.Now().UnixMilli() - gs.MatchStartTime) / 1000,
					}

					data, _ := json.Marshal(result)
					_ = dispatcher.BroadcastMessage(OpCodeGameOver, data, nil, nil, true)
					updateLeaderboard(ctx, nk, logger, uid, leavingUID)
					RecordMatchFinished(ctx, nk, logger, gs, uid, leavingUID, false)
					break
				}
			}
		}

		delete(gs.Players, leavingUID)
		delete(gs.Marks, leavingUID)
	}

	gs.PlayerCount = len(gs.Players)

	// End the match if no players remain
	if gs.PlayerCount == 0 {
		return nil
	}

	return gs
}

// MatchLoop is the game's tick function, processing moves and checking timers.
func (h *Handler) MatchLoop(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, messages []runtime.MatchData) interface{} {
	gs := state.(*State)

	if gs.GameOver || len(gs.Players) < 2 {
		return gs
	}

	// --- Timer check for timed mode ---
	if gs.Mode == "timed" && gs.TurnDeadline > 0 {
		if time.Now().UnixMilli() > gs.TurnDeadline {
			logger.Info("Timer expired for turn %s", gs.CurrentTurn)

			var timedOutUID, winnerUID string
			for uid, mark := range gs.Marks {
				if mark == gs.CurrentTurn {
					timedOutUID = uid
				} else {
					winnerUID = uid
				}
			}

			if timedOutUID != "" && winnerUID != "" {
				gs.GameOver = true
				gs.Winner = winnerUID

				result := GameOverMessage{
					Winner:     winnerUID,
					WinnerMark: gs.Marks[winnerUID],
					WinnerName: gs.Players[winnerUID].Username,
					LoserName:  gs.Players[timedOutUID].Username,
					Reason:     "timeout",
					Board:      gs.Board,
					MoveCount:  gs.MoveCount,
					Duration:   (time.Now().UnixMilli() - gs.MatchStartTime) / 1000,
				}

				data, _ := json.Marshal(result)
				_ = dispatcher.BroadcastMessage(OpCodeGameOver, data, nil, nil, true)
				updateLeaderboard(ctx, nk, logger, winnerUID, timedOutUID)
				RecordMatchFinished(ctx, nk, logger, gs, winnerUID, timedOutUID, false)
				return gs
			}
		}
	}

	// --- Process incoming moves ---
	for _, msg := range messages {
		if msg.GetOpCode() != OpCodeMove {
			continue
		}

		senderUID := msg.GetUserId()
		senderMark := gs.Marks[senderUID]

		// Validate: is it this player's turn?
		if senderMark != gs.CurrentTurn {
			logger.Warn("Player %s tried to move out of turn", senderUID)
			continue
		}

		// Parse move
		var move MoveMessage
		if err := json.Unmarshal(msg.GetData(), &move); err != nil {
			logger.Warn("Invalid move data from %s: %v", senderUID, err)
			continue
		}

		// Validate: is the position valid and empty?
		if !IsValidMove(gs.Board, move.Position) {
			logger.Warn("Invalid move position %d from %s", move.Position, senderUID)
			continue
		}

		// Apply the move
		gs.Board[move.Position] = senderMark
		gs.MoveCount++

		// Check for winner
		if winnerMark := CheckWinner(gs.Board); winnerMark != "" {
			var winnerUID, loserUID string
			for uid, mark := range gs.Marks {
				if mark == winnerMark {
					winnerUID = uid
				} else {
					loserUID = uid
				}
			}

			gs.GameOver = true
			gs.Winner = winnerUID

			result := GameOverMessage{
				Winner:     winnerUID,
				WinnerMark: winnerMark,
				WinnerName: gs.Players[winnerUID].Username,
				LoserName:  gs.Players[loserUID].Username,
				Reason:     "win",
				Board:      gs.Board,
				MoveCount:  gs.MoveCount,
				Duration:   (time.Now().UnixMilli() - gs.MatchStartTime) / 1000,
			}

			data, _ := json.Marshal(result)
			_ = dispatcher.BroadcastMessage(OpCodeGameOver, data, nil, nil, true)
			broadcastState(dispatcher, gs)
			updateLeaderboard(ctx, nk, logger, winnerUID, loserUID)
			RecordMatchFinished(ctx, nk, logger, gs, winnerUID, loserUID, false)
			return gs
		}

		// Check for draw
		if IsDraw(gs.Board) {
			gs.GameOver = true
			gs.Winner = "draw"

			result := GameOverMessage{
				Winner:     "draw",
				WinnerMark: "",
				Reason:     "draw",
				Board:      gs.Board,
				MoveCount:  gs.MoveCount,
				Duration:   (time.Now().UnixMilli() - gs.MatchStartTime) / 1000,
			}

			data, _ := json.Marshal(result)
			_ = dispatcher.BroadcastMessage(OpCodeGameOver, data, nil, nil, true)
			broadcastState(dispatcher, gs)

			// Participation points for both
			for uid, pi := range gs.Players {
				_, _ = nk.LeaderboardRecordWrite(ctx, leaderboardID, uid, pi.Username, 1, 0, nil, nil)
			}
			RecordMatchFinished(ctx, nk, logger, gs, "", "", true)
			return gs
		}

		// Switch turns
		if gs.CurrentTurn == "X" {
			gs.CurrentTurn = "O"
		} else {
			gs.CurrentTurn = "X"
		}

		// Reset timer for timed mode
		if gs.Mode == "timed" {
			gs.TurnDeadline = time.Now().UnixMilli() + timedTurnMs
		}

		broadcastState(dispatcher, gs)
	}

	return gs
}

// MatchTerminate is called when a match is being shut down.
func (h *Handler) MatchTerminate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, graceSeconds int) interface{} {
	logger.Info("Match terminated")
	return state
}

// MatchSignal handles external signals sent to an active match.
func (h *Handler) MatchSignal(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, data string) (interface{}, string) {
	return state, "signal_received"
}

// broadcastState sends the current game state to all connected clients.
func broadcastState(dispatcher runtime.MatchDispatcher, gs *State) {
	payload := StateMessage{
		Board:        gs.Board,
		CurrentTurn:  gs.CurrentTurn,
		Players:      gs.Players,
		Marks:        gs.Marks,
		Score:        gs.Score,
		GameOver:     gs.GameOver,
		Winner:       gs.Winner,
		Mode:         gs.Mode,
		TurnDeadline: gs.TurnDeadline,
		MoveCount:    gs.MoveCount,
		Round:        gs.Round,
		PlayerCount:  gs.PlayerCount,
	}

	data, _ := json.Marshal(payload)
	_ = dispatcher.BroadcastMessage(OpCodeStateUpdate, data, nil, nil, true)
}

func accountDisplayName(ctx context.Context, nk runtime.NakamaModule, userID string) string {
	if userID == "" {
		return ""
	}
	acc, err := nk.AccountGetId(ctx, userID)
	if err != nil || acc == nil || acc.User == nil {
		return ""
	}
	if acc.User.DisplayName != "" {
		return acc.User.DisplayName
	}
	if acc.User.Username != "" {
		return acc.User.Username
	}
	return ""
}

// updateLeaderboard awards points to winner and loser after a match.
func updateLeaderboard(ctx context.Context, nk runtime.NakamaModule, logger runtime.Logger, winnerUID, loserUID string) {
	wname := accountDisplayName(ctx, nk, winnerUID)
	lname := accountDisplayName(ctx, nk, loserUID)
	if wname == "" {
		wname = "Commander"
	}
	if lname == "" {
		lname = "Commander"
	}
	if _, err := nk.LeaderboardRecordWrite(ctx, leaderboardID, winnerUID, wname, 10, 0, nil, nil); err != nil {
		logger.Error("Failed to update leaderboard for winner: %v", err)
	}
	if _, err := nk.LeaderboardRecordWrite(ctx, leaderboardID, loserUID, lname, 1, 0, nil, nil); err != nil {
		logger.Error("Failed to update leaderboard for loser: %v", err)
	}
	logger.Info("Leaderboard updated — winner=%s, loser=%s", winnerUID, loserUID)
}

func init() {
	// Ensure Handler implements runtime.Match at compile time
	var _ runtime.Match = (*Handler)(nil)
	_ = fmt.Sprint
}
