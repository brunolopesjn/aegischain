package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	n           = 5000
	warmupCount = 100
)

var (
	baseURL        = flag.String("url", "http://203.0.113.14:9080", "URL base do Aegis Proxy")
	studyUID       = flag.String("study-uid", "1.3.6.1.4.1.14519.5.2.1.8162.5966.211496415587356532518109028398", "StudyInstanceUID já autorizado no ledger")
	patientID      = flag.String("patient-id", "patient-e2e-1", "Patient ID já autorizado no ledger")
	providerID     = flag.String("provider-id", "dr-silva", "Provider ID já autorizado no ledger")
	concurrency    = flag.Int("concurrency", 200, "Máximo de requisições simultâneas no modo concorrente")
	timeoutSec     = flag.Int("timeout", 30, "Timeout por requisição, em segundos")
	skipSequential = flag.Bool("skip-sequential", false, "Pula o bloco sequencial (útil para repetir só o concorrente em múltiplas execuções)")
)

type callResult struct {
	duration   time.Duration
	statusCode int
	err        error
	warmup     bool
}

var httpClient *http.Client

func main() {
	flag.Parse()

	transport := &http.Transport{
		MaxIdleConns:        *concurrency * 2,
		MaxIdleConnsPerHost: *concurrency * 2,
		MaxConnsPerHost:     0, // sem limite
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient = &http.Client{
		Timeout:   time.Duration(*timeoutSec) * time.Second,
		Transport: transport,
	}

	url := fmt.Sprintf("%s/dicom-web/studies/%s/series", *baseURL, *studyUID)
	fmt.Printf("== Alvo: %s ==\n", url)
	fmt.Printf("== X-Patient-ID: %s | X-Invoker-ID: %s ==\n\n", *patientID, *providerID)

	fmt.Println("== Verificação prévia: requisição única, deve retornar 200 ==")
	r := doRequest(url)
	if r.err != nil {
		log.Fatalf("Requisição de verificação falhou: %v", r.err)
	}
	if r.statusCode != 200 {
		log.Fatalf("Requisição de verificação retornou status %d, esperado 200 - confirme que o consent está GRANTED e não expirado antes de rodar o benchmark", r.statusCode)
	}
	fmt.Printf("OK - status 200, latência: %v\n\n", r.duration)

	if !*skipSequential {
		fmt.Println("== Sequential ==")
		benchmark(url, "E2E WADO-RS (series)", false)
		fmt.Println()
	} else {
		fmt.Println("== Sequential pulado (-skip-sequential) ==\n")
	}

	fmt.Printf("== Concurrent (max %d in-flight) ==\n", *concurrency)
	benchmark(url, "E2E WADO-RS (series)", true)
}

func doRequest(url string) callResult {
	start := time.Now()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return callResult{err: err}
	}
	req.Header.Set("X-Patient-ID", *patientID)
	req.Header.Set("X-Invoker-ID", *providerID)

	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return callResult{duration: elapsed, err: err}
	}
	defer resp.Body.Close()

	buf := make([]byte, 32*1024)
	for {
		_, rerr := resp.Body.Read(buf)
		if rerr != nil {
			break
		}
	}

	return callResult{duration: elapsed, statusCode: resp.StatusCode}
}

func benchmark(url, label string, concurrent bool) {
	results := make([]callResult, n)
	errSamples := make(chan string, 20)
	statusSamples := make(chan int, 20)

	wallStart := time.Now()
	if concurrent {
		var wg sync.WaitGroup
		var completed int64
		sem := make(chan struct{}, *concurrency)
		for i := 0; i < n; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				r := doRequest(url)
				r.warmup = i < warmupCount
				results[i] = r
				if r.err != nil {
					select {
					case errSamples <- r.err.Error():
					default:
					}
				} else if r.statusCode != 200 {
					select {
					case statusSamples <- r.statusCode:
					default:
					}
				}
				done := atomic.AddInt64(&completed, 1)
				if done%250 == 0 {
					fmt.Printf("  ... %d/%d\n", done, n)
				}
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < n; i++ {
			r := doRequest(url)
			r.warmup = i < warmupCount
			results[i] = r
			if r.err != nil {
				select {
				case errSamples <- r.err.Error():
				default:
				}
			} else if r.statusCode != 200 {
				select {
				case statusSamples <- r.statusCode:
				default:
				}
			}
			if (i+1)%250 == 0 {
				fmt.Printf("  ... %d/%d\n", i+1, n)
			}
		}
	}
	wallDuration := time.Since(wallStart)

	close(errSamples)
	first := true
	for msg := range errSamples {
		if first {
			fmt.Println("  -- Amostra de erros de rede/timeout (até 20) --")
			first = false
		}
		fmt.Printf("     %s\n", msg)
	}

	close(statusSamples)
	statusCounts := map[int]int{}
	for sc := range statusSamples {
		statusCounts[sc]++
	}
	if len(statusCounts) > 0 {
		fmt.Println("  -- Amostra de status HTTP não-200 (até 20 capturados) --")
		for code, count := range statusCounts {
			fmt.Printf("     status %d: %d ocorrência(s) na amostra\n", code, count)
		}
	}

	report(label, concurrent, results, wallDuration)
}

func report(label string, concurrent bool, results []callResult, wallDuration time.Duration) {
	var latencies []time.Duration
	errCount := 0
	non200 := 0
	warmupSeen := 0

	for _, r := range results {
		if r.warmup {
			warmupSeen++
			continue
		}
		if r.err != nil {
			errCount++
			continue
		}
		if r.statusCode != 200 {
			non200++
			continue
		}
		latencies = append(latencies, r.duration)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	mode := "sequential"
	if concurrent {
		mode = "concurrent"
	}

	if len(latencies) == 0 {
		fmt.Printf("[%s] %-28s TODAS as requisições medidas falharam (erros=%d, non200=%d)\n", mode, label, errCount, non200)
		return
	}

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	throughput := float64(len(latencies)) / wallDuration.Seconds()

	fmt.Printf("[%s] %-28s p50=%-10v p95=%-10v throughput=%.2f req/s (ok=%d, warmup=%d, errs=%d, non200=%d, wall=%v)\n",
		mode, label, p50, p95, throughput, len(latencies), warmupSeen, errCount, non200, wallDuration)
}