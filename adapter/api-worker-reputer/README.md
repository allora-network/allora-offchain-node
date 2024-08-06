# Allora Offchain API Adapter

This repository contains the adapter module for the Allora Offchain Node. The adapter is responsible for connecting the offchain node to external systems via hitting an URL, for inferences, forecasts or source of truth.

It is intended to be used by configuration.

This Adapter is intended to be used to send inferences and/or forecasts and/or source_truth from external services which provide an API endpoint.

## Config

To use and configure this adapter, please use the WorkerConfig object. 
Example as Worker:

```
Worker: []lib.WorkerConfig{
    TopicId:             1,
    InferenceEntrypoint: apiAdapter.NewAlloraAdapter(),
    ForecastEntrypoint:  apiAdapter.NewAlloraAdapter(),
    LoopSeconds:         5,
    Parameters: map[string]string{
        "Token":             "ETH",
        "InferenceEndpoint": "http://localhost:8000/inference/{Token}",
        "ForecastEndpoint":  "http://localhost:8000/forecast/{TopicId}/{BlockHeight}",
    },
},
```

Example as Reputer: 
```
Reputer: []lib.ReputerConfig{
    {
        TopicId:           1,
        ReputerEntrypoint: apiAdapter.NewAlloraAdapter(),
        LoopSeconds:       30,
        MinStake:          100000,
        Parameters: map[string]string{
            "SourceOfTruthEndpoint": "http://localhost:8000/groundtruth/{Token}/{BlockHeight}",
            "Token":                 "ethereum",
        },
    },
},
```

## Parameters 
The parameters section contains additional properties the user wants to use to configure their URLs to hit.

### Worker

`InferenceEndpoint` is required if `InferenceEntrypoint` is defined.
`ForecastEndpoint` is required if `ForecastEntrypoint` is defined.

`InferenceEndpoint`: provides the inference endpoint to hit. It supports URL template variables.
`ForecastEndpoint`: provides the forecast endpoint to hit. It supports URL template variables.

If it is not desired to send inferences or forecasts, it can be configured by setting that specific entrypoint to nil. Example, for not sending inferences:
```
InferenceEntrypoint: nil
```

### Reputer

`SourceOfTruthEndpoint`is required if `ReputerEntrypoint` is defined.

### Additional Parameters 

Any additional parameter can be defined freely, like `Token` in the example, and be used in the endpoint templates.
Additional parameters do not support template variables.


## Template variables

The URLs support template variables as defined from the Parameters section. 

In addition, it supports two special variables: 
* TopicId: as defined in WorkerConfig object
* BlockHeight: the blockheight at which the operation happens


## Usage

* Set up your inference and/or forecast models and serve results via an API. 
* Add a Worker configuration like the above in your config.go, configuring your endpoints appropriately.
* Configure the rest of the Allora Offchain Node (e.g. wallet)
* Run the Allora Offchain Node

