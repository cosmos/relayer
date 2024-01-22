package processor

import (
	"time"

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
	BlockQueryFailure     *prometheus.CounterVec
	ClientExpiration      *prometheus.GaugeVec
	ClientTrustingPeriod  *prometheus.GaugeVec
	UnrelayedPackets      *prometheus.GaugeVec
	UnrelayedAcks         *prometheus.GaugeVec
}

func (m *PrometheusMetrics) AddPacketsObserved(pathName, chain, channel, port, eventType string, count int) {
	m.PacketObservedCounter.WithLabelValues(pathName, chain, channel, port, eventType).Add(float64(count))
}

func (m *PrometheusMetrics) IncPacketsRelayed(pathName, chain, channel, port, eventType string) {
	m.PacketRelayedCounter.WithLabelValues(pathName, chain, channel, port, eventType).Inc()
}

func (m *PrometheusMetrics) SetLatestHeight(chain string, height int64) {
	m.LatestHeightGauge.WithLabelValues(chain).Set(float64(height))
}

func (m *PrometheusMetrics) SetWalletBalance(chain, gasPrice, key, address, denom string, balance float64) {
	m.WalletBalance.WithLabelValues(chain, gasPrice, key, address, denom).Set(balance)
}

func (m *PrometheusMetrics) SetFeesSpent(chain, gasPrice, key, address, denom string, amount float64) {
	m.FeesSpent.WithLabelValues(chain, gasPrice, key, address, denom).Set(amount)
}

func (m *PrometheusMetrics) SetClientExpiration(pathName, chain, clientID, trustingPeriod string, timeToExpiration time.Duration) {
	m.ClientExpiration.WithLabelValues(pathName, chain, clientID, trustingPeriod).Set(timeToExpiration.Seconds())
}

func (m *PrometheusMetrics) SetClientTrustingPeriod(pathName, chain, clientID string, trustingPeriod time.Duration) {
	m.ClientTrustingPeriod.WithLabelValues(pathName, chain, clientID).Set(trustingPeriod.Abs().Seconds())
}

func (m *PrometheusMetrics) IncBlockQueryFailure(chain, err string) {
	m.BlockQueryFailure.WithLabelValues(chain, err).Inc()
}

func (m *PrometheusMetrics) IncTxFailure(pathName, chain, errDesc string) {
	m.TxFailureError.WithLabelValues(pathName, chain, errDesc).Inc()
}

func (m *PrometheusMetrics) SetUnrelayedPackets(pathName, srcChain, destChain, srcChannel, destChannel string, unrelayedPackets int) {
	m.UnrelayedPackets.WithLabelValues(pathName, srcChain, destChain, srcChannel, destChannel).Set(float64(unrelayedPackets))
}

func (m *PrometheusMetrics) SetUnrelayedAcks(pathName, srcChain, destChain, srcChannel, destChannel string, UnrelayedAcks int) {
	m.UnrelayedAcks.WithLabelValues(pathName, srcChain, destChain, srcChannel, destChannel).Set(float64(UnrelayedAcks))
}

func NewPrometheusMetrics() *PrometheusMetrics {
	packetLabels := []string{"path_name", "chain", "channel", "port", "type"}
	heightLabels := []string{"chain"}
	txFailureLabels := []string{"path_name", "chain", "cause"}
	blockQueryFailureLabels := []string{"chain", "type"}
	walletLabels := []string{"chain", "gas_price", "key", "address", "denom"}
	clientExpirationLables := []string{"path_name", "chain", "client_id", "trusting_period"}
	clientTrustingPeriodLables := []string{"path_name", "chain", "client_id"}
	unrelayedSeqsLabels := []string{"path_name", "src_chain", "dest_chain", "src_channel", "dest_channel"}
	registry := prometheus.NewRegistry()
	registerer := promauto.With(registry)
	return &PrometheusMetrics{
		Registry: registry,
		PacketObservedCounter: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_observed_packets_total",
			Help: "The total number of observed packets",
		}, packetLabels),
		PacketRelayedCounter: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_relayed_packets_total",
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
			Name: "cosmos_relayer_tx_errors_total",
			Help: "The total number of tx failures broken up into categories. See https://github.com/cosmos/relayer/blob/main/docs/advanced_usage.md#monitoring for list of catagories. 'Tx Failure' is the catch-all category",
		}, txFailureLabels),
		BlockQueryFailure: registerer.NewCounterVec(prometheus.CounterOpts{
			Name: "cosmos_relayer_block_query_errors_total",
			Help: "The total number of block query failures. The failures are separated into two categories: 'RPC Client' and 'IBC Header'",
		}, blockQueryFailureLabels),
		ClientExpiration: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_client_expiration_seconds",
			Help: "Seconds until the client expires",
		}, clientExpirationLables),
		ClientTrustingPeriod: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_client_trusting_period_seconds",
			Help: "The trusting period (in seconds) of the client",
		}, clientTrustingPeriodLables),
		UnrelayedPackets: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_unrelayed_packets",
			Help: "Current number of unrelayed packets on both the source and destination chains for a specific path and channel",
		}, unrelayedSeqsLabels),
		UnrelayedAcks: registerer.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cosmos_relayer_unrelayed_acks",
			Help: "Current number of unrelayed acknowledgements on both the source and destination chains for a specific path and channel",
		}, unrelayedSeqsLabels),
	}
}
