# allora-offchain-node

Allora off-chain nodes publish inferences, forecasts, and losses informed by a configurable ground truth to the Allora chain.

## How to run with docker
1. Clone the repository
2. Make sure to remove any .env file so it doesn't clash with the automated environment variables
3. Copy config.example.json and populate with your variables. You can either populate with your existing wallet or leave it empty for it to be autocreated


```shell
cp config.example.json config.json
```
4. Run

```shell
chmod +x init.docker
./init.docker 
```

from the root diectory. This will:
   - Automatically create allora keys for you. You will have to request for some tokens from faucet to be able to register your worker and stake your reputer. You can find your address in ./data/env_file
   - Automatically export the needed variables from the account created to be used by the offchain node and bundles it with the your provided config.json and then pass them to the node as environemnt variable

5. Run `docker compose up --build`. This will:
   - Run the both the offchain node and the source services, communicating through endpoints attached to the internal dns

Please note that the environment variable will be created as bumdle of your config.json and allora account secrets, please make sure to remove every sectrets before commiting to remote git repository


## How to run without docker

1. Clone the repository
2. Install Go 1.22.5
3. Install the dependencies:

```shell
go mod download
```

4. Copy environment variables:

```shell
cp .env.example .env
```

5. Fill in the environment variables in `.env` with your own values
6. If you're a worker...
   1. Configure your inference and/or forecast models in `adapters/` and/or another source linked to by adapters in `adapters/` directory.
7. If you're a reputer...
   1. Configure your repute and/or loss models in `adapters/` and/or another source linked to by adapters in `adapters/` directory.
8. Map each topic to the appropriate adapter in `config.json`.
9. Run the following commands:

```shell
chmod +x start.local
./start.local
```


## How to configure

There are several ways to configure the node. In order of preference, you can do any of these: 
* Set the `ALLORA_OFFCHAIN_NODE_CONFIG_JSON` env var with a configuration as a JSON string.
* Set the `ALLORA_OFFCHAIN_NODE_CONFIG_FILE_PATH` env var pointing to a file, which contains configuration as JSON. An example if provided in `config.example.json`.

Each option completely overwrites the other options.


This is the entrypoint for the application that simply builds and runs the Go program.

It spins off a distinct processes per role worker, reputer per topic configered in `config.json`.

### Worker process

1. Spawn a go routine per topic
2. Get topic data from chain via RPC. Hold this in memory
3. Check if wallet registered in topic as worker
4. If wallet not registered in topic as worker then attempt to register
   1. Fail if failed to register
5. Every config.loop_seconds seconds...
   1. Get and set latest_open_worker_nonce_from_chain from the chain
   2. If latest_open_worker_nonce_from_chain does not exist or nil then continue to next loop
      1. i.e. wait another config.loop_seconds
   3. Retry request_retries times with uniform backoff:
      1. Invoke configured `inferenceEntrypoint`, `forecastEntrypoint` for topic and get results
         1. Else, break this inner retry loop
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
7. Every config.loop_seconds seconds...
   1. Get and set latest_open_reputer_nonce_from_chain from the chain
   2. If latest_open_reputer_nonce_from_chain does not exist or nil then continue to next loop
      1. i.e. wait another config.loop_seconds
   3. Retry request_retries times with uniform backoff:
      1. Invoke configured `truthEntrypoint, lossEntrypoint` for topic and get results
         1. Else, break this inner retry loop
      2. Attempt to commit loss bundle to the chain
         1. Log success/failures as usual

## Future Work

* For now, we put adapters to generate or relay reputer/worker data in packages.
   * Should use modules instead of packages
   * Then in JSON one can specify which modules to use for which topics and automatically load them with a script that calls `go get ...`
* Make lambda function adapters => super cheap to continuously run for all those with AWS accounts

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
