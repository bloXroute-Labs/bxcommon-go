package sdnsdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/bloXroute-Labs/bxcommon-go/cache"
	"github.com/bloXroute-Labs/bxcommon-go/cert"
	"github.com/bloXroute-Labs/bxcommon-go/clock"
	"github.com/bloXroute-Labs/bxcommon-go/syncmap"
	"github.com/bloXroute-Labs/bxcommon-go/types"
	"github.com/gorilla/mux"

	"github.com/bloXroute-Labs/bxcommon-go/sdnsdk/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// RemoteInitiatedPort is a special constant used to indicate connections initiated from the remote
	RemoteInitiatedPort = 0

	// LocalInitiatedPort is a special constant used to indicate connections initiated locally
	LocalInitiatedPort = 0

	// PriorityQueueInterval represents the minimum amount of time that must be elapsed between non highest priority messages intended to sent along the connection
	PriorityQueueInterval = 500 * time.Microsecond

	// PrivateCert is a sample relay proxy private cert
	PrivateCert = "-----BEGIN CERTIFICATE-----\nMIICzTCCAlOgAwIBAgIUNmdelk/FEuWx4ZnHSj8DWwslUBowCgYIKoZIzj0EAwIw\nbzELMAkGA1UEBhMCVVMxETAPBgNVBAgMCElsbGlub2lzMREwDwYDVQQHDAhFdmFu\nc3RvbjEbMBkGA1UECgwSYmxvWHJvdXRlIExBQlMgSW5jMR0wGwYDVQQDDBRibG9Y\ncm91dGUudGVzdG5ldC5DQTAeFw0yMzEwMjMyMTIxNTVaFw0zMzEwMTMwMDAwMDBa\nMGwxCzAJBgNVBAYTAiAgMQswCQYDVQQIEwIgIDELMAkGA1UEBxMCICAxGzAZBgNV\nBAoTEmJsb1hyb3V0ZSBMQUJTIEluYzEmMCQGA1UEAwwdYmxvWHJvdXRlLnRlc3Ru\nZXQucmVsYXlfcHJveHkwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATytPpc10LjWK5X\na+Cd19jNoC2O7K/pU1b8itOF9viDAe3iExyk6ChjMzsUcmj08KrYqAhp1dYK/S5H\nyUcZT89ic+KBrL6UZ4qoGpb51leLGoYjVGxgxHRjmekx8yih+8ejgbIwga8wHwYD\nVR0jBBgwFoAUkN1jVx3ISR8ATM29MLuIZ0K0dJEwDAYDVR0TAQH/BAIwADAOBgNV\nHQ8BAf8EBAMCBeAwFAYFPoJNolwEC1JFTEFZX1BST1hZMC0GBT6CTaJdBCQzZTZj\nMWFiZS1mYzdjLTRlNTMtOWNiZS1hNTUzYjkzMGRkMzAwFwYFPoJNol4EDmJsb1hy\nb3V0ZSBMQUJTMBAGBT6CTaJfBAdnZW5lcmFsMAoGCCqGSM49BAMCA2gAMGUCMQD+\ntUzg7JhIA4ZTZiglRAgOtC8JpzwFYl6oLrUXjKknIvmRpUUzUAO6EZ4fJThAYkoC\nMHqtXZtvFqJUuJObqdxKXqhu0+mkvLJvBSG+29dQP3cJoHQwsS7gVV4vlH7FYg1a\n4Q==\n-----END CERTIFICATE-----\n"

	// PrivateKey is a sample relay proxy private key
	PrivateKey = "-----BEGIN EC PRIVATE KEY-----\nMIGkAgEBBDB5RCTQ8rvYyoixxC/QD9wQpsvhh1XZH1p/1zcc4gbkypEUcbUDHAu2\nvnsHevQZTiSgBwYFK4EEACKhZANiAATytPpc10LjWK5Xa+Cd19jNoC2O7K/pU1b8\nitOF9viDAe3iExyk6ChjMzsUcmj08KrYqAhp1dYK/S5HyUcZT89ic+KBrL6UZ4qo\nGpb51leLGoYjVGxgxHRjmekx8yih+8c=\n-----END EC PRIVATE KEY-----\n"

	// SSLTestPath is the path intermediary SSL certificates are created in for test cases
	SSLTestPath = "ssl"

	// CACertFolder is the folder at which intermediary CA certificates are created in for test cases
	CACertFolder = "ssl/ca"

	// CACertPath is the path of the intermediary CA certificate
	CACertPath = "ssl/ca/ca_cert.pem"

	// RegistrationCert is a sample relay proxy registration cert
	RegistrationCert = "-----BEGIN CERTIFICATE-----\nMIICtTCCAjygAwIBAgIUVdro/ObWSYNBWo3CoyHcc4/PtHswCgYIKoZIzj0EAwIw\nbzELMAkGA1UEBhMCVVMxETAPBgNVBAgMCElsbGlub2lzMREwDwYDVQQHDAhFdmFu\nc3RvbjEbMBkGA1UECgwSYmxvWHJvdXRlIExBQlMgSW5jMR0wGwYDVQQDDBRibG9Y\ncm91dGUudGVzdG5ldC5DQTAeFw0yMTEyMDYwODQ1MDlaFw0zMTEyMDQwMDAwMDBa\nMGwxCzAJBgNVBAYTAiAgMQswCQYDVQQIDAIgIDELMAkGA1UEBwwCICAxGzAZBgNV\nBAoMEmJsb1hyb3V0ZSBMQUJTIEluYzEmMCQGA1UEAwwdYmxvWHJvdXRlLnRlc3Ru\nZXQucmVsYXlfcHJveHkwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARdRXcc5IZ9lKus\nnQh9DWBawedHtpxy8VfztWBvB64oNMMV9H8vb2XRNHhwUMo3SG/SFh+y/fKGaBSQ\nsuZMreLnP4SeQDR1PfibfyhmkwCQyDKQ9jhyKADm0K8X1wBLlQyjgZswgZgwHwYD\nVR0jBBgwFoAUkN1jVx3ISR8ATM29MLuIZ0K0dJEwKAYDVR0RBCEwH4IdYmxvWHJv\ndXRlLnRlc3RuZXQucmVsYXlfcHJveHkwDAYDVR0TAQH/BAIwADAOBgNVHQ8BAf8E\nBAMCBeAwFAYFPoJNolwEC1JFTEFZX1BST1hZMBcGBT6CTaJeBA5ibG9Ycm91dGUg\nTEFCUzAKBggqhkjOPQQDAgNnADBkAjAVVCopO1KqhH4NUlBATOQSgGK0Kp2S2f3O\nmm4UinZcb8gXEQx4x2porpgNsRYtYjsCMAp6DMQxiIuLz1syPEvTByR4wmtNTlRf\nsbfs0ShKmVhQSBIbnIKerpX+Pl1YYoSeqA==\n-----END CERTIFICATE-----\n"

	// RegistrationKey is a sample relay proxy registration key
	RegistrationKey = "-----BEGIN EC PRIVATE KEY-----\nMIGkAgEBBDDAvjme5SkD0OrhdntQurzuvjg8S620n/q2dWEd4qwTyrscWZ5eHgQM\neZPK5gp+wv6gBwYFK4EEACKhZANiAARdRXcc5IZ9lKusnQh9DWBawedHtpxy8Vfz\ntWBvB64oNMMV9H8vb2XRNHhwUMo3SG/SFh+y/fKGaBSQsuZMreLnP4SeQDR1Pfib\nfyhmkwCQyDKQ9jhyKADm0K8X1wBLlQw=\n-----END EC PRIVATE KEY-----\n"

	// CACert is a sample CA certificate
	CACert = "-----BEGIN CERTIFICATE-----\nMIICkDCCAhWgAwIBAgIUQGhfIhpMxaSE0r/jjB35VclkTYgwCgYIKoZIzj0EAwIw\nazELMAkGA1UEBhMCVVMxETAPBgNVBAgMCElsbGlub2lzMREwDwYDVQQHDAhFdmFu\nc3RvbjEbMBkGA1UECgwSYmxvWHJvdXRlIExBQlMgSW5jMRkwFwYDVQQDDBBibG9Y\ncm91dGUuZGV2LkNBMB4XDTIwMTEwOTIyMzY0NloXDTQ4MDMyNzAwMDAwMFowazEL\nMAkGA1UEBhMCVVMxETAPBgNVBAgMCElsbGlub2lzMREwDwYDVQQHDAhFdmFuc3Rv\nbjEbMBkGA1UECgwSYmxvWHJvdXRlIExBQlMgSW5jMRkwFwYDVQQDDBBibG9Ycm91\ndGUuZGV2LkNBMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEd5IkD4wqWVGbq0jCehjr\nyvOEkbD5vCYIes4UwRH9Z7YSfeKQrOSaW1LUzHCYvqOTLfgHoAC0lS1v9+OlT/LZ\nboey8h4TZwLZ9zLbHzyVcTww01ZFNeBndQ2EmdaSYdqKo3oweDAfBgNVHSMEGDAW\ngBQHnCDGbOz7WL0VB9Kd0YdQSpsnQzAdBgNVHQ4EFgQUB5wgxmzs+1i9FQfSndGH\nUEqbJ0MwGwYDVR0RBBQwEoIQYmxvWHJvdXRlLmRldi5DQTAMBgNVHRMEBTADAQH/\nMAsGA1UdDwQEAwIB/jAKBggqhkjOPQQDAgNpADBmAjEAo6kPPChOytP961lFjKFb\n+zfEPm6sHtBxmgeDMhQwqb1erIIsYfU6zVaA82g9REHvAjEAoLfzcjEq91/Jlcmn\nCSgJY3JUPIocBek+o9cKczwz1ZDuzGscMOF0J4fpTwAyJOUP\n-----END CERTIFICATE-----"
)

