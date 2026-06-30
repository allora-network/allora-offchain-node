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

Example as Reputer ("gt" in this context means "ground truth"): 
```
Reputer: []lib.ReputerConfig{
    {
        "topicId": 1,
        "groundTruthEntrypointName": "apiAdapter",
        "lossFunctionEntrypointName": "apiAdapter",
        "loopSeconds": 30,
        "minStake": 100000,
        "groundTruthParameters": {
          "GroundTruthEndpoint": "http://localhost:8888/gt/{Token}/{BlockHeight}",
          "Token": "ETHUSD"
        },
        "lossFunctionParameters": {
          "LossFunctionService": "http://localhost:5000",
          "LossMethodOptions": {
            "loss_method": "huber",
            "delta": "1.0"
          }
        }
    }
},
```

## Parameters

The parameters section contains additional properties the user wants to use to configure their URLs to hit.
In the case of the reputer, there are two parameters sections, one for the ground truth and one for the loss function.
In particular, the `LossMethodOptions` are specific to the loss function and passed unconverted to the loss function service.
They can be used to pass additional parameters to the loss function service. For example, the `delta` parameter is passed to the huber loss function like this (or as per defined in the loss function service of choice): 

```
"LossMethodOptions": {
    "loss_method": "huber",
    "delta": "1.0"
}
```


### Worker

`InferenceEndpoint` is required if `InferenceEntrypoint` is defined.
`ForecastEndpoint` is required if `ForecastEntrypoint` is defined.

`InferenceEndpoint`: provides the scalar inference endpoint to hit. It returns a single scalar value as plain text. It supports URL template variables.
`LabeledInferenceEndpoint`: provides the multi-label (vector) inference endpoint to hit, used for example by classification models. It supports URL template variables. When this parameter is present, the adapter fetches a labeled inference instead of a scalar one (see "Scalar vs multi-label" below).
`ForecastEndpoint`: provides the forecast endpoint to hit. It supports URL template variables.

If it is not desired to send inferences or forecasts, it can be configured by setting that specific entrypoint to nil. Example, for not sending inferences:
```
InferenceEntrypoint: nil
```

### Reputer

The following endpoints are used:
* `GroundTruthEndpoint`: provides the scalar ground truth endpoint to hit. It returns a single scalar value as plain text. It does support template variables.
* `LabeledGroundTruthEndpoint`: provides the multi-label (vector) ground truth endpoint to hit. It supports template variables. When this parameter is present, the adapter fetches a labeled ground truth instead of a scalar one (see "Scalar vs multi-label" below).
* `LossFunctionService`: provides the scalar loss function service to hit on loss calculation and the endpoint to know whether the loss function is never negative. These are appended to create `/calculate` and `/is_never_negative` endpoints respectively. They do not support template variables.
* `LabeledLossFunctionService`: provides the multi-label loss function service. Like `LossFunctionService`, it is appended with `/calculate` and `/is_never_negative`. It is required when reputing on multi-label values. It does not support template variables.

### Scalar vs multi-label

The adapter supports both scalar (single-value) and multi-label (vector) payloads, such as classification where the payload is a dictionary of `Label:value` pairs.

* Worker: if `LabeledInferenceEndpoint` is set, the labeled inference path is used; otherwise the scalar `InferenceEndpoint` path is used.
* Reputer: if `LabeledGroundTruthEndpoint` is set, the labeled ground truth path is used; otherwise the scalar `GroundTruthEndpoint` path is used. When the values being reputed are multi-label, the `LabeledLossFunctionService` is used to compute loss and to check whether the loss function is never negative; for single-label values the scalar `LossFunctionService` is used.

The multi-label inference and ground truth endpoints must return a JSON array of `{"label", "value"}` objects, e.g.:
```
[
  {"label": "UP",   "value": "0.3"},
  {"label": "MID",  "value": "0.4"},
  {"label": "DOWN", "value": "0.3"}
]
```
The `value` may be encoded either as a JSON string (`"0.3"`) or as a JSON number (`0.3`). The array ordering is preserved.

The labeled loss service's `/calculate` endpoint receives `y_true` and `y_pred` as arrays of `{"label", "value"}` objects, alongside `options`. Carrying the labels lets the service join each prediction to its ground truth by label rather than by array position. The offchain node also validates, before calling the service, that the predicted labels match the ground-truth labels exactly (same set, no missing/extra/duplicate labels) and fails loudly on a mismatch instead of computing a loss against misaligned classes. Example payload:
```
{
  "y_true": [{"label": "up", "value": "1.0"}, {"label": "down", "value": "0.0"}],
  "y_pred": [{"label": "up", "value": "0.7"}, {"label": "down", "value": "0.3"}],
  "options": {"method": "sqe"}
}
```


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

