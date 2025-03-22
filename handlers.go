package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/ethereum/go-ethereum/common"
	"github.com/korjavin/uniswapfetcher/uniswap"
	"go.uber.org/zap"
)

type BotHandlers struct {
	bot           *gotgbot.Bot
	db            *Database
	uniswapClient uniswap.Client
	logger        *zap.SugaredLogger
}

func NewBotHandlers(bot *gotgbot.Bot, db *Database, uniswapClient uniswap.Client, logger *zap.SugaredLogger) *BotHandlers {
	return &BotHandlers{
		bot:           bot,
		db:            db,
		uniswapClient: uniswapClient,
		logger:        logger,
	}
}

func (h *BotHandlers) RegisterHandlers(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("start", h.handleStart))
	dispatcher.AddHandler(handlers.NewCommand("add_wallet", h.handleAddWallet))
	dispatcher.AddHandler(handlers.NewCommand("remove_wallet", h.handleRemoveWallet))
	dispatcher.AddHandler(handlers.NewCommand("list_wallets", h.handleListWallets))
	dispatcher.AddHandler(handlers.NewCommand("status", h.handleStatus))
}

func (h *BotHandlers) handleStart(b *gotgbot.Bot, ctx *ext.Context) error {
	h.logger.Infow("Received start command", "user_id", ctx.EffectiveUser.Id)

	msg := `Welcome to Uniswap Position Tracker!
Available commands:
/add_wallet <address> - Add wallet to track
/remove_wallet <address> - Remove wallet
/list_wallets - Show tracked wallets
/status - Show positions status`

	_, err := ctx.EffectiveMessage.Reply(b, msg, &gotgbot.SendMessageOpts{})
	return err
}

