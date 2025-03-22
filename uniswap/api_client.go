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

// UniswapSubgraphURL is the endpoint for Uniswap's V3 subgraph on The Graph protocol.
// The subgraph indexes Uniswap V3 data from the blockchain and provides a GraphQL API
// to efficiently query positions, pools, and other Uniswap-related data.
// Documentation: https://docs.uniswap.org/protocol/reference/api/subgraph

const (
	UniswapSubgraphURLV3 = "https://gateway.thegraph.com/api/%s/subgraphs/id/5zvR82QoaXYFyDEKLZ9t6v9adgnptxYpKpSbxtgVENFV"
	UniswapSubgraphURLV4 = "https://gateway.thegraph.com/api/%s/subgraphs/id/DiYPVdygkfjDWhbxGSqAQxwBKmfKnkWQojqeM2rkLb3G"
)

// GraphQLError represents a GraphQL error response
type GraphQLError struct {
	Message string `json:"message"`
}

// GraphQLResponse represents a GraphQL response with possible errors
type GraphQLResponse struct {
	Data   interface{}    `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

// PositionData represents the structure of position data in GraphQL responses for V3
type PositionData struct {
	Positions []struct {
		ID                  string `json:"id"`
		Owner               string `json:"owner"`
		DepositedToken0     string `json:"depositedToken0"`
		DepositedToken1     string `json:"depositedToken1"`
		WithdrawnToken0     string `json:"withdrawnToken0"`
		WithdrawnToken1     string `json:"withdrawnToken1"`
		CollectedFeesToken0 string `json:"collectedFeesToken0"`
		CollectedFeesToken1 string `json:"collectedFeesToken1"`
		Liquidity           string `json:"liquidity"`
		TickLower           string `json:"tickLower"`
		TickUpper           string `json:"tickUpper"`
		Pool                struct {
			FeeTier     string `json:"feeTier"`
			Token0Price string `json:"token0Price"`
			Token1Price string `json:"token1Price"`
		} `json:"pool"`
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
	} `json:"positions"`
}

// V4PositionData represents the structure of position data in GraphQL responses for V4
type V4PositionData struct {
	Positions []struct {
		ID                 string `json:"id"`
		Owner              string `json:"owner"`
		CreatedAtTimestamp string `json:"createdAtTimestamp"`
		Pool               struct {
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
			SqrtPrice string `json:"sqrtPrice"`
			Tick      string `json:"tick"`
			Liquidity string `json:"liquidity"`
			FeeTier   string `json:"feeTier"`
		} `json:"pool"`
		Liquidity       string `json:"liquidity"`
		TickLower       string `json:"tickLower"`
		TickUpper       string `json:"tickUpper"`
		DepositedToken0 string `json:"depositedToken0"`
		DepositedToken1 string `json:"depositedToken1"`
		WithdrawnToken0 string `json:"withdrawnToken0"`
		WithdrawnToken1 string `json:"withdrawnToken1"`
		CollectedToken0 string `json:"collectedToken0"`
		CollectedToken1 string `json:"collectedToken1"`
	} `json:"positions"`
}

// APIClient implements a client for the Uniswap V3 subgraph API.
// The subgraph provides indexed blockchain data for Uniswap V3, making it efficient
// to query position details, pool statistics, and historical data without direct
// blockchain calls.
type APIClient struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
	apiKey     string
}

// NewAPIClient creates a new Uniswap API client
func NewAPIClient(logger *zap.SugaredLogger, apiKey string) (*APIClient, error) {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		apiKey: apiKey,
	}, nil
}

// GetPositions fetches all Uniswap positions for a given wallet address using the subgraph API.
func (c *APIClient) GetPositions(ctx context.Context, req PositionRequest) ([]Position, error) {
	var allPositions []Position

	if req.IncludeV3 {
		positions, err := c.getVersionPositions(ctx, req.WalletAddress, fmt.Sprintf(UniswapSubgraphURLV3, c.apiKey), VersionV3)
		if err != nil {
			c.logger.Warnw("Failed to fetch V3 positions", "error", err)
		} else {
			allPositions = append(allPositions, positions...)
		}
	}

	if req.IncludeV4 {
		positions, err := c.getVersionPositions(ctx, req.WalletAddress, fmt.Sprintf(UniswapSubgraphURLV4, c.apiKey), VersionV4)
		if err != nil {
			c.logger.Warnw("Failed to fetch V4 positions", "error", err)
		} else {
			allPositions = append(allPositions, positions...)
		}
	}

	return allPositions, nil
}

func (c *APIClient) getVersionPositions(ctx context.Context, wallet common.Address, url string, version PositionVersion) ([]Position, error) {
	var query string

	if version == VersionV3 {
		query = fmt.Sprintf(`{
			positions(where: { owner: "%s" }) {
				id
				owner
				depositedToken0
				depositedToken1
				withdrawnToken0
				withdrawnToken1
				collectedFeesToken0
				collectedFeesToken1
				liquidity
				tickLower
				tickUpper
				pool {
					feeTier
					token0Price
					token1Price
				}
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
			}
		}`, strings.ToLower(wallet.Hex()))
	} else if version == VersionV4 {
		// V4 has a different schema, use appropriate fields
		query = fmt.Sprintf(`{
			positions(where: { owner: "%s" }) {
				id
				owner
				createdAtTimestamp
				pool {
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
					sqrtPrice
					tick
					liquidity
					feeTier
				}
			}
		}`, strings.ToLower(wallet.Hex()))
	}

	resp, err := c.executeGraphQLQuery(ctx, url, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	if version == VersionV3 {
		var graphResp struct {
			Data PositionData `json:"data"`
		}
		if err := json.Unmarshal(resp, &graphResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		positions := c.parsePositionData(&graphResp.Data, version)
		return positions, nil
	} else {
		var graphResp struct {
			Data V4PositionData `json:"data"`
		}
		if err := json.Unmarshal(resp, &graphResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		positions := c.parseV4PositionData(&graphResp.Data)
		return positions, nil
	}
}

func (c *APIClient) executeGraphQLQuery(ctx context.Context, url, query string) ([]byte, error) {
	body, err := json.Marshal(map[string]string{
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	obfuscatedKey := c.apiKey
	if len(obfuscatedKey) > 4 {
		obfuscatedKey = strings.Repeat("*", len(obfuscatedKey)-4) + obfuscatedKey[len(obfuscatedKey)-4:]
	}

	c.logger.Debugw("Making GraphQL request",
		"url", url,
		"apiKey", obfuscatedKey,
		"bodyLength", len(body))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debugw("Got GraphQL response",
		"statusCode", resp.StatusCode,
		"contentLength", len(respBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Check for GraphQL errors
	var graphQLResp GraphQLResponse
	if err := json.Unmarshal(respBody, &graphQLResp); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(graphQLResp.Errors) > 0 {
		c.logger.Errorw("GraphQL query returned errors",
			"errors", graphQLResp.Errors,
			"query", query)
		return nil, fmt.Errorf("GraphQL errors: %v", graphQLResp.Errors)
	}

	return respBody, nil
}

func (c *APIClient) parsePositionData(data *PositionData, version PositionVersion) []Position {
	var positions []Position
	for _, p := range data.Positions {
		token0Decimals, _ := strconv.ParseUint(p.Token0.Decimals, 10, 8)
		token1Decimals, _ := strconv.ParseUint(p.Token1.Decimals, 10, 8)
		feeTier, _ := strconv.ParseUint(p.Pool.FeeTier, 10, 32)

		tickLower, _ := strconv.ParseInt(p.TickLower, 10, 64)
		tickUpper, _ := strconv.ParseInt(p.TickUpper, 10, 64)

		// Calculate amounts
		depositedToken0 := stringToBigInt(p.DepositedToken0)
		depositedToken1 := stringToBigInt(p.DepositedToken1)
		withdrawnToken0 := stringToBigInt(p.WithdrawnToken0)
		withdrawnToken1 := stringToBigInt(p.WithdrawnToken1)
		amount0 := new(big.Int).Sub(depositedToken0, withdrawnToken0)
		amount1 := new(big.Int).Sub(depositedToken1, withdrawnToken1)

		pos := Position{
			ID:      stringToBigInt(p.ID),
			Version: version,
			Owner:   common.HexToAddress(p.Owner),
			Token0: Token{
				Address:  common.HexToAddress(p.Token0.ID),
				Symbol:   p.Token0.Symbol,
				Decimals: uint8(token0Decimals),
			},
			Token1: Token{
				Address:  common.HexToAddress(p.Token1.ID),
				Symbol:   p.Token1.Symbol,
				Decimals: uint8(token1Decimals),
			},
			Amount0:         amount0,
			Amount1:         amount1,
			DepositedToken0: stringToBigInt(p.DepositedToken0),
			DepositedToken1: stringToBigInt(p.DepositedToken1),
			UnclaimedFees0:  stringToBigInt(p.CollectedFeesToken0),
			UnclaimedFees1:  stringToBigInt(p.CollectedFeesToken1),
			FeeTier:         uint32(feeTier),
			CreatedAt:       time.Now(),
			TickLower:       int(tickLower),
			TickUpper:       int(tickUpper),
			Liquidity:       stringToBigInt(p.Liquidity),
			CurrentPrice:    stringToBigFloat(p.Pool.Token0Price),
			PriceLower:      tickToPrice(tickLower),
			PriceUpper:      tickToPrice(tickUpper),
		}
		positions = append(positions, pos)
	}
	return positions
}

// parseV4PositionData parses V4 position data from the API response
func (c *APIClient) parseV4PositionData(data *V4PositionData) []Position {
	var positions []Position
	for _, p := range data.Positions {
		// Parse token decimals
		token0Decimals, _ := strconv.ParseUint(p.Pool.Token0.Decimals, 10, 8)
		token1Decimals, _ := strconv.ParseUint(p.Pool.Token1.Decimals, 10, 8)
		feeTier, _ := strconv.ParseUint(p.Pool.FeeTier, 10, 32)

		// Parse timestamp
		timestamp, err := strconv.ParseInt(p.CreatedAtTimestamp, 10, 64)
		if err != nil {
			timestamp = time.Now().Unix()
		}

		// Parse ticks
		tickLower, _ := strconv.ParseInt(p.TickLower, 10, 64)
		tickUpper, _ := strconv.ParseInt(p.TickUpper, 10, 64)

		// Parse liquidity, deposited, withdrawn, and collected tokens
		liquidity := stringToBigInt(p.Liquidity)
		depositedToken0 := stringToBigInt(p.DepositedToken0)
		depositedToken1 := stringToBigInt(p.DepositedToken1)
		withdrawnToken0 := stringToBigInt(p.WithdrawnToken0)
		withdrawnToken1 := stringToBigInt(p.WithdrawnToken1)
		collectedToken0 := stringToBigInt(p.CollectedToken0)
		collectedToken1 := stringToBigInt(p.CollectedToken1)

		amount0 := new(big.Int).Sub(depositedToken0, withdrawnToken0)
		amount1 := new(big.Int).Sub(depositedToken1, withdrawnToken1)

		pos := Position{
			ID:      stringToBigInt(p.ID),
			Version: VersionV4,
			Owner:   common.HexToAddress(p.Owner),
			Token0: Token{
				Address:  common.HexToAddress(p.Pool.Token0.ID),
				Symbol:   p.Pool.Token0.Symbol,
				Decimals: uint8(token0Decimals),
			},
			Token1: Token{
				Address:  common.HexToAddress(p.Pool.Token1.ID),
				Symbol:   p.Pool.Token1.Symbol,
				Decimals: uint8(token1Decimals),
			},
			Amount0:         amount0,
			Amount1:         amount1,
			UnclaimedFees0:  collectedToken0,
			UnclaimedFees1:  collectedToken1,
			FeeTier:         uint32(feeTier),
			CreatedAt:       time.Unix(timestamp, 0),
			TickLower:       int(tickLower),
			TickUpper:       int(tickUpper),
			Liquidity:       liquidity,
			CurrentPrice:    calculateCurrentPrice(p.Pool.SqrtPrice),
			DepositedToken0: depositedToken0,
			DepositedToken1: depositedToken1,
			WithdrawnToken0: withdrawnToken0,
			WithdrawnToken1: withdrawnToken1,
		}
		positions = append(positions, pos)
	}
	return positions
}

// Close closes the client and releases any resources
func (c *APIClient) Close() {
	c.httpClient.CloseIdleConnections()
}

// Helper functions
func stringToBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}

func stringToBigFloat(s string) *big.Float {
	f := new(big.Float)
	f.SetString(s)
	return f
}

func tickToPrice(tick int64) *big.Float {
	price := big.NewFloat(1.0001)
	return price.SetMantExp(price, int(tick))
}

func calculateCurrentPrice(priceStr string) *big.Float {
	price := new(big.Float)
	price.SetPrec(256)
	price.SetString(priceStr)
	return price
}
