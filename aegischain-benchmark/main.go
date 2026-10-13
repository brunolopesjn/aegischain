package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	n           = 5000 // total de transações por bloco medido
	warmupCount = 100  // primeiras N requisições descartadas do cálculo de p50/p95
)

var (
	baseURL        = flag.String("url", "http://localhost:8080", "URL base da API")
	concurrency    = flag.Int("concurrency", 200, "máximo de requisições simultâneas em voo no modo concorrente")
	runTag         = flag.String("run-tag", fmt.Sprintf("%d", time.Now().Unix()), "Tag única embutida nos providerID do modo concorrente/sequencial, evita colisão ao repetir execução sem resetar o banco")
	skipSeed       = flag.Bool("skip-seed", false, "Pula a fase de seed, reaproveitando patient-i/study-i já no banco")
	skipSequential = flag.Bool("skip-sequential", false, "Pula os três blocos sequenciais")
)

type reqBuilder func(i int) (*http.Request, error)

type callResult struct {
	duration   time.Duration
	statusCode int
	err        error
	warmup     bool
}

func main() {
	flag.Parse()

	// BUGFIX: sem um Transport customizado, o Go usa http.DefaultTransport,
	// cujo MaxIdleConnsPerHost padrão é 2 - com *concurrency requisições
	// simultâneas contra o MESMO host, isso causa contenção/fila do lado do
	// CLIENTE (handshake TCP repetido), inflando artificialmente latência e
	// taxa de erro sob carga (mesmo bug já identificado e corrigido no
	// driver E2E do Aegis Proxy).
	transport := &http.Transport{
		MaxIdleConns:        *concurrency * 2,
		MaxIdleConnsPerHost: *concurrency * 2,
		MaxConnsPerHost:     0, // sem limite
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	if !*skipSeed {
		fmt.Printf("== Seed: criando %d ownership records + consents GRANTED ==\n", n)
		seed(client)
	} else {
		fmt.Println("== Seed pulado (-skip-seed), reaproveitando banco existente ==")
	}

	if !*skipSequential {
		fmt.Println()
		fmt.Println("== Sequential ==")
		benchmark(client, "CreateConsent (write)", false, createConsentBuilder("seq"))
		benchmark(client, "UpdateConsentStatus (write)", false, updateConsentBuilder("seq"))
		benchmark(client, "CheckAccess (read)", false, checkAccessBuilder)
	} else {
		fmt.Println()
		fmt.Println("== Sequential pulado (-skip-sequential) ==")
	}

	fmt.Println()
	fmt.Printf("== Concurrent (max %d in-flight, run-tag=%s) ==\n", *concurrency, *runTag)
	benchmark(client, "CreateConsent (write)", true, createConsentBuilder("conc"))
	benchmark(client, "UpdateConsentStatus (write)", true, updateConsentBuilder("conc"))
	benchmark(client, "CheckAccess (read)", true, checkAccessBuilder)
}

// seed cria n ownership records (um paciente distinto por iteração) e n
// consents já GRANTED para dr-silva, usados como alvo fixo do benchmark de
// CheckAccess. Fase de setup, não medida.
//
// IMPORTANTE: a API precisa estar rodando com SEED_PATIENT_COUNT >= n e
// HospitalAMSP/Clinic1MSP já cadastrados em institutions, senão esta função
// vai começar a falhar com 404 Patient not registered / Unauthorized.
func seed(client *http.Client) {
	for i := 0; i < n; i++ {
		patientID := fmt.Sprintf("patient-%d", i)
		studyUID := fmt.Sprintf("study-%d", i)
		ownerInvokerID := fmt.Sprintf("patient-owner-%d", i)

		body, _ := json.Marshal(map[string]string{
			"patientId": patientID,
			"studyUid":  studyUID,
			"proxyUrl":  "https://clinic1.example/wado",
		})
		req, _ := http.NewRequest("POST", *baseURL+"/ownership", bytes.NewReader(body))
		req.Header.Set("X-MSP-ID", "Clinic1MSP")
		req.Header.Set("X-Invoker-ID", ownerInvokerID)
		req.Header.Set("Content-Type", "application/json")
		drain(client.Do(req))

		body2, _ := json.Marshal(map[string]string{"patientId": patientID, "studyUid": studyUID})
		req2, _ := http.NewRequest("POST", *baseURL+"/consent/request", bytes.NewReader(body2))
		req2.Header.Set("X-MSP-ID", "HospitalAMSP")
		req2.Header.Set("X-Invoker-ID", "dr-silva")
		req2.Header.Set("Content-Type", "application/json")
		drain(client.Do(req2))

		expiry := time.Now().Add(2 * time.Hour)
		body3, _ := json.Marshal(map[string]any{"approve": true, "expiryTimestamp": expiry})
		url := fmt.Sprintf("%s/consent/%s/%s/dr-silva", *baseURL, patientID, studyUID)
		req3, _ := http.NewRequest("PUT", url, bytes.NewReader(body3))
		req3.Header.Set("X-Invoker-ID", ownerInvokerID)
		req3.Header.Set("Content-Type", "application/json")
		drain(client.Do(req3))
	}
	fmt.Printf("seed concluído: %d ownership records + %d consents GRANTED\n", n, n)
}

func createConsentBuilder(prefix string) reqBuilder {
	return func(i int) (*http.Request, error) {
		patientID := fmt.Sprintf("patient-%d", i)
		studyUID := fmt.Sprintf("study-%d", i)
		body, _ := json.Marshal(map[string]string{"patientId": patientID, "studyUid": studyUID})
		req, err := http.NewRequest("POST", *baseURL+"/consent/request", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-MSP-ID", "HospitalBMSP")
		req.Header.Set("X-Invoker-ID", fmt.Sprintf("bench-provider-%s-%s-%d", prefix, *runTag, i))
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}
}

func updateConsentBuilder(prefix string) reqBuilder {
	return func(i int) (*http.Request, error) {
		patientID := fmt.Sprintf("patient-%d", i)
		studyUID := fmt.Sprintf("study-%d", i)
		providerID := fmt.Sprintf("bench-provider-%s-%s-%d", prefix, *runTag, i)
		ownerInvokerID := fmt.Sprintf("patient-owner-%d", i)

		expiry := time.Now().Add(1 * time.Hour)
		body, _ := json.Marshal(map[string]any{"approve": true, "expiryTimestamp": expiry})
		url := fmt.Sprintf("%s/consent/%s/%s/%s", *baseURL, patientID, studyUID, providerID)
		req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Invoker-ID", ownerInvokerID)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}
}

func checkAccessBuilder(i int) (*http.Request, error) {
	patientID := fmt.Sprintf("patient-%d", i)
	studyUID := fmt.Sprintf("study-%d", i)
	url := fmt.Sprintf("%s/consent/%s/%s/dr-silva/check", *baseURL, patientID, studyUID)
	return http.NewRequest("GET", url, nil)
}

func benchmark(client *http.Client, label string, concurrent bool, builder reqBuilder) {
	results := make([]callResult, n)

	wallStart := time.Now()
	if concurrent {
		var wg sync.WaitGroup
		sem := make(chan struct{}, *concurrency) // limita requisições simultâneas em voo
		for i := 0; i < n; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = timedCall(client, builder, i)
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < n; i++ {
			results[i] = timedCall(client, builder, i)
		}
	}
	wallDuration := time.Since(wallStart)

	report(label, concurrent, results, wallDuration)
}

func timedCall(client *http.Client, builder reqBuilder, i int) callResult {
	req, err := builder(i)
	warmup := i < warmupCount
	if err != nil {
		return callResult{err: err, warmup: warmup}
	}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return callResult{duration: elapsed, err: err, warmup: warmup}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return callResult{duration: elapsed, statusCode: resp.StatusCode, warmup: warmup}
}

func drain(resp *http.Response, err error) {
	if err != nil || resp == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func report(label string, concurrent bool, results []callResult, wallDuration time.Duration) {
	var latencies []time.Duration
	errCount := 0
	warmupCountSeen := 0
	statusCounts := map[int]int{}

	for _, r := range results {
		if r.warmup {
			warmupCountSeen++
			continue // warm-up nunca entra no cálculo de percentil/status
		}
		if r.err != nil {
			errCount++
			continue
		}
		statusCounts[r.statusCode]++
		latencies = append(latencies, r.duration)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	mode := "sequential"
	if concurrent {
		mode = "concurrent"
	}

	if len(latencies) == 0 {
		fmt.Printf("[%s] %-28s TODAS as %d requisições medidas falharam (transport error)\n", mode, label, len(results)-warmupCountSeen)
		return
	}

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	throughput := float64(len(latencies)) / wallDuration.Seconds()

	fmt.Printf("[%s] %-28s p50=%-10v p95=%-10v throughput=%.2f req/s (measured=%d, warmup=%d, errs=%d, status=%v, wall=%v)\n",
		mode, label, p50, p95, throughput, len(latencies), warmupCountSeen, errCount, statusCounts, wallDuration)
}