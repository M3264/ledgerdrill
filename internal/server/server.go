package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"ledgerdrill/internal/lab"
	"ledgerdrill/internal/store"
	"ledgerdrill/web"
	"net/http"
	"net/url"
	"strings"
)

type Server struct {
	Store  *store.Store
	Engine *lab.Engine
	Proxy  *lab.Proxy
	Token  string
	static http.Handler
	index  []byte
}

func New(s *store.Store, e *lab.Engine, p *lab.Proxy, token string) (*Server, error) {
	assets, err := fs.Sub(web.Files, "build")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		index, err = fs.ReadFile(web.Files, "fallback.html")
		if err != nil {
			return nil, err
		}
	}
	return &Server{Store: s, Engine: e, Proxy: p, Token: token, static: http.FileServer(http.FS(assets)), index: index}, nil
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func errorReply(w http.ResponseWriter, status int, msg string) {
	jsonReply(w, status, map[string]string{"error": msg})
}
func (s *Server) authorized(r *http.Request) bool {
	return s.Token == "" || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(s.Token)) == 1
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	path := r.URL.Path
	if path == "/healthz" && r.Method == "GET" {
		jsonReply(w, 200, map[string]string{"status": "ok", "network": "testnet"})
		return
	}
	if path == "/api/config" && r.Method == "GET" {
		w.Header().Set("Cache-Control", "no-store")
		jsonReply(w, 200, map[string]any{"scenarios": lab.Scenarios, "network": "Stellar testnet", "asset": "XLM", "amount": "1.0000000", "busy": s.Engine.Busy(), "auth_required": s.Token != ""})
		return
	}
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/horizon/") {
		w.Header().Set("Cache-Control", "no-store")
		if !sameOrigin(r) {
			errorReply(w, 403, "Cross-origin requests are not allowed. Use the local dashboard or CLI.")
			return
		}
		if !s.authorized(r) {
			errorReply(w, 401, "Enter the configured access token to use this instance.")
			return
		}
		if strings.HasPrefix(path, "/horizon/") {
			parts := strings.SplitN(strings.TrimPrefix(path, "/horizon/"), "/", 2)
			if len(parts) < 2 {
				errorReply(w, 400, "Proxy URL must end with a slash.")
				return
			}
			s.Proxy.Serve(w, r, parts[0], "/"+parts[1])
			return
		}
		s.api(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		errorReply(w, 405, "Use GET for the dashboard.")
		return
	}
	if strings.HasPrefix(path, "/_app/") || strings.HasPrefix(path, "/fonts/") || path == "/favicon.svg" {
		w.Header().Set("Cache-Control", "public,max-age=86400")
		s.static.ServeHTTP(w, r)
		return
	}
	if path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	if r.Method == "GET" {
		w.Write(s.index)
	}
}
func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/api/challenges" && r.Method == "GET" {
		rows, err := s.Store.List("challenge", 100)
		if err != nil {
			errorReply(w, 500, "Could not load saved payouts.")
			return
		}
		jsonReply(w, 200, map[string]any{"challenges": rows})
		return
	}
	if path == "/api/challenges" && r.Method == "POST" {
		challenge, key, err := s.Engine.StartChallenge()
		if errors.Is(err, lab.ErrBusy) {
			errorReply(w, 409, err.Error())
			return
		}
		if err != nil {
			errorReply(w, 500, "Could not start the payout. Try again.")
			return
		}
		jsonReply(w, 202, map[string]any{"challenge": challenge, "decision_key": key})
		return
	}
	if strings.HasPrefix(path, "/api/challenges/") {
		parts := strings.Split(strings.TrimPrefix(path, "/api/challenges/"), "/")
		if len(parts) == 2 && parts[1] == "decision" && r.Method == "POST" {
			var body struct {
				Key    string `json:"key"`
				Action string `json:"action"`
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&body); err != nil {
				errorReply(w, 400, "Send a valid recovery choice.")
				return
			}
			err := s.Engine.ChooseChallenge(parts[0], body.Key, body.Action)
			if errors.Is(err, lab.ErrChallengeOwner) {
				errorReply(w, 403, err.Error())
				return
			}
			if errors.Is(err, lab.ErrDecision) {
				errorReply(w, 409, err.Error())
				return
			}
			if err != nil {
				errorReply(w, 400, err.Error())
				return
			}
			jsonReply(w, 202, map[string]string{"status": "accepted"})
			return
		}
		if r.Method == "GET" && (len(parts) == 1 || len(parts) == 2 && parts[1] == "report") {
			var challenge lab.Challenge
			err := s.Store.Get("challenge", parts[0], &challenge)
			if errors.Is(err, store.ErrNotFound) {
				errorReply(w, 404, "Payout not found. Start a new challenge.")
				return
			}
			if err != nil {
				errorReply(w, 500, "Could not read this payout.")
				return
			}
			if len(parts) == 2 {
				w.Header().Set("Content-Disposition", "attachment; filename=ledgerdrill-payout-"+challenge.ID+".json")
			}
			jsonReply(w, 200, challenge)
			return
		}
		errorReply(w, 404, "Payout endpoint not found.")
		return
	}
	if path == "/api/runs" && r.Method == "GET" {
		rows, err := s.Store.List("run", 100)
		if err != nil {
			errorReply(w, 500, "Could not load saved runs.")
			return
		}
		jsonReply(w, 200, map[string]any{"runs": rows, "busy": s.Engine.Busy()})
		return
	}
	if (path == "/api/runs" || path == "/api/sessions") && r.Method == "POST" {
		var body struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			errorReply(w, 400, "Send JSON with one supported scenario.")
			return
		}
		if !lab.ValidScenario(body.Scenario) {
			errorReply(w, 400, "Choose a supported fault scenario.")
			return
		}
		if path == "/api/sessions" {
			session, err := s.Proxy.Create(body.Scenario)
			if err != nil {
				errorReply(w, 500, "Could not create a proxy session.")
				return
			}
			jsonReply(w, 201, map[string]any{"session": session, "proxy_path": "/horizon/" + session.ID + "/"})
			return
		}
		run, err := s.Engine.Start(body.Scenario)
		if errors.Is(err, lab.ErrBusy) {
			errorReply(w, 409, err.Error())
			return
		}
		if err != nil {
			errorReply(w, 500, "Could not start or journal the comparison.")
			return
		}
		jsonReply(w, 202, run)
		return
	}
	if strings.HasPrefix(path, "/api/runs/") && r.Method == "GET" {
		parts := strings.Split(strings.TrimPrefix(path, "/api/runs/"), "/")
		if len(parts) > 2 || len(parts) == 2 && parts[1] != "report" {
			http.NotFound(w, r)
			return
		}
		var run lab.Run
		err := s.Store.Get("run", parts[0], &run)
		if errors.Is(err, store.ErrNotFound) {
			errorReply(w, 404, "Run not found. Choose a saved run or start a new comparison.")
			return
		}
		if err != nil {
			errorReply(w, 500, "Could not read the run.")
			return
		}
		if len(parts) == 2 {
			w.Header().Set("Content-Disposition", "attachment; filename=ledgerdrill-"+run.ID+".json")
		}
		jsonReply(w, 200, run)
		return
	}
	if strings.HasPrefix(path, "/api/sessions/") && r.Method == "GET" {
		session, err := s.Proxy.Get(strings.TrimPrefix(path, "/api/sessions/"))
		if errors.Is(err, store.ErrNotFound) {
			errorReply(w, 404, "Proxy session not found.")
			return
		}
		if err != nil {
			errorReply(w, 500, "Could not load proxy session.")
			return
		}
		jsonReply(w, 200, session)
		return
	}
	errorReply(w, 404, "Endpoint not found.")
}
