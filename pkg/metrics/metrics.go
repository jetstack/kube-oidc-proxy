// Copyright Jetstack Ltd. See LICENSE for details.
package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"
)

const (
	promNamespace = "kube_oidc_proxy"
)

type Metrics struct {
	*http.Server

	registry *prometheus.Registry

	// Metrics for incoming client requests
	clientRequests *prometheus.CounterVec
	clientDuration *prometheus.HistogramVec

	// Metrics for outgoing server requests
	serverRequests *prometheus.CounterVec
	serverDuration *prometheus.HistogramVec

	// Metrics for authentication of incoming requests
	oidcAuthCount *prometheus.CounterVec

	// Metrics for token reviews
	tokenReviewDuration *prometheus.HistogramVec
}

func New() *Metrics {
	var (
		clientRequests = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: promNamespace,
				Name:      "http_client_requests",
				Help:      "The number of incoming requests.",
			},
			[]string{"code", "path"},
		)
		clientDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: promNamespace,
				Name:      "http_client_duration_seconds",
				Help:      "The duration in seconds for incoming client requests to be responded to.",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
			},
			[]string{"remote_address"},
		)

		serverRequests = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: promNamespace,
				Name:      "http_server_requests",
				Help:      "The number of outgoing server requests.",
			},
			[]string{"code", "path"},
		)
		serverDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: promNamespace,
				Name:      "http_server_duration_seconds",
				Help:      "The duration in seconds for outgoing server requests to be responded to.",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
			},
			[]string{"remote_address"},
		)

		oidcAuthCount = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: promNamespace,
				Name:      "oidc_authentication_count",
				Help:      "The count for OIDC authentication. Authenticated requests are 1, else 0.",
			},
			[]string{"authenticated", "remote_address", "user"},
		)

		tokenReviewDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: promNamespace,
				Name:      "token_review_duration_seconds",
				Help:      "The duration in seconds for a token review lookup. Authenticated requests are 1, else 0.",
				Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
			},
			[]string{"authenticated", "code", "remote_address", "user"},
		)
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(clientRequests)
	registry.MustRegister(clientDuration)
	registry.MustRegister(serverRequests)
	registry.MustRegister(serverDuration)
	registry.MustRegister(oidcAuthCount)
	registry.MustRegister(tokenReviewDuration)

	return &Metrics{
		registry: registry,

		clientRequests: clientRequests,
		clientDuration: clientDuration,

		serverRequests: serverRequests,
		serverDuration: serverDuration,

		oidcAuthCount:       oidcAuthCount,
		tokenReviewDuration: tokenReviewDuration,
	}
}

// Start will register the Prometheus metrics, and start the Prometheus server
func (m *Metrics) Start(listenAddress string) error {
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}

	m.Server = &http.Server{
		Addr:           ln.Addr().String(),
		ReadTimeout:    8 * time.Second,
		WriteTimeout:   8 * time.Second,
		MaxHeaderBytes: 1 << 15, // 1 MiB
		Handler:        router,
	}

	go func() {
		klog.Infof("serving metrics on %s/metrics", ln.Addr())

		if err := m.Serve(ln); err != nil {
			klog.Errorf("failed to serve prometheus metrics: %s", err)
			return
		}
	}()

	return nil
}

func (m *Metrics) Shutdown() error {
	// If metrics server is not started than exit early
	if m.Server == nil {
		return nil
	}

	klog.Info("shutting down Prometheus metrics server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := m.Server.Shutdown(ctx); err != nil {
		return fmt.Errorf("prometheus metrics server shutdown failed: %s", err)
	}

	klog.Info("prometheus metrics server gracefully stopped")

	return nil
}

func (m *Metrics) ObserveClient(code int, path, remoteAddress string, duration time.Duration) {
	m.clientRequests.With(prometheus.Labels{
		"code": strconv.Itoa(code),
		"path": path,
	}).Inc()

	m.clientDuration.With(prometheus.Labels{
		"remote_address": remoteAddress,
	}).Observe(duration.Seconds())
}

func (m *Metrics) ObserveServer(code int, path, remoteAddress string, duration time.Duration) {
	m.serverRequests.With(prometheus.Labels{
		"code": strconv.Itoa(code),
		"path": path,
	}).Inc()

	m.serverDuration.With(prometheus.Labels{
		"remote_address": remoteAddress,
	}).Observe(duration.Seconds())
}

func (m *Metrics) IncrementOIDCAuthCount(authenticated bool, remoteAddress, user string) {
	m.oidcAuthCount.With(prometheus.Labels{
		"authenticated":  boolToIntString(authenticated),
		"remote_address": remoteAddress,
		"user":           user,
	}).Inc()
}

func (m *Metrics) ObserveTokenReivewLookup(authenticated bool, code int, remoteAddress, user string, duration time.Duration) {
	m.tokenReviewDuration.With(prometheus.Labels{
		"authenticated":  boolToIntString(authenticated),
		"code":           strconv.Itoa(code),
		"remote_address": remoteAddress,
		"user":           user,
	}).Observe(duration.Seconds())
}

func boolToIntString(b bool) string {
	var i int
	if b {
		i = 1
	}
	return strconv.Itoa(i)
}
