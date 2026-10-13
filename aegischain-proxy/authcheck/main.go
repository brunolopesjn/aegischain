package main

import (
	"log"
	"net/http"
	"os"
	"regexp"
)

var (
	backend         AccessChecker
	studyUIDPattern = regexp.MustCompile(`/studies/([^/]+)`)
)

func main() {
	backendMode := getEnvOrDefault("BACKEND_MODE", "baseline")

	switch backendMode {
	case "baseline":
		baselineURL := getEnvOrDefault("BASELINE_API_URL", "http://host.docker.internal:8080")
		backend = NewBaselineBackend(baselineURL)
		log.Printf("authcheck starting in BASELINE mode (backend=%s)", baselineURL)
	case "fabric":
		fb, err := NewFabricBackend()
		if err != nil {
			log.Fatalf("failed to initialize Fabric backend: %v", err)
		}
		backend = fb
		log.Printf("authcheck starting in FABRIC mode")
	default:
		log.Fatalf("unknown BACKEND_MODE: %q (expected 'baseline' or 'fabric')", backendMode)
	}

	http.HandleFunc("/check", handleCheck)
	log.Printf("authcheck listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}

func handleCheck(w http.ResponseWriter, r *http.Request) {
	originalURI := r.Header.Get("X-Original-URI")
	patientID := r.Header.Get("X-Patient-ID")
	providerID := r.Header.Get("X-Invoker-ID")

	if patientID == "" || providerID == "" {
		http.Error(w, "missing identity headers", http.StatusUnauthorized)
		return
	}

	matches := studyUIDPattern.FindStringSubmatch(originalURI)
	if len(matches) < 2 {
		http.Error(w, "study UID not found in request", http.StatusBadRequest)
		return
	}
	studyUID := matches[1]

	permitted, err := backend.CheckAccess(patientID, providerID, studyUID)
	if err != nil {
		log.Printf("backend CheckAccess error: %v", err)
		http.Error(w, "internal error checking access", http.StatusBadGateway)
		return
	}

	if permitted {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "access denied", http.StatusForbidden)
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}