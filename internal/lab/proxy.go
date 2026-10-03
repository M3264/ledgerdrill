package lab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"io"
	"ledgerdrill/internal/store"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Session struct {
	ID            string     `json:"id"`
	Scenario      string     `json:"scenario"`
	CreatedAt     string     `json:"created_at"`
	Attempts      int        `json:"attempts"`
	HiddenLookups int        `json:"hidden_lookups"`
	FaultApplied  bool       `json:"fault_applied"`
	Events        []Event    `json:"events"`
	Transactions  []Evidence `json:"transactions"`
}
type sessionState struct {
	mu   sync.Mutex
	data Session
}
type Proxy struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
	Store    *store.Store
	HTTP     *http.Client
	upstream string
}

func NewProxy(s *store.Store) *Proxy {
	return &Proxy{sessions: map[string]*sessionState{}, Store: s, HTTP: &http.Client{Timeout: 80 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, upstream: HorizonURL}
}
func (p *Proxy) Create(scenario string) (Session, error) {
	if !ValidScenario(scenario) {
		return Session{}, fmt.Errorf("choose a supported scenario")
	}
	d := Session{ID: NewID(), Scenario: scenario, CreatedAt: Now(), Events: []Event{}, Transactions: []Evidence{}}
	if e := p.Store.Put("session", d.ID, d); e != nil {
		return Session{}, e
	}
	p.mu.Lock()
	p.sessions[d.ID] = &sessionState{data: d}
	p.mu.Unlock()
	return d, nil
}
func (p *Proxy) state(id string) (*sessionState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s := p.sessions[id]; s != nil {
		return s, nil
	}
	var d Session
	if e := p.Store.Get("session", id, &d); e != nil {
		return nil, e
	}
	s := &sessionState{data: d}
	p.sessions[id] = s
	return s, nil
}
func (p *Proxy) Get(id string) (Session, error) {
	s, e := p.state(id)
	if e != nil {
		return Session{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.Marshal(s.data)
	if e != nil {
		return Session{}, e
	}
	var d Session
	e = json.Unmarshal(b, &d)
	return d, e
}
func proxyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"status": status, "title": message, "ledgerdrill_fault": strings.HasPrefix(message, "Injected")})
}

var accountPath = regexp.MustCompile(`^/accounts/G[A-Z2-7]{55}(/(transactions|payments|operations))?$`)
var transactionPath = regexp.MustCompile(`^/transactions/[a-f0-9]{64}$`)

func allowedPath(method, path string) bool {
	if method == "POST" {
		return path == "/transactions"
	}
	return method == "GET" && (path == "/" || path == "/fee_stats" || accountPath.MatchString(path) || transactionPath.MatchString(path))
}
func (p *Proxy) Serve(w http.ResponseWriter, r *http.Request, id, path string) {
	if !allowedPath(r.Method, path) {
		proxyError(w, 400, "This proxy supports classic account reads and transaction submission on testnet only.")
		return
	}
	s, e := p.state(id)
	if e != nil {
		proxyError(w, 404, "Proxy session not found. Create a new session.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := &s.data
	event := func(kind, msg, hash string, status int) {
		d.Events = append(d.Events, Event{Now(), kind, msg, hash, status})
	}
	save := func() bool {
		if e := p.Store.Put("session", id, d); e != nil {
			proxyError(w, 500, "Could not journal this request. Nothing further was submitted.")
			return false
		}
		return true
	}
	var body []byte
	hash := ""
	if r.Method == "POST" {
		if d.Attempts >= 20 {
			proxyError(w, 429, "Session submission limit reached. Create a new session.")
			return
		}
		body, e = io.ReadAll(io.LimitReader(r.Body, 131073))
		if e != nil || len(body) > 131072 {
			proxyError(w, 400, "Transaction body must be at most 128 KiB.")
			return
		}
		form, e := url.ParseQuery(string(body))
		if e != nil {
			proxyError(w, 400, "Use a form-encoded tx envelope.")
			return
		}
		tx, e := txnbuild.TransactionFromXDR(form.Get("tx"))
		if e != nil {
			proxyError(w, 400, "Invalid Stellar transaction envelope.")
			return
		}
		hash, e = tx.HashHex(Network)
		if e != nil {
			proxyError(w, 400, "Could not hash this envelope.")
			return
		}
		d.Attempts++
		event("submit", "Signed envelope received; testnet hash recorded before broadcast.", hash, 0)
		found := false
		for _, t := range d.Transactions {
			if t.Hash == hash {
				found = true
			}
		}
		if !found {
			d.Transactions = append(d.Transactions, Evidence{Hash: hash, Explorer: "https://stellar.expert/explorer/testnet/tx/" + hash})
		}
		if d.Scenario == "before_submit" && !d.FaultApplied {
			d.FaultApplied = true
			event("fault", "Injected failure before broadcast. This attempt never reached Stellar.", hash, 503)
			if save() {
				proxyError(w, 503, "Injected failure before submission")
			}
			return
		}
		// Persist the attempt and unknown outcome before contacting the network.
		if !save() {
			return
		}
	}
	if transactionPath.MatchString(path) && d.HiddenLookups > 0 {
		lookupHash := strings.TrimPrefix(path, "/transactions/")
		known := false
		for _, t := range d.Transactions {
			if t.Hash == lookupHash && t.Confirmed {
				known = true
			}
		}
		if known {
			d.HiddenLookups--
			event("fault", "Injected not-found response; confirmed payment temporarily hidden.", lookupHash, 404)
			if save() {
				proxyError(w, 404, "Injected delayed transaction visibility")
			}
			return
		}
	}
	req, e := http.NewRequestWithContext(r.Context(), r.Method, p.upstream+path, bytes.NewReader(body))
	if e != nil {
		proxyError(w, 500, "Could not construct upstream request.")
		return
	}
	req.URL.RawQuery = r.URL.RawQuery
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, e := p.HTTP.Do(req)
	if e != nil {
		event("network", "Upstream response unavailable; submission outcome may be unknown.", hash, 502)
		if save() {
			proxyError(w, 502, "Horizon unavailable. Reconcile by hash before retrying.")
		}
		return
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		event("network", "Could not read upstream response; reconcile by hash.", hash, 502)
		if save() {
			proxyError(w, 502, "Incomplete Horizon response")
		}
		return
	}
	if r.Method == "POST" {
		event("upstream", fmt.Sprintf("Horizon returned HTTP %d.", resp.StatusCode), hash, resp.StatusCode)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var t struct {
				Hash       string `json:"hash"`
				Successful bool   `json:"successful"`
				Ledger     int32  `json:"ledger"`
			}
			if json.Unmarshal(b, &t) == nil && t.Hash == hash {
				for i := range d.Transactions {
					if d.Transactions[i].Hash == hash {
						d.Transactions[i].Confirmed = true
						d.Transactions[i].Successful = t.Successful
						d.Transactions[i].Ledger = t.Ledger
					}
				}
			}
			if !d.FaultApplied && (d.Scenario == "lost_confirmation" || d.Scenario == "delayed_lookup") {
				d.FaultApplied = true
				if d.Scenario == "delayed_lookup" {
					d.HiddenLookups = 2
				}
				event("fault", "Payment reached Stellar. Successful submission response replaced with a timeout.", hash, 504)
				if save() {
					proxyError(w, 504, "Injected lost confirmation. Payment may have settled.")
				}
				return
			}
		}
	}
	if !save() {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-LedgerDrill-Upstream", "stellar-testnet")
	w.WriteHeader(resp.StatusCode)
	w.Write(b)
}
