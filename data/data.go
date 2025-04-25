package data

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gary/bitfinex-lending-bot/util.go"
	"github.com/gorilla/websocket"
)

type Client struct {
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
	BaseURL    string
}

// FundingOfferRequest represents a funding offer request
type FundingOfferRequest struct {
	Type   string `json:"type"`   // Order type (LIMIT, FRRDELTAVAR, FRRDELTAFIX)
	Symbol string `json:"symbol"` // Symbol for desired pair (fUSD, fBTC, etc.)
	Amount string `json:"amount"` // Amount (positive for offer, negative for bid)
	Rate   string `json:"rate"`   // Daily rate
	Period int    `json:"period"` // Time period of offer (2-120 days)
	Flags  int    `json:"flags"`  // Optional flags
}

// FundingOffer represents a funding offer response
type FundingOffer struct {
	ID             int       `json:"id"`          // Offer ID
	Symbol         string    `json:"symbol"`      // The currency of the offer
	CreatedAt      time.Time `json:"mts_create"`  // Creation timestamp
	UpdatedAt      time.Time `json:"mts_update"`  // Update timestamp
	Amount         float64   `json:"amount"`      // Current amount
	AmountOriginal float64   `json:"amount_orig"` // Original amount
	Type           string    `json:"type"`        // Offer type
	Flags          int       `json:"flags"`       // Flags active on the offer
	Status         string    `json:"status"`      // Offer status
	Rate           float64   `json:"rate"`        // Rate of the offer
	Period         int       `json:"period"`      // Period of the offer
	Notify         bool      `json:"notify"`      // Notify flag
	Hidden         int       `json:"hidden"`      // Hidden flag
	Renew          bool      `json:"renew"`       // Renew flag
}

type BitfinexError struct {
	StatusCode int
	ErrorCode  string
	Message    string
	RawBody    string
}

type FundingStat struct {
	Timestamp             int64   `json:"mts"`
	FRR                   float64 `json:"frr"`
	AveragePeriod         float64 `json:"avg_period"`
	FundingAmount         float64 `json:"funding_amount"`
	FundingAmountUsed     float64 `json:"funding_amount_used"`
	FundingBelowThreshold float64 `json:"funding_below_threshold"`
}

// FundingOfferInfo represents current active funding offer information
type FundingOfferInfo struct {
	Currency string  // Currency code
	Amount   float64 // Amount
	Rate     float64 // Annual interest rate
	Period   float64 // Period (days)
}

// BitfinexOffer represents each quote item returned by Bitfinex API
// Bitfinex API returns format:
// [OFFER_ID, PERIOD, RATE, AMOUNT, ...]
type BitfinexOffer struct {
	OfferID int     // Quote ID
	Period  int     // Period in days
	Rate    float64 // Interest rate
	Amount  float64 // Amount (positive for ask, negative for bid)
}

// TradeMessage represents a trade message
type TradeMessage struct {
	ID        int64
	Timestamp int64
	Amount    float64
	Rate      float64
	Period    int
}

// TradeSubscription represents a trade subscription
type TradeSubscription struct {
	conn      *websocket.Conn
	done      chan struct{}
	onMessage func(TradeMessage)
}

// FundingCredit represents a funding credit
type FundingCredit struct {
	ID     int64
	Status string
	Amount float64
}

func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		APIKey:    apiKey,
		APISecret: apiSecret,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		BaseURL: "https://api.bitfinex.com",
	}
}

func (c *Client) SendRequest(method, path string, body interface{}) ([]byte, error) {
	// Serialize request body
	var bodyStr string
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error serializing request body: %w", err)
		}
		bodyStr = string(jsonData)
	}

	// Generate nonce
	nonce := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	// Create signature payload
	signaturePayload := "/api/" + path + nonce + bodyStr

	// Calculate signature
	h := hmac.New(sha512.New384, []byte(c.APISecret))
	h.Write([]byte(signaturePayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Create request
	url := c.BaseURL + "/" + path
	req, err := http.NewRequest(method, url, bytes.NewBufferString(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("bfx-nonce", nonce)
	req.Header.Set("bfx-apikey", c.APIKey)
	req.Header.Set("bfx-signature", signature)

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errorResp []interface{}
		err := json.Unmarshal(respBody, &errorResp)

		bfxErr := BitfinexError{
			StatusCode: resp.StatusCode,
			RawBody:    string(respBody),
		}

		if err == nil && len(errorResp) >= 3 {
			if code, ok := errorResp[1].(string); ok {
				bfxErr.ErrorCode = code
			}
			if msg, ok := errorResp[2].(string); ok {
				bfxErr.Message = msg
			}
		} else {
			bfxErr.Message = "Failed to parse error response"
		}

		return nil, bfxErr
	}

	return respBody, nil
}

func (e BitfinexError) Error() string {
	return fmt.Sprintf("Bitfinex API Error [%s]: %s (Status Code: %d)",
		e.ErrorCode, e.Message, e.StatusCode)
}

func SendBitfinexRequest(apikey, apisecret, apiPath, requestBody string) ([]byte, error) {
	// Generate nonce (millisecond timestamp)
	nonce := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	// Create signature payload
	signaturePayload := "/api/" + apiPath + nonce + requestBody

	// Calculate HMAC-SHA384 signature
	h := hmac.New(sha512.New384, []byte(apisecret))
	h.Write([]byte(signaturePayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Create HTTP request
	url := "https://api.bitfinex.com/" + apiPath
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set all necessary headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("bfx-nonce", nonce)
	req.Header.Set("bfx-apikey", apikey)
	req.Header.Set("bfx-signature", signature)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errorResp []interface{}
		if err := json.Unmarshal(respBody, &errorResp); err != nil {
			return nil, fmt.Errorf("failed to parse error response: %v, raw response: %s", err, string(respBody))
		} else if len(errorResp) >= 3 {
			return nil, fmt.Errorf("API error [%v]: %v", errorResp[1], errorResp[2])
		}
		return nil, fmt.Errorf("unknown API error: %s", string(respBody))
	}

	return respBody, nil
}

func (c *Client) GetFundingStat(symbol string) ([]FundingStat, error) {
	path := fmt.Sprintf("v2/funding/stats/%s/hist", symbol)
	respBody, err := c.SendRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get funding statistics: %w", err)
	}

	// Parse the response into a slice of FundingStat
	stats, err := parseFundingStats(respBody)
	if err != nil {
		return nil, fmt.Errorf("error parsing FundingStat: %w", err)
	}

	return stats, nil
}

func parseFundingStats(data []byte) ([]FundingStat, error) {
	var rawStats [][]interface{}
	if err := json.Unmarshal(data, &rawStats); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	stats := make([]FundingStat, 0, len(rawStats))
	for _, raw := range rawStats {
		if len(raw) < 12 {
			continue
		}

		ts, ok1 := util.SafeFloat64(raw[0])
		frr, ok2 := util.SafeFloat64(raw[3])
		avgPeriod, ok3 := util.SafeFloat64(raw[4])
		fundingAmt, ok4 := util.SafeFloat64(raw[7])
		fundingUsed, ok5 := util.SafeFloat64(raw[8])
		fundingBelow, ok6 := util.SafeFloat64(raw[11])

		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			continue
		}

		stat := FundingStat{
			Timestamp:             int64(ts),
			FRR:                   frr,
			AveragePeriod:         avgPeriod,
			FundingAmount:         fundingAmt,
			FundingAmountUsed:     fundingUsed,
			FundingBelowThreshold: fundingBelow,
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

func (c *Client) GetNewestTrades() ([]byte, error) {
	path := "v2/trades/fUSD/hist?limit=125&sort=-1"
	return c.SendRequest("GET", path, nil)
}

// SubscribeToTrades subscribes to trade messages
func (c *Client) SubscribeToTrades(symbol string, onMessage func(TradeMessage)) (*TradeSubscription, error) {
	url := "wss://api-pub.bitfinex.com/ws/2"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}

	// Build subscription message
	msg := map[string]interface{}{
		"event":   "subscribe",
		"channel": "trades",
		"symbol":  symbol,
	}

	if err := conn.WriteJSON(msg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("error sending subscription message: %w", err)
	}

	sub := &TradeSubscription{
		conn:      conn,
		done:      make(chan struct{}),
		onMessage: onMessage,
	}

	// Start listening goroutine
	go sub.listen()

	return sub, nil
}

// listen listens for WebSocket messages
func (s *TradeSubscription) listen() {
	defer s.conn.Close()

	for {
		select {
		case <-s.done:
			return
		default:
			_, message, err := s.conn.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %v", err)
				return
			}

			// Parse message
			var msg []interface{}
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error parsing message: %v", err)
				continue
			}

			// Process trade message
			if len(msg) >= 2 && msg[1] == "te" && len(msg) >= 3 {
				data, ok := msg[2].([]interface{})
				if !ok || len(data) < 5 {
					continue
				}

				id, _ := util.SafeInt64(data[0])
				ts, _ := util.SafeInt64(data[1])
				amount, _ := util.SafeFloat64(data[2])
				rate, _ := util.SafeFloat64(data[3])
				period, _ := util.SafeInt(data[4])

				trade := TradeMessage{
					ID:        id,
					Timestamp: ts,
					Amount:    amount,
					Rate:      rate,
					Period:    period,
				}

				s.onMessage(trade)
			}
		}
	}
}

// Close closes the subscription
func (s *TradeSubscription) Close() {
	close(s.done)
}

// Wallet represents a single wallet entry
type Wallet struct {
	Type               string                 // Wallet type (exchange, margin, funding)
	Currency           string                 // Currency code
	Balance            float64                // Balance
	UnsettledInterest  float64                // Unsettled interest
	AvailableBalance   float64                // Available balance
	LastChange         string                 // Last change description
	LastChangeMetadata map[string]interface{} // Last change metadata
}

// GetWallets retrieves all wallets and returns a map of funding wallet balances
func (c *Client) GetWallets() (map[string]float64, error) {
	respBody, err := c.SendRequest("POST", "v2/auth/r/wallets", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallets: %w", err)
	}

	// Parse JSON response
	var wallets [][]interface{}
	if err := json.Unmarshal(respBody, &wallets); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %w", err)
	}

	// Create a map to store funding wallet balances
	fundingBalances := make(map[string]float64)

	for _, wallet := range wallets {
		if len(wallet) >= 5 {
			walletType := wallet[0].(string)
			currency := wallet[1].(string)

			// Only process funding type wallets
			if walletType == "funding" {
				availableBalance := wallet[4].(float64)
				totalBalance := availableBalance
				fundingBalances[currency] = totalBalance
			}
		}
	}

	return fundingBalances, nil
}

