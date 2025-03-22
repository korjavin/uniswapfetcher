package uniswap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
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

// APIClient implements a client for the Uniswap V3 subgraph API.
// The subgraph provides indexed blockchain data for Uniswap V3, making it efficient
// to query position details, pool statistics, and historical data without direct
// blockchain calls.
type APIClient struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

func NewAPIClient(logger *zap.SugaredLogger) (*APIClient, error) {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}, nil
}

// GetPositions fetches all Uniswap V3 positions for a given wallet address using the subgraph API.
// The subgraph maintains an index of all positions, their tokens, pools, and related metrics,
// making it much more efficient than querying the blockchain directly.
func (c *APIClient) GetPositions(ctx context.Context, req PositionRequest) ([]Position, error) {
	c.logger.Debugw("Getting positions for wallet", "address", req.WalletAddress.Hex())
	// GraphQL query for positions
	query := `
	query ($owner: String!) {
		positions(where: {owner: $owner}) {
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
				token0Price
				token1Price
			}
			depositedToken0
			depositedToken1
			withdrawnToken0
			withdrawnToken1
			collectedFeesToken0
			collectedFeesToken1
			liquidity
			tickLower
			tickUpper
		}
	}`
	// Variables for the query
	variables := map[string]interface{}{
		"owner": strings.ToLower(req.WalletAddress.Hex()),
	}
	c.logger.Debugw("Executing GraphQL query", "variables", variables)

	// Execute query
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
					ID          string `json:"id"`
					FeeTier     string `json:"feeTier"`
					Token0Price string `json:"token0Price"`
					Token1Price string `json:"token1Price"`
				} `json:"pool"`
				Liquidity       string `json:"liquidity"`
				TickLower       string `json:"tickLower"`
				TickUpper       string `json:"tickUpper"`
				DepositedToken0 string `json:"depositedToken0"`
				DepositedToken1 string `json:"depositedToken1"`
				WithdrawnToken0 string `json:"withdrawnToken0"`
				WithdrawnToken1 string `json:"withdrawnToken1"`
			} `json:"positions"`
		} `json:"data"`
	}

	if err := c.executeGraphQLQuery(ctx, query, variables, &response); err != nil {
		c.logger.Errorw("Failed to fetch positions", "error", err)
		return nil, fmt.Errorf("failed to fetch positions: %w", err)
	}

	c.logger.Debugw("Got positions response", "positionCount", len(response.Data.Positions))

	var positions []Position
	for _, p := range response.Data.Positions {
		// Convert data to Position struct
		position, err := c.convertToPosition(p)
		if err != nil {
			c.logger.Warnw("Failed to convert position", "id", p.ID, "error", err)
			continue
		}
		positions = append(positions, position)
	}

	return positions, nil
}

func (c *APIClient) executeGraphQLQuery(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	c.logger.Debugw("Making GraphQL request",
		"url", UniswapSubgraphURLV3,
		"bodyLength", len(bodyBytes),
		"query", query,
		"variables", variables)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf(UniswapSubgraphURLV3, os.Getenv("API_KEY")), bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debugw("Got GraphQL response",
		"statusCode", resp.StatusCode,
		"contentLength", len(respBody),
		"response", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	// First try to decode as GraphQLResponse to check for errors
	var graphQLResp GraphQLResponse
	if err := json.Unmarshal(respBody, &graphQLResp); err != nil {
		return fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(graphQLResp.Errors) > 0 {
		c.logger.Errorw("GraphQL query returned errors",
			"errors", graphQLResp.Errors,
			"query", query,
			"variables", variables)
		return fmt.Errorf("GraphQL errors: %v", graphQLResp.Errors)
	}

	// If no errors, decode the full response into the result
	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to decode response data: %w", err)
	}

	return nil
}

// Helper function to convert string to big.Int
func stringToBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}

func (c *APIClient) Close() {
	// Nothing to close for HTTP client
}

func calculateCurrentPrice(priceStr string) *big.Float {
	price := new(big.Float)
	price.SetPrec(256)
	price.SetString(priceStr)
	return price
}

func tickToPrice(tick int64) *big.Float {
	price := big.NewFloat(1.0001)
	return price.SetMantExp(price, int(tick))
}

func (c *APIClient) convertToPosition(p struct {
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
		ID          string `json:"id"`
		FeeTier     string `json:"feeTier"`
		Token0Price string `json:"token0Price"`
		Token1Price string `json:"token1Price"`
	} `json:"pool"`
	Liquidity       string `json:"liquidity"`
	TickLower       string `json:"tickLower"`
	TickUpper       string `json:"tickUpper"`
	DepositedToken0 string `json:"depositedToken0"`
	DepositedToken1 string `json:"depositedToken1"`
	WithdrawnToken0 string `json:"withdrawnToken0"`
	WithdrawnToken1 string `json:"withdrawnToken1"`
}) (Position, error) {
	token0Decimals, _ := strconv.ParseUint(p.Token0.Decimals, 10, 8)
	token1Decimals, _ := strconv.ParseUint(p.Token1.Decimals, 10, 8)
	feeTier, _ := strconv.ParseUint(p.Pool.FeeTier, 10, 32)

	// Calculate amounts
	depositedToken0 := stringToBigInt(p.DepositedToken0)
	depositedToken1 := stringToBigInt(p.DepositedToken1)
	withdrawnToken0 := stringToBigInt(p.WithdrawnToken0)
	withdrawnToken1 := stringToBigInt(p.WithdrawnToken1)
	amount0 := new(big.Int).Sub(depositedToken0, withdrawnToken0)
	amount1 := new(big.Int).Sub(depositedToken1, withdrawnToken1)

	// Calculate price range
	tickLower, _ := strconv.ParseInt(p.TickLower, 10, 64)
	tickUpper, _ := strconv.ParseInt(p.TickUpper, 10, 64)
	priceLower := tickToPrice(tickLower)
	priceUpper := tickToPrice(tickUpper)
	currentPrice := calculateCurrentPrice(p.Pool.Token0Price)

	return Position{
		ID:      stringToBigInt(p.ID),
		Version: VersionV3,
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
		Amount0:      amount0,
		Amount1:      amount1,
		FeeTier:      uint32(feeTier),
		CreatedAt:    time.Now(), // Note: The Graph API doesn't provide creation time in this query
		TickLower:    int(tickLower),
		TickUpper:    int(tickUpper),
		Liquidity:    stringToBigInt(p.Liquidity),
		PriceLower:   priceLower,
		PriceUpper:   priceUpper,
		CurrentPrice: currentPrice,
	}, nil
}

type GetSwapsVars struct {
	Skip      int    `json:"skip"`
	First     int    `json:"first"`
	OrderBy   string `json:"orderBy"`
	OrderDir  string `json:"orderDirection"`
	PairParam string `json:"pairAddress"`
}