type handlerArgs struct {
	method  string
	pattern string
	handler func(w http.ResponseWriter, r *http.Request)
}

func cleanupFiles() {
	_ = os.Remove(blockchainNetworksCacheFileName)
	_ = os.Remove(blockchainNetworkCacheFileName)
	_ = os.Remove(nodeModelCacheFileName)
	_ = os.Remove(potentialRelaysFileName)
	_ = os.Remove(accountModelsFileName)
}

func testSDNHTTP() realSDNHTTP {
	return realSDNHTTP{
		relays: message.Peers{
			{IP: "1.1.1.1", Port: 1},
			{IP: "2.2.2.2", Port: 2},
		},
		getPingLatencies: func(peers message.Peers) []nodeLatencyInfo {
			var nlis []nodeLatencyInfo
			for _, peer := range peers {
				nlis = append(nlis, nodeLatencyInfo{
					IP:   peer.IP,
					Port: peer.Port,
				})
			}
			return nlis
		},
		nodeModel: &message.NodeModel{
			NodeID:               "35299c61-55ad-4565-85a3-0cd985953fac",
			BlockchainNetworkNum: LocalInitiatedPort,
		},
	}
}

func TestRegister_BlockchainNetworkNumberUpdated(t *testing.T) {
	testTable := []struct {
		nodeModel         message.NodeModel
		networkNumber     types.NetworkNum
		jsonRespNodeModel string
	}{
		{
			nodeModel:         message.NodeModel{NodeID: "35299c61-55ad-4565-85a3-0cd985953fac", ExternalIP: "11.113.164.111", Protocol: "Ethereum", Network: "Mainnet"},
			networkNumber:     5,
			jsonRespNodeModel: `{"node_type": "EXTERNAL_GATEWAY", "external_port": 1801, "non_ssl_port": 0, "external_ip": "11.113.164.111", "online": false, "sdn_connection_alive": false, "network": "Mainnet", "protocol": "Ethereum", "node_id": "35299c61-55ad-4565-85a3-0cd985953fac", "sid_start": null, "sid_end": null, "next_sid_start": null, "next_sid_end": null, "sid_expire_time": 259200, "last_pong_time": 0.0, "is_gateway_miner": false, "is_internal_gateway": false, "source_version": "2.108.3.0", "protocol_version": 24, "blockchain_network_num": 10, "blockchain_ip": "52.221.255.145", "blockchain_port": 3000, "hostname": "MacBook-Pro.attlocal.net", "sdn_id": "1e5c6fda-f775-49d4-bd11-287526c07f0f", "os_version": "darwin", "continent": "NA", "split_relays": true, "country": "United States", "region": null, "idx": null, "has_fully_updated_tx_service": false, "node_start_time": "2021-12-30 12:25:26-0500", "node_public_key": "8720705f39ea1ff2eabb38d424136d545005173943062f92cf9cd1f212392c1c0a2ee7ff44ecb84df17140fa7feeee939f0a2b6b3efd3ae5fda72966d4fc0ac1", "baseline_route_redundancy": 0, "baseline_source_redundancy": 0, "private_ip": null, "csr": "", "cert": null, "platform_provider": null, "account_id": "34ff3406-cc74-4cc7-9d9a-9ef8bdda59b1", "latest_source_version": null, "should_update_source_version": false, "assigning_short_ids": false, "node_privileges": "general", "first_seen_time": "1640720639.40804", "is_docker": true, "using_private_ip_connection": false, "private_node": false, "relay_type": ""}`,
		},
		{
			nodeModel:         message.NodeModel{NodeID: "35299c61-55ad-4565-85a3-0cd985953fac", ExternalIP: "11.113.164.112", Protocol: "Ethereum", Network: "Testnet"},
			networkNumber:     23,
			jsonRespNodeModel: `{"node_type": "EXTERNAL_GATEWAY", "external_port": 1801, "non_ssl_port": 0, "external_ip": "11.113.164.112", "online": false, "sdn_connection_alive": false, "network": "Testnet", "protocol": "Ethereum", "node_id": "35299c61-55ad-4565-85a3-0cd985953fac", "sid_start": null, "sid_end": null, "next_sid_start": null, "next_sid_end": null, "sid_expire_time": 259200, "last_pong_time": 0.0, "is_gateway_miner": false, "is_internal_gateway": false, "source_version": "2.108.3.0", "protocol_version": 24, "blockchain_network_num": 10, "blockchain_ip": "52.221.255.145", "blockchain_port": 3000, "hostname": "MacBook-Pro.attlocal.net", "sdn_id": "1e5c6fda-f775-49d4-bd11-287526c07f0f", "os_version": "darwin", "continent": "NA", "split_relays": true, "country": "United States", "region": null, "idx": null, "has_fully_updated_tx_service": false, "node_start_time": "2021-12-30 12:25:26-0500", "node_public_key": "8720705f39ea1ff2eabb38d424136d545005173943062f92cf9cd1f212392c1c0a2ee7ff44ecb84df17140fa7feeee939f0a2b6b3efd3ae5fda72966d4fc0ac1", "baseline_route_redundancy": 0, "baseline_source_redundancy": 0, "private_ip": null, "csr": "", "cert": null, "platform_provider": null, "account_id": "34ff3406-cc74-4cc7-9d9a-9ef8bdda59b1", "latest_source_version": null, "should_update_source_version": false, "assigning_short_ids": false, "node_privileges": "general", "first_seen_time": "1640720639.40804", "is_docker": true, "using_private_ip_connection": false, "private_node": false, "relay_type": ""}`,
		},
	}

	for _, testCase := range testTable {
		t.Run(fmt.Sprint(testCase), func(t *testing.T) {
			handler1 := mockNodesServer(t, testCase.nodeModel.NodeID, testCase.nodeModel.ExternalPort, testCase.nodeModel.ExternalIP, testCase.nodeModel.Protocol, testCase.nodeModel.Network, testCase.networkNumber, "")

			handler2, _ := mockNodeModelServer(t, testCase.jsonRespNodeModel)

			var m []handlerArgs
			m = append(m, handlerArgs{method: "POST", pattern: "/nodes", handler: handler1})
			m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeId}", handler: handler2})

			server := mockRouter(m)
			defer func() {
				server.Close()
			}()
			testCerts := SetupTestCerts()
			s := realSDNHTTP{
				sdnURL:   server.URL,
				sslCerts: &testCerts,
				nodeModel: &message.NodeModel{
					Protocol: testCase.nodeModel.Protocol,
					Network:  testCase.nodeModel.Network,
				},
			}

			err := s.Register()

			assert.NoError(t, err)
			assert.Equal(t, testCase.nodeModel.Network, s.nodeModel.Network)
			assert.Equal(t, testCase.nodeModel.Protocol, s.nodeModel.Protocol)
			assert.Equal(t, testCase.networkNumber, s.nodeModel.BlockchainNetworkNum)
		})
	}
}