func (c *Client) GetRawBookHighest() ([]byte, error) {
	path := "v2/book/fUSD/R0?len=100"
	return c.SendRequest("GET", path, nil)
}

// FindHighestRateForShortestPeriod parses Bitfinex API response data to find the highest rate for the shortest period
func FindHighestRateForShortestPeriod(data []byte) (*BitfinexOffer, error) {
	// Parse JSON data
	var rawOffers [][]interface{}
	err := json.Unmarshal(data, &rawOffers)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	// Convert raw data to structured BitfinexOffer objects
	offers := make([]BitfinexOffer, 0, len(rawOffers))

	for _, rawOffer := range rawOffers {
		if len(rawOffer) < 4 {
			continue // Skip invalid format data
		}

		offerID, okID := util.ToInt(rawOffer[0])
		period, okPeriod := util.ToInt(rawOffer[1])
		rate, okRate := util.ToFloat64(rawOffer[2])
		amount, okAmount := util.ToFloat64(rawOffer[3])

		if !okID || !okPeriod || !okRate || !okAmount {
			continue
		}

		// Only consider ask orders (positive amount)
		if amount >= 0 {
			continue
		}

		offers = append(offers, BitfinexOffer{
			OfferID: offerID,
			Period:  period,
			Rate:    rate,
			Amount:  amount,
		})
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no valid offers found")
	}

	// Sort by period
	sort.Slice(offers, func(i, j int) bool {
		return offers[i].Period < offers[j].Period
	})

	// Find shortest period
	shortestPeriod := offers[0].Period

	// Find highest rate among shortest period
	var bestOffer BitfinexOffer
	highestRate := -1.0

	for _, offer := range offers {
		if offer.Period == shortestPeriod && offer.Rate > highestRate {
			highestRate = offer.Rate
			bestOffer = offer
		}
	}

	return &bestOffer, nil
}

// FindHighestLendingRate finds the highest lending rate that meets the minimum period requirement
func FindHighestLendingRate(data []byte, minPeriod int) (*BitfinexOffer, error) {
	var rawOffers [][]interface{}
	err := json.Unmarshal(data, &rawOffers)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	offers := make([]BitfinexOffer, 0, len(rawOffers))

	for _, rawOffer := range rawOffers {
		if len(rawOffer) < 4 {
			continue
		}

		offerID, okID := util.ToInt(rawOffer[0])
		period, okPeriod := util.ToInt(rawOffer[1])
		rate, okRate := util.ToFloat64(rawOffer[2])
		amount, okAmount := util.ToFloat64(rawOffer[3])

		if !okID || !okPeriod || !okRate || !okAmount {
			continue
		}

		// Only consider borrowing orders (negative amount)
		if amount >= 0 {
			continue
		}

		// Filter out orders that don't meet minimum period requirement
		if period < minPeriod {
			continue
		}

		offers = append(offers, BitfinexOffer{
			OfferID: offerID,
			Period:  period,
			Rate:    rate,
			Amount:  math.Abs(amount), // Convert to positive value for easier understanding
		})
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no valid lending offers found")
	}

	// Find the highest rate order
	highestRateOffer := offers[0]
	for _, offer := range offers[1:] {
		if offer.Rate > highestRateOffer.Rate {
			highestRateOffer = offer
		} else if offer.Rate == highestRateOffer.Rate && offer.Period < highestRateOffer.Period {
			// If rates are equal, prefer shorter period
			highestRateOffer = offer
		}
	}

	// Output result
	fmt.Printf("Found highest rate lending offer:\n")
	fmt.Printf("Rate: %.6f%% (%.6f decimal)\n", highestRateOffer.Rate*100, highestRateOffer.Rate)
	fmt.Printf("Period: %d days\n", highestRateOffer.Period)
	fmt.Printf("Amount: %.2f USD\n", highestRateOffer.Amount)
	fmt.Printf("Order ID: %d\n", highestRateOffer.OfferID)

	return &highestRateOffer, nil
}

