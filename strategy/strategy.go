package strategy

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gary/bitfinex-lending-bot/data"
	"github.com/joho/godotenv"
)

// Distribution represents the fund allocation ratio
type Distribution struct {
	Fix     float64 // Fixed lending ratio
	Predict float64 // Predictive lending ratio
}

// CurrentPredictOrder represents the current prediction order
type CurrentPredictOrder struct {
	ID     int       // Order ID
	Rate   float64   // Interest rate
	Period int       // Period (days)
	Since  time.Time // Creation time
}

// StrategyManager manages the execution of lending strategies
func StrategyManager() {
	// Initialize order status
	currentPredictOrder := []CurrentPredictOrder{}
	currentOrderBool := false

	// Set fund allocation ratio
	distribution := Distribution{
		Fix:     0.5, // 50% for fixed lending
		Predict: 0.5, // 50% for predictive lending
	}

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("BITFINEX_API_KEY")
	apiSecret := os.Getenv("BITFINEX_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatal("API key and secret must be set in environment variables.")
	}

	// Create API client
	client := data.NewClient(apiKey, apiSecret)

	// 1. Get total balance
	usdBalance, ustBalance, err := client.GetTotalWalletBalance()
	if err != nil {
		log.Fatal("Error getting total wallet balance:", err)
	}
	fmt.Printf("Total balance: %.2f USD, %.2f UST\n", usdBalance, ustBalance)

	// 2. Get available balance
	wallets, err := client.GetWallets()
	if err != nil {
		log.Fatal("Error getting wallets:", err)
	}

	var availableUsdBalance float64
	if balance, exists := wallets["USD"]; exists {
		availableUsdBalance = balance
		fmt.Printf("Available fund balance: %.2f USD\n", availableUsdBalance)
	} else {
		fmt.Println("USD funding wallet not found")
		return
	}

	// 3. Calculate allocation amounts
	fixUsdBalance := usdBalance * distribution.Fix
	predictUsdBalance := usdBalance * distribution.Predict

	fmt.Printf("Allocation strategy: Fixed lending %.2f USD (%.1f%%), Predictive lending %.2f USD (%.1f%%)\n",
		fixUsdBalance, distribution.Fix*100,
		predictUsdBalance, distribution.Predict*100)

	// 4. Calculate amount needed for lending
	alreadyLentAmount := usdBalance - availableUsdBalance

	// Calculate remaining amount to lend
	remainFixUsdBalance := fixUsdBalance - (alreadyLentAmount * distribution.Fix)
	remainPredictUsdBalance := predictUsdBalance - (alreadyLentAmount * distribution.Predict)

	fmt.Printf("Already lent: %.2f USD\n", alreadyLentAmount)
	fmt.Printf("Remaining fixed lending: %.2f USD\n", remainFixUsdBalance)
	fmt.Printf("Remaining predictive lending: %.2f USD\n", remainPredictUsdBalance)

	// 5. Handle fixed lending
	if remainFixUsdBalance > 150 {
		// Check available balance
		if availableUsdBalance < remainFixUsdBalance {
			fmt.Printf("Warning: Available balance %.2f USD is insufficient for fixed lending requirement %.2f USD\n",
				availableUsdBalance, remainFixUsdBalance)
			remainFixUsdBalance = availableUsdBalance // Adjust to available balance
		}

		if remainFixUsdBalance > 150 {
			// Find best offer
			highest, err := client.GetRawBookHighest()
			if err != nil {
				log.Printf("Error getting book: %v", err)
				return
			}

			bestOffer, err := data.FindHighestRateForShortestPeriod(highest)
			if err != nil {
				log.Printf("Error finding highest lending rate: %v", err)
				return
			}

			fmt.Println("\nBest offer found:")
			fmt.Printf("Offer ID: %d\n", bestOffer.OfferID)
			fmt.Printf("Period: %d days\n", bestOffer.Period)
			fmt.Printf("Rate: %.6f%%\n", bestOffer.Rate*100)
			fmt.Printf("Amount: %.2f USD\n", bestOffer.Amount)

			// Submit fixed lending order
			offer := data.FundingOfferRequest{
				Type:   "LIMIT",
				Symbol: "fUSD",
				Amount: fmt.Sprintf("%.2f", remainFixUsdBalance),
				Rate:   fmt.Sprintf("%.6f", bestOffer.Rate),
				Period: bestOffer.Period,
				Flags:  0,
			}

			fmt.Printf("Submitting fixed lending order: %.2f USD @ %.6f%% for %d days\n",
				remainFixUsdBalance, bestOffer.Rate*100, bestOffer.Period)

			res, err := client.SubmitFundingOffer(offer)
			if err != nil {
				log.Printf("Failed to submit fixed lending order: %v", err)
			} else {
				fmt.Printf("Successfully submitted fixed lending order: ID=%d, Status=%s\n", res.ID, res.Status)
			}
			availableUsdBalance -= remainFixUsdBalance
		}
	} else {
		fmt.Println("No fixed lending requirement")
	}

	// 6. Handle predictive lending
	if remainPredictUsdBalance > 150 {
		// Check available balance
		if availableUsdBalance < remainPredictUsdBalance {
			fmt.Printf("Warning: Available balance %.2f USD is insufficient for predictive lending requirement %.2f USD\n",
				availableUsdBalance, remainPredictUsdBalance)
			remainPredictUsdBalance = availableUsdBalance // Adjust to available balance
		}

		if remainPredictUsdBalance > 150 {
			// Get latest funding statistics
			stats, err := client.GetFundingStat("fUSD")
			if err != nil {
				log.Printf("Failed to get funding statistics: %v", err)
				return
			}

			if len(stats) > 0 {
				// Cancel existing prediction orders if any
				if currentOrderBool {
					for _, order := range currentPredictOrder {
						err := client.CancelFundingOffer(order.ID)
						if err != nil {
							log.Printf("Failed to cancel order (ID: %d): %v", order.ID, err)
						}
					}
					currentPredictOrder = []CurrentPredictOrder{} // Clear slice
					currentOrderBool = false
				}

				var latestStat = stats[0]
				fmt.Printf("\nLatest funding statistics:\n")
				fmt.Printf("Timestamp: %d\n", latestStat.Timestamp)
				fmt.Printf("FRR (Flash Return Rate): %.6f%%\n", latestStat.FRR*365*100)
				fmt.Printf("Average Period: %.2f days\n", latestStat.AveragePeriod)
				fmt.Printf("Total Funding: %.2f USD\n", latestStat.FundingAmount)
				fmt.Printf("Used Funding: %.2f USD\n", latestStat.FundingAmountUsed)
				fmt.Printf("Below Threshold Funding: %.2f USD\n", latestStat.FundingBelowThreshold)

				// Calculate predicted rate (FRR * 1.3)
				predictRate := latestStat.FRR * 1.3

				// Submit predictive lending order
				offer := data.FundingOfferRequest{
					Type:   "LIMIT",
					Symbol: "fUSD",
					Amount: fmt.Sprintf("%.2f", remainPredictUsdBalance),
					Rate:   fmt.Sprintf("%.6f", predictRate),
					Period: 2,
					Flags:  0,
				}

				fmt.Printf("Submitting predictive lending order: %.2f USD @ %.6f%% for %d days\n",
					remainPredictUsdBalance, predictRate*100, 2)

				res, err := client.SubmitFundingOffer(offer)
				if err != nil {
					log.Printf("Failed to submit predictive lending order: %v", err)
				} else {
					current := CurrentPredictOrder{
						ID:     res.ID,
						Rate:   res.Rate,
						Period: res.Period,
						Since:  res.CreatedAt,
					}
					currentPredictOrder = append(currentPredictOrder, current)
					currentOrderBool = true
					fmt.Printf("Successfully submitted predictive lending order: ID=%d, Status=%s\n", res.ID, res.Status)
				}
			}
		}
	} else {
		fmt.Println("No predictive lending requirement")
	}

	time.Sleep(300 * time.Second)
}