func TestDirectRelayConnections_IfPingOver40MSLogsWarning(t *testing.T) {
	jsonRespRelays := `[{"ip":"8.208.101.30", "port":1809}, {"ip":"47.90.133.153", "port":1809}]`
	nodeModel := message.NodeModel{
		NodeID:     "35299c61-55ad-4565-85a3-0cd985953fac",
		ExternalIP: "11.113.164.111",
		Protocol:   "Ethereum",
		Network:    "Mainnet",
	}
	IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}

	testTable := []struct {
		name      string
		latencies []nodeLatencyInfo
		ip        string
		port      int64
	}{
		{
			"Latency 5",
			[]nodeLatencyInfo{{Latency: 5, IP: "8.208.101.30", Port: 1809}},
			"8.208.101.30",
			1809,
		},
		{
			"Latency 20",
			[]nodeLatencyInfo{{Latency: 20, IP: "1.1.1.1", Port: 41}},
			"1.1.1.1",
			41,
		},
		{
			"Latency 5, 41",
			[]nodeLatencyInfo{{Latency: 5, IP: "1.1.1.2", Port: 42}, {Latency: 41, IP: "1.1.1.3", Port: 43}},
			"1.1.1.2",
			42,
		},
		{
			"Latency 41",
			[]nodeLatencyInfo{{Latency: 41, IP: "1.1.1.3", Port: 43}},
			"1.1.1.3",
			43,
		},
		{
			"Latency 1000, 2000",
			[]nodeLatencyInfo{{Latency: 1000, IP: "1.1.1.4", Port: 44}, {Latency: 2000, IP: "1.1.1.5", Port: 45}},
			"1.1.1.4",
			44,
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			defer cleanupFiles()
			sslCerts := cert.SSLCerts{}
			handler3, _ := mockRelaysServer(t, jsonRespRelays)
			var m []handlerArgs
			m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeID}/{networkNum}/potential-relays", handler: handler3})

			server := mockRouter(m)
			defer func() {
				server.Close()
			}()

			sdn := NewSDNHTTP(&sslCerts, server.URL, nodeModel, "").(*realSDNHTTP)

			getPingLatenciesFunction := func(peers message.Peers) []nodeLatencyInfo {
				return testCase.latencies
			}
			sdn.getPingLatencies = getPingLatenciesFunction

			autoRelayInstructions := make(chan RelayInstruction)
			err := sdn.DirectRelayConnections("auto", 1, autoRelayInstructions, syncmap.NewStringMapOf[types.RelayInfo]())
			assert.NoError(t, err)
			var selectedRelay RelayInstruction
			select {
			case selectedRelay = <-autoRelayInstructions:
			case <-time.After(1 * time.Second):
				t.Fatalf("Timeout: No relay instruction received within 1 seconds")
			}
			assert.Equal(t, testCase.ip, selectedRelay.IP)
			assert.Equal(t, testCase.port, selectedRelay.Port)
		})
	}
}

