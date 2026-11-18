package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "ghep"
)

type metrics struct {
	eventsReceivedTotal      prometheus.Counter
	eventsHandledTotal       prometheus.Counter
	eventsHandledFailedTotal prometheus.Counter
}

func newMetrics() *metrics {
	m := &metrics{
		eventsReceivedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "events_received_total",
			Help:      "every valid webhook that hits POST /events",
			Namespace: namespace,
		}),
		eventsHandledTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "events_handled_total",
			Help:        "events actually routed to a team",
			ConstLabels: prometheus.Labels{},
		}),
		eventsHandledFailedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "events_handled_failed_total",
			Help:        "events actually routed to a team, but failed",
			ConstLabels: prometheus.Labels{},
		}),
	}

	return m
}
