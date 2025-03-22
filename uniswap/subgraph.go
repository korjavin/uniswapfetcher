package uniswap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

const (
	// Uniswap V3 Subgraph URL
	UniswapV3SubgraphURL = "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3"

	// Uniswap V4 Subgraph URL (this is a placeholder, replace with actual URL when available)
	UniswapV4SubgraphURL = "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v4"
)

// SubgraphClient implements the Client interface using The Graph API
type SubgraphClient struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

// NewSubgraphClient creates a new Uniswap client using The Graph API
func NewSubgraphClient(logger *zap.SugaredLogger) (*SubgraphClient, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	logger.Infow("Initialized Uniswap Subgraph client")

	return &SubgraphClient{
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

// GetPositions fetches all positions for a given wallet address
func (c *SubgraphClient) GetPositions(ctx context.Context, req PositionRequest) ([]Position, error) {
	c.logger.Infow("Fetching positions using Subgraph API", "wallet", req.WalletAddress.Hex())

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

// getV3Positions fetches V3 positions from the Subgraph API
func (c *SubgraphClient) getV3Positions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V3 positions from Subgraph", "wallet", walletAddress.Hex())

	// GraphQL query to fetch positions
	query := `
	{
		positions(where: {owner: "` + strings.ToLower(walletAddress.Hex()) + `"}) {
			id
			owner
			token0 {
				id
				symbol
				decimals
			}
			token1 {
				id
				symbol
				decimals
			}
			pool {
				id
				feeTier
				sqrtPrice
				tick
			}
			tickLower {
				tickIdx
			}
			tickUpper {
				tickIdx
			}
			liquidity
			depositedToken0
			depositedToken1
			withdrawnToken0
			withdrawnToken1
			collectedFeesToken0
			collectedFeesToken1
			transaction {
				timestamp
			}
		}
	}`

	// Execute the query
	var response struct {
		Data struct {
			Positions []struct {
				ID     string `json:"id"`
				Owner  string `json:"owner"`
				Token0 struct {
					ID       string `json:"id"`
					Symbol   string `json:"symbol"`
					Decimals string `json:"decimals"`
				} `json:"token0"`
				Token1 struct {
					ID       string `json:"id"`
					Symbol   string `json:"symbol"`
					Decimals string `json:"decimals"`
				} `json:"token1"`
				Pool struct {
					ID        string `json:"id"`
					FeeTier   string `json:"feeTier"`
					SqrtPrice string `json:"sqrtPrice"`
					Tick      string `json:"tick"`
				} `json:"pool"`
				TickLower struct {
					TickIdx string `json:"tickIdx"`
				} `json:"tickLower"`
				TickUpper struct {
					TickIdx string `json:"tickIdx"`
				} `json:"tickUpper"`
				Liquidity           string `json:"liquidity"`
				DepositedToken0     string `json:"depositedToken0"`
				DepositedToken1     string `json:"depositedToken1"`
				WithdrawnToken0     string `json:"withdrawnToken0"`
				WithdrawnToken1     string `json:"withdrawnToken1"`
				CollectedFeesToken0 string `json:"collectedFeesToken0"`
				CollectedFeesToken1 string `json:"collectedFeesToken1"`
				Transaction         struct {
					Timestamp string `json:"timestamp"`
				} `json:"transaction"`
			} `json:"positions"`
		} `json:"data"`
	}

	err := c.executeGraphQLQuery(ctx, UniswapV3SubgraphURL, query, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	// Convert the response to Position objects
	var positions []Position
	for _, pos := range response.Data.Positions {
		// Parse token decimals
		token0Decimals, _ := strconv.ParseUint(pos.Token0.Decimals, 10, 8)
		token1Decimals, _ := strconv.ParseUint(pos.Token1.Decimals, 10, 8)

		// Parse position ID
		positionID, _ := strconv.ParseUint(pos.ID, 10, 64)

		// Parse ticks
		tickLower, _ := strconv.ParseInt(pos.TickLower.TickIdx, 10, 32)
		tickUpper, _ := strconv.ParseInt(pos.TickUpper.TickIdx, 10, 32)

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
		timestampInt, _ := strconv.ParseInt(pos.Transaction.Timestamp, 10, 64)
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
			Token0:         Token{Address: common.HexToAddress(pos.Token0.ID), Symbol: pos.Token0.Symbol, Decimals: uint8(token0Decimals)},
			Token1:         Token{Address: common.HexToAddress(pos.Token1.ID), Symbol: pos.Token1.Symbol, Decimals: uint8(token1Decimals)},
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
		c.logger.Infow("Fetched V3 position from Subgraph",
			"positionId", position.ID.String(),
			"tokenPair", fmt.Sprintf("%s/%s", position.Token0.Symbol, position.Token1.Symbol),
			"liquidity", position.Liquidity.String(),
			"amount0", formatBigInt(position.Amount0, int(position.Token0.Decimals)),
			"amount1", formatBigInt(position.Amount1, int(position.Token1.Decimals)))
	}

	return positions, nil
}

// getV4Positions fetches V4 positions from the Subgraph API
func (c *SubgraphClient) getV4Positions(ctx context.Context, walletAddress common.Address) ([]Position, error) {
	c.logger.Infow("Fetching V4 positions from Subgraph", "wallet", walletAddress.Hex())

	// Note: Uniswap V4 is still in development, and the Subgraph API may not be available yet
	// This is a placeholder implementation that will need to be updated when V4 is fully released

	// For now, we'll return an empty array
	return []Position{}, nil
}

// executeGraphQLQuery executes a GraphQL query against the specified URL
func (c *SubgraphClient) executeGraphQLQuery(ctx context.Context, url, query string, result interface{}) error {
	c.logger.Debugw("Executing GraphQL query", "url", url)

	// Prepare the request body
	reqBody := map[string]string{
		"query": query,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debugw("GraphQL response", "status", resp.Status, "body", string(respBody))

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response
	err = json.Unmarshal(respBody, result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// Close closes the client and releases any resources
func (c *SubgraphClient) Close() {
	// Nothing to close
}