func (c *Client) GetTotalWalletBalance() (float64, float64, error) {
	respBody, err := c.SendRequest("POST", "v2/auth/r/wallets", nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get wallets: %w", err)
	}

	var wallets [][]interface{}
	if err := json.Unmarshal(respBody, &wallets); err != nil {
		return 0, 0, fmt.Errorf("failed to parse wallets: %w", err)
	}

	var usdBalance float64
	var ustBalance float64

	for _, wallet := range wallets {
		if len(wallet) >= 3 {
			walletType := wallet[0].(string)
			currency := wallet[1].(string)
			balance := wallet[2].(float64)

			if walletType == "funding" {
				switch currency {
				case "USD":
					usdBalance = balance
				case "UST":
					ustBalance = balance
				}
			}
		}
	}

	return usdBalance, ustBalance, nil
}

// SubmitFundingOffer submits a new funding offer and returns the offer details
func (c *Client) SubmitFundingOffer(offer FundingOfferRequest) (*FundingOffer, error) {
	// Validate required parameters
	if offer.Symbol == "" {
		return nil, fmt.Errorf("symbol cannot be empty")
	}
	if offer.Amount == "" {
		return nil, fmt.Errorf("amount cannot be empty")
	}
	if offer.Rate == "" {
		return nil, fmt.Errorf("rate cannot be empty")
	}
	if offer.Period < 2 || offer.Period > 120 {
		return nil, fmt.Errorf("period must be between 2 and 120 days")
	}

	// If type is not specified, default to LIMIT
	if offer.Type == "" {
		offer.Type = "LIMIT"
	}

	// Send request to Bitfinex API
	respBody, err := c.SendRequest("POST", "v2/auth/w/funding/offer/submit", offer)
	if err != nil {
		return nil, fmt.Errorf("failed to submit funding offer: %v", err)
	}

	// Parse the response
	var response []interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if the response has the expected format
	if len(response) < 8 {
		return nil, fmt.Errorf("invalid response format")
	}

	// Extract the offer data
	offerData, ok := response[4].([]interface{})
	if !ok || len(offerData) < 20 {
		return nil, fmt.Errorf("invalid offer data format")
	}

	// Create and populate the FundingOffer
	result := &FundingOffer{
		ID:             int(offerData[0].(float64)),
		Symbol:         offerData[1].(string),
		CreatedAt:      time.Unix(0, int64(offerData[2].(float64))*int64(time.Millisecond)),
		UpdatedAt:      time.Unix(0, int64(offerData[3].(float64))*int64(time.Millisecond)),
		Amount:         offerData[4].(float64),
		AmountOriginal: offerData[5].(float64),
		Type:           offerData[6].(string),
		Flags:          int(offerData[9].(float64)),
		Status:         offerData[10].(string),
		Rate:           offerData[14].(float64),
		Period:         int(offerData[15].(float64)),
		Notify:         offerData[16].(bool),
		Hidden:         int(offerData[17].(float64)),
		Renew:          offerData[19].(bool),
	}

	return result, nil
}

// CancelFundingOffer cancels an existing funding offer
func (c *Client) CancelFundingOffer(offerID int) error {
	payload := map[string]interface{}{
		"id": offerID,
	}

	// Send the request to cancel the funding offer
	respBody, err := c.SendRequest("POST", "v2/auth/w/funding/offer/cancel", payload)
	if err != nil {
		return fmt.Errorf("failed to cancel funding offer: %v", err)
	}

	// Parse the response
	var response []interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if the response indicates success
	if len(response) >= 7 && response[6].(string) != "SUCCESS" {
		return fmt.Errorf("failed to cancel funding offer: %s", response[7].(string))
	}

	return nil
}
