package lab

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"ledgerdrill/internal/store"
)

func TestChallengeOnlyOwnerCanChooseAndOnlyOnce(t *testing.T) {
	ticket := &challengeTicket{id: "payout", owner: sha256.Sum256([]byte("browser-capability")), deadline: time.Now().Add(time.Minute), choices: make(chan string, 1)}
	e := &Engine{activeChallenge: ticket}
	if err := e.ChooseChallenge("payout", "someone-else", "retry"); !errors.Is(err, ErrChallengeOwner) {
		t.Fatalf("wrong browser allowed: %v", err)
	}
	if err := e.ChooseChallenge("payout", "browser-capability", "invalid"); err == nil {
		t.Fatal("invalid recovery accepted")
	}
	if err := e.ChooseChallenge("payout", "browser-capability", "reconcile"); err != nil {
		t.Fatal(err)
	}
	if err := e.ChooseChallenge("payout", "browser-capability", "retry"); !errors.Is(err, ErrDecision) {
		t.Fatalf("second recovery accepted: %v", err)
	}
	if action := <-ticket.choices; action != "reconcile" {
		t.Fatalf("queued %q", action)
	}
	if len(ticket.choices) != 0 {
		t.Fatal("multiple payment recoveries queued")
	}
}
func TestExpiredChallengeCannotSendAnotherPayment(t *testing.T) {
	ticket := &challengeTicket{id: "expired", owner: sha256.Sum256([]byte("key")), deadline: time.Now().Add(-time.Second), choices: make(chan string, 1)}
	e := &Engine{activeChallenge: ticket}
	if err := e.ChooseChallenge("expired", "key", "retry"); !errors.Is(err, ErrDecision) {
		t.Fatal("expired payout accepted a new transfer")
	}
	if len(ticket.choices) != 0 {
		t.Fatal("expired payout queued a payment")
	}
}
func TestRestartInterruptsWaitingChallengeWithoutReplay(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := Challenge{ID: "waiting", Status: "waiting", Result: ClientResult{Events: []Event{{Hash: "original-hash"}}}}
	if err = db.Put("challenge", c.ID, c); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(db, NewProxy(db), "http://localhost", "", context.Background())
	if err = e.Recover(); err != nil {
		t.Fatal(err)
	}
	if err = db.Get("challenge", c.ID, &c); err != nil {
		t.Fatal(err)
	}
	if c.Status != "interrupted" || c.Result.Events[0].Hash != "original-hash" || e.Busy() {
		t.Fatal("restart lost the original hash or resumed a payout")
	}
	if err = e.ChooseChallenge(c.ID, "any-key", "retry"); !errors.Is(err, ErrDecision) {
		t.Fatal("restart recreated recovery authorization")
	}
}
