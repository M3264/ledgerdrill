package lab

import (
	"encoding/json"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"ledgerdrill/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func envelope(t *testing.T) (string, string) {
	t.Helper()
	kp := keypair.MustRandom()
	dst := keypair.MustRandom()
	a := txnbuild.SimpleAccount{AccountID: kp.Address(), Sequence: 100}
	tx, e := txnbuild.NewTransaction(txnbuild.TransactionParams{SourceAccount: &a, IncrementSequenceNum: true, BaseFee: 100, Operations: []txnbuild.Operation{&txnbuild.Payment{Destination: dst.Address(), Asset: txnbuild.NativeAsset{}, Amount: "1"}}, Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(120)}})
	if e != nil {
		t.Fatal(e)
	}
	tx, e = tx.Sign(Network, kp)
	if e != nil {
		t.Fatal(e)
	}
	b, e := tx.Base64()
	if e != nil {
		t.Fatal(e)
	}
	h, e := tx.HashHex(Network)
	if e != nil {
		t.Fatal(e)
	}
	return b, h
}
func TestFaultsPreserveEnvelopeAndDoNotInventSettlement(t *testing.T) {
	for _, scenario := range Scenarios {
		t.Run(scenario.ID, func(t *testing.T) {
			s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			p := NewProxy(s)
			encoded, hash := envelope(t)
			submits := 0
			seen := map[string]bool{}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					submits++
					r.ParseForm()
					if r.Form.Get("tx") != encoded {
						t.Error("proxy changed signed envelope")
					}
					seen[hash] = true
				}
				json.NewEncoder(w).Encode(map[string]any{"hash": hash, "successful": true, "ledger": 42})
			}))
			defer upstream.Close()
			p.upstream = upstream.URL
			session, e := p.Create(scenario.ID)
			if e != nil {
				t.Fatal(e)
			}
			post := func() int {
				r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"tx": {encoded}}.Encode()))
				w := httptest.NewRecorder()
				p.Serve(w, r, session.ID, "/transactions")
				return w.Code
			}
			first := post()
			if scenario.ID == "before_submit" {
				if first != 503 || submits != 0 {
					t.Fatalf("pre-broadcast fault reached network: status=%d attempts=%d", first, submits)
				}
			} else {
				if first != 504 || submits != 1 {
					t.Fatalf("lost response didn't relay exactly once: status=%d attempts=%d", first, submits)
				}
			}
			// Load from SQLite into a fresh proxy, proving one-shot faults survive restart.
			p = NewProxy(s)
			p.upstream = upstream.URL
			if scenario.ID == "delayed_lookup" {
				for i := 0; i < 3; i++ {
					w := httptest.NewRecorder()
					p.Serve(w, httptest.NewRequest("GET", "/", nil), session.ID, "/transactions/"+hash)
					want := 404
					if i == 2 {
						want = 200
					}
					if w.Code != want {
						t.Fatalf("lookup %d: got %d want %d", i, w.Code, want)
					}
				}
			}
			if status := post(); status != 200 {
				t.Fatalf("fault reapplied after restart: %d", status)
			}
			if len(seen) != 1 {
				t.Fatal("same envelope created multiple unique settlements")
			}
			d, e := p.Get(session.ID)
			if e != nil {
				t.Fatal(e)
			}
			if d.Attempts != 2 || len(d.Transactions) != 1 || !d.Transactions[0].Confirmed || !d.Transactions[0].Successful {
				t.Fatalf("incorrect journal: %+v", d)
			}
		})
	}
}
func TestUpstreamRejectionIsNotReportedAsConfirmed(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p := NewProxy(s)
	u := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400); w.Write([]byte(`{"status":400}`)) }))
	defer u.Close()
	p.upstream = u.URL
	d, _ := p.Create("lost_confirmation")
	b, _ := envelope(t)
	w := httptest.NewRecorder()
	p.Serve(w, httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"tx": {b}}.Encode())), d.ID, "/transactions")
	d, e = p.Get(d.ID)
	if e != nil {
		t.Fatal(e)
	}
	if w.Code != 400 || d.FaultApplied || d.Transactions[0].Confirmed {
		t.Fatal("rejected transaction marked as settled or injected fault")
	}
}
func TestProxyRejectsUnsafePathsAndInvalidEnvelope(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p := NewProxy(s)
	d, _ := p.Create("before_submit")
	for _, path := range []string{"//evil.invalid/", "/../transactions", "/transactions?url=https://evil.invalid", "/accounts/secret"} {
		w := httptest.NewRecorder()
		p.Serve(w, httptest.NewRequest("GET", "/", nil), d.ID, path)
		if w.Code != 400 {
			t.Fatalf("accepted unsafe path: %s", path)
		}
	}
	w := httptest.NewRecorder()
	p.Serve(w, httptest.NewRequest("POST", "/", strings.NewReader("tx=not-xdr")), d.ID, "/transactions")
	if w.Code != 400 {
		t.Fatal("accepted malformed envelope")
	}
	d, _ = p.Get(d.ID)
	if d.Attempts != 0 {
		t.Fatal("invalid envelope consumed scenario")
	}
}
