package client

import (
	"allora_offchain_node/types"
	"encoding/json"
	"fmt"
	"strconv"
)

func GetAccountInfo(address string, apiURL string) (seqint, accnum uint64, err error) {
	resp, err := HTTPGet(apiURL + "/cosmos/auth/v1beta1/accounts/" + address)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get initial sequence: %v", err)
	}

	var accountRes types.AccountResult
	err = json.Unmarshal(resp, &accountRes)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal account result: %v", err)
	}

	seqint, err = strconv.ParseUint(accountRes.Account.Sequence, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert sequence to int: %v", err)
	}

	accnum, err = strconv.ParseUint(accountRes.Account.AccountNumber, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert account number to int: %v", err)
	}

	return seqint, accnum, nil
}
