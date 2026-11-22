package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Local-only QA testlab server (127.0.0.1:8090) demonstrating the difference
// between an unprotected check-then-act handler and a properly locked + idempotent handler.

type voucherState struct {
	mu           sync.Mutex
	usedCount    int
	maxUses      int
	transactions []string
	idempCache   map[string]redeemResponse
}

type redeemResponse struct {
	TxID      string `json:"tx_id"`
	Status    string `json:"status"`
	UsedCount int    `json:"used_count"`
}

var (
	state = &voucherState{
		maxUses:    1,
		idempCache: make(map[string]redeemResponse),
	}
	txSeq atomic.Int64
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/reset", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.usedCount = 0
		state.maxUses = 1
		state.transactions = nil
		state.idempCache = make(map[string]redeemResponse)
		state.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"status": "reset", "used_count": 0, "max_uses": 1})
	})

	mux.HandleFunc("/api/v1/state", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"used_count":   state.usedCount,
			"max_uses":     state.maxUses,
			"transactions": state.transactions,
		})
	})

	// Intentionally unprotected check-then-act handler for local QA demonstration.
	mux.HandleFunc("/api/v1/vulnerable/redeem", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		// Read phase (unlocked)
		state.mu.Lock()
		current := state.usedCount
		limit := state.maxUses
		state.mu.Unlock()

		if current >= limit {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "voucher already redeemed"})
			return
		}

		// Simulated DB/service processing window between read and write
		time.Sleep(15 * time.Millisecond)

		// Write phase (blind increment without re-checking condition)
		id := fmt.Sprintf("tx_%04d", txSeq.Add(1))
		state.mu.Lock()
		state.usedCount++
		used := state.usedCount
		state.transactions = append(state.transactions, id)
		state.mu.Unlock()

		writeJSON(w, http.StatusOK, redeemResponse{
			TxID:      id,
			Status:    "redeemed",
			UsedCount: used,
		})
	})

	// Properly protected handler using critical-section locking + Idempotency-Key support.
	mux.HandleFunc("/api/v1/safe/redeem", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		idempKey := r.Header.Get("Idempotency-Key")

		state.mu.Lock()
		defer state.mu.Unlock()

		if idempKey != "" {
			if cached, exists := state.idempCache[idempKey]; exists {
				writeJSON(w, http.StatusOK, cached)
				return
			}
		}

		if state.usedCount >= state.maxUses {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "voucher already redeemed"})
			return
		}

		// Simulated processing happens safely inside the atomic critical section
		time.Sleep(5 * time.Millisecond)

		id := fmt.Sprintf("tx_%04d", txSeq.Add(1))
		state.usedCount++
		state.transactions = append(state.transactions, id)
		resp := redeemResponse{
			TxID:      id,
			Status:    "redeemed",
			UsedCount: state.usedCount,
		}
		if idempKey != "" {
			state.idempCache[idempKey] = resp
		}
		writeJSON(w, http.StatusOK, resp)
	})

	addr := "127.0.0.1:8090"
	log.Printf("RaceGuard local testlab listening on http://%s (localhost only)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
