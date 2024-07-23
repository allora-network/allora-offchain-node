# allora-offchain-node
Allora off-chain nodes publish inferences, forecasts, and losses to the Allora chain.

## Off-chain worker/reputer modifications

Starting fresh of off-chain nodes.

Could be written in Golang/Python/Typescript. Golang probs easier for go routines and has Cosmos client (Ignite).

Ideally every component here is written in the same language.

Overall Off-Chain Flow and Configuration

There's 1 script to write:

```shell
./run.sh config.json
```

The 1 argument here is a config.json file where the model and source of truth is specified.

Use the following schema:

```json
{
  "options": {
    "wallet": "keys.txt",
    "reset_db": true,
    "request_retries": 3,
    "node": "http://rpc.allora.network",
    "loop_seconds": 60,
    "min_stake_to_repute": "50000"
  },
  "worker": [
    {
      "topic_id": 1,
      "inference_entrypoint": "topic_1_model_1.go",  // we expect this to take block height arg
      "forecast_entrypoint": "topic_1_model_2.go"    // we expect this to take block height arg
    },
    ...
  ],
  "reputer": [
    {
      "topic_id": 1,
      // We expect this to accept or gather "loss name/id", current_nonce, topic.
      // In the file, we should get previous losses & other parameters of loss request.
      "loss_entrypoint": "topic_1_reputer_model.go"
    },
    ...
  ]
}
```

### Possible simplifications

* assume config.json exists in same directory as script and is indeed called config.json
* always reset_db => don't make it an option
* just use default value instead of loop_seconds => don't make it a option
* just use default value instead of request_retries => don't make it a option

## run.sh

Be sure to log all successes and failures. Extensive logging and clear tracing is key to be able to help others.

Could spin off a distinct processes per role worker, reputer

### Worker process

1. Spawn a go routine per topic
2. Get topic data from chain via RPC. Hold this in memory
3. Check if wallet registered in topic as worker
4. If wallet not registered in topic as worker then attempt to register
   1. Fail if failed to register
5. Every config.loop_seconds seconds…
   1. Get and set latest_open_worker_nonce_from_chain from the chain
   2. If latest_open_worker_nonce_from_chain does not exist or nil then continue to next loop
      1. i.e. wait another config.loop_seconds
   3. Get current block height from chain and set current_block_height
   4. Retry request_retries times (ideally with backoff or some fixed wait time):
      1. If current_block_height ≥ latest_open_worker_nonce_from_chain && current_block_height ≤ latest_open_worker_nonce_from_chain + topic.window, then
         1. Invoke configured inference_entrypoint, forecast_entrypoint for topic and get results
            1. These files should gather and compute data to produce inference and forecast for appropriate timestep and return them
         1. Else, break this inner retry loop
      1. Attempt to commit inference and forecast bundle to the chain
         1. Log success/failures as usual

### Reputer process:

1. Spawn a go routine per topic
2. Get topic data from chain via RPC. Hold this in memory
3. Check if wallet registered in topic as reputer
4. If wallet not registered in topic as reputer then attempt to register
   1. Fail if failed to register
5. Get current stake from reputer on topic (not including delegate stake)
   1. Fail if failed to get
6. If config.min_stake_to_repute > current_stake then attempt to add difference in stake (config.min_stake_to_repute - current_stake) to hit the configured minimum, using config.wallet
   1. Fail if failed to add stake
   2. If success or if condition met, then continue with rest of loop
7. Every config.loop_seconds seconds…
   1. Get and set latest_open_reputer_nonce_from_chain from the chain
   2. If latest_open_reputer_nonce_from_chain does not exist or nil then continue to next loop
      1. i.e. wait another config.loop_seconds
   3. Get current block height from chain and set current_block_height
   4. Retry request_retries times (ideally with backoff or some fixed wait time):
      1. If current_block_height ≥ latest_open_reputer_nonce_from_chain && current_block_height ≤ latest_open_reputer_nonce_from_chain + topic.epoch_length, then
         1. Invoke configured loss_entrypoint for topic and get results
            1. loss_entrypoint should gather necessary data to compute losses and return them
         2. Else, break this inner retry loop
      2. Attempt to commit loss bundle to the chain
         1. Log success/failures as usual

### Queries and Transactions Needed by Workers and Reputers

Please mock these for now until they're solidified in a PR (at least that) on allora-chain

* GetTopic (already exists)
* Get latest unsealed (worker) nonce
* latest_open_worker_nonce_from_chain
* Get latest unsealed (reputer) nonce
* latest_open_reputer_nonce_from_chain
* Commit worker bundle tx
* Commit reputer bundle tx

