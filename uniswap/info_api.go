package uniswap

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

const (
	// Uniswap Info API URL
	UniswapInfoAPIURL = "https://api.uniswap.org/v1"
)

// InfoAPIClient implements the Client interface using the Uniswap Info API
type InfoAPIClient struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

// NewInfoAPIClient creates a new Uniswap client using the Info API
func NewInfoAPIClient(logger *zap.SugaredLogger) (*InfoAPIClient, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	logger.Infow("Initialized Uniswap Info API client")

	return &InfoAPIClient{
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

// GetPositions fetches all positions for a given wallet address
func (c *InfoAPIClient) GetPositions(ctx context.Context, req PositionRequest) ([]Position, error) {
	c.logger.Infow("Fetching positions using Info API", "wallet", req.WalletAddress.Hex())

	var positions []Position
	var err error

	// Fetch V3 positions if requested
	if req.IncludeV3 {
		v3Positions, err := c.getV3Positions(ctx, req.WalletAddress)
		if err != nil {
			c.logger.Errorw("Failed to fetch V3 positions", "error", err)
		} else {
			positions = append(positions, v3Positions...)
			c.logger.Infow("Fetched V3 positions", "count", len(v3Positions))
		}
	}

	// Fetch V4 positions if requested
	if req.IncludeV4 {
		v4Positions, err := c.getV4Positions(ctx, req.WalletAddress)
		if err != nil {
			c.logger.Errorw("Failed to fetch V4 positions", "error", err)
		} else {
			positions = append(positions, v4Positions...)
			c.logger.Infow("Fetched V4 positions", "count", len(v4Positions))
		}
	}

	c.logger.Infow("Fetched positions", "count", len(positions))
	return positions, err
}

// getV3Positions fetches V3 positions from the Info API
func (c *InfoAPIClient) getV3Positions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V3 positions from Info API", "wallet", walletAddress.Hex())

	// Construct the API URL
	url := fmt.Sprintf("%s/positions?owner=%s", UniswapInfoAPIURL, strings.ToLower(walletAddress.Hex()))
	c.logger.Debugw("Making API request", "url", url)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response
	var response struct {
		Positions []struct {
			ID    string `json:"id"`
			Owner string `json:"owner"`
			Pool  struct {
				ID      string `json:"id"`
				FeeTier string `json:"feeTier"`
				Token0  struct {
					ID       string `json:"id"`
					Symbol   string `json:"symbol"`
					Decimals string `json:"decimals"`
				} `json:"token0"`
				Token1 struct {
					ID       string `json:"id"`
					Symbol   string `json:"symbol"`
					Decimals string `json:"decimals"`
				} `json:"token1"`
				SqrtPrice string `json:"sqrtPrice"`
				Tick      string `json:"tick"`
			} `json:"pool"`
			TickLower           string `json:"tickLower"`
			TickUpper           string `json:"tickUpper"`
			Liquidity           string `json:"liquidity"`
			DepositedToken0     string `json:"depositedToken0"`
			DepositedToken1     string `json:"depositedToken1"`
			WithdrawnToken0     string `json:"withdrawnToken0"`
			WithdrawnToken1     string `json:"withdrawnToken1"`
			CollectedFeesToken0 string `json:"collectedFeesToken0"`
			CollectedFeesToken1 string `json:"collectedFeesToken1"`
			CreatedAtTimestamp  string `json:"createdAtTimestamp"`
		} `json:"positions"`
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debugw("API response", "positions", len(response.Positions))

	// Convert the response to Position objects
	var positions []Position
	for _, pos := range response.Positions {
		// Parse token decimals
		token0Decimals, _ := strconv.ParseUint(pos.Pool.Token0.Decimals, 10, 8)
		token1Decimals, _ := strconv.ParseUint(pos.Pool.Token1.Decimals, 10, 8)

		// Parse position ID
		positionID, _ := strconv.ParseUint(pos.ID, 10, 64)

		// Parse ticks
		tickLower, _ := strconv.ParseInt(pos.TickLower, 10, 32)
		tickUpper, _ := strconv.ParseInt(pos.TickUpper, 10, 32)

		// Parse fee tier
		feeTier, _ := strconv.ParseUint(pos.Pool.FeeTier, 10, 32)

		// Parse liquidity
		liquidity := new(big.Int)
		liquidity.SetString(pos.Liquidity, 10)

		// Parse amounts
		depositedToken0 := new(big.Int)
		depositedToken0.SetString(pos.DepositedToken0, 10)

		depositedToken1 := new(big.Int)
		depositedToken1.SetString(pos.DepositedToken1, 10)

		withdrawnToken0 := new(big.Int)
		withdrawnToken0.SetString(pos.WithdrawnToken0, 10)

		withdrawnToken1 := new(big.Int)
		withdrawnToken1.SetString(pos.WithdrawnToken1, 10)

		// Calculate current amounts
		amount0 := new(big.Int).Sub(depositedToken0, withdrawnToken0)
		amount1 := new(big.Int).Sub(depositedToken1, withdrawnToken1)

		// Parse unclaimed fees
		unclaimedFees0 := new(big.Int)
		unclaimedFees0.SetString(pos.CollectedFeesToken0, 10)

		unclaimedFees1 := new(big.Int)
		unclaimedFees1.SetString(pos.CollectedFeesToken1, 10)

		// Parse creation timestamp
		timestampInt, _ := strconv.ParseInt(pos.CreatedAtTimestamp, 10, 64)
		createdAt := time.Unix(timestampInt, 0)

		// Calculate price range
		tickToPrice := func(tick int64) *big.Float {
			price := big.NewFloat(1.0001)
			price.SetPrec(256)
			price = price.SetMantExp(price, int(tick))
			return price
		}

		priceLower := tickToPrice(tickLower)
		priceUpper := tickToPrice(tickUpper)

		// Calculate current price
		currentTick, _ := strconv.ParseInt(pos.Pool.Tick, 10, 64)
		currentPrice := tickToPrice(currentTick)

		// Create the position
		position := Position{
			ID:             big.NewInt(int64(positionID)),
			Version:        VersionV3,
			Owner:          common.HexToAddress(pos.Owner),
			Token0:         Token{Address: common.HexToAddress(pos.Pool.Token0.ID), Symbol: pos.Pool.Token0.Symbol, Decimals: uint8(token0Decimals)},
			Token1:         Token{Address: common.HexToAddress(pos.Pool.Token1.ID), Symbol: pos.Pool.Token1.Symbol, Decimals: uint8(token1Decimals)},
			Amount0:        amount0,
			Amount1:        amount1,
			FeeTier:        uint32(feeTier),
			CreatedAt:      createdAt,
			TickLower:      int(tickLower),
			TickUpper:      int(tickUpper),
			Liquidity:      liquidity,
			UnclaimedFees0: unclaimedFees0,
			UnclaimedFees1: unclaimedFees1,
			PriceLower:     priceLower,
			PriceUpper:     priceUpper,
			CurrentPrice:   currentPrice,
		}

		positions = append(positions, position)
		c.logger.Infow("Fetched V3 position from Info API",
			"positionId", position.ID.String(),
			"tokenPair", fmt.Sprintf("%s/%s", position.Token0.Symbol, position.Token1.Symbol),
			"liquidity", position.Liquidity.String(),
			"amount0", formatBigInt(position.Amount0, int(position.Token0.Decimals)),
			"amount1", formatBigInt(position.Amount1, int(position.Token1.Decimals)))
	}

	return positions, nil
}

// getV4Positions fetches V4 positions from the Info API
func (c *InfoAPIClient) getV4Positions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V4 positions from Info API", "wallet", walletAddress.Hex())

	// Note: Uniswap V4 is still in development, and the Info API may not support V4 positions yet
	// This is a placeholder implementation that will need to be updated when V4 is fully released

	// For now, we'll return an empty array
	return []Position{}, nil
}

// Close closes the client and releases any resources
func (c *InfoAPIClient) Close() {
	// Nothing to close
}
