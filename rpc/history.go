package rpc

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
	"github.com/nebula-strike/nebula-server/match"
)

const (
	historyCollection = "nebula_strike"
	historyKey        = "combat_history"
)

// GetMatchHistoryResponse is the JSON payload for get_match_history.
type GetMatchHistoryResponse struct {
	Entries []match.CombatHistoryEntry `json:"entries"`
}

// GetMatchHistoryRPC returns the authenticated user's combat history from storage.
func GetMatchHistoryRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		b, _ := json.Marshal(GetMatchHistoryResponse{Entries: []match.CombatHistoryEntry{}})
		return string(b), nil
	}
	read := []*runtime.StorageRead{{
		Collection: historyCollection,
		Key:        historyKey,
		UserID:     userID,
	}}
	objs, err := nk.StorageRead(ctx, read)
	if err != nil {
		logger.Error("get_match_history StorageRead: %v", err)
		b, _ := json.Marshal(GetMatchHistoryResponse{Entries: []match.CombatHistoryEntry{}})
		return string(b), nil
	}
	if len(objs) == 0 || objs[0].Value == "" {
		b, _ := json.Marshal(GetMatchHistoryResponse{Entries: []match.CombatHistoryEntry{}})
		return string(b), nil
	}
	var resp GetMatchHistoryResponse
	if err := json.Unmarshal([]byte(objs[0].Value), &resp); err != nil {
		logger.Error("get_match_history unmarshal: %v", err)
		b, _ := json.Marshal(GetMatchHistoryResponse{Entries: []match.CombatHistoryEntry{}})
		return string(b), nil
	}
	if resp.Entries == nil {
		resp.Entries = []match.CombatHistoryEntry{}
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
