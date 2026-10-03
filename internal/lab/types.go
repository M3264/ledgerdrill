package lab

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const HorizonURL = "https://horizon-testnet.stellar.org"
const Network = "Test SDF Network ; September 2015"

type Scenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Expected    string `json:"expected"`
}

var Scenarios = []Scenario{
	{"before_submit", "Never reaches the network", "Reject the first submission before it reaches Stellar. A retry should send exactly one payment.", "Both clients should settle one payment."},
	{"lost_confirmation", "Paid, but confirmation lost", "Forward the payment to Stellar, then replace its successful response with a timeout.", "The naive client pays twice; reconciliation finds the original payment."},
	{"delayed_lookup", "Confirmation stays hidden", "Lose the submission response and hide the first two transaction lookups. Not found does not mean not paid.", "The naive client pays twice; the reconciled client waits for evidence."},
}

func ValidScenario(id string) bool {
	for _, s := range Scenarios {
		if s.ID == id {
			return true
		}
	}
	return false
}
func NewID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

type Event struct {
	At      string `json:"at"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Hash    string `json:"hash,omitempty"`
	Status  int    `json:"status,omitempty"`
}
type Evidence struct {
	Hash              string `json:"hash"`
	Confirmed         bool   `json:"confirmed"`
	Successful        bool   `json:"successful"`
	Ledger            int32  `json:"ledger,omitempty"`
	Explorer          string `json:"explorer"`
	Amount            string `json:"amount,omitempty"`
	VerificationError string `json:"verification_error,omitempty"`
}
type ClientResult struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Source       string     `json:"source,omitempty"`
	Destination  string     `json:"destination,omitempty"`
	SessionID    string     `json:"session_id,omitempty"`
	Attempts     int        `json:"attempts"`
	Settlements  int        `json:"settlements"`
	Received     string     `json:"received"`
	Verdict      string     `json:"verdict"`
	Events       []Event    `json:"events"`
	Transactions []Evidence `json:"transactions"`
}
type Run struct {
	ID         string         `json:"id"`
	Scenario   string         `json:"scenario"`
	Status     string         `json:"status"`
	CreatedAt  string         `json:"created_at"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Network    string         `json:"network"`
	Asset      string         `json:"asset"`
	Amount     string         `json:"amount"`
	Error      string         `json:"error,omitempty"`
	Clients    []ClientResult `json:"clients"`
}
