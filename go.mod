module allora_offchain_node

go 1.21.5

require (
	allora_offchain_node/pkg/worker_coin_predictor_10min_eth v0.0.0
	allora_offchain_node/pkg/worker_coin_predictor_20min v0.0.0
	allora_offchain_node/pkg/reputer_coingecko_l1_norm v0.0.0
// allora_offchain_node/pkg/worker2 v0.0.0
// allora_offchain_node/pkg/reputer1 v0.0.0
// allora_offchain_node/pkg/reputer2 v0.0.0
)

replace (
	allora_offchain_node/pkg/coin_predictor_10min_eth => ./pkg/worker_coin_predictor_10min_eth
	allora_offchain_node/pkg/coin_predictor_20min => ./pkg/worker_coin_predictor_20min
	allora_offchain_node/pkg/reputer_coingecko_l1_norm => ./pkg/reputer_coingecko_l1_norm
// 	allora_offchain_node/pkg/worker2 => ./pkg/worker2
// 	allora_offchain_node/pkg/reputer1 => ./pkg/reputer1
// 	allora_offchain_node/pkg/reputer2 => ./pkg/reputer2
)
