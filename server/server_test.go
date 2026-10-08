package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestAllocate(t *testing.T) {
	shares := []int64{2500, 3300, 1000, 3200}
	cases := []struct {
		amount int64
		want   []int64
	}{
		{30_000, []int64{7_500, 9_900, 3_000, 9_600}},
		{41_237, []int64{10_309, 13_608, 4_124, 13_196}},
		// Tie on .5 between the first two shares goes to the earlier one.
		{28_750, []int64{7_188, 9_487, 2_875, 9_200}},
		{1, []int64{0, 1, 0, 0}},
		{0, []int64{0, 0, 0, 0}},
	}
	for _, c := range cases {
		got, err := Allocate(c.amount, shares)
		if err != nil {
			t.Fatalf("Allocate(%d): %v", c.amount, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Allocate(%d) = %v, want %v", c.amount, got, c.want)
		}
	}
}

func TestAllocateAlwaysSumsToAmount(t *testing.T) {
	shares := []int64{3333, 3333, 3334}
	for amount := int64(0); amount < 5_000; amount++ {
		got, err := Allocate(amount, shares)
		if err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, v := range got {
			sum += v
		}
		if sum != amount {
			t.Fatalf("Allocate(%d) sums to %d", amount, sum)
		}
	}
}

func TestAllocateRejectsBadShares(t *testing.T) {
	for _, shares := range [][]int64{{5000, 4999}, {10001}, {-100, 10100}, {}} {
		if _, err := Allocate(100, shares); err == nil {
			t.Errorf("Allocate with shares %v: want error", shares)
		}
	}
}

func TestSeededTransfersMatchFrontendMocks(t *testing.T) {
	s, err := newSeededStore()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{
		"xfer_dep_1_rule_savings": 7_500, "xfer_dep_1_rule_roth": 9_900, "xfer_dep_1_rule_card": 3_000,
		"xfer_dep_2_rule_savings": 10_309, "xfer_dep_2_rule_roth": 13_608, "xfer_dep_2_rule_card": 4_124,
		"xfer_dep_3_rule_savings": 7_188, "xfer_dep_3_rule_roth": 9_487, "xfer_dep_3_rule_card": 2_875,
	}
	got := s.ExpectedTransfers()
	if len(got) != len(want) {
		t.Fatalf("got %d transfers, want %d", len(got), len(want))
	}
	for _, x := range got {
		if want[x.ID] != x.AmountCents {
			t.Errorf("%s = %d, want %d", x.ID, x.AmountCents, want[x.ID])
		}
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := newSeededStore()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newRouter(s, "http://localhost:5173"))
	t.Cleanup(srv.Close)
	return srv
}

func postDeposit(t *testing.T, url, key, body string) (int, DepositResult) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+"/api/deposits", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res DepositResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func TestCreateDepositIsIdempotent(t *testing.T) {
	srv := newTestServer(t)
	body := `{"accountId":"acc_checking","amountCents":50000,"postedAt":"2026-10-08T09:00:00-05:00","description":"PAYROLL"}`

	status, first := postDeposit(t, srv.URL, "abc", body)
	if status != http.StatusCreated {
		t.Fatalf("first POST: status %d", status)
	}
	if first.Deposit.ID != "dep_4" || len(first.ExpectedTransfers) != 3 {
		t.Fatalf("unexpected result: %+v", first)
	}
	if first.ExpectedTransfers[0].DueDate != "2026-10-11" {
		t.Errorf("due date = %s, want 2026-10-11", first.ExpectedTransfers[0].DueDate)
	}

	status, replay := postDeposit(t, srv.URL, "abc", body)
	if status != http.StatusOK || !reflect.DeepEqual(first, replay) {
		t.Fatalf("replay: status %d, result %+v", status, replay)
	}

	resp, err := http.Get(srv.URL + "/api/deposits")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var deps []Deposit
	_ = json.NewDecoder(resp.Body).Decode(&deps)
	if len(deps) != 4 {
		t.Errorf("got %d deposits after replay, want 4", len(deps))
	}
}

func TestCreateDepositValidation(t *testing.T) {
	srv := newTestServer(t)
	for _, body := range []string{
		`{"accountId":"acc_checking","amountCents":0}`,
		`{"accountId":"acc_nope","amountCents":100}`,
		`{"accountId":"acc_checking","amountCents":100,"postedAt":"yesterday"}`,
		`{"accountId":"acc_checking","amountCents":1.5}`,
		`not json`,
	} {
		if status, _ := postDeposit(t, srv.URL, "", body); status != http.StatusBadRequest {
			t.Errorf("body %s: status %d, want 400", body, status)
		}
	}
}
