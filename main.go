// Package main is the Nakama Go plugin entry point for Nebula Strike.
// It registers the match handler, leaderboard, and RPC endpoints.
package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/runtime"
	"github.com/nebula-strike/nebula-server/match"
	"github.com/nebula-strike/nebula-server/rpc"
)

const leaderboardID = "nebula_strike_global"

// InitModule is the plugin entry point called by Nakama on startup.
func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	logger.Info("Nebula Strike server module loaded")

	// Register the authoritative match handler
	if err := initializer.RegisterMatch(match.ModuleName, match.NewHandler); err != nil {
		logger.Error("Failed to register match handler: %v", err)
		return err
	}

	// Create the global leaderboard (idempotent — safe on every startup)
	authoritative := true
	sortOrder := "desc"
	operator := "incr"
	resetSchedule := ""
	metadata := map[string]interface{}{}

	if err := nk.LeaderboardCreate(ctx, leaderboardID, authoritative, sortOrder, operator, resetSchedule, metadata); err != nil {
		logger.Error("Failed to create leaderboard: %v", err)
		return err
	}
	logger.Info("Leaderboard '%s' created/verified", leaderboardID)

	// Register RPC endpoints
	if err := initializer.RegisterRpc("create_match", rpc.CreateMatchRPC); err != nil {
		logger.Error("Failed to register create_match RPC: %v", err)
		return err
	}

	if err := initializer.RegisterRpc("get_leaderboard", rpc.GetLeaderboardRPC); err != nil {
		logger.Error("Failed to register get_leaderboard RPC: %v", err)
		return err
	}

	if err := initializer.RegisterRpc("get_match_history", rpc.GetMatchHistoryRPC); err != nil {
		logger.Error("Failed to register get_match_history RPC: %v", err)
		return err
	}

	if err := initializer.RegisterMatchmakerMatched(match.OnMatchmakerMatched); err != nil {
		logger.Error("Failed to register matchmaker: %v", err)
		return err
	}

	logger.Info("Nebula Strike server initialized successfully")
	return nil
}
