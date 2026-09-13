package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SigmaUno/sigmaproof/internal/server"
	"github.com/SigmaUno/sigmaproof/internal/storage"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8080", "development HTTP listen address")
	storePath := flag.String("store", "", "private local evidence store path; enables experimental engine endpoints")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	handler := server.Handler()
	engineStatus := "evidence engine unavailable"
	var store *storage.Store
	if *storePath != "" {
		if !loopbackListen(*addr) {
			log.Fatal("refusing to expose unauthenticated development engine on a non-loopback listen address")
		}
		var err error
		store, err = storage.Open(*storePath)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		handler = server.HandlerWithStore(store)
		engineStatus = "unauthenticated experimental evidence engine enabled"
	}
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("development daemon listening on %s; %s", *addr, engineStatus)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	stop()
	<-done
}

func loopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
