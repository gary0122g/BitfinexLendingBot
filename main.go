package main

import (
	"github.com/gary/bitfinex-lending-bot/strategy"
)

func main() {
	for {
		strategy.StrategyManager()
	}
}
