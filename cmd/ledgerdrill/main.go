package main

import (
	"context"
	"flag"
	"fmt"
	"ledgerdrill/internal/lab"
	"ledgerdrill/internal/server"
	"ledgerdrill/internal/store"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	addr := flag.String("addr", "127.0.0.1:8098", "HTTP listen address; non-loopback binding requires LEDGERDRILL_TOKEN")
	data := flag.String("data", "./data", "SQLite journal directory")
	flag.Parse()
	token := os.Getenv("LEDGERDRILL_TOKEN")
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) && len(token) < 24 {
		return fmt.Errorf("non-loopback binding requires LEDGERDRILL_TOKEN with at least 24 characters")
	}
	s, err := store.Open(filepath.Join(*data, "ledgerdrill.db"))
	if err != nil {
		return err
	}
	defer s.Close()
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	localHost := "127.0.0.1"
	if strings.Contains(host, ":") {
		localHost = "::1"
	}
	base := "http://" + net.JoinHostPort(localHost, port)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	p := lab.NewProxy(s)
	e := lab.NewEngine(s, p, base, token, ctx)
	if err = e.Recover(); err != nil {
		return err
	}
	handler, err := server.New(s, e, p, token)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 100 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Printf("LedgerDrill ready at %s — Stellar testnet only", base)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(listener) }()
	select {
	case err = <-done:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err = srv.Shutdown(shutdown); err != nil {
			return err
		}
	}
	for e.Busy() {
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}