func TestDirectRelayConnections_IncorrectArgs(t *testing.T) {
	testTable := []struct {
		name          string
		relaysString  string
		expectedError error
	}{
		{
			name:          "no args",
			expectedError: fmt.Errorf("no --relays/relay-ip arguments were provided"),
		},
		{
			name:          "empty string",
			relaysString:  " ",
			expectedError: fmt.Errorf("argument to --relays/relay-ip is empty or has an extra comma"),
		},
		{
			name:          "incorrect host",
			relaysString:  "1:2:3",
			expectedError: fmt.Errorf("relay from --relays/relay-ip was given in the incorrect format '1:2:3', should be IP:Port"),
		},
		{
			name:          "no relay before comma",
			relaysString:  ",127.0.0.1",
			expectedError: fmt.Errorf("argument to --relays/relay-ip is empty or has an extra comma"),
		},
		{
			name:          "no relay after comma",
			relaysString:  "127.0.0.1,",
			expectedError: fmt.Errorf("argument to --relays/relay-ip is empty or has an extra comma"),
		},
		{
			name:          "space after comma",
			relaysString:  "127.0.0.1, ",
			expectedError: fmt.Errorf("argument to --relays/relay-ip is empty or has an extra comma"),
		},
	}

	s := testSDNHTTP()

	for _, testCase := range testTable {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			err := s.DirectRelayConnections(testCase.relaysString, 2, make(chan RelayInstruction), syncmap.NewStringMapOf[types.RelayInfo]())
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}

func TestSDNHTTP_GetAutoConnectedRelays(t *testing.T) {
	ignoredRelays := syncmap.NewStringMapOf[types.RelayInfo]()
	// static and connected should not return as auto relay
	ignoredRelays.Store("1", types.RelayInfo{IsConnected: true, IsStatic: true})
	ignoredRelays.Store("2", types.RelayInfo{IsConnected: true})
	ignoredRelays.Store("3", types.RelayInfo{IsConnected: false})
	ignoredRelays.Store("4", types.RelayInfo{IsConnected: true})
	s := testSDNHTTP()
	autoConnectedRelays := s.getAutoConnectedRelays(ignoredRelays)
	assert.Len(t, autoConnectedRelays, 2)
	assert.True(t, autoConnectedRelays["2"].IsConnected)
	assert.True(t, autoConnectedRelays["4"].IsConnected)
}

func TestFindFastestAvailableRelays(t *testing.T) {
	s := testSDNHTTP()
	// in real sdn code the list come sorted
	latencies := []nodeLatencyInfo{{Latency: 3, IP: "4", Port: 1809}, {Latency: 8, IP: "3", Port: 1809}, {Latency: 10, IP: "5", Port: 1809}, {Latency: 15, IP: "1", Port: 1809}, {Latency: 26, IP: "2", Port: 1809}}
	autoRelay := make(map[string]types.RelayInfo)
	autoRelay["1"] = types.RelayInfo{IsConnected: true}
	autoRelay["2"] = types.RelayInfo{IsConnected: true}
	autoRelay["3"] = types.RelayInfo{IsConnected: true}
	fastestAvailableRelays := s.findFastestAvailableRelays(latencies, autoRelay)
	assert.Len(t, fastestAvailableRelays, 2)
	assert.Equal(t, fastestAvailableRelays[0].IP, "4")
	assert.Equal(t, fastestAvailableRelays[1].IP, "5")
}

func TestFindRelaysToSwitch(t *testing.T) {
	s := testSDNHTTP()
	autoRelay := make(map[string]types.RelayInfo)
	autoRelay["1"] = types.RelayInfo{IsConnected: true, Latency: 15, Port: 1809}
	autoRelay["2"] = types.RelayInfo{IsConnected: true, Latency: 26, Port: 1809}
	autoRelay["3"] = types.RelayInfo{IsConnected: true, Latency: 8, Port: 1809}
	fastestAvailableRelays := []nodeLatencyInfo{{Latency: 3, IP: "4", Port: 1809}, {Latency: 10, IP: "5", Port: 1809}}
	relaysToSwitch := s.findRelaysToSwitch(autoRelay, fastestAvailableRelays)
	// relays 1 and 2 can be switched (relay 4 is fastest and not connected) but 3 is not more with 10 ms
	assert.Len(t, relaysToSwitch, 2)
	// relay 1 have only relay 4 that faster more than 10 ms
	key1 := relayToSwitch{ip: "1", port: 1809}
	assert.Contains(t, relaysToSwitch, key1)
	assert.Equal(t, relaysToSwitch[key1][0].IP, "4")

	// relay 2 have both relays that faster more them 10 ms
	key2 := relayToSwitch{ip: "2", port: 1809}
	assert.Contains(t, relaysToSwitch, key2)
	assert.Equal(t, relaysToSwitch[key2][0].IP, "4")
	assert.Equal(t, relaysToSwitch[key2][1].IP, "5")

}

func TestDirectRelayConnections_RelayLimit2(t *testing.T) {
	jsonRespRelays := `[{"ip":"1.1.1.1", "port":1809}, {"ip":"2.2.2.2", "port":1809}]`
	latencies := []nodeLatencyInfo{{Latency: 5, IP: "1.1.1.1", Port: 1809}, {Latency: 6, IP: "2.2.2.2", Port: 1809}}
	nodeModel := message.NodeModel{
		NodeID:     "35299c61-55ad-4565-85a3-0cd985953fac",
		ExternalIP: "11.113.164.111",
		Protocol:   "Ethereum",
		Network:    "Mainnet",
	}
	IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}

	testTable := []struct {
		name           string
		relaysString   string
		expectedRelays relayMap
		expectedError  error
	}{
		{
			name:           "one auto",
			relaysString:   "auto",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "two autos",
			relaysString:   "auto, auto",
			expectedRelays: relayMap{"1.1.1.1": 1809, "2.2.2.2": 1809},
		},
		{
			name:           "an auto and a relay",
			relaysString:   "auto, 1.1.1.1",
			expectedRelays: relayMap{"1.1.1.1": 1809, "2.2.2.2": 1809},
		},
		{
			name:           "one relay",
			relaysString:   "1.1.1.1",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "two relays",
			relaysString:   "1.1.1.1, 2.2.2.2",
			expectedRelays: relayMap{"1.1.1.1": 1809, "2.2.2.2": 1809},
		},
		{
			name:           "two relays, only one has port",
			relaysString:   "1.1.1.1:34, 2.2.2.2",
			expectedRelays: relayMap{"1.1.1.1": 34, "2.2.2.2": 1809},
		},
		{
			name:           "two relays, both have ports",
			relaysString:   "1.1.1.1:34, 2.2.2.2:56",
			expectedRelays: relayMap{"1.1.1.1": 34, "2.2.2.2": 56},
		},
		{
			name:           "three relays",
			relaysString:   "4.4.4.4, 2.2.2.2:22, 1.1.1.1",
			expectedRelays: relayMap{"4.4.4.4": 1809, "2.2.2.2": 22},
		},
		{
			name:          "incorrect port",
			relaysString:  "1.1.1.1, 2.2.2.2:abc",
			expectedError: fmt.Errorf("port provided abc is not valid - strconv.Atoi: parsing \"abc\": invalid syntax"),
		},
		{
			name:          "incorrect host",
			relaysString:  "1:1:1, 1.1.1.1",
			expectedError: fmt.Errorf("relay from --relays/relay-ip was given in the incorrect format '1:1:1', should be IP:Port"),
		},
		{
			name:           "duplicate relay ips",
			relaysString:   "1.1.1.1, 1.1.1.1:34",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "duplicate relay ips #2",
			relaysString:   "1.1.1.1:1, 1.1.1.1:2, 2.2.2.2:3, 2.2.2.2:4",
			expectedRelays: relayMap{"1.1.1.1": 1, "2.2.2.2": 3},
		},
		{
			name:           "duplicate relay ips with auto after",
			relaysString:   "1.1.1.1, 1.1.1.1:2, auto",
			expectedRelays: relayMap{"1.1.1.1": 1809, "2.2.2.2": 1809},
		},
		{
			name:           "auto relay doesn't overlap with configured relay",
			relaysString:   "auto, 1.1.1.1",
			expectedRelays: relayMap{"1.1.1.1": 1809, "2.2.2.2": 1809},
		},
		{
			name:           "auto relay doesn't overlap with configured relay #2",
			relaysString:   "2.2.2.2, auto, 1.1.1.1",
			expectedRelays: relayMap{"2.2.2.2": 1809, "1.1.1.1": 1809},
		},
	}

	for _, testCase := range testTable {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			defer cleanupFiles()

			sslCerts := cert.SSLCerts{}
			handler3, _ := mockRelaysServer(t, jsonRespRelays)
			var m []handlerArgs
			m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeID}/{networkNum}/potential-relays", handler: handler3})

			server := mockRouter(m)
			defer func() {
				server.Close()
			}()

			sdn := NewSDNHTTP(&sslCerts, server.URL, nodeModel, "").(*realSDNHTTP)
			getPingLatenciesFunction := func(peers message.Peers) []nodeLatencyInfo {
				return latencies
			}
			sdn.getPingLatencies = getPingLatenciesFunction

			expectedRelayCount := len(testCase.expectedRelays)
			relayInstructions := make(chan RelayInstruction, expectedRelayCount)

			err := sdn.DirectRelayConnections(testCase.relaysString, 2, relayInstructions, syncmap.NewStringMapOf[types.RelayInfo]())
			assert.Equal(t, testCase.expectedError, err)

			timer := time.NewTimer(1 * time.Second)

			for i := 0; i < expectedRelayCount; i++ {
				select {
				case <-timer.C:
					t.Fail()
					return
				case instruction := <-relayInstructions:
					r, ok := testCase.expectedRelays[instruction.IP]
					assert.True(t, ok, "received instruction for unexpected relay")
					assert.Equal(t, r, instruction.Port)
					delete(testCase.expectedRelays, instruction.IP)
				}
			}

			timer = time.NewTimer(time.Millisecond)
			select {
			case <-timer.C:
				break
			case <-relayInstructions:
				t.Fail()
			}

			assert.Equal(t, 0, len(testCase.expectedRelays))
		})
	}
}

func TestDirectRelayConnections_RelayLimit1(t *testing.T) {
	jsonRespRelays := `[{"ip":"1.1.1.1", "port":1809}, {"ip":"2.2.2.2", "port":1809}]`
	latencies := []nodeLatencyInfo{{Latency: 5, IP: "1.1.1.1", Port: 1809}, {Latency: 6, IP: "2.2.2.2", Port: 1809}}
	nodeModel := message.NodeModel{
		NodeID:     "35299c61-55ad-4565-85a3-0cd985953fac",
		ExternalIP: "11.113.164.111",
		Protocol:   "Ethereum",
		Network:    "Mainnet",
	}
	IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}

	testTable := []struct {
		name           string
		relaysString   string
		expectedRelays relayMap
		expectedError  error
	}{
		{
			name:           "one auto",
			relaysString:   "auto",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "two autos",
			relaysString:   "auto, auto",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "an auto and a relay",
			relaysString:   "auto, 1.1.1.1",
			expectedRelays: relayMap{"1.1.1.1": 1809},
		},
		{
			name:           "one relay",
			relaysString:   "2.2.2.2",
			expectedRelays: relayMap{"2.2.2.2": 1809},
		},
		{
			name:           "two relays",
			relaysString:   "3.3.3.3, 4.4.4.4",
			expectedRelays: relayMap{"3.3.3.3": 1809},
		},
		{
			name:           "two relays - duplicates",
			relaysString:   "3.3.3.3:14, 3.3.3.3:15",
			expectedRelays: relayMap{"3.3.3.3": 14},
		},
		{
			name:           "one relay with port",
			relaysString:   "1.1.1.1:34",
			expectedRelays: relayMap{"1.1.1.1": 34},
		},
		{
			name:           "incorrect port",
			relaysString:   "1.1.1.1:abc",
			expectedRelays: relayMap{},
			expectedError:  fmt.Errorf("port provided abc is not valid - strconv.Atoi: parsing \"abc\": invalid syntax"),
		},
		{
			name:           "incorrect host",
			relaysString:   "127.0.0.9999",
			expectedRelays: relayMap{},
			expectedError:  fmt.Errorf("host provided 127.0.0.9999 is not valid - lookup 127.0.0.9999: no such host"),
		},
		{
			name:           "incorrect host with port",
			relaysString:   "127.0.0.9999:1234",
			expectedRelays: relayMap{},
			expectedError:  fmt.Errorf("host provided 127.0.0.9999 is not valid - lookup 127.0.0.9999: no such host"),
		},
	}

	for _, testCase := range testTable {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			defer cleanupFiles()

			sslCerts := cert.SSLCerts{}
			handler3, _ := mockRelaysServer(t, jsonRespRelays)
			var m []handlerArgs
			m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeID}/{networkNum}/potential-relays", handler: handler3})

			server := mockRouter(m)
			defer func() {
				server.Close()
			}()

			sdn := NewSDNHTTP(&sslCerts, server.URL, nodeModel, "").(*realSDNHTTP)
			getPingLatenciesFunction := func(peers message.Peers) []nodeLatencyInfo {
				return latencies
			}
			sdn.getPingLatencies = getPingLatenciesFunction

			expectedRelayCount := len(testCase.expectedRelays)
			relayInstructions := make(chan RelayInstruction, expectedRelayCount)

			err := sdn.DirectRelayConnections(testCase.relaysString, 1, relayInstructions, syncmap.NewStringMapOf[types.RelayInfo]())
			assert.Equal(t, testCase.expectedError, err)

			for i := 0; i < expectedRelayCount; i++ {
				select {
				case <-clock.RealClock{}.Timer(time.Millisecond * 2).Alert():
					t.Fail()
					return
				case instruction := <-relayInstructions:
					r, ok := testCase.expectedRelays[instruction.IP]
					assert.True(t, ok, "received instruction for unexpected relay")
					assert.Equal(t, r, instruction.Port)
					delete(testCase.expectedRelays, instruction.IP)
				}
			}

			select {
			case <-clock.RealClock{}.Timer(time.Millisecond * 2).Alert():
				break
			case <-relayInstructions:
				t.Fail()
			}

			assert.Equal(t, 0, len(testCase.expectedRelays))
		})
	}
}

