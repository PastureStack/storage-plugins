package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func startHealthCheck(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid health check port number: %d", port)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	address := ":" + strconv.Itoa(port)
	logrus.Infof("Listening for health checks on %s/healthcheck", address)
	return (&http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}).ListenAndServe()
}
