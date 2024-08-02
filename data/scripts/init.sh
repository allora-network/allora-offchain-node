#!/bin/bash

set -e

NAME="${NAME:-sample_offchain_node}"  

if allorad keys --home=/data/.allorad --keyring-backend test show $NAME > /dev/null 2>&1 ; then
    echo "allora account: $NAME already imported"
else
    echo "creating allora account: $NAME"
    output=$(allorad keys add $NAME --home=/data/.allorad --keyring-backend test 2>&1)
    address=$(echo "$output" | grep 'address:' | sed 's/.*address: //')
    mnemonic=$(echo "$output" | tail -n 1)

    echo "ALLORA_ACCOUNT_NAME=$NAME" > /data/env_file
    echo "ALLORA_ACCOUNT_ADDRESS=$address" >> /data/env_file
    echo "ALLORA_ACCOUNT_MNEMONIC=\"$mnemonic\"" >> /data/env_file
    echo "ALLORA_ACCOUNT_PASSPHRASE=secret" >> /data/env_file

    echo "Environment variables saved to /data/env_file"
fi

