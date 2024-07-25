# allora-offchain-node
Allora off-chain nodes publish inferences, forecasts, and losses to the Allora chain.

## How to run

```shell
./run.sh
```

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
         2. Else, break this inner retry loop
      2. Attempt to commit inference and forecast bundle to the chain
         1. Log success/failures as usual

### Reputer process

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
* Get latest open (worker) nonce
   * latest_open_worker_nonce_from_chain
* Get latest open (reputer) nonce
   * latest_open_reputer_nonce_from_chain
* Commit worker bundle tx
* Commit reputer bundle tx

## Associated Project

https://linear.app/upshot/project/removing-b7s-1a1aeb0a6477/overview

## Future Work

* For now, we put all topic-specific reputer/worker logic in packages.
   * In immediate next iteration, should create modules for various modules and sources of ground truth
   * Then in JSON, can specify which modules to use for which topics and automatically load them
* Could make this whole thing (or modules wihtin it) a lambda function => super cheap
* Use better logging library

## Notes

topic.EpochLastEnded + N*topic.EpochLength
N=1,2,3,4...

1. Get topic state
	1. Could check for topic active first to minimize compute: IsTopicActive query
2. Calculate start and end of next window
	1. `soonest_start = topic.EpochLastEnded + topic.EpochLength`
	2. `soonest_end = soonest_start + topic.WindowLength`
3. Try to get inferences in that window
	1. Look at how we use Ignite client in allora-inference-base
4. Regardless if success/fail, wait for next window and try again

For reputers, you don't have a window, you have the full epoch length to submit a loss bundle.
Note: The loss bundle is a bit more complicated than the inference and forecast bundles. It requires a bit more data and computation. You can also get this from allora-inference-base and/or where we define the loss functions
   * They take the form of ReputerValueBundle (as defined in allora-chain)

Is there a way to get latest "time per block" from the chain.
^That'd be ideal.

If not, then you can assume 5sec/block

### Intended workflow

{
  "topic": 1,
  "model": "model_module_name"
}

1. clone this repo
2. run some script (TBD) and that script will:
--> map json to go file format
--> `go get ...` to get the model module
3. ./run.sh

It is up to the user to ensure that they have the correct model module installed and that it is compatible with the topic they are trying to work on, and they specify this model in `config.go`

