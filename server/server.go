// Command server is the Sluice backend (Phase 2): it serves the types in
// web/src/types over HTTP and computes deposit splits. Data lives in
// memory behind the Store interface until Phase 3 adds Postgres.
//
// Money is always int64 cents and shares are int64 basis points
// (10000 = 100%). No floats touch money anywhere in this file.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ---------------------------------------------------------------------------
// Types (mirror web/src/types/index.ts)

type Account struct {
	ID                           string `json:"id"`
	Name                         string `json:"name"`
	Institution                  string `json:"institution"`
	Kind                         string `json:"kind"`
	BalanceCents                 int64  `json:"balanceCents"`
	RateBps                      *int64 `json:"rateBps,omitempty"`
	AnnualContributionLimitCents *int64 `json:"annualContributionLimitCents,omitempty"`
	YTDContributionsCents        *int64 `json:"ytdContributionsCents,omitempty"`
}

type AllocationRule struct {
	ID                   string `json:"id"`
	DestinationAccountID string `json:"destinationAccountId"`
	ShareBps             int64  `json:"shareBps"`
}

type Ruleset struct {
	ID            string           `json:"id"`
	Version       int              `json:"version"`
	EffectiveFrom string           `json:"effectiveFrom"`
	Rules         []AllocationRule `json:"rules"`
}

type Deposit struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AmountCents int64  `json:"amountCents"`
	PostedAt    string `json:"postedAt"`
	Description string `json:"description"`
}

type ExpectedTransfer struct {
	ID             string `json:"id"`
	DepositID      string `json:"depositId"`
	RulesetID      string `json:"rulesetId"`
	RulesetVersion int    `json:"rulesetVersion"`
	RuleID         string `json:"ruleId"`
	FromAccountID  string `json:"fromAccountId"`
	ToAccountID    string `json:"toAccountId"`
	AmountCents    int64  `json:"amountCents"`
	DueDate        string `json:"dueDate"`
}

type Transaction struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AmountCents int64  `json:"amountCents"`
	PostedAt    string `json:"postedAt"`
	Description string `json:"description"`
}

type ReconciliationEntry struct {
	ExpectedTransferID   string `json:"expectedTransferId"`
	Status               string `json:"status"`
	MatchedTransactionID string `json:"matchedTransactionId,omitempty"`
	DifferenceCents      int64  `json:"differenceCents"`
}

// ---------------------------------------------------------------------------
// Allocation engine

const totalBps = 10000

// transferWindow is how long after a deposit lands its transfers are due.
const transferWindow = 3 * 24 * time.Hour

var errBadRuleset = errors.New("ruleset shares must sum to 10000 bps")

// Allocate splits amountCents across shares using largest-remainder
// rounding: every share gets its floor, then leftover cents go one at a
// time to the shares with the biggest remainders. Ties go to the earlier
// share. The result always sums exactly to amountCents.
func Allocate(amountCents int64, sharesBps []int64) ([]int64, error) {
	var sum int64
	for _, s := range sharesBps {
		if s < 0 {
			return nil, errBadRuleset
		}
		sum += s
	}
	if sum != totalBps {
		return nil, errBadRuleset
	}

	out := make([]int64, len(sharesBps))
	rems := make([]int64, len(sharesBps))
	var allocated int64
	for i, s := range sharesBps {
		out[i] = amountCents * s / totalBps
		rems[i] = amountCents * s % totalBps
		allocated += out[i]
	}

	order := make([]int, len(sharesBps))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return rems[order[a]] > rems[order[b]] })

	for i := int64(0); i < amountCents-allocated; i++ {
		out[order[i]]++
	}
	return out, nil
}

// PlanTransfers turns a deposit into the transfers the ruleset calls for.
// Rules pointing at the deposit's own account produce no transfer: that
// share simply stays put.
func PlanTransfers(d Deposit, rs Ruleset) ([]ExpectedTransfer, error) {
	posted, err := time.Parse(time.RFC3339, d.PostedAt)
	if err != nil {
		return nil, fmt.Errorf("deposit %s: bad postedAt: %w", d.ID, err)
	}
	due := posted.Add(transferWindow).Format(time.DateOnly)

	shares := make([]int64, len(rs.Rules))
	for i, r := range rs.Rules {
		shares[i] = r.ShareBps
	}
	amounts, err := Allocate(d.AmountCents, shares)
	if err != nil {
		return nil, err
	}

	xfers := []ExpectedTransfer{}
	for i, r := range rs.Rules {
		if r.DestinationAccountID == d.AccountID || amounts[i] == 0 {
			continue
		}
		xfers = append(xfers, ExpectedTransfer{
			ID:             fmt.Sprintf("xfer_%s_%s", d.ID, r.ID),
			DepositID:      d.ID,
			RulesetID:      rs.ID,
			RulesetVersion: rs.Version,
			RuleID:         r.ID,
			FromAccountID:  d.AccountID,
			ToAccountID:    r.DestinationAccountID,
			AmountCents:    amounts[i],
			DueDate:        due,
		})
	}
	return xfers, nil
}

// ---------------------------------------------------------------------------
// Store

type NewDeposit struct {
	AccountID   string `json:"accountId"`
	AmountCents int64  `json:"amountCents"`
	PostedAt    string `json:"postedAt"`
	Description string `json:"description"`
}

type DepositResult struct {
	Deposit           Deposit            `json:"deposit"`
	ExpectedTransfers []ExpectedTransfer `json:"expectedTransfers"`
}

var errUnknownAccount = errors.New("unknown account")

type Store interface {
	Accounts() []Account
	Rulesets() []Ruleset
	Deposits() []Deposit
	ExpectedTransfers() []ExpectedTransfer
	Transactions() []Transaction
	Reconciliation() []ReconciliationEntry
	// CreateDeposit records a deposit and its planned transfers. Calling it
	// again with the same idempotency key returns the original result and
	// created=false.
	CreateDeposit(idempotencyKey string, in NewDeposit) (res DepositResult, created bool, err error)
}

type memStore struct {
	mu             sync.RWMutex
	accounts       []Account
	rulesets       []Ruleset
	deposits       []Deposit
	transfers      []ExpectedTransfer
	transactions   []Transaction
	reconciliation []ReconciliationEntry
	idempotency    map[string]DepositResult
	nextDeposit    int
}

func (s *memStore) Accounts() []Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Account{}, s.accounts...)
}

func (s *memStore) Rulesets() []Ruleset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Ruleset{}, s.rulesets...)
}

func (s *memStore) Deposits() []Deposit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Deposit{}, s.deposits...)
}

func (s *memStore) ExpectedTransfers() []ExpectedTransfer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ExpectedTransfer{}, s.transfers...)
}

func (s *memStore) Transactions() []Transaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Transaction{}, s.transactions...)
}

func (s *memStore) Reconciliation() []ReconciliationEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ReconciliationEntry{}, s.reconciliation...)
}

func (s *memStore) CreateDeposit(key string, in NewDeposit) (DepositResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key != "" {
		if prev, ok := s.idempotency[key]; ok {
			return prev, false, nil
		}
	}

	found := false
	for _, a := range s.accounts {
		if a.ID == in.AccountID {
			found = true
			break
		}
	}
	if !found {
		return DepositResult{}, false, errUnknownAccount
	}

	// The latest ruleset version is the one in force.
	rs := s.rulesets[len(s.rulesets)-1]

	d := Deposit{
		ID:          fmt.Sprintf("dep_%d", s.nextDeposit),
		AccountID:   in.AccountID,
		AmountCents: in.AmountCents,
		PostedAt:    in.PostedAt,
		Description: in.Description,
	}
	xfers, err := PlanTransfers(d, rs)
	if err != nil {
		return DepositResult{}, false, err
	}

	s.nextDeposit++
	s.deposits = append(s.deposits, d)
	s.transfers = append(s.transfers, xfers...)
	res := DepositResult{Deposit: d, ExpectedTransfers: xfers}
	if key != "" {
		s.idempotency[key] = res
	}
	return res, true, nil
}