func TestDirectRelayConnections_UpdateAutoRelays(t *testing.T) {
	t.Skip()
	testTable := []struct {
		name                      string
		relaysArgument            string
		initialPingLatencies      []nodeLatencyInfo
		expectedInitialAutoRelays relayMap
		addPingLatencies          []nodeLatencyInfo
		expectedFinalAutoRelays   relayMap
	}{
		{
			name:           "two autos, both relays updated",
			relaysArgument: "auto, auto",
			initialPingLatencies: []nodeLatencyInfo{
				{IP: "10.10.10.10", Port: 10, Latency: 10},
				{IP: "11.11.11.11", Port: 11, Latency: 11},
			},
			expectedInitialAutoRelays: relayMap{
				"10.10.10.10": 10,
				"11.11.11.11": 11,
			},
			addPingLatencies: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
				{IP: "8.8.8.8", Port: 8, Latency: 8},
			},
			expectedFinalAutoRelays: relayMap{
				"7.7.7.7": 7,
				"8.8.8.8": 8,
			},
		},
		{
			name:           "two autos, one relay updated",
			relaysArgument: "auto, auto",
			initialPingLatencies: []nodeLatencyInfo{
				{IP: "10.10.10.10", Port: 10, Latency: 10},
				{IP: "11.11.11.11", Port: 11, Latency: 11},
			},
			expectedInitialAutoRelays: relayMap{
				"10.10.10.10": 10,
				"11.11.11.11": 11,
			},
			addPingLatencies: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
			},
			expectedFinalAutoRelays: relayMap{
				"7.7.7.7":     7,
				"10.10.10.10": 10,
			},
		},
		{
			name:           "one auto, relay updated",
			relaysArgument: "auto",
			initialPingLatencies: []nodeLatencyInfo{
				{IP: "10.10.10.10", Port: 10, Latency: 10},
				{IP: "11.11.11.11", Port: 11, Latency: 11},
			},
			expectedInitialAutoRelays: relayMap{
				"10.10.10.10": 10,
			},
			addPingLatencies: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
			},
			expectedFinalAutoRelays: relayMap{
				"7.7.7.7": 7,
			},
		},
		{
			name:                      "two autos, no ping latencies at beginning",
			relaysArgument:            "auto, auto",
			initialPingLatencies:      []nodeLatencyInfo{},
			expectedInitialAutoRelays: relayMap{},
			addPingLatencies: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
				{IP: "8.8.8.8", Port: 8, Latency: 8},
			},
			expectedFinalAutoRelays: relayMap{
				"7.7.7.7": 7,
				"8.8.8.8": 8,
			},
		},
		{
			name:           "two autos, not enough ping latencies at beginning",
			relaysArgument: "auto, auto",
			initialPingLatencies: []nodeLatencyInfo{
				{IP: "10.10.10.10", Port: 10, Latency: 10},
			},
			expectedInitialAutoRelays: relayMap{
				"10.10.10.10": 10,
				"":            0,
			},
			addPingLatencies: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
				{IP: "8.8.8.8", Port: 8, Latency: 8},
			},
			expectedFinalAutoRelays: relayMap{
				"7.7.7.7": 7,
				"8.8.8.8": 8,
			},
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			s := testSDNHTTP()
			s.getPingLatencies = func(peers message.Peers) []nodeLatencyInfo {
				return testCase.initialPingLatencies
			}

			relayInstructions := make(chan RelayInstruction)
			go func() {
				for {
					<-relayInstructions
				}
			}()

			err := s.DirectRelayConnections(testCase.relaysArgument, 2, relayInstructions, syncmap.NewStringMapOf[types.RelayInfo]())
			require.Nil(t, err)
			time.Sleep(time.Millisecond * 2)

			testCase.initialPingLatencies = append(testCase.addPingLatencies, testCase.initialPingLatencies...)
			time.Sleep(time.Millisecond * 5)
		})
	}
}

func TestDirectRelayConnections_UpdateAutoRelaysTwice(t *testing.T) {
	t.Skip()
	testTable := []struct {
		name                    string
		relaysArgument          string
		initialPingLatencies    []nodeLatencyInfo
		expectedAutoRelays1     relayMap
		addPingLatencies1       []nodeLatencyInfo
		expectedAutoRelays2     relayMap
		addPingLatencies2       []nodeLatencyInfo
		expectedFinalAutoRelays relayMap
	}{
		{
			name:           "two autos, both relays updated",
			relaysArgument: "auto, auto",
			initialPingLatencies: []nodeLatencyInfo{
				{IP: "10.10.10.10", Port: 10, Latency: 10},
				{IP: "11.11.11.11", Port: 11, Latency: 11},
			},
			expectedAutoRelays1: relayMap{
				"10.10.10.10": 10,
				"11.11.11.11": 11,
			},
			addPingLatencies1: []nodeLatencyInfo{
				{IP: "7.7.7.7", Port: 7, Latency: 7},
				{IP: "8.8.8.8", Port: 8, Latency: 8},
			},
			expectedAutoRelays2: relayMap{
				"7.7.7.7": 7,
				"8.8.8.8": 8,
			},
			addPingLatencies2: []nodeLatencyInfo{
				{IP: "6.6.6.6", Port: 6, Latency: 6},
			},
			expectedFinalAutoRelays: relayMap{
				"6.6.6.6": 6,
				"7.7.7.7": 7,
			},
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			s := testSDNHTTP()
			s.getPingLatencies = func(peers message.Peers) []nodeLatencyInfo {
				return testCase.initialPingLatencies
			}

			relayInstructions := make(chan RelayInstruction)
			go func() {
				for {
					<-relayInstructions
				}
			}()

			err := s.DirectRelayConnections(testCase.relaysArgument, 2, relayInstructions, syncmap.NewStringMapOf[types.RelayInfo]())
			require.Nil(t, err)
			time.Sleep(time.Millisecond * 2)

			testCase.initialPingLatencies = append(testCase.addPingLatencies1, testCase.initialPingLatencies...)
			time.Sleep(time.Millisecond * 5)

			testCase.initialPingLatencies = append(testCase.addPingLatencies2, testCase.initialPingLatencies...)
			time.Sleep(time.Millisecond * 5)
		})
	}
}

func TestSDNHTTP_CacheFiles_ServiceUnavailable_SDN_BlockchainNetworks(t *testing.T) {
	testCase := struct {
		nodeModel                  message.NodeModel
		networkNumber              types.NetworkNum
		jsonRespServiceUnavailable string
	}{
		nodeModel:                  message.NodeModel{ExternalIP: "172.0.0.1"},
		networkNumber:              5,
		jsonRespServiceUnavailable: `{"message": "503 Service Unavailable" }`,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		defer cleanupFiles()
		// using bad certificate so get/post to bxapi will fail
		sslCerts := cert.SSLCerts{}

		handler1 := mockServiceError(t, 503, testCase.jsonRespServiceUnavailable)
		var m []handlerArgs
		m = append(m, handlerArgs{method: "GET", pattern: "/blockchain-networks/{networkNum}", handler: handler1})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		// using bad sdn url so get/post to bxapi will fail
		sdn := NewSDNHTTP(&sslCerts, server.URL, testCase.nodeModel, "").(*realSDNHTTP)
		url := fmt.Sprintf("%v/blockchain-networks/%v", sdn.SDNURL(), testCase.networkNumber)

		networks := generateNetworks()
		// generate blockchainNetworks.json file which contains networks using UpdateCacheFile method
		writeToFile(t, networks, blockchainNetworksCacheFileName)

		// calling to httpWithCache -> tying to get blockchain networks from bxapi
		// bxapi is not responsive
		// -> trying to load the blockchain networks from cache file
		resp, err := sdn.httpWithCache(url, http.MethodGet, blockchainNetworksCacheFileName, nil)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		cachedNetwork := []*message.BlockchainNetwork{}
		assert.Nil(t, json.Unmarshal(resp, &cachedNetwork))
		assert.Equal(t, networks, cachedNetwork)
	})
}

func TestSDNHTTP_CacheFiles_ServiceUnavailable_SDN_Node(t *testing.T) {
	testCase := struct {
		nodeModel                  message.NodeModel
		jsonRespServiceUnavailable string
	}{
		nodeModel:                  message.NodeModel{ExternalIP: "172.0.0.1"},
		jsonRespServiceUnavailable: `{"message": "503 Service Unavailable" }`,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		defer cleanupFiles()
		// using bad certificate so get/post to bxapi will fail
		sslCerts := cert.SSLCerts{}

		handler1 := mockServiceError(t, 503, testCase.jsonRespServiceUnavailable)
		var m []handlerArgs
		m = append(m, handlerArgs{method: "POST", pattern: "/nodes", handler: handler1})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		// using bad sdn url so get/post to bxapi will fail
		sdn := NewSDNHTTP(&sslCerts, server.URL, testCase.nodeModel, "").(*realSDNHTTP)

		nodeModel := generateNodeModel()
		// generate nodemodel.json file which contains nodeModel using UpdateCacheFile method
		writeToFile(t, nodeModel, nodeModelCacheFileName)

		// calling to httpWithCache -> tying to get node model from bxapi
		// bxapi is not responsive
		// -> trying to load the node model from cache file
		resp, err := sdn.httpWithCache(sdn.sdnURL+"/nodes", http.MethodPost, nodeModelCacheFileName, bytes.NewBuffer(sdn.NodeModel().Pack()))
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		cachedNodeModel := &message.NodeModel{}
		assert.Nil(t, json.Unmarshal(resp, &cachedNodeModel))
		assert.Equal(t, nodeModel, cachedNodeModel)
	})
}

func TestSDNHTTP_CacheFiles_ServiceUnavailable_SDN_Relays(t *testing.T) {
	testCase := struct {
		nodeModel                  message.NodeModel
		jsonRespServiceUnavailable string
	}{
		nodeModel:                  message.NodeModel{NodeID: "35299c61-55ad-4565-85a3-0cd985953fac", BlockchainNetworkNum: 5},
		jsonRespServiceUnavailable: `{"message": "503 Service Unavailable" }`,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		defer cleanupFiles()
		// using bad certificate so get/post to bxapi will fail
		sslCerts := cert.SSLCerts{}

		handler1 := mockServiceError(t, 503, testCase.jsonRespServiceUnavailable)
		var m []handlerArgs
		m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeID}/{networkNum}/potential-relays", handler: handler1})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		// using bad sdn url so get/post to bxapi will fail
		sdn := NewSDNHTTP(&sslCerts, server.URL, testCase.nodeModel, "").(*realSDNHTTP)
		url := fmt.Sprintf("%v/nodes/%v/%v/potential-relays", sdn.SDNURL(), sdn.NodeModel().NodeID, sdn.NodeModel().BlockchainNetworkNum)
		peers := generatePeers()
		// generate potentialrelays.json file which contains peers using UpdateCacheFile method
		writeToFile(t, peers, potentialRelaysFileName)

		// calling to httpWithCache -> tying to get peers from bxapi
		// bxapi is not responsive
		// -> trying to load the peers from cache file
		resp, err := sdn.httpWithCache(url, http.MethodGet, potentialRelaysFileName, nil)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		cachedPeers := message.Peers{}
		assert.Nil(t, json.Unmarshal(resp, &cachedPeers))
		assert.Equal(t, peers, cachedPeers)
	})
}

func TestSDNHTTP_CacheFiles_ServiceUnavailable_SDN_Account(t *testing.T) {
	testCase := struct {
		nodeModel                  message.NodeModel
		jsonRespServiceUnavailable string
	}{
		nodeModel:                  message.NodeModel{AccountID: "e64yrte6547"},
		jsonRespServiceUnavailable: `{"message": "503 Service Unavailable" }`,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		defer cleanupFiles()
		// using bad certificate so get/post to bxapi will fail
		sslCerts := cert.SSLCerts{}

		handler1 := mockServiceError(t, 503, testCase.jsonRespServiceUnavailable)
		var m []handlerArgs
		m = append(m, handlerArgs{method: "GET", pattern: "/account/{accountID}", handler: handler1})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		// using bad sdn url so get/post to bxapi will fail
		sdn := NewSDNHTTP(&sslCerts, server.URL, testCase.nodeModel, "").(*realSDNHTTP)

		accountModel := generateAccountModel()
		// generate accountmodel.json file which contains accountModel using UpdateCacheFile method
		writeToFile(t, accountModel, accountModelsFileName)
		url := fmt.Sprintf("%v/%v/%v", sdn.SDNURL(), "account", sdn.NodeModel().AccountID)

		// calling to httpWithCache -> tying to get account model from bxapi
		// bxapi is not responsive
		// -> trying to load the account model from cache file
		resp, err := sdn.httpWithCache(url, http.MethodGet, accountModelsFileName, nil)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		cachedAccountModel := message.Account{}
		assert.Nil(t, json.Unmarshal(resp, &cachedAccountModel))
		assert.Equal(t, accountModel, cachedAccountModel)
	})
}

func TestSDNHTTP_InitGateway(t *testing.T) {
	testCase := struct {
		nodeModel          message.NodeModel
		networkNumber      types.NetworkNum
		jsonRespNetwork    string
		jsonRespRelays     string
		jsonAccount        string
		expectedRelayLimit message.BDNServiceLimit
	}{
		nodeModel:          message.NodeModel{NodeID: "35299c61-55ad-4565-85a3-0cd985953fac", ExternalIP: "11.113.164.111", Protocol: "Ethereum", Network: "Mainnet", AccountID: "e64yrte6547"},
		networkNumber:      5,
		jsonRespNetwork:    `{"min_tx_age_seconds":0,"min_tx_network_fee":0, "network":"Mainnet", "network_num":5,"protocol":"Ethereum"}`,
		jsonRespRelays:     `[{"ip":"8.208.101.30", "port":1809}, {"ip":"47.90.133.153", "port":1809}]`,
		jsonAccount:        `{"account_id":"e64yrte6547","blockchain_protocol":"","blockchain_network":"","tier_name":"", "relay_limit":{"expire_date":"", "msg_quota": {"limit":0}}, "private_transaction_fee": {"expire_date": "2999-01-01", "msg_quota": {"interval": "WITHOUT_INTERVAL", "service_type": "MSG_QUOTA", "limit": 13614113913969504939, "behavior_limit_ok": "ALERT", "behavior_limit_fail": "BLOCK_ALERT"}}}`,
		expectedRelayLimit: 2,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		defer cleanupFiles()

		sslCerts := cert.NewSSLCertsPrivateKey(PrivateKey)
		sslCerts.SavePrivateCert(PrivateCert)

		handler1 := mockNodesServer(t, testCase.nodeModel.NodeID, testCase.nodeModel.ExternalPort, testCase.nodeModel.ExternalIP, testCase.nodeModel.Protocol, testCase.nodeModel.Network, testCase.networkNumber, testCase.nodeModel.AccountID)
		handler2, _ := mockBlockchainNetworkServer(t, testCase.jsonRespNetwork)
		handler3, _ := mockRelaysServer(t, testCase.jsonRespRelays)
		handler4, _ := mockAccountServer(t, testCase.jsonAccount)

		var m []handlerArgs
		m = append(m, handlerArgs{method: "POST", pattern: "/nodes", handler: handler1})
		m = append(m, handlerArgs{method: "GET", pattern: "/blockchain-networks/{networkNum}", handler: handler2})
		m = append(m, handlerArgs{method: "GET", pattern: "/nodes/{nodeID}/{networkNum}/potential-relays", handler: handler3})
		m = append(m, handlerArgs{method: "GET", pattern: "/account/{accountID}", handler: handler4})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		sdn := NewSDNHTTP(sslCerts, server.URL, message.NodeModel{}, "").(*realSDNHTTP)

		assert.Nil(t, sdn.InitGateway(types.EthereumProtocol, "Mainnet"))
		assert.Equal(t, testCase.expectedRelayLimit, sdn.accountModel.RelayLimit.MsgQuota.Limit)
	})
}

func TestSDNHTTP_InitGateway_Fail(t *testing.T) {
	testCase := struct {
		nodeModel                  message.NodeModel
		networkNumber              types.NetworkNum
		jsonRespServiceUnavailable string
	}{
		nodeModel:                  message.NodeModel{NodeID: "35299c61-55ad-4565-85a3-0cd985953fac", ExternalIP: "11.113.164.111", Protocol: "Ethereum", Network: "Mainnet", AccountID: "e64yrte6547"},
		networkNumber:              5,
		jsonRespServiceUnavailable: `{"message": "503 Service Unavailable" }`,
	}
	t.Run(fmt.Sprint(testCase), func(t *testing.T) {

		sslCerts := cert.NewSSLCertsPrivateKey(PrivateKey)
		sslCerts.SavePrivateCert(PrivateCert)

		handler1 := mockServiceError(t, 503, testCase.jsonRespServiceUnavailable)
		var m []handlerArgs
		m = append(m, handlerArgs{method: "POST", pattern: "/nodes", handler: handler1})

		server := mockRouter(m)
		defer func() {
			server.Close()
		}()

		IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
		sdn := NewSDNHTTP(sslCerts, server.URL, message.NodeModel{}, "").(*realSDNHTTP)

		os.Remove(nodeModelCacheFileName)
		assert.NotNil(t, sdn.InitGateway(types.EthereumProtocol, "Mainnet"))
	})
}

func TestSDNHTTP_HttpPostBadRequestDetailsResponse(t *testing.T) {
	sslCerts := cert.SSLCerts{}

	IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
	sdn := NewSDNHTTP(&sslCerts, "", message.NodeModel{ExternalIP: "localhost"}, "").(*realSDNHTTP)
	testCase := struct {
		nodeModel         message.NodeModel
		jsonRespNodeModel string
	}{

		nodeModel:         message.NodeModel{NodeType: "FOO"},
		jsonRespNodeModel: `{"message": "Bad Request", "details": "Foo not a valid type"}`,
	}

	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		router := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, err := w.Write([]byte(testCase.jsonRespNodeModel))
			if err != nil {
				t.FailNow()
			}

		}
		pattern := "/nodes"
		router.HandleFunc(pattern, handler).Methods("POST")
		server := httptest.NewServer(router)
		defer func() {
			server.Close()
		}()

		url := fmt.Sprintf("%v/nodes", server.URL)
		sdn.nodeModel.NodeType = testCase.nodeModel.NodeType
		resp, err := sdn.http(url, http.MethodPost, bytes.NewBuffer(sdn.NodeModel().Pack()))
		assert.NotNil(t, err)
		assert.Nil(t, resp)
	})
}

func TestSDNHTTP_HttpGetBadRequestDetailsResponse(t *testing.T) {
	sslCerts := cert.SSLCerts{}
	IPResolverHolder = &MockIPResolver{IP: "11.111.111.111"}
	sdn := NewSDNHTTP(&sslCerts, "", message.NodeModel{ExternalIP: "localhost"}, "").(*realSDNHTTP)
	testCase := struct {
		nodeModel         message.NodeModel
		jsonRespNodeModel string
	}{

		nodeModel:         message.NodeModel{NodeType: "FOO", NodeID: "0f54c509-06f0-4bdd-8fc0-3bdf1ac119ed"},
		jsonRespNodeModel: `{"message": "Bad Request", "details": "Foo not a valid type"}`,
	}

	t.Run(fmt.Sprint(testCase), func(t *testing.T) {
		router := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, err := w.Write([]byte(testCase.jsonRespNodeModel))
			if err != nil {
				t.FailNow()
			}

		}
		pattern := "/nodes/{nodeId}"
		router.HandleFunc(pattern, handler).Methods("GET")
		server := httptest.NewServer(router)
		defer func() {
			server.Close()
		}()

		url := fmt.Sprintf("%v/nodes/%v", server.URL, testCase.nodeModel.NodeID)
		sdn.nodeModel.NodeType = testCase.nodeModel.NodeType
		resp, err := sdn.http(url, http.MethodGet, bytes.NewBuffer(sdn.NodeModel().Pack()))
		assert.NotNil(t, err)
		assert.Nil(t, resp)
	})
}

func TestSDNHTTP_HttpPostBodyError(t *testing.T) {
	testCase := struct {
		nodeModel         message.NodeModel
		networkNumber     types.NetworkNum
		jsonRespNodeModel string
	}{

		nodeModel:         message.NodeModel{NodeType: "TEST"},
		jsonRespNodeModel: `{"message": "Bad Request", "details": "TEST not a valid type"}`,
	}

	t.Run(fmt.Sprint(testCase), func(t *testing.T) {

		router := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1")
			w.WriteHeader(502)
		}
		pattern := "/nodes"
		router.HandleFunc(pattern, handler).Methods("POST")
		server := httptest.NewServer(router)
		defer func() {
			server.Close()
		}()

		testCerts := SetupTestCerts()
		sdn := realSDNHTTP{
			sdnURL:   server.URL,
			sslCerts: &testCerts,
			nodeModel: &message.NodeModel{
				NodeType: testCase.nodeModel.NodeType,
			},
		}

		url := fmt.Sprintf("%v/nodes", sdn.SDNURL())
		resp, err := sdn.http(url, http.MethodPost, bytes.NewBuffer(sdn.NodeModel().Pack()))
		assert.NotNil(t, err)
		assert.Nil(t, resp)
	})
}

func TestSDNHTTP_HttpPostUnmarshallError(t *testing.T) {
	testCase := struct {
		nodeModel         message.NodeModel
		networkNumber     types.NetworkNum
		jsonRespNodeModel string
	}{

		nodeModel:         message.NodeModel{NodeType: "TEST"},
		jsonRespNodeModel: `{"message": 3}`,
	}

	t.Run(fmt.Sprint(testCase), func(t *testing.T) {

		router := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, err := w.Write([]byte(testCase.jsonRespNodeModel))
			if err != nil {
				t.FailNow()
			}
		}
		pattern := "/nodes"
		router.HandleFunc(pattern, handler).Methods("POST")
		server := httptest.NewServer(router)
		defer func() {
			server.Close()
		}()

		testCerts := SetupTestCerts()
		sdn := realSDNHTTP{
			sdnURL:   server.URL,
			sslCerts: &testCerts,
			nodeModel: &message.NodeModel{
				NodeType: testCase.nodeModel.NodeType,
			},
		}

		url := fmt.Sprintf("%v/nodes", sdn.SDNURL())
		resp, err := sdn.http(url, http.MethodPost, bytes.NewBuffer(sdn.NodeModel().Pack()))
		assert.NotNil(t, err)
		assert.Nil(t, resp)
	})
}