func (h *BotHandlers) handleAddWallet(b *gotgbot.Bot, ctx *ext.Context) error {
	h.logger.Infow("Received add_wallet command", "user_id", ctx.EffectiveUser.Id)

	// Extract wallet address from command
	args := ctx.Args()
	h.logger.Debugw("Command arguments", "args", args)

	// The first argument is the command itself, so we need at least 2 arguments
	if len(args) < 2 {
		_, err := ctx.EffectiveMessage.Reply(b, "Please provide a wallet address: /add_wallet <address>", &gotgbot.SendMessageOpts{})
		return err
	}

	walletAddress := args[1] // Use the second argument, which is the actual address

	// Validate Ethereum address
	h.logger.Debugw("Validating Ethereum address", "address", walletAddress, "isValid", common.IsHexAddress(walletAddress))

	// Check if address has 0x prefix
	if len(walletAddress) < 2 || walletAddress[:2] != "0x" {
		h.logger.Debugw("Address missing 0x prefix", "address", walletAddress)
		_, err := ctx.EffectiveMessage.Reply(b, "Ethereum address must start with '0x'. Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Check if address has correct length
	if len(walletAddress) != 42 {
		h.logger.Debugw("Address has incorrect length", "address", walletAddress, "length", len(walletAddress))
		_, err := ctx.EffectiveMessage.Reply(b, "Ethereum address must be 42 characters long (including '0x' prefix). Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Use go-ethereum's validation
	if !common.IsHexAddress(walletAddress) {
		h.logger.Debugw("Address failed go-ethereum validation", "address", walletAddress)
		_, err := ctx.EffectiveMessage.Reply(b, "Invalid Ethereum address format. Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Normalize address
	normalizedAddress := common.HexToAddress(walletAddress).Hex()

	// Add wallet to database
	err := h.db.AddWallet(ctx.EffectiveUser.Id, normalizedAddress)
	if err != nil {
		h.logger.Errorw("Failed to add wallet", "error", err)
		_, err := ctx.EffectiveMessage.Reply(b, "Failed to add wallet. Please try again later.", &gotgbot.SendMessageOpts{})
		return err
	}

	_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Wallet %s added successfully.", normalizedAddress), &gotgbot.SendMessageOpts{})
	return err
}

func (h *BotHandlers) handleRemoveWallet(b *gotgbot.Bot, ctx *ext.Context) error {
	h.logger.Infow("Received remove_wallet command", "user_id", ctx.EffectiveUser.Id)

	// Extract wallet address from command
	args := ctx.Args()
	h.logger.Debugw("Command arguments for remove_wallet", "args", args)

	// The first argument is the command itself, so we need at least 2 arguments
	if len(args) < 2 {
		_, err := ctx.EffectiveMessage.Reply(b, "Please provide a wallet address: /remove_wallet <address>", &gotgbot.SendMessageOpts{})
		return err
	}

	walletAddress := args[1] // Use the second argument, which is the actual address

	// Validate Ethereum address
	h.logger.Debugw("Validating Ethereum address for removal", "address", walletAddress, "isValid", common.IsHexAddress(walletAddress))

	// Check if address has 0x prefix
	if len(walletAddress) < 2 || walletAddress[:2] != "0x" {
		h.logger.Debugw("Address missing 0x prefix", "address", walletAddress)
		_, err := ctx.EffectiveMessage.Reply(b, "Ethereum address must start with '0x'. Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Check if address has correct length
	if len(walletAddress) != 42 {
		h.logger.Debugw("Address has incorrect length", "address", walletAddress, "length", len(walletAddress))
		_, err := ctx.EffectiveMessage.Reply(b, "Ethereum address must be 42 characters long (including '0x' prefix). Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Use go-ethereum's validation
	if !common.IsHexAddress(walletAddress) {
		h.logger.Debugw("Address failed go-ethereum validation", "address", walletAddress)
		_, err := ctx.EffectiveMessage.Reply(b, "Invalid Ethereum address format. Please provide a valid address.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Normalize address
	normalizedAddress := common.HexToAddress(walletAddress).Hex()

	// Remove wallet from database
	err := h.db.RemoveWallet(ctx.EffectiveUser.Id, normalizedAddress)
	if err != nil {
		h.logger.Errorw("Failed to remove wallet", "error", err)
		_, err := ctx.EffectiveMessage.Reply(b, "Failed to remove wallet. Please try again later.", &gotgbot.SendMessageOpts{})
		return err
	}

	_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Wallet %s removed successfully.", normalizedAddress), &gotgbot.SendMessageOpts{})
	return err
}

func (h *BotHandlers) handleListWallets(b *gotgbot.Bot, ctx *ext.Context) error {
	h.logger.Infow("Received list_wallets command", "user_id", ctx.EffectiveUser.Id)

	// Get wallets from database
	wallets, err := h.db.GetWallets(ctx.EffectiveUser.Id)
	if err != nil {
		h.logger.Errorw("Failed to get wallets", "error", err)
		_, err := ctx.EffectiveMessage.Reply(b, "Failed to retrieve wallets. Please try again later.", &gotgbot.SendMessageOpts{})
		return err
	}

	// Format response
	var msg string
	if len(wallets) == 0 {
		msg = "You don't have any wallets added yet. Use /add_wallet <address> to add one."
	} else {
		msg = "Your tracked wallets:\n\n"
		for i, wallet := range wallets {
			msg += fmt.Sprintf("%d. %s\n", i+1, wallet)
		}
		msg += "\nUse /status to check positions for these wallets."
	}

	_, err = ctx.EffectiveMessage.Reply(b, msg, &gotgbot.SendMessageOpts{})
	return err
}

func (h *BotHandlers) handleStatus(b *gotgbot.Bot, ctx *ext.Context) error {
	h.logger.Infow("Received status command", "user_id", ctx.EffectiveUser.Id)

	// Send initial message
	statusMsg, err := ctx.EffectiveMessage.Reply(b, "Fetching Uniswap positions... This may take a moment.", &gotgbot.SendMessageOpts{})
	if err != nil {
		return err
	}

	// Get wallets from database
	wallets, err := h.db.GetWallets(ctx.EffectiveUser.Id)
	if err != nil {
		h.logger.Errorw("Failed to get wallets", "error", err)
		_, _, err = statusMsg.EditText(b, "Failed to retrieve wallets. Please try again later.", &gotgbot.EditMessageTextOpts{})
		return err
	}

	if len(wallets) == 0 {
		_, _, err = statusMsg.EditText(b, "You don't have any wallets added yet. Use /add_wallet <address> to add one.", &gotgbot.EditMessageTextOpts{})
		return err
	}

	// Create context with timeout
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch positions for each wallet
	var allPositions []uniswap.Position
	for _, wallet := range wallets {
		// Update status message
		_, _, err = statusMsg.EditText(b, fmt.Sprintf("Fetching positions for wallet %s...", wallet), &gotgbot.EditMessageTextOpts{})
		if err != nil {
			h.logger.Warnw("Failed to update status message", "error", err)
		}

		// Create position request
		req := uniswap.PositionRequest{
			WalletAddress: common.HexToAddress(wallet),
			IncludeV3:     true,
			IncludeV4:     true,
		}

		// Fetch positions
		positions, err := h.uniswapClient.GetPositions(bgCtx, req)
		if err != nil {
			h.logger.Errorw("Failed to fetch positions", "wallet", wallet, "error", err)
			continue
		}

		allPositions = append(allPositions, positions...)
	}

	// Format response
	var msg string
	if len(allPositions) == 0 {
		msg = "No Uniswap positions found for your wallets."
	} else {
		msg = fmt.Sprintf("Found %d Uniswap positions:\n\n", len(allPositions))

		// Group positions by wallet
		positionsByWallet := make(map[string][]uniswap.Position)
		for _, pos := range allPositions {
			wallet := pos.Owner.Hex()
			positionsByWallet[wallet] = append(positionsByWallet[wallet], pos)
		}

		// Format each wallet's positions
		for wallet, positions := range positionsByWallet {
			msg += fmt.Sprintf("Wallet: %s\n", wallet)
			msg += "--------------------\n"

			for i, pos := range positions {
				summary := uniswap.FormatPositionSummary(pos)

				msg += fmt.Sprintf("%d. %s %s\n", i+1, summary.TokenPair, summary.Version)
				msg += fmt.Sprintf("   ID: %s\n", summary.ID)
				msg += fmt.Sprintf("   Created: %s\n", summary.CreatedAt)
				msg += fmt.Sprintf("   Amounts: %s\n", summary.Amounts)
				msg += fmt.Sprintf("   Price Range: %s\n", summary.PriceRange)
				msg += fmt.Sprintf("   In Range: %v\n", summary.InRange)
				msg += fmt.Sprintf("   Unclaimed Fees: %s\n\n", summary.UnclaimedFees)
			}
		}
	}

	// Update final message
	_, _, err = statusMsg.EditText(b, msg, &gotgbot.EditMessageTextOpts{})
	return err
}
