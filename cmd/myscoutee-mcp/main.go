package main

import (
	"context"
	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"github.com/fssrepository/myscoutee-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		probe := &http.Client{Timeout: 2 * time.Second}
		response, err := probe.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			os.Exit(1)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client, err := api.NewClient(os.Getenv("MYSCOUTEE_API_BASE_URL"), os.Getenv("MYSCOUTEE_API_TOKEN"), os.Getenv("MYSCOUTEE_CLIENT_ID"))
	if err != nil {
		log.Fatal(err)
	}
	if publicBase := os.Getenv("MYSCOUTEE_PUBLIC_API_BASE_URL"); publicBase != "" {
		public, err := api.NewClient(publicBase, "", "")
		if err != nil {
			log.Fatal(err)
		}
		client.PublicBase = public.Base
	}
	switch os.Getenv("MYSCOUTEE_MCP_TRANSPORT") {
	case "", "stdio":
		if err = server.New(client, client.Token != "").Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
			log.Fatal("MCP transport failed")
		}
	case "http":
		handler, err := server.PublicHandler(client)
		if err != nil {
			log.Fatal(err)
		}
		addr := os.Getenv("MYSCOUTEE_MCP_LISTEN")
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		httpServer := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 25 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
		go func() {
			<-ctx.Done()
			shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_ = httpServer.Shutdown(shutdown)
		}()
		if err = httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("MCP HTTP transport failed")
		}
	default:
		log.Fatal("MYSCOUTEE_MCP_TRANSPORT must be stdio or http")
	}
}
