package apiadapter

import (
	"testing"

	"allora_offchain_node/lib"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseJSONToLabeledValues covers the accepted wire formats (string and numeric
// values, with order preserved and whitespace trimmed) and the local pre-validation
// that rejects malformed model output before it can reach the chain.
func TestParseJSONToLabeledValues(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectError   bool
		errorContains string
		expected      []lib.LabeledValue
	}{
		{
			name: "string values - order preserved",
			json: `[{"label":"UP","value":"0.3"},{"label":"DOWN","value":"0.7"}]`,
			expected: []lib.LabeledValue{
				{Label: "UP", Value: "0.3"},
				{Label: "DOWN", Value: "0.7"},
			},
		},
		{
			name: "numeric values normalized to string",
			json: `[{"label":"UP","value":0.3},{"label":"DOWN","value":0.7}]`,
			expected: []lib.LabeledValue{
				{Label: "UP", Value: "0.3"},
				{Label: "DOWN", Value: "0.7"},
			},
		},
		{
			name: "mixed string and numeric values",
			json: `[{"label":"A","value":"1.5"},{"label":"B","value":2}]`,
			expected: []lib.LabeledValue{
				{Label: "A", Value: "1.5"},
				{Label: "B", Value: "2"},
			},
		},
		{
			name: "labels and values are trimmed",
			json: `[{"label":"  UP  ","value":"  0.3  "}]`,
			expected: []lib.LabeledValue{
				{Label: "UP", Value: "0.3"},
			},
		},
		{
			name:     "empty array is valid",
			json:     `[]`,
			expected: []lib.LabeledValue{},
		},
		{
			name:          "invalid json errors",
			json:          `not json`,
			expectError:   true,
			errorContains: "",
		},
		{
			name:          "empty label rejected",
			json:          `[{"label":"","value":"0.3"}]`,
			expectError:   true,
			errorContains: "empty label",
		},
		{
			name:          "whitespace-only label rejected",
			json:          `[{"label":"   ","value":"0.3"}]`,
			expectError:   true,
			errorContains: "empty label",
		},
		{
			name:          "missing label rejected",
			json:          `[{"value":"0.3"}]`,
			expectError:   true,
			errorContains: "empty label",
		},
		{
			name:          "duplicate label rejected",
			json:          `[{"label":"UP","value":"0.3"},{"label":"UP","value":"0.7"}]`,
			expectError:   true,
			errorContains: `duplicate label "UP"`,
		},
		{
			name:          "duplicate label after trim rejected",
			json:          `[{"label":"UP","value":"0.3"},{"label":"  UP  ","value":"0.7"}]`,
			expectError:   true,
			errorContains: `duplicate label "UP"`,
		},
		{
			name:          "empty string value rejected",
			json:          `[{"label":"UP","value":""}]`,
			expectError:   true,
			errorContains: "empty value",
		},
		{
			name:          "missing value rejected",
			json:          `[{"label":"UP"}]`,
			expectError:   true,
			errorContains: "empty value",
		},
		{
			name:          "null value rejected",
			json:          `[{"label":"UP","value":null}]`,
			expectError:   true,
			errorContains: "empty value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONToLabeledValues(tt.json)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
