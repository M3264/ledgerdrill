package lab

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
)

var ErrDecision = errors.New("this payout is not waiting for a decision; reopen it or start a new challenge")
var ErrChallengeOwner = errors.New("only the browser that started this payout can choose its recovery")

type Challenge struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	CreatedAt  string       `json:"created_at"`
	FinishedAt string       `json:"finished_at,omitempty"`
	Deadline   string       `json:"deadline,omitempty"`
	Choice     string       `json:"choice,omitempty"`
	Scenario   string       `json:"scenario,omitempty"`
	Error      string       `json:"error,omitempty"`
	Recipient  string       `json:"recipient"`
	Amount     string       `json:"amount"`
	Result     ClientResult `json:"result"`
}
type challengeTicket struct {
	id       string
	owner    [32]byte
	choices  chan string
	chosen   bool
	deadline time.Time
	mu       sync.Mutex
}

func (e *Engine) StartChallenge() (Challenge, string, error) {
	if !e.busy.CompareAndSwap(false, true) {
		return Challenge{}, "", ErrBusy
	}
	key := NewID()
	c := Challenge{ID: NewID(), Status: "funding", CreatedAt: Now(), Recipient: "Rowan", Amount: "1.0000000", Result: ClientResult{ID: "player", Status: "waiting", Received: "0.0000000", Events: []Event{}, Transactions: []Evidence{}}}
	if err := e.Store.Put("challenge", c.ID, c); err != nil {
		e.busy.Store(false)
		return Challenge{}, "", err
	}
	ticket := &challengeTicket{id: c.ID, owner: sha256.Sum256([]byte(key)), choices: make(chan string, 1)}
	e.challengeMu.Lock()
	e.activeChallenge = ticket
	e.challengeMu.Unlock()
	go e.executeChallenge(c, ticket)
	return c, key, nil
}
func (e *Engine) ChooseChallenge(id, key, action string) error {
	if action != "retry" && action != "reconcile" {
		return fmt.Errorf("choose retry or reconcile")
	}
	e.challengeMu.Lock()
	ticket := e.activeChallenge
	e.challengeMu.Unlock()
	if ticket == nil || ticket.id != id {
		return ErrDecision
	}
	owner := sha256.Sum256([]byte(key))
	if subtle.ConstantTimeCompare(owner[:], ticket.owner[:]) != 1 {
		return ErrChallengeOwner
	}
	ticket.mu.Lock()
	defer ticket.mu.Unlock()
	if ticket.chosen || ticket.deadline.IsZero() || !time.Now().Before(ticket.deadline) {
		return ErrDecision
	}
	ticket.chosen = true
	ticket.choices <- action
	return nil
}
func (e *Engine) executeChallenge(c Challenge, ticket *challengeTicket) {
	defer func() {
		e.challengeMu.Lock()
		if e.activeChallenge == ticket {
			e.activeChallenge = nil
		}
		e.challengeMu.Unlock()
		e.busy.Store(false)
	}()
	ctx, cancel := context.WithTimeout(e.Context, 6*time.Minute)
	defer cancel()
	save := func() error { return e.Store.Put("challenge", c.ID, c) }
	log := func(kind, message, hash string) error {
		c.Result.Events = append(c.Result.Events, Event{At: Now(), Kind: kind, Message: message, Hash: hash})
		return save()
	}
	fail := func(err error) {
		c.Status = "failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			c.Status = "interrupted"
		}
		c.Error = err.Error()
		c.FinishedAt = Now()
		_ = save()
	}
	source, err := keypair.Random()
	if err != nil {
		fail(err)
		return
	}
	recipient, err := keypair.Random()
	if err != nil {
		fail(err)
		return
	}
	c.Result.Source = source.Address()
	c.Result.Destination = recipient.Address()
	if err = save(); err != nil {
		fail(err)
		return
	}
	for _, kp := range []*keypair.Full{source, recipient} {
		if err = e.fund(ctx, kp.Address()); err != nil {
			fail(fmt.Errorf("test account setup failed: %w. Try again when Friendbot is available", err))
			return
		}
	}
	// The scenario is deliberately unrevealed until the player acts. Every payout
	// uses a real randomly selected fault, never a fabricated settlement result.
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(Scenarios))))
	if err != nil {
		fail(err)
		return
	}
	scenario := Scenarios[n.Int64()].ID
	session, err := e.Proxy.Create(scenario)
	if err != nil {
		fail(err)
		return
	}
	c.Result.SessionID = session.ID
	c.Status = "sending"
	c.Result.Status = "running"
	if err = save(); err != nil {
		fail(err)
		return
	}
	envelope, hash, err := e.buildWithTimeout(ctx, source, recipient.Address(), "challenge-"+c.ID[:8], 300)
	if err != nil {
		fail(err)
		return
	}
	if err = log("prepared", "Built one 1 XLM payout. Retained its original signed envelope and hash.", hash); err != nil {
		fail(err)
		return
	}
	code, submissionErr := e.submit(ctx, session.ID, envelope)
	if submissionErr != nil || (code != 503 && code != 504) {
		fail(fmt.Errorf("the controlled failure could not be completed (HTTP %d). Inspect the original hash; no recovery was chosen", code))
		return
	}
	if err = log("uncertain", "The connection returned an error. The client cannot tell whether the payout settled.", hash); err != nil {
		fail(err)
		return
	}
	ticket.mu.Lock()
	ticket.deadline = time.Now().Add(2 * time.Minute)
	c.Deadline = ticket.deadline.UTC().Format(time.RFC3339Nano)
	c.Status = "waiting"
	err = save()
	ticket.mu.Unlock()
	if err != nil {
		fail(err)
		return
	}
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	select {
	case c.Choice = <-ticket.choices:
	case <-timer.C:
		c.Status = "expired"
		c.Error = "No recovery was chosen within 2 minutes. No additional payment was sent."
		c.Scenario = scenario
		c.FinishedAt = Now()
		e.finishChallenge(ctx, &c, session.ID)
		_ = save()
		return
	case <-ctx.Done():
		fail(ctx.Err())
		return
	}
	c.Status = "recovering"
	if err = log("decision", map[string]string{"retry": "You chose to send a fresh payment.", "reconcile": "You chose to check the original transaction."}[c.Choice], hash); err != nil {
		fail(err)
		return
	}
	client := "reconciled"
	if c.Choice == "retry" {
		client = "fresh"
	}
	recoveryErr := e.recoverPayment(ctx, source, recipient.Address(), client, session.ID, envelope, hash, log)
	c.Status = "verifying"
	if err = save(); err != nil {
		fail(err)
		return
	}
	verified := e.finishChallenge(ctx, &c, session.ID)
	c.Scenario = scenario
	c.FinishedAt = Now()
	c.Status = "complete"
	if !verified || recoveryErr != nil || c.Result.Settlements == 0 {
		c.Status = "inconclusive"
		c.Error = "Recovery or ledger verification did not finish. Inspect the saved transaction hashes."
	}
	if recoveryErr != nil {
		_ = log("error", recoveryErr.Error(), "")
	}
	if err = save(); err != nil {
		fail(err)
	}
}
func (e *Engine) finishChallenge(ctx context.Context, c *Challenge, sessionID string) bool {
	snapshot, err := e.Proxy.Get(sessionID)
	if err != nil {
		c.Error = "Could not read the saved proxy trace"
		return false
	}
	c.Result.Attempts = snapshot.Attempts
	c.Result.Transactions = snapshot.Transactions
	failed := e.verifyClient(ctx, &c.Result, c.Result.Destination)
	c.Result.Status = "complete"
	switch {
	case failed:
		c.Result.Status = "inconclusive"
		c.Result.Verdict = "The ledger check could not finish"
	case c.Result.Settlements > 1:
		c.Result.Verdict = "Rowan received two payments"
	case c.Result.Settlements == 1:
		c.Result.Verdict = "Rowan received one payment"
	default:
		c.Result.Verdict = "No successful payout was verified"
	}
	return !failed
}
func (e *Engine) recoverChallenges() error {
	rows, err := e.Store.List("challenge", 10000)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var c Challenge
		if err = json.Unmarshal(row, &c); err != nil {
			return err
		}
		if c.Status == "funding" || c.Status == "sending" || c.Status == "waiting" || c.Status == "recovering" || c.Status == "verifying" {
			c.Status = "interrupted"
			c.FinishedAt = Now()
			c.Error = "The server restarted. This payout will never be replayed. Inspect its original hash before starting again."
			if err = e.Store.Put("challenge", c.ID, c); err != nil {
				return err
			}
		}
	}
	return nil
}
