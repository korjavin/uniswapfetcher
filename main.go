package main

import (
	"os"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/korjavin/uniswapfetcher/uniswap"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sugar := logger.Sugar()

	// Get bot token from environment
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		sugar.Fatal("TELEGRAM_TOKEN environment variable is required")
	}

	// Get Infura API key from environment
	infuraKey := os.Getenv("INFURA_API_KEY")
	if infuraKey == "" {
		sugar.Fatal("INFURA_API_KEY environment variable is required")
	}

	// Initialize database
	db, err := initDB()
	if err != nil {
		sugar.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Uniswap client with direct Ethereum RPC calls
	infuraURL := "https://mainnet.infura.io/v3/" + infuraKey
	uniswapClient, err := uniswap.NewClient(infuraURL, sugar)
	if err != nil {
		sugar.Fatalf("Failed to initialize Uniswap client: %v", err)
	}
	defer uniswapClient.Close()

	// Initialize bot with increased timeout
	bot, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: 60 * time.Second, // Increase timeout to 60 seconds
		},
	})
	if err != nil {
		sugar.Fatalf("Failed to create bot: %v", err)
	}

	// Create dispatcher
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			sugar.Errorw("Error in handler", "error", err)
			return ext.DispatcherActionNoop
		},
	})

	// Create updater
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{})

	// Setup handlers
	handlers := NewBotHandlers(bot, db, uniswapClient, sugar)
	handlers.RegisterHandlers(dispatcher)

	// Start bot
	sugar.Info("Bot started successfully")
	err = updater.StartPolling(bot, &ext.PollingOpts{
		DropPendingUpdates: true,
	})
	if err != nil {
		sugar.Fatalf("Failed to start polling: %v", err)
	}

	// Keep the bot running
	updater.Idle()
}
