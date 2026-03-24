package match

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/heroiclabs/nakama-common/runtime"
)

// OnMatchmakerMatched creates an authoritative match when the matchmaker pairs players.
func OnMatchmakerMatched(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, entries []runtime.MatchmakerEntry) (string, error) {
	if len(entries) < 2 {
		return "", fmt.Errorf("need at least 2 players for a match, got %d", len(entries))
	}
	mode := "classic"
	if props := entries[0].GetProperties(); props != nil {
		if m, ok := props["mode"].(string); ok && m != "" {
			mode = m
		}
	}
	params := map[string]interface{}{
		"mode": mode,
	}
	matchID, err := nk.MatchCreate(ctx, ModuleName, params)
	if err != nil {
		logger.Error("MatchCreate from matchmaker failed: %v", err)
		return "", err
	}
	logger.Info("Matchmaker created match %s mode=%s", matchID, mode)
	return matchID, nil
}
