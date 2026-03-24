// Package rpc provides server-side RPC endpoints called by the frontend.
package rpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"github.com/nebula-strike/nebula-server/match"
)

const leaderboardID = "nebula_strike_global"

// CreateMatchRequest is the RPC input payload for creating a new match room.
type CreateMatchRequest struct {
	Mode string `json:"mode"`
}

// CreateMatchResponse is the RPC output with the generated match ID (room code).
type CreateMatchResponse struct {
	MatchID string `json:"matchId"`
}

// LeaderboardRecord represents a single player's leaderboard entry.
type LeaderboardRecord struct {
	OwnerID  string `json:"ownerId"`
	Username string `json:"username"`
	Score    int64  `json:"score"`
	Rank     int64  `json:"rank"`
}

// LeaderboardResponse is the RPC output containing all leaderboard records.
type LeaderboardResponse struct {
	Records []LeaderboardRecord `json:"records"`
	// MyRank is the authenticated user's row (rank + score), even if outside the top 20.
	MyRank *LeaderboardRecord `json:"myRank,omitempty"`
}

func leaderboardDisplayName(ctx context.Context, nk runtime.NakamaModule, r *api.LeaderboardRecord) string {
	if r == nil {
		return "Commander"
	}
	var stored string
	if r.Username != nil {
		stored = strings.TrimSpace(r.Username.Value)
	}
	if stored != "" && stored != "Unknown" {
		return stored
	}
	if acc, err := nk.AccountGetId(ctx, r.OwnerId); err == nil && acc != nil && acc.User != nil {
		if acc.User.DisplayName != "" {
			return acc.User.DisplayName
		}
		if acc.User.Username != "" {
			return acc.User.Username
		}
	}
	return "Commander"
}

// CreateMatchRPC handles the "create_match" RPC call.
// Called when a player clicks "CREATE ROOM" in the lobby.
func CreateMatchRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	mode := "classic"

	if payload != "" {
		var req CreateMatchRequest
		if err := json.Unmarshal([]byte(payload), &req); err == nil && req.Mode != "" {
			mode = req.Mode
		}
	}

	hostName := ""
	if userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok && userID != "" {
		if acc, err := nk.AccountGetId(ctx, userID); err == nil && acc != nil && acc.User != nil {
			if acc.User.DisplayName != "" {
				hostName = acc.User.DisplayName
			} else if acc.User.Username != "" {
				hostName = acc.User.Username
			}
		}
	}

	params := map[string]interface{}{
		"mode": mode,
		"host": hostName,
	}

	matchID, err := nk.MatchCreate(ctx, match.ModuleName, params)
	if err != nil {
		logger.Error("Failed to create match: %v", err)
		return "", err
	}

	logger.Info("Created match %s — mode: %s", matchID, mode)

	resp := CreateMatchResponse{MatchID: matchID}
	data, _ := json.Marshal(resp)
	return string(data), nil
}

// GetLeaderboardRPC handles the "get_leaderboard" RPC call.
// Returns the top 20 players from the global leaderboard.
func GetLeaderboardRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	records, _, _, _, err := nk.LeaderboardRecordsList(ctx, leaderboardID, nil, 20, "", 0)
	if err != nil {
		logger.Error("Failed to fetch leaderboard: %v", err)
		resp := LeaderboardResponse{Records: []LeaderboardRecord{}}
		data, _ := json.Marshal(resp)
		return string(data), nil
	}

	var entries []LeaderboardRecord
	for _, r := range records {
		entries = append(entries, LeaderboardRecord{
			OwnerID:  r.OwnerId,
			Username: leaderboardDisplayName(ctx, nk, r),
			Score:    r.Score,
			Rank:     r.Rank,
		})
	}

	if entries == nil {
		entries = []LeaderboardRecord{}
	}

	var myRank *LeaderboardRecord
	if uid, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok && uid != "" {
		_, ownerRecs, _, _, err2 := nk.LeaderboardRecordsList(ctx, leaderboardID, []string{uid}, 1, "", 0)
		if err2 == nil && len(ownerRecs) > 0 {
			o := ownerRecs[0]
			row := LeaderboardRecord{
				OwnerID:  o.OwnerId,
				Username: leaderboardDisplayName(ctx, nk, o),
				Score:    o.Score,
				Rank:     o.Rank,
			}
			myRank = &row
		}
	}

	resp := LeaderboardResponse{Records: entries, MyRank: myRank}
	data, _ := json.Marshal(resp)
	return string(data), nil
}
