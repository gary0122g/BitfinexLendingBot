
# Bitfinex Lending Bot

## Overview
This project is a cryptocurrency lending bot designed for the Bitfinex platform, currently in active development. The bot implements an automated lending strategy to optimize lending returns while maintaining a balance between stability and opportunistic gains.

## Core Strategy
The bot employs a dual-strategy approach to fund allocation:

### 1. Fixed Lending (50%)
- Allocates half of the available funds to fixed-rate lending
- Ensures stable and consistent returns
- Automatically selects the highest rate available for the shortest period
- Provides a baseline income stream

### 2. Predictive Lending (50%)
- Uses the remaining funds for dynamic rate lending
- Based on Flash Return Rate (FRR) with a multiplier (currently 1.3x)
- Aims to capture higher returns during favorable market conditions
- Adjusts lending rates based on market dynamics

## Current State
The project has established essential infrastructure including:
- Robust Bitfinex API integration
- Secure authentication handling
- Real-time market data processing
- Basic wallet management
- Order execution system
- Fund allocation management

## Future Development
While the basic infrastructure is in place, the core focus for future development is on enhancing the lending strategies, which are crucial for maximizing returns. Planned improvements include:

- Advanced rate prediction algorithms
- Dynamic allocation ratios
- Market trend analysis
- Risk management systems
- Performance analytics
- Multiple currency support
- Enhanced error handling

## Important Note
This project is still under development. The lending strategy is the key to improving returns, and we are actively working on optimizing and expanding the strategic capabilities of the bot.

## Requirements
- Go 1.x
- Bitfinex API credentials
- Environment variables setup (.env file)

## Getting Started
1. Clone the repository
2. Set up your environment variables in `.env`:
   ```
   BITFINEX_API_KEY=your_api_key
   BITFINEX_API_SECRET=your_api_secret
   ```
3. Build and run the project

## Disclaimer
This bot is experimental and should be used with caution. Always start with small amounts and monitor the bot's performance carefully. Cryptocurrency lending carries inherent risks, and past performance does not guarantee future results.

## Contributing
We welcome contributions, especially in the areas of:
- Lending strategy optimization
- Market analysis algorithms
- Risk management features
- Performance monitoring tools

## License
[MIT License](LICENSE)

