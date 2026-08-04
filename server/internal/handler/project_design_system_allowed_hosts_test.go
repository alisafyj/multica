package handler

import "testing"

func TestProjectDesignSystemAllowedHostsIncludesFirstPartyCDNWithoutStorageCredentials(t *testing.T) {
	h := &Handler{}

	hosts := h.projectDesignSystemAllowedHosts()
	if countAllowedHost(hosts, "static.soyoung.com") == 0 {
		t.Fatalf("allowed hosts = %v, want static.soyoung.com", hosts)
	}
}

func TestProjectDesignSystemAllowedHostsDeduplicatesStorageDomains(t *testing.T) {
	h := &Handler{
		Storage:            &mockStorage{},
		DesignAssetStorage: &mockStorage{},
	}

	hosts := h.projectDesignSystemAllowedHosts()
	if countAllowedHost(hosts, "static.soyoung.com") != 1 {
		t.Fatalf("allowed hosts = %v, want one static.soyoung.com", hosts)
	}
	if countAllowedHost(hosts, "cdn.example.com") != 1 {
		t.Fatalf("allowed hosts = %v, want one cdn.example.com", hosts)
	}
}

func countAllowedHost(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