func TestSDNHTTP_FillInAccountDefaults(t *testing.T) {
	now := time.Now().UTC()
	targetAccount := message.GetDefaultEliteAccount(now)
	tp := reflect.TypeOf(targetAccount)
	numFields := tp.NumField()
	for i := 0; i < numFields; i++ {
		reflect.ValueOf(&targetAccount).Elem().FieldByName(tp.Field(i).Name).Set(reflect.Zero(tp.Field(i).Type))
	}

	sdnhttp := testSDNHTTP()

	targetAccount, err := sdnhttp.fillInAccountDefaults(&targetAccount, now)

	assert.NoError(t, err)
	assert.Equal(t, message.GetDefaultEliteAccount(now), targetAccount)

}

func mockNodesServer(t *testing.T, nodeID types.NodeID, externalPort int64, externalIP, protocol, network string, blockchainNetworkNum types.NetworkNum, accountID types.AccountID) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		requestBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.FailNow()
		}

		var requestNodeModel message.NodeModel
		err = json.Unmarshal(requestBytes, &requestNodeModel)
		if err != nil || requestNodeModel.Protocol != protocol || requestNodeModel.Network != network {
			t.FailNow()
		}

		if requestNodeModel.BlockchainNetworkNum == 0 {
			requestNodeModel.BlockchainNetworkNum = blockchainNetworkNum
		}
		responseNodeModel := message.NodeModel{
			NodeID:               nodeID,
			ExternalIP:           externalIP,
			ExternalPort:         externalPort,
			Protocol:             protocol,
			Network:              network,
			BlockchainNetworkNum: requestNodeModel.BlockchainNetworkNum,
			AccountID:            accountID,
		}

		responseBytes, err := json.Marshal(responseNodeModel)
		if err != nil {
			t.FailNow()
		}

		_, err = w.Write(responseBytes)
		if err != nil {
			t.FailNow()
		}
	}
}

func mockServiceError(t *testing.T, statusCode int, unavailableJSON string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, err := w.Write([]byte(unavailableJSON))
		if err != nil {
			t.FailNow()
		}
	}
}

func mockNodeModelServer(t *testing.T, nodeModel string) (func(w http.ResponseWriter, r *http.Request), message.NodeModel) {

	var requestNodeModel message.NodeModel
	err := json.Unmarshal([]byte(nodeModel), &requestNodeModel)
	if err != nil {
		fmt.Println(err.Error())
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(nodeModel))
		if err != nil {
			t.FailNow()
		}
	}, requestNodeModel
}

func mockBlockchainNetworkServer(t *testing.T, nodeModel string) (func(w http.ResponseWriter, r *http.Request), message.BlockchainNetwork) {
	var network message.BlockchainNetwork
	err := json.Unmarshal([]byte(nodeModel), &network)
	if err != nil {
		fmt.Println(err.Error())
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(nodeModel))
		if err != nil {
			t.FailNow()
		}
	}, network
}

func mockRelaysServer(t *testing.T, nodeModel string) (func(w http.ResponseWriter, r *http.Request), message.Peers) {
	var relays message.Peers
	err := json.Unmarshal([]byte(nodeModel), &relays)
	if err != nil {
		fmt.Println(err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(nodeModel))
		if err != nil {
			t.FailNow()
		}
	}, relays
}

func mockAccountServer(t *testing.T, nodeModel string) (func(w http.ResponseWriter, r *http.Request), message.Account) {
	var account message.Account
	err := json.Unmarshal([]byte(nodeModel), &account)
	if err != nil {
		fmt.Println(err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(nodeModel))
		if err != nil {
			t.FailNow()
		}
	}, account
}

func mockRouter(handlerArgs []handlerArgs) *httptest.Server {
	router := mux.NewRouter()
	for _, args := range handlerArgs {
		router.HandleFunc(args.pattern, args.handler).Methods(args.method)
	}
	server := httptest.NewServer(router)
	return server
}

func generateAccountModel() message.Account {
	accountModel := message.Account{SecretHash: "1234"}
	return accountModel
}

func generatePeers() message.Peers {
	peers := message.Peers{}
	peers = append(peers, message.Peer{IP: "8.208.101.30", Port: 1809})
	peers = append(peers, message.Peer{IP: "47.90.133.153", Port: 1809})
	return peers
}

func generateNodeModel() *message.NodeModel {
	nodeModel := &message.NodeModel{NodeType: "EXTERNAL_GATEWAY", ExternalPort: 1809, IsDocker: true}
	return nodeModel
}

func generateNetworks() []*message.BlockchainNetwork {
	var networks []*message.BlockchainNetwork
	network1 := &message.BlockchainNetwork{AllowGasPriceChangeReuseSenderNonce: 1.1, AllowedFromTier: "Developer", SendCrossGeo: true, Network: "Mainnet", Protocol: "Ethereum", NetworkNum: 5}
	network2 := &message.BlockchainNetwork{AllowGasPriceChangeReuseSenderNonce: 1.1, AllowedFromTier: "Enterprise", SendCrossGeo: true, Network: "BSC-Mainnet", Protocol: "Ethereum", NetworkNum: 10}
	networks = append(networks, network1)
	networks = append(networks, network2)
	return networks
}

func generateTestNetwork() *message.BlockchainNetwork {
	return &message.BlockchainNetwork{AllowGasPriceChangeReuseSenderNonce: 1.1, AllowedFromTier: "Developer", SendCrossGeo: true, Network: "TestNetwork", Protocol: "TestProtocol", NetworkNum: 0}
}

func writeToFile(t *testing.T, data interface{}, fileName string) {
	value, err := json.Marshal(data)
	if err != nil {
		t.FailNow()
	}

	if cache.UpdateCacheFile("", fileName, value) != nil {
		t.FailNow()
	}
}

// SetupTestCerts uses the test certs specified in constants to return an utils.SSLCerts object for connection testing
func SetupTestCerts() cert.SSLCerts {
	defer CleanupSSLCerts()
	SetupSSLFiles("test")
	return NewTestCertsWithoutSetup()
}

// NewTestCertsWithoutSetup uses the test certs specified in constants to return an utils.SSLCerts object for connection testing. This function does not do any setup/teardown of writing said files temporarily to disk.
func NewTestCertsWithoutSetup() cert.SSLCerts {
	return cert.NewSSLCerts(SSLTestPath, SSLTestPath, "test")
}

// SetupSSLFiles writes the fixed test certificates to disk for loading into an SSL context.
func SetupSSLFiles(certName string) {
	setupRegistrationFiles(certName)
	setupPrivateFiles(certName)
	SetupCAFiles()
}

func setupRegistrationFiles(certName string) {
	makeFolders(certName)
	writeCerts("registration_only", certName, RegistrationCert, RegistrationKey)
}

func setupPrivateFiles(certName string) {
	makeFolders(certName)
	writeCerts("private", certName, PrivateCert, PrivateKey)
}

// SetupCAFiles writes the CA files to disk for loading into an SSL context.
func SetupCAFiles() {
	err := os.MkdirAll(CACertFolder, 0755)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(CACertPath, []byte(CACert), 0644)
	if err != nil {
		panic(err)
	}
}

func makeFolders(name string) {
	privatePath := path.Join(SSLTestPath, name, "private")
	registrationPath := path.Join(SSLTestPath, name, "registration_only")
	err := os.MkdirAll(privatePath, 0755)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(registrationPath, 0755)
	if err != nil {
		panic(err)
	}
}

func writeCerts(folder, name, cert, key string) {
	p := path.Join(SSLTestPath, name, folder)
	keyPath := path.Join(p, fmt.Sprintf("%v_cert.pem", name))
	certPath := path.Join(p, fmt.Sprintf("%v_key.pem", name))

	err := ioutil.WriteFile(keyPath, []byte(cert), 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(certPath, []byte(key), 0644)
	if err != nil {
		panic(err)
	}
}

// CleanupSSLCerts clears the temporary SSL certs written to disk.
func CleanupSSLCerts() {
	_ = os.RemoveAll(SSLTestPath)
}
