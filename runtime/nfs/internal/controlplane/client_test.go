package controlplane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientUsesDiscoveredSameOriginEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	requireAuth := func(w http.ResponseWriter, r *http.Request) bool {
		user, password, ok := r.BasicAuth()
		if !ok || user != "access" || password != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("/v2-beta", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		w.Header().Set("X-API-Schemas", server.URL+"/v2-beta/schemas")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	mux.HandleFunc("/v2-beta/schemas", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{
			map[string]interface{}{"id": "volume", "links": map[string]string{"collection": server.URL + "/v2-beta/volumes"}},
			map[string]interface{}{"id": "storageDriver", "links": map[string]string{"collection": server.URL + "/v2-beta/storageDrivers"}},
			map[string]interface{}{"id": "host", "links": map[string]string{"collection": server.URL + "/v2-beta/hosts"}},
		}})
	})
	mux.HandleFunc("/v2-beta/volumes", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		if got := r.URL.Query().Get("name"); got != "data" {
			t.Fatalf("name filter = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{
			map[string]interface{}{
				"id": "1v1", "name": "data", "state": "active",
				"links":   map[string]string{"self": server.URL + "/v2-beta/volumes/1v1"},
				"actions": map[string]string{"update": server.URL + "/v2-beta/volumes/1v1?action=update"},
			},
		}})
	})
	mux.HandleFunc("/v2-beta/volumes/1v1", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuth(w, r) {
			return
		}
		if r.Method == http.MethodPut {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "1v1", "name": "data", "state": "inactive"})
			return
		}
		if r.Method == http.MethodPost && r.URL.Query().Get("action") == "update" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "1v1", "name": "data", "state": "active"})
			return
		}
		http.Error(w, "unexpected request", http.StatusBadRequest)
	})
	mux.HandleFunc("/v2-beta/storageDrivers", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	})
	mux.HandleFunc("/v2-beta/hosts", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	})

	client, err := NewClient(&ClientOpts{URL: server.URL + "/v1", AccessKey: "access", SecretKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	volumes, err := client.Volume.List(&ListOpts{Filters: map[string]interface{}{"name": "data"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(volumes.Data) != 1 || volumes.Data[0].Id != "1v1" {
		t.Fatalf("unexpected volumes: %#v", volumes.Data)
	}
	updated, err := client.Volume.Update(&volumes.Data[0], &Volume{Name: "data"})
	if err != nil || updated.State != "inactive" {
		t.Fatalf("update: %#v, %v", updated, err)
	}
	activated, err := client.Volume.ActionUpdate(&volumes.Data[0])
	if err != nil || activated.State != "active" {
		t.Fatalf("action update: %#v, %v", activated, err)
	}
}

func TestClientRejectsCrossOriginCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Schemas", r.URL.String())
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{
			map[string]interface{}{"id": "volume", "links": map[string]string{"collection": "https://attacker.invalid/volumes"}},
		}})
	}))
	defer server.Close()
	if _, err := NewClient(&ClientOpts{URL: server.URL}); err == nil {
		t.Fatal("expected cross-origin schema to be rejected")
	}
}
