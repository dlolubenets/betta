package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

func TestReqs(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	tt := []struct {
		name           string
		state          string
		amount         string
		transactionID  string
		sourceType     string
		expectedStatus int
	}{
		{
			name:           "ok win",
			state:          "win",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "game",
			expectedStatus: 200,
		}, {
			name:           "ok lost",
			state:          "lost",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "game",
			expectedStatus: 200,
		}, {
			name:           "bad lost",
			state:          "lost",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "game",
			expectedStatus: 400,
		}, {
			name:           "ok source type server",
			state:          "win",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "server",
			expectedStatus: 200,
		}, {
			name:           "ok source type payment",
			state:          "win",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "payment",
			expectedStatus: 200,
		}, {
			name:           "bad wrong source type",
			state:          "win",
			amount:         "15.5",
			transactionID:  fmt.Sprintf("%d", rand.Int()),
			sourceType:     "asdf",
			expectedStatus: 400,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tp := TransactionPayload{
				State:         tc.state,
				Amount:        tc.amount,
				TransactionID: tc.transactionID,
			}
			tpBytes, err := json.Marshal(tp)
			if err != nil {
				t.Error(err)
			}
			req, err := http.NewRequest("POST", "http://localhost:8083/transaction", bytes.NewBuffer(tpBytes))
			if err != nil {
				t.Error(err)
			}
			req.Header.Set("Source-Type", tc.sourceType)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
			}
			if tc.expectedStatus != resp.StatusCode {
				t.Errorf("wrong status code: expected: %d, actual: %d", tc.expectedStatus, resp.StatusCode)
			}
		})
	}

	// check idempotence
	t.Run(fmt.Sprintf("idempotence check for %s", tt[0].name), func(t *testing.T) {
		tp := TransactionPayload{
			State:         tt[0].state,
			Amount:        tt[0].amount,
			TransactionID: tt[0].transactionID,
		}
		tpBytes, err := json.Marshal(tp)
		if err != nil {
			t.Error(err)
		}
		req, err := http.NewRequest("POST", "http://localhost:8083/transaction", bytes.NewBuffer(tpBytes))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("Source-Type", tt[0].sourceType)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		if tt[0].expectedStatus != resp.StatusCode {
			t.Errorf("wrong status code: expected: %d, actual: %d", tt[0].expectedStatus, resp.StatusCode)
		}
	})
}
