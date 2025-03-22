package uniswap

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// Client is the interface for interacting with Uniswap
type Client interface {
	// GetPositions fetches all positions for a given wallet address
	GetPositions(ctx context.Context, req PositionRequest) ([]Position, error)

	// Close closes the client and releases any resources
	Close()
}

// UniswapClient implements the Client interface
type UniswapClient struct {
	ethClient *ethclient.Client
	v3Client  V3Client
	v4Client  V4Client
	logger    *zap.SugaredLogger
	mu        sync.Mutex
}

// NewClient creates a new Uniswap client
func NewClient(ethURL string, logger *zap.SugaredLogger) (*UniswapClient, error) {
	ethClient, err := ethclient.Dial(ethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	v3Client, err := NewV3Client(ethClient, logger)
	if err != nil {
		ethClient.Close()
		return nil, fmt.Errorf("failed to create V3 client: %w", err)
	}

	v4Client, err := NewV4Client(ethClient, logger)
	if err != nil {
		ethClient.Close()
		return nil, fmt.Errorf("failed to create V4 client: %w", err)
	}

	return &UniswapClient{
		ethClient: ethClient,
		v3Client:  v3Client,
		v4Client:  v4Client,
		logger:    logger,
	}, nil
}

// GetPositions fetches all positions for a given wallet address
func (c *UniswapClient) GetPositions(ctx context.Context, req PositionRequest) ([]Position, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Infow("Fetching positions", "wallet", req.WalletAddress.Hex())

	var positions []Position
	var wg sync.WaitGroup
	var mu sync.Mutex
	var v3Err, v4Err error

	// Fetch V3 positions if requested
	if req.IncludeV3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v3Positions, err := c.v3Client.GetPositions(ctx, req.WalletAddress)
			if err != nil {
				v3Err = err
				c.logger.Errorw("Failed to fetch V3 positions", "error", err)
				return
			}

			mu.Lock()
			positions = append(positions, v3Positions...)
			mu.Unlock()

			c.logger.Infow("Fetched V3 positions", "count", len(v3Positions))
		}()
	}

	// Fetch V4 positions if requested
	if req.IncludeV4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v4Positions, err := c.v4Client.GetPositions(ctx, req.WalletAddress)
			if err != nil {
				v4Err = err
				c.logger.Errorw("Failed to fetch V4 positions", "error", err)
				return
			}

			mu.Lock()
			positions = append(positions, v4Positions...)
			mu.Unlock()

			c.logger.Infow("Fetched V4 positions", "count", len(v4Positions))
		}()
	}

	wg.Wait()

	// Handle errors
	if v3Err != nil && v4Err != nil {
		return nil, fmt.Errorf("failed to fetch positions: V3: %v, V4: %v", v3Err, v4Err)
	} else if v3Err != nil && req.IncludeV3 {
		c.logger.Warnw("Failed to fetch V3 positions, returning only V4 positions", "error", v3Err)
	} else if v4Err != nil && req.IncludeV4 {
		c.logger.Warnw("Failed to fetch V4 positions, returning only V3 positions", "error", v4Err)
	}

	c.logger.Infow("Fetched positions", "count", len(positions))
	return positions, nil
}

// Close closes the client and releases any resources
func (c *UniswapClient) Close() {
	c.ethClient.Close()
}

// FormatPositionSummary formats a position into a human-readable summary
func FormatPositionSummary(position Position) PositionSummary {
	inRange := false
	if position.CurrentPrice != nil && position.PriceLower != nil && position.PriceUpper != nil {
		inRange = position.CurrentPrice.Cmp(position.PriceLower) >= 0 && position.CurrentPrice.Cmp(position.PriceUpper) <= 0
	}

	return PositionSummary{
		ID:            position.ID.String(),
		Version:       string(position.Version),
		TokenPair:     fmt.Sprintf("%s/%s", position.Token0.Symbol, position.Token1.Symbol),
		Amounts:       fmt.Sprintf("%s %s, %s %s", formatBigInt(position.Amount0, int(position.Token0.Decimals)), position.Token0.Symbol, formatBigInt(position.Amount1, int(position.Token1.Decimals)), position.Token1.Symbol),
		PriceRange:    fmt.Sprintf("%s - %s", formatBigFloat(position.PriceLower), formatBigFloat(position.PriceUpper)),
		UnclaimedFees: fmt.Sprintf("%s %s, %s %s", formatBigInt(position.UnclaimedFees0, int(position.Token0.Decimals)), position.Token0.Symbol, formatBigInt(position.UnclaimedFees1, int(position.Token1.Decimals)), position.Token1.Symbol),
		CreatedAt:     position.CreatedAt.Format("2006-01-02 15:04:05"),
		InRange:       inRange,
	}
}

// Helper functions for formatting big numbers
func formatBigInt(n *big.Int, decimals int) string {
	if n == nil {
		return "0"
	}

	// Create a copy of n to avoid modifying the original
	value := new(big.Int).Set(n)

	// If decimals is 0, just return the string representation
	if decimals == 0 {
		return value.String()
	}

	// Convert to a decimal representation
	// First, get the integer part
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(value, divisor)

	// Then, get the fractional part
	remainder := new(big.Int).Mod(value, divisor)

	// Format the fractional part with leading zeros
	fracStr := remainder.String()
	for len(fracStr) < decimals {
		fracStr = "0" + fracStr
	}

	// Trim trailing zeros
	for len(fracStr) > 0 && fracStr[len(fracStr)-1] == '0' {
		fracStr = fracStr[:len(fracStr)-1]
	}

	// Combine integer and fractional parts
	if len(fracStr) > 0 {
		return intPart.String() + "." + fracStr
	}

	return intPart.String()
}

func formatBigFloat(n *big.Float) string {
	if n == nil {
		return "0"
	}

	// Format with 4 decimal places
	return fmt.Sprintf("%.4f", n)
}
