#!/bin/bash

set -e

NAME="${NAME:-offchain_node}"  

if allorad keys --home=/data/.allorad --keyring-backend test show $NAME > /dev/null 2>&1 ; then
    echo "allora account: $NAME already imported"
    echo "ENV_LOADED=true" >> /data/env_file
else
    echo "creating allora account: $NAME"
    output=$(allorad keys add $NAME --home=/data/.allorad --keyring-backend test 2>&1)
    address=$(echo "$output" | grep 'address:' | sed 's/.*address: //')
    mnemonic=$(echo "$output" | tail -n 1)

    # Parse and update the JSON string
    updated_json=$(echo "$ALLORA_OFFCHAIN_NODE_CONFIG_JSON" | jq --arg name "$NAME" --arg mnemonic "$mnemonic" --arg passphrase "secret" '
    .wallet.addressKeyName = $name |
    .wallet.addressRestoreMnemonic = $mnemonic |
    .wallet.addressAccountPassphrase = $passphrase
    ')

    stringified_json=$(echo "$updated_json" | jq -c .)

    echo "ALLORA_OFFCHAIN_NODE_CONFIG_JSON='$stringified_json'" > /data/env_file
    echo ALLORA_OFFCHAIN_ACCOUNT_ADDRESS=$address >> /data/env_file
    echo "NAME=$NAME" >> /data/env_file
    echo "ENV_LOADED=true" >> /data/env_file

    

    echo "Updated ALLORA_OFFCHAIN_NODE_CONFIG_JSON saved to /data/env_file"
fi
