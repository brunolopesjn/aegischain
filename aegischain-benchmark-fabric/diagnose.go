package main

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
)

type phaseResult struct {
	endorseMs float64
	submitMs  float64
	commitMs  float64
	totalMs   float64
	err       error
}

func runDiagnostic(ownershipContracts, consentContracts []*client.Contract, n int) {
	poolSize := len(consentContracts)

	fmt.Printf("== Diagnóstico: seed de %d registros, paralelo (pool=%d) ==\n", n, poolSize)
	seedStart := time.Now()
	{
		var wg sync.WaitGroup
		var completed int64
		sem := make(chan struct{}, *concurrency)
		for i := 0; i < n; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()

				oc := ownershipContracts[i%poolSize]
				patientID := fmt.Sprintf("diag-patient-%d", i)
				studyUID := fmt.Sprintf("diag-study-%d", i)
				ownerInvokerID := fmt.Sprintf("diag-owner-%d", i)
				_, _ = oc.SubmitTransaction("CreateOwnershipRecord", patientID, studyUID, "https://clinic1.example/wado", ownerInvokerID)

				done := atomic.AddInt64(&completed, 1)
				if done%100 == 0 {
					fmt.Printf("  seed: %d/%d\n", done, n)
				}
			}(i)
		}
		wg.Wait()
	}
	fmt.Printf("== Seed concluído em %v ==\n", time.Since(seedStart))

	fmt.Printf("== Diagnóstico: %d transações RequestAccess concorrentes, decompostas (pool=%d) ==\n", n, poolSize)
	diagStart := time.Now()

	results := make([]phaseResult, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)

	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			cc := consentContracts[i%poolSize]

			patientID := fmt.Sprintf("diag-patient-%d", i)
			studyUID := fmt.Sprintf("diag-study-%d", i)
			providerID := fmt.Sprintf("diag-provider-%d", i)

			var r phaseResult

			t0 := time.Now()
			proposal, err := cc.NewProposal("RequestAccess",
				client.WithArguments(patientID, providerID, studyUID))
			if err != nil {
				r.err = fmt.Errorf("NewProposal: %w", err)
				results[i] = r
				return
			}

			transaction, err := proposal.Endorse()
			t1 := time.Now()
			if err != nil {
				r.err = fmt.Errorf("Endorse: %w", err)
				results[i] = r
				return
			}
			r.endorseMs = t1.Sub(t0).Seconds() * 1000

			commit, err := transaction.Submit()
			t2 := time.Now()
			if err != nil {
				r.err = fmt.Errorf("Submit: %w", err)
				results[i] = r
				return
			}
			r.submitMs = t2.Sub(t1).Seconds() * 1000

			status, err := commit.Status()
			t3 := time.Now()
			if err != nil {
				r.err = fmt.Errorf("Status: %w", err)
				results[i] = r
				return
			}
			if !status.Successful {
				r.err = fmt.Errorf("commit not successful, code=%v", status.Code)
				results[i] = r
				return
			}
			r.commitMs = t3.Sub(t2).Seconds() * 1000
			r.totalMs = t3.Sub(t0).Seconds() * 1000

			results[i] = r
		}(i)
	}
	wg.Wait()
	diagDuration := time.Since(diagStart)
	fmt.Printf("== Bloco concorrente concluído em %v ==\n\n", diagDuration)

	reportDiagnostic(results)
}

func reportDiagnostic(results []phaseResult) {
	var valid []phaseResult
	errCount := 0
	for _, r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		valid = append(valid, r)
	}
	fmt.Printf("Transações válidas: %d, erros: %d\n\n", len(valid), errCount)

	sort.Slice(valid, func(i, j int) bool { return valid[i].totalMs < valid[j].totalMs })

	mid := len(valid) / 2
	fast := valid[:mid]
	slow := valid[mid:]

	printBucket("RÁPIDO (metade inferior)", fast)
	printBucket("LENTO (metade superior)", slow)

	if len(valid) > 1 {
		maxGap := 0.0
		gapAt := 0
		for i := 1; i < len(valid); i++ {
			gap := valid[i].totalMs - valid[i-1].totalMs
			if gap > maxGap {
				maxGap = gap
				gapAt = i
			}
		}
		fmt.Printf("\nMaior salto na distribuição ordenada: %.1fms -> %.1fms (gap de %.1fms, na posição %d/%d)\n",
			valid[gapAt-1].totalMs, valid[gapAt].totalMs, maxGap, gapAt, len(valid))
	}

	fmt.Println("\n-- Amostra de transações individuais (10 primeiras, ordenadas por total) --")
	fmt.Printf("%-10s %-10s %-10s %-10s\n", "endorse", "submit", "commit", "total")
	for i := 0; i < 10 && i < len(valid); i++ {
		r := valid[i]
		fmt.Printf("%-10.1f %-10.1f %-10.1f %-10.1f\n", r.endorseMs, r.submitMs, r.commitMs, r.totalMs)
	}
	fmt.Println("... (final, mesmas colunas) ...")
	for i := len(valid) - 10; i < len(valid); i++ {
		if i < 0 {
			continue
		}
		r := valid[i]
		fmt.Printf("%-10.1f %-10.1f %-10.1f %-10.1f\n", r.endorseMs, r.submitMs, r.commitMs, r.totalMs)
	}
}

func printBucket(label string, bucket []phaseResult) {
	var sumEndorse, sumSubmit, sumCommit, sumTotal float64
	for _, r := range bucket {
		sumEndorse += r.endorseMs
		sumSubmit += r.submitMs
		sumCommit += r.commitMs
		sumTotal += r.totalMs
	}
	n := float64(len(bucket))
	fmt.Printf("[%s] n=%d\n", label, len(bucket))
	fmt.Printf("  endorse: %.1fms | submit: %.1fms | commit: %.1fms | total: %.1fms\n",
		sumEndorse/n, sumSubmit/n, sumCommit/n, sumTotal/n)
}