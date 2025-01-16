package lib

import (
	"encoding/json"
	"fmt"
	"strconv"

	cosmossdk_io_math "cosmossdk.io/math"
)

type Address = string
type Allo = int64
type BlockHeight = int64

// FlexibleCosmosIntAmount represents amounts of tokens, where the amount can be specified or as a string
type FlexibleCosmosIntAmount struct {
	Number cosmossdk_io_math.Int
}

func (fv *FlexibleCosmosIntAmount) String() string {
	fmt.Println("fv.Number", fv.Number)
	return fv.Number.String()
}

func (fv *FlexibleCosmosIntAmount) UnmarshalJSON(data []byte) error {
	// Handle number
	if data[0] >= '0' && data[0] <= '9' {
		var num int64
		if err := json.Unmarshal(data, &num); err != nil {
			return fmt.Errorf("failed to parse number: %w", err)
		}
		fv.Number = cosmossdk_io_math.NewInt(num)
		return nil
	}

	// Handle string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		num, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert string to number: %w", err)
		}
		fv.Number = cosmossdk_io_math.NewInt(num)
		return nil
	}

	return fmt.Errorf("invalid value: %s", string(data))
}
