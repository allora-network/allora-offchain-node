# Gas and Fees

Allora off-chain nodes submits data to the Allora Network in the form of transactions. These transactions are configured in a per-wallet basis. The wallet configuration is in `config.json` under the `wallet` field.

## Notions

- **Gas**: the amount of computational work required to execute a transaction. It is expressed in gas units, typically denominated in `uallo`.
- **Fees**: the amount of `uallo` paid for a transaction, paid at a specific gas price.

Allora Network implements the [Feemarket](https://github.com/skip-mev/feemarket) module to introduce variability in the gas price in the chain. The offchain node user is highly encouraged to set the gas prices to `auto`, so the node can automatically calculate the gas price based on chain's feemarket-based gas prices, otherwise the node will use a constant gas price which may lead to transaction failure.

## Settings

### Fee and gas calculation
- `BaseGas` (int): set the base gas for a transaction. This is the minimum gas that will be used for a transaction. Recommended: `200000`.
- `GasPerByte` (int): set the gas price per byte for a transaction. This is the price of the transaction in `uallo` per byte. Recommended: `1`.
- `maxFees` (float): set the max fees that can be paid for a transaction. They are expressed numerically in `uallo`. It is recommended to adjust this value based on experience to optimize results, although a conservative value of `500000` could be a good starting point.

### Gas prices
The chain's feemarket module specifies the variable gas price. The offchain node may configure a fixed price or set it to auto, in which case the node will automatically calculate the gas price based on chain's feemarket-based gas prices. This is the recommended setting.
- `gasPrices` (string): can be set to `auto` or a specific gas price. If set to `auto`, the node will automatically calculate the gas price based on chain's feemarket-based gas prices. This is the recommended setting, since feemarket introduces variability in the gas price in the chain. Recommended: `auto`.
- `gasPriceUpdateInterval` (int): is the interval in seconds at which the node will update the gas price from the network. This is only relevant when `gasPrices` is set to `auto`. Recommended: similar to estimated block duration. It can vary greatly, so it is recommended to set it to a conservative value and adjust accordingly.


## Adjustments

### Out of gas
A transaction must define its gas limit. In case of a failure, the node will parse the `used` amount expressed by the chain plus a small gas excess times number of iterations to help retrials.

### Insufficient fees
If gas is OK, the transaction may incur in an insufficient fees error. This means the gas price at which gas is paid is not enough to pay for the transaction as per current gas prices in the chain.
The node will retry with the amount expressed by the chain, provided it is lower than the `maxFees` set by the user.