// newSeededStore loads the same data as web/src/mocks, except expected
// transfers are computed by the engine rather than hardcoded.
func newSeededStore() (*memStore, error) {
	ptr := func(v int64) *int64 { return &v }

	s := &memStore{
		idempotency: map[string]DepositResult{},
		accounts: []Account{
			{ID: "acc_checking", Name: "Everyday Checking", Institution: "First Plains Bank", Kind: "checking", BalanceCents: 84_212},
			{ID: "acc_savings", Name: "High-Yield Savings", Institution: "Northwind Savings", Kind: "savings", BalanceCents: 312_450, RateBps: ptr(400)},
			{ID: "acc_roth", Name: "Roth IRA", Institution: "Contoso Brokerage", Kind: "retirement", BalanceCents: 528_900,
				AnnualContributionLimitCents: ptr(750_000), YTDContributionsCents: ptr(410_000)},
			{ID: "acc_card", Name: "Rewards Card", Institution: "First Plains Bank", Kind: "credit", BalanceCents: 146_075, RateBps: ptr(2499)},
		},
		rulesets: []Ruleset{{
			ID: "rs_main", Version: 1, EffectiveFrom: "2026-09-01T00:00:00-05:00",
			Rules: []AllocationRule{
				{ID: "rule_savings", DestinationAccountID: "acc_savings", ShareBps: 2500},
				{ID: "rule_roth", DestinationAccountID: "acc_roth", ShareBps: 3300},
				{ID: "rule_card", DestinationAccountID: "acc_card", ShareBps: 1000},
				{ID: "rule_spending", DestinationAccountID: "acc_checking", ShareBps: 3200},
			},
		}},
		deposits: []Deposit{
			{ID: "dep_1", AccountID: "acc_checking", AmountCents: 30_000, PostedAt: "2026-09-11T09:00:00-05:00", Description: "PAYROLL ACME CO"},
			{ID: "dep_2", AccountID: "acc_checking", AmountCents: 41_237, PostedAt: "2026-09-25T09:00:00-05:00", Description: "PAYROLL ACME CO"},
			{ID: "dep_3", AccountID: "acc_checking", AmountCents: 28_750, PostedAt: "2026-10-06T09:00:00-05:00", Description: "PAYROLL ACME CO"},
		},
		transactions: []Transaction{
			{ID: "txn_101", AccountID: "acc_checking", AmountCents: 30_000, PostedAt: "2026-09-11T09:00:00-05:00", Description: "PAYROLL ACME CO"},
			{ID: "txn_102", AccountID: "acc_checking", AmountCents: 41_237, PostedAt: "2026-09-25T09:00:00-05:00", Description: "PAYROLL ACME CO"},
			{ID: "txn_103", AccountID: "acc_checking", AmountCents: 28_750, PostedAt: "2026-10-06T09:00:00-05:00", Description: "PAYROLL ACME CO"},
			{ID: "txn_201", AccountID: "acc_savings", AmountCents: 7_500, PostedAt: "2026-09-12T14:10:00-05:00", Description: "TRANSFER FROM FIRST PLAINS"},
			{ID: "txn_202", AccountID: "acc_roth", AmountCents: 9_900, PostedAt: "2026-09-13T10:02:00-05:00", Description: "ROTH CONTRIBUTION"},
			{ID: "txn_203", AccountID: "acc_card", AmountCents: 3_000, PostedAt: "2026-09-12T08:45:00-05:00", Description: "PAYMENT THANK YOU"},
			{ID: "txn_204", AccountID: "acc_savings", AmountCents: 10_309, PostedAt: "2026-09-26T11:30:00-05:00", Description: "TRANSFER FROM FIRST PLAINS"},
			{ID: "txn_205", AccountID: "acc_card", AmountCents: 4_024, PostedAt: "2026-09-27T19:12:00-05:00", Description: "PAYMENT THANK YOU"},
			{ID: "txn_206", AccountID: "acc_savings", AmountCents: 7_188, PostedAt: "2026-10-06T16:40:00-05:00", Description: "TRANSFER FROM FIRST PLAINS"},
		},
		// Real matching arrives in Phase 5; until then this is fixed data.
		reconciliation: []ReconciliationEntry{
			{ExpectedTransferID: "xfer_dep_1_rule_savings", Status: "matched", MatchedTransactionID: "txn_201"},
			{ExpectedTransferID: "xfer_dep_1_rule_roth", Status: "matched", MatchedTransactionID: "txn_202"},
			{ExpectedTransferID: "xfer_dep_1_rule_card", Status: "matched", MatchedTransactionID: "txn_203"},
			{ExpectedTransferID: "xfer_dep_2_rule_savings", Status: "matched", MatchedTransactionID: "txn_204"},
			{ExpectedTransferID: "xfer_dep_2_rule_roth", Status: "missed", DifferenceCents: -13_608},
			{ExpectedTransferID: "xfer_dep_2_rule_card", Status: "partial", MatchedTransactionID: "txn_205", DifferenceCents: -100},
			{ExpectedTransferID: "xfer_dep_3_rule_savings", Status: "matched", MatchedTransactionID: "txn_206"},
			{ExpectedTransferID: "xfer_dep_3_rule_roth", Status: "pending"},
			{ExpectedTransferID: "xfer_dep_3_rule_card", Status: "pending"},
		},
		nextDeposit: 4,
	}

	for _, d := range s.deposits {
		xfers, err := PlanTransfers(d, s.rulesets[0])
		if err != nil {
			return nil, err
		}
		s.transfers = append(s.transfers, xfers...)
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// HTTP

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// allowDevOrigin lets the Vite dev server call the API from the browser.
func allowDevOrigin(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func newRouter(store Store, corsOrigin string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(allowDevOrigin(corsOrigin))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Get("/accounts", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.Accounts())
		})
		r.Get("/rulesets", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.Rulesets())
		})
		r.Get("/deposits", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.Deposits())
		})
		r.Post("/deposits", func(w http.ResponseWriter, r *http.Request) {
			var in NewDeposit
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&in); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
			if in.AmountCents <= 0 {
				writeError(w, http.StatusBadRequest, "amountCents must be positive")
				return
			}
			if in.PostedAt == "" {
				in.PostedAt = time.Now().Format(time.RFC3339)
			} else if _, err := time.Parse(time.RFC3339, in.PostedAt); err != nil {
				writeError(w, http.StatusBadRequest, "postedAt must be RFC 3339")
				return
			}

			res, created, err := store.CreateDeposit(r.Header.Get("Idempotency-Key"), in)
			switch {
			case errors.Is(err, errUnknownAccount):
				writeError(w, http.StatusBadRequest, "unknown accountId")
			case err != nil:
				log.Printf("create deposit: %v", err)
				writeError(w, http.StatusInternalServerError, "could not create deposit")
			case created:
				writeJSON(w, http.StatusCreated, res)
			default:
				writeJSON(w, http.StatusOK, res)
			}
		})
		r.Get("/expected-transfers", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.ExpectedTransfers())
		})
		r.Get("/transactions", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.Transactions())
		})
		r.Get("/reconciliation", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, store.Reconciliation())
		})
	})
	return r
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:5173"
	}

	store, err := newSeededStore()
	if err != nil {
		log.Fatalf("seed store: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(store, corsOrigin),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("sluice server listening on http://localhost:%s", port)
	log.Fatal(srv.ListenAndServe())
}
