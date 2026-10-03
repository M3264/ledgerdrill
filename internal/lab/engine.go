package lab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"io"
	"ledgerdrill/internal/store"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrBusy = errors.New("the shared testnet lab is in use; wait for the current payout or comparison to finish")

type Engine struct {
	Store           *store.Store
	Proxy           *Proxy
	HTTP            *http.Client
	BaseURL         string
	Token           string
	busy            atomic.Bool
	Context         context.Context
	challengeMu     sync.Mutex
	activeChallenge *challengeTicket
}

func NewEngine(s *store.Store, p *Proxy, base, token string, ctx context.Context) *Engine {
	return &Engine{Store: s, Proxy: p, HTTP: &http.Client{Timeout: 90 * time.Second}, BaseURL: base, Token: token, Context: ctx}
}
func (e *Engine) Busy() bool { return e.busy.Load() }
func (e *Engine) Start(scenario string) (Run, error) {
	if !ValidScenario(scenario) {
		return Run{}, fmt.Errorf("choose a supported scenario")
	}
	if !e.busy.CompareAndSwap(false, true) {
		return Run{}, ErrBusy
	}
	r := Run{ID: NewID(), Scenario: scenario, Status: "funding", CreatedAt: Now(), Network: "Stellar testnet", Asset: "XLM", Amount: "1.0000000", Clients: []ClientResult{}}
	for _, id := range []string{"naive", "reconciled"} {
		r.Clients = append(r.Clients, ClientResult{ID: id, Status: "waiting", Received: "0.0000000", Verdict: "Waiting for test accounts", Events: []Event{}, Transactions: []Evidence{}})
	}
	if err := e.Store.Put("run", r.ID, r); err != nil {
		e.busy.Store(false)
		return Run{}, err
	}
	go e.execute(r)
	return r, nil
}
func (e *Engine) execute(r Run) {
	defer e.busy.Store(false)
	ctx, cancel := context.WithTimeout(e.Context, 6*time.Minute)
	defer cancel()
	save := func() error { return e.Store.Put("run", r.ID, r) }
	fail := func(err error) {
		r.Status = "failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			r.Status = "interrupted"
		}
		r.Error = err.Error()
		r.FinishedAt = Now()
		_ = save()
	}
	// Keys exist only for this run and are never logged, exported or persisted.
	sources := make([]*keypair.Full, 2)
	destination, err := keypair.Random()
	if err != nil {
		fail(err)
		return
	}
	for i := range sources {
		sources[i], err = keypair.Random()
		if err != nil {
			fail(err)
			return
		}
		r.Clients[i].Source = sources[i].Address()
		r.Clients[i].Destination = destination.Address()
	}
	if err = save(); err != nil {
		fail(err)
		return
	}
	for _, kp := range append(sources, destination) {
		if err = e.fund(ctx, kp.Address()); err != nil {
			fail(fmt.Errorf("test account setup failed: %w. Retry once Friendbot is available", err))
			return
		}
	}
	r.Status = "running"
	if err = save(); err != nil {
		fail(err)
		return
	}
	for i, kp := range sources {
		c := &r.Clients[i]
		c.Status = "running"
		c.Verdict = "Submitting through the fault proxy"
		session, er := e.Proxy.Create(r.Scenario)
		if er != nil {
			fail(er)
			return
		}
		c.SessionID = session.ID
		if er = save(); er != nil {
			fail(er)
			return
		}
		log := func(kind, message, hash string) error {
			c.Events = append(c.Events, Event{At: Now(), Kind: kind, Message: message, Hash: hash})
			return save()
		}
		er = e.exercise(ctx, kp, destination.Address(), c.ID, session.ID, log)
		snapshot, se := e.Proxy.Get(session.ID)
		if se != nil {
			fail(se)
			return
		}
		c.Attempts = snapshot.Attempts
		c.Transactions = snapshot.Transactions
		verificationFailed := e.verifyClient(ctx, c, destination.Address())
		c.Status = "complete"
		if er != nil || verificationFailed {
			c.Status = "inconclusive"
			c.Verdict = "Recovery did not finish; inspect ledger evidence"
			if er != nil {
				_ = log("error", er.Error(), "")
			}
		} else if c.Settlements == 1 && c.Received == "1.0000000" {
			c.Verdict = "One payment confirmed"
		} else if c.Settlements > 1 {
			c.Verdict = "Duplicate payment confirmed"
		} else {
			c.Status = "inconclusive"
			c.Verdict = "No successful payment verified"
		}
		if er = save(); er != nil {
			fail(er)
			return
		}
	}
	r.Status = "complete"
	for _, c := range r.Clients {
		if c.Status != "complete" {
			r.Status = "inconclusive"
		}
	}
	r.FinishedAt = Now()
	if err = save(); err != nil {
		fail(err)
	}
}
func (e *Engine) fund(ctx context.Context, address string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://friendbot.stellar.org/?addr="+address, nil)
	if err != nil {
		return err
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Friendbot returned HTTP %d", resp.StatusCode)
	}
	return nil
}
func (e *Engine) read(ctx context.Context, endpoint string, v any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return 0, err
	}
	if strings.HasPrefix(endpoint, e.BaseURL+"/") && e.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.Token)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return resp.StatusCode, nil
	}
	return resp.StatusCode, json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(v)
}
func (e *Engine) build(ctx context.Context, kp *keypair.Full, dest, memo string) (string, string, error) {
	return e.buildWithTimeout(ctx, kp, dest, memo, 120)
}
func (e *Engine) buildWithTimeout(ctx context.Context, kp *keypair.Full, dest, memo string, timeout int64) (string, string, error) {
	var a struct {
		Sequence string `json:"sequence"`
	}
	status, err := e.read(ctx, HorizonURL+"/accounts/"+kp.Address(), &a)
	if err != nil {
		return "", "", err
	}
	if status != 200 {
		return "", "", fmt.Errorf("account lookup returned HTTP %d", status)
	}
	sequence, err := strconv.ParseInt(a.Sequence, 10, 64)
	if err != nil {
		return "", "", err
	}
	account := txnbuild.SimpleAccount{AccountID: kp.Address(), Sequence: sequence}
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{SourceAccount: &account, IncrementSequenceNum: true, BaseFee: 1000, Operations: []txnbuild.Operation{&txnbuild.Payment{Destination: dest, Asset: txnbuild.NativeAsset{}, Amount: "1.0000000"}}, Memo: txnbuild.MemoText(memo), Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(timeout)}})
	if err != nil {
		return "", "", err
	}
	tx, err = tx.Sign(Network, kp)
	if err != nil {
		return "", "", err
	}
	encoded, err := tx.Base64()
	if err != nil {
		return "", "", err
	}
	hash, err := tx.HashHex(Network)
	return encoded, hash, err
}
func (e *Engine) submit(ctx context.Context, id, envelope string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/horizon/"+id+"/transactions", strings.NewReader(url.Values{"tx": {envelope}}.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if e.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.Token)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, nil
}
func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (e *Engine) exercise(ctx context.Context, kp *keypair.Full, dest, client, id string, log func(string, string, string) error) error {
	envelope, hash, err := e.build(ctx, kp, dest, "drill-"+client+"-"+id[:8])
	if err != nil {
		return err
	}
	if err = log("prepared", "Built one 1 XLM payment; retained its envelope and hash.", hash); err != nil {
		return err
	}
	status, err := e.submit(ctx, id, envelope)
	if err == nil && status >= 200 && status < 300 {
		return log("confirmed", "Submission response confirms settlement.", hash)
	}
	if err = log("uncertain", fmt.Sprintf("Submission returned HTTP %d. Outcome needs reconciliation.", status), hash); err != nil {
		return err
	}
	return e.recoverPayment(ctx, kp, dest, client, id, envelope, hash, log)
}
func (e *Engine) recoverPayment(ctx context.Context, kp *keypair.Full, dest, client, id, envelope, hash string, log func(string, string, string) error) error {
	var err error
	var status int
	if client == "naive" || client == "fresh" {
		if snap, se := e.Proxy.Get(id); se == nil && client == "naive" && snap.Scenario == "delayed_lookup" {
			var v any
			code, re := e.read(ctx, e.BaseURL+"/horizon/"+id+"/transactions/"+hash, &v)
			if re != nil {
				return re
			}
			if err = log("lookup", fmt.Sprintf("Single lookup returned HTTP %d; naive client treats that as unpaid.", code), hash); err != nil {
				return err
			}
		}
		if err = log("retry", "Naive recovery builds a fresh payment instead of reconciling the original.", ""); err != nil {
			return err
		}
		envelope, hash, err = e.build(ctx, kp, dest, "retry-"+id[:8])
		if err != nil {
			return err
		}
		status, err = e.submit(ctx, id, envelope)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("fresh retry returned HTTP %d", status)
		}
		return log("confirmed", "Fresh payment confirmed. Ledger verification checks whether both settled.", hash)
	}
	for i := 0; i < 5; i++ {
		var t struct {
			Successful bool `json:"successful"`
		}
		code, re := e.read(ctx, e.BaseURL+"/horizon/"+id+"/transactions/"+hash, &t)
		if re == nil && code == 200 {
			if !t.Successful {
				return fmt.Errorf("original transaction was included but failed")
			}
			return log("recovered", "Found the original successful transaction. No fresh payment created.", hash)
		}
		if re != nil {
			return re
		}
		if err = log("lookup", fmt.Sprintf("Lookup returned HTTP %d; original envelope remains the only payment intent.", code), hash); err != nil {
			return err
		}
		if code != 404 && code != 429 && code < 500 {
			return fmt.Errorf("unexpected lookup status %d", code)
		}
		if i == 2 {
			if err = log("retry", "Resubmitting the exact original signed envelope is safe across retries.", hash); err != nil {
				return err
			}
			retryStatus, retryErr := e.submit(ctx, id, envelope)
			if retryErr == nil && retryStatus >= 200 && retryStatus < 300 {
				return log("recovered", "Original envelope confirmed on retry; no new payment created.", hash)
			}
		}
		if err = wait(ctx, 1200*time.Millisecond); err != nil {
			return err
		}
	}
	return fmt.Errorf("confirmation remains unknown; retained original hash for inspection")
}
func (e *Engine) verify(ctx context.Context, hash, destination string) (Evidence, error) {
	v := Evidence{Hash: hash, Explorer: "https://stellar.expert/explorer/testnet/tx/" + hash}
	var t struct {
		Hash       string `json:"hash"`
		Successful bool   `json:"successful"`
		Ledger     int32  `json:"ledger"`
		Envelope   string `json:"envelope_xdr"`
	}
	code, err := e.read(ctx, HorizonURL+"/transactions/"+hash, &t)
	if err != nil {
		return v, err
	}
	if code == 404 {
		return v, nil
	}
	if code != 200 {
		return v, fmt.Errorf("independent ledger verification returned HTTP %d", code)
	}
	if t.Hash != hash {
		return v, fmt.Errorf("upstream hash mismatch")
	}
	v.Confirmed = true
	v.Successful = t.Successful
	v.Ledger = t.Ledger
	if !t.Successful {
		return v, nil
	}
	generic, err := txnbuild.TransactionFromXDR(t.Envelope)
	if err != nil {
		return v, err
	}
	tx, ok := generic.Transaction()
	if !ok {
		return v, fmt.Errorf("unexpected fee bump in reference payment")
	}
	var paid int64
	for _, op := range tx.Operations() {
		if p, ok := op.(*txnbuild.Payment); ok && p.Destination == destination {
			if _, native := p.Asset.(txnbuild.NativeAsset); native {
				n, er := amount.ParseInt64(p.Amount)
				if er != nil {
					return v, er
				}
				paid += n
			}
		}
	}
	v.Amount = amount.StringFromInt64(paid)
	return v, nil
}
func (e *Engine) verifyClient(ctx context.Context, c *ClientResult, destination string) bool {
	var total int64
	failed := false
	c.Settlements = 0
	for j := range c.Transactions {
		verified, err := e.verify(ctx, c.Transactions[j].Hash, destination)
		if err != nil {
			c.Transactions[j].VerificationError = err.Error()
			failed = true
			continue
		}
		c.Transactions[j] = verified
		if verified.Confirmed && verified.Successful && verified.Amount != "" {
			n, err := amount.ParseInt64(verified.Amount)
			if err != nil {
				failed = true
				continue
			}
			if n > 0 {
				total += n
				c.Settlements++
			}
		}
	}
	c.Received = amount.StringFromInt64(total)
	return failed
}
func (e *Engine) Recover() error {
	rows, err := e.Store.List("run", 10000)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var r Run
		if err = json.Unmarshal(row, &r); err != nil {
			return err
		}
		if r.Status != "funding" && r.Status != "running" {
			continue
		}
		r.Status = "interrupted"
		r.Error = "Server stopped during this run. It will not replay or create new payments. Inspect saved hashes on Stellar Expert."
		r.FinishedAt = Now()
		if err = e.Store.Put("run", r.ID, r); err != nil {
			return err
		}
	}
	return e.recoverChallenges()
}
