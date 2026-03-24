package match

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	historyCollection = "nebula_strike"
	historyKey        = "combat_history"
	maxHistoryEntries = 30
)

// CombatHistoryEntry is stored per user for the combat history RPC.
type CombatHistoryEntry struct {
	ID         string `json:"id"`
	Opponent   string `json:"opponent"`
	OpponentID string `json:"opponentId"`
	Mode       string `json:"mode"`
	Result     string `json:"result"` // victory | defeat | draw
	RPDelta    int    `json:"rpDelta"`
	OccurredAt string `json:"occurredAt"` // RFC3339
	MatchType  string `json:"matchType"`
}

type combatHistoryPayload struct {
	Entries []CombatHistoryEntry `json:"entries"`
}

// AppendCombatHistory prepends a row to the user's combat history (server-side).
func AppendCombatHistory(ctx context.Context, nk runtime.NakamaModule, logger runtime.Logger, userID string, entry CombatHistoryEntry) {
	if userID == "" {
		return
	}
	if entry.OpponentID != "" && entry.OpponentID == userID {
		return
	}
	read := []*runtime.StorageRead{{
		Collection: historyCollection,
		Key:        historyKey,
		UserID:     userID,
	}}
	objs, err := nk.StorageRead(ctx, read)
	if err != nil {
		logger.Error("StorageRead combat history: %v", err)
		return
	}
	var payload combatHistoryPayload
	if len(objs) > 0 && objs[0] != nil && objs[0].Value != "" {
		if err := json.Unmarshal([]byte(objs[0].Value), &payload); err != nil {
			logger.Error("Unmarshal combat history: %v", err)
			payload = combatHistoryPayload{}
		}
	}
	payload.Entries = append([]CombatHistoryEntry{entry}, payload.Entries...)
	if len(payload.Entries) > maxHistoryEntries {
		payload.Entries = payload.Entries[:maxHistoryEntries]
	}
	b, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Marshal combat history: %v", err)
		return
	}
	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{{
		Collection:      historyCollection,
		Key:             historyKey,
		UserID:          userID,
		Value:           string(b),
		Version:         "",
		PermissionRead:  1,
		PermissionWrite: 0,
	}})
	if err != nil {
		logger.Error("StorageWrite combat history: %v", err)
	}
}

// NewCombatEntry builds a history row with a unique id.
func NewCombatEntry(opponentName, opponentID, mode, result string, rpDelta int) CombatHistoryEntry {
	return CombatHistoryEntry{
		ID:         fmt.Sprintf("%d-%s", time.Now().UnixMilli(), opponentID),
		Opponent:   opponentName,
		OpponentID: opponentID,
		Mode:       mode,
		Result:     result,
		RPDelta:    rpDelta,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		MatchType:  "Ranked Match",
	}
}

// RecordMatchFinished writes combat history for both players (draw or decisive result).
func RecordMatchFinished(ctx context.Context, nk runtime.NakamaModule, logger runtime.Logger, gs *State, winnerUID, loserUID string, isDraw bool) {
	mode := gs.Mode
	if isDraw {
		uids := make([]string, 0, len(gs.Players))
		for uid := range gs.Players {
			uids = append(uids, uid)
		}
		if len(uids) != 2 {
			return
		}
		a, b := uids[0], uids[1]
		pa, pb := gs.Players[a], gs.Players[b]
		if pa == nil || pb == nil {
			return
		}
		AppendCombatHistory(ctx, nk, logger, a, NewCombatEntry(pb.Username, b, mode, "draw", 1))
		AppendCombatHistory(ctx, nk, logger, b, NewCombatEntry(pa.Username, a, mode, "draw", 1))
		return
	}
	if winnerUID == "" || loserUID == "" {
		return
	}
	w := gs.Players[winnerUID]
	l := gs.Players[loserUID]
	if w == nil || l == nil {
		return
	}
	AppendCombatHistory(ctx, nk, logger, winnerUID, NewCombatEntry(l.Username, loserUID, mode, "victory", 10))
	AppendCombatHistory(ctx, nk, logger, loserUID, NewCombatEntry(w.Username, winnerUID, mode, "defeat", 1))
}
