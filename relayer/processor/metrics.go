package processor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusMetrics struct {
	Registry              *prometheus.Registry
	PacketObservedCounter *prometheus.CounterVec
	PacketRelayedCounter  *prometheus.CounterVec
	LatestHeightGauge     *prometheus.GaugeVec
	WalletBalance         *prometheus.GaugeVec
	FeesSpent             *prometheus.GaugeVec
	TxFailureError        *prometheus.CounterVec
}

func (m *PrometheusMetrics) AddPacketsObserved(path, chain, channel, port, eventType string, count int) {
	m.PacketObservedCounter.WithLabelValues(path, chain, channel, port, eventType).Add(float64(count))
}

func (m *PrometheusMetrics) IncPacketsRelayed(path, chain, channel, port, eventType string) {
	m.PacketRelayedCounter.WithLabelValues(path, chain, channel, port, eventType).Inc()
}

func (m *PrometheusMetrics) SetLatestHeight(chain string, height int64) {
	m.LatestHeightGauge.WithLabelValues(chain).Set(float64(height))
}

func (m *PrometheusMetrics) SetWalletBalance(chain, key, address, denom string, balance float64) {
	m.WalletBalance.WithLabelValues(chain, key, address, denom).Set(balance)
}

func (m *PrometheusMetrics) SetFeesSpent(chain, key, address, denom string, amount float64) {
	m.FeesSpent.WithLabelValues(chain, key, address, denom).Set(amount)
}

func (m *PrometheusMetrics) IncTxFailure(path, chain, errDesc string) {
	m.TxFailureError.WithLabelValues(path, chain, errDesc).Inc()
}

func NewPrometheusMetrics() *PrometheusMetrics {
	packetLabels := []string{"path", "chain", "channel", "port", "type"}
	heightLabels := []string{"chain"}
	walletLabels := []string{"chain", "key", "address", "denom"}
	txFailureLabels := []string{"path", "chain", "err"}
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)
	return &PrometheusMetrics{
		Registry: registry,
		PacketObservedCounter: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_observed_packets",
			Help: "The total number of observed packets",
		}, packetLabels),
		PacketRelayedCounter: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_relayed_packets",
			Help: "The total number of relayed packets",
		}, packetLabels),
		LatestHeightGauge: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_chain_latest_height",
			Help: "The current height of the chain",
		}, heightLabels),
		WalletBalance: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_wallet_balance",
			Help: "The current balance for the relayer's wallet",
		}, walletLabels),
		FeesSpent: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_fees_spent",
			Help: "The amount of fees spent from the relayer's wallet",
		}, walletLabels),
		TxFailureError: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_tx_failure",
			Help: "The total number of tx failures broken up into categories. See https://github.com/cosmos/relayer/blob/main/docs/advanced_usage.md#monitoring for list of catagories. 'Tx Failure' is the catch-all category",
		}, txFailureLabels),
	}
}
