# Gas and Fees

Allora off-chain nodes submits data to the Allora Network in the form of transactions. These transactions are configured in a per-wallet basis. The wallet configuration is in `config.json` under the `wallet` field.

## Notions

- **Gas**: the amount of computational work required to execute a transaction. It is expressed in gas units, typically denominated in `uallo`.
- **Fees**: the amount of `uallo` paid for a transaction, paid at a specific gas price.

Allora Network implements the [Feemarket](https://github.com/skip-mev/feemarket) module to introduce variability in the gas price in the chain. The offchain node user is highly encouraged to set the gas prices to `auto`, so the node can automatically calculate the gas price based on chain's feemarket-based gas prices, otherwise the node will use a constant gas price which may lead to transaction failure.

## Settings


### Gas calculation
- `gasAdjustment` (float): is the adjustment factor for the gas used. This is used to increase the factor provided by the chain to account for the actual gas price in the chain. Recommended: `1.2`.


### Gas Prices

The chain's feemarket module specifies the variable gas price. The offchain node may configure a fixed price or set it to auto, in which case the node will automatically calculate the gas price based on chain's feemarket-based gas prices. This is the recommended setting.
- `maxFees` (Number): set the max fees that can be paid for a transaction. They are expressed numerically in `uallo`. It is recommended to adjust this value based on experience to optimize results, although a value of `5000000` could be a good starting point.
- `gasPrices` (string): can be set to `auto` or a specific gas price. If set to `auto`, the node will automatically calculate the gas price based on chain's feemarket-based gas prices. This is the recommended setting, since feemarket introduces variability in the gas price in the chain. Recommended: `auto`.
- `gasPriceUpdateInterval` (int): is the interval in seconds at which the node will update the gas price from the network. This is only relevant when `gasPrices` is set to `auto`. Recommended: similar to estimated block duration. It can vary greatly, so it is recommended to set it to a conservative value and adjust accordingly.


### Insufficient Fees

If gas is OK, the transaction may incur in an insufficient fees error. This means the gas price at which gas is paid is not enough to pay for the transaction as per current gas prices in the chain.
The node will retry with the amount returned by the chain's feemarket module endpoints, provided the resultant amount is lower than the `maxFees` set by the user, otherwise `maxFees` will be used.



