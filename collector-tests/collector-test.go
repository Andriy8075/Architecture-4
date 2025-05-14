package collector_tests

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

var results sync.Map

func handleResult(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ClientID string `json:"client_id"`
		Server   string `json:"server"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results.Store(data.ClientID, data.Server)
	w.WriteHeader(http.StatusOK)
}

func TestCollectResults(t *testing.T) {
	// Start HTTP server
	http.HandleFunc("/results", handleResult)
	go http.ListenAndServe(":8080", nil)

	// Wait for all client reports
	expectedClients, _ := strconv.Atoi(os.Getenv("EXPECTED_CLIENTS"))
	timeout := time.After(2 * time.Minute)

	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for client results")
		default:
			count := 0
			results.Range(func(_, _ interface{}) bool {
				count++
				return true
			})

			if count >= expectedClients {
				goto Analyze
			}
			time.Sleep(1 * time.Second)
		}
	}

Analyze:
	// Analyze distribution
	serverCounts := make(map[string]int)
	clientServers := make(map[string]string)

	results.Range(func(key, value interface{}) bool {
		clientID := key.(string)
		server := value.(string)
		serverCounts[server]++
		clientServers[clientID] = server
		return true
	})

	t.Logf("Load distribution: %v", serverCounts)

	// Verify all clients got consistent servers
	for client, server := range clientServers {
		if server == "" {
			t.Errorf("Client %s got no server assignment", client)
		}
	}

	// Verify load is distributed
	if len(serverCounts) < 2 {
		t.Errorf("Expected load distribution across multiple servers, got: %v", serverCounts)
	}
}
