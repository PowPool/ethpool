package proxy

import (
	"github.com/MiningPool0826/ethpool/api"
	"github.com/MiningPool0826/ethpool/payouts"
	"github.com/MiningPool0826/ethpool/policy"
	"github.com/MiningPool0826/ethpool/storage"
)

type Config struct {
	Name                      string        `json:"-"`
	LocalIP                   string        `json:"-"`
	Log                       Log           `json:"log"`
	Cluster                   []ClusterNode `json:"cluster"`
	Proxy                     Proxy         `json:"proxy"`
	Api                       api.ApiConfig `json:"api"`
	Upstream                  []Upstream    `json:"upstream"`
	UpstreamCheckInterval     string        `json:"upstreamCheckInterval"`
	UpstreamCoinBaseEncrypted string        `json:"upstreamCoinBaseEncrypted"`
	UpstreamCoinBase          string        `json:"-"`

	Threads int `json:"threads"`

	Coin          string                 `json:"coin"`
	Redis         storage.Config         `json:"redis"`
	RedisFailover storage.ConfigFailover `json:"redisFailover"`

	BlockUnlocker payouts.UnlockerConfig `json:"unlocker"`
	Payouts       payouts.PayoutsConfig  `json:"payouts"`

	NewrelicName    string `json:"newrelicName"`
	NewrelicKey     string `json:"newrelicKey"`
	NewrelicVerbose bool   `json:"newrelicVerbose"`
	NewrelicEnabled bool   `json:"newrelicEnabled"`
}

type Proxy struct {
	Enabled              bool   `json:"enabled"`
	Listen               string `json:"listen"`
	LimitHeadersSize     int    `json:"limitHeadersSize"`
	LimitBodySize        int64  `json:"limitBodySize"`
	BehindReverseProxy   bool   `json:"behindReverseProxy"`
	BlockRefreshInterval string `json:"blockRefreshInterval"`
	Difficulty           int64  `json:"difficulty"`
	StateUpdateInterval  string `json:"stateUpdateInterval"`
	HashrateExpiration   string `json:"hashrateExpiration"`

	Policy policy.Config `json:"policy"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum        Stratum    `json:"stratum"`
	StratumVIP     StratumVIP `json:"stratumVIP"`
	StratumTls     StratumTls `json:"stratumTls"`
	StratumMaxConn int        `json:"stratumMaxConn"`

	DiffAdjust DiffAdjust `json:"diffAdjust"`
}

type Stratum struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Timeout string `json:"timeout"`
}

type StratumVIP struct {
	Enabled   bool   `json:"enabled"`
	PortRange string `json:"portRange"`
	Timeout   string `json:"timeout"`
}

type StratumTls struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Timeout string `json:"timeout"`
	TlsCert string `json:"tlsCert"`
	TlsKey  string `json:"tlsKey"`
}

type DiffAdjust struct {
	Enabled          bool   `json:"enabled"`
	AdjustInv        string `json:"adjustInv"`
	ExpectShareCount int64  `json:"expectShareCount"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Timeout string `json:"timeout"`
}

type ClusterNode struct {
	NodeName string `json:"nodeName"`
	NodeIp   string `json:"nodeIp"`
}

type Log struct {
	LogSetLevel int `json:"logSetLevel"`
}
