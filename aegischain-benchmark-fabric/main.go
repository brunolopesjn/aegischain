package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	n           = 5000
	warmupCount = 100
)

var (
	mspID                  = flag.String("msp-id", "Org1MSP", "MSP ID da identidade cliente")
	certPath               = flag.String("cert", "", "Caminho do certificado X.509 (signcerts/cert.pem)")
	keystoreDir            = flag.String("keystore", "", "Diretório da chave privada (keystore/)")
	tlsCertPath            = flag.String("tls-cert", "", "Caminho do certificado TLS do peer (ca.crt)")
	peerEndpoint           = flag.String("peer-endpoint", "localhost:7051", "Endereço gRPC do peer")
	gatewayPeer            = flag.String("gateway-peer", "peer0.org1.example.com", "Nome do peer (SNI TLS)")
	channelName            = flag.String("channel", "aegischain", "Nome do canal")
	chaincodeName          = flag.String("chaincode", "aegis-consent", "Nome do chaincode de consent")
	ownershipChaincodeName = flag.String("ownership-chaincode", "aegis-ownership", "Nome do chaincode de ownership")
	concurrency            = flag.Int("concurrency", 200, "Máximo de transações simultâneas em voo no modo concorrente")
	skipSeed               = flag.Bool("skip-seed", false, "Pula a fase de seed, reaproveitando patient-i/study-i já no ledger")
	skipSequential         = flag.Bool("skip-sequential", false, "Pula os três blocos sequenciais")
	runTag                 = flag.String("run-tag", fmt.Sprintf("%d", time.Now().Unix()), "Tag única embutida nos providerID do modo concorrente, evita colisão ao repetir execução sem reiniciar a rede")
	diagnoseN              = flag.Int("diagnose", 0, "Se >0, roda modo diagnóstico com N transações RequestAccess decompostas em fases (endorse/submit/commit), em vez do benchmark normal")
	connPool               = flag.Int("conn-pool", 1, "Número de conexões gRPC independentes no modo -diagnose (1 = comportamento original, N = pool distribuído)")
)

type callResult struct {
	duration time.Duration
	err      error
	warmup   bool
}

type txBuilder func(i int) (fn string, args []string)

func main() {
	flag.Parse()
	if *certPath == "" || *keystoreDir == "" || *tlsCertPath == "" {
		log.Fatal("obrigatório: -cert, -keystore, -tls-cert")
	}

	if *diagnoseN > 0 {
		ownershipPool, consentPool := connectContractPool(*connPool)
		runDiagnostic(ownershipPool, consentPool, *diagnoseN)
		return
	}

	ownershipContract, consentContract := connectContract()

	if !*skipSeed {
		fmt.Printf("== Seed: criando %d ownership records + consents GRANTED (dr-silva) ==\n", n)
		seed(ownershipContract, consentContract)
	} else {
		fmt.Println("== Seed pulado (-skip-seed), reaproveitando ledger existente ==")
	}

	if !*skipSequential {
		fmt.Println()
		fmt.Println("== Sequential ==")
		benchmark(consentContract, "RequestAccess (write)", false, requestAccessArgsBuilder("seq"))
		benchmark(consentContract, "UpdateConsentStatus (write)", false, updateConsentArgsBuilder("seq"))
		benchmark(consentContract, "CheckAccess (read)", false, checkAccessArgsBuilder)
	} else {
		fmt.Println()
		fmt.Println("== Sequential pulado (-skip-sequential) ==")
	}

	fmt.Println()
	fmt.Printf("== Concurrent (max %d in-flight, run-tag=%s) ==\n", *concurrency, *runTag)
	benchmark(consentContract, "RequestAccess (write)", true, requestAccessArgsBuilder("conc"))
	benchmark(consentContract, "UpdateConsentStatus (write)", true, updateConsentArgsBuilder("conc"))
	benchmark(consentContract, "CheckAccess (read)", true, checkAccessArgsBuilder)
}

func seed(ownershipContract, consentContract *client.Contract) {
	var wg sync.WaitGroup
	var completed int64
	sem := make(chan struct{}, *concurrency)

	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			patientID := fmt.Sprintf("patient-%d", i)
			studyUID := fmt.Sprintf("study-%d", i)
			ownerInvokerID := fmt.Sprintf("patient-owner-%d", i)

			if _, err := ownershipContract.SubmitTransaction("CreateOwnershipRecord", patientID, studyUID, "https://clinic1.example/wado", ownerInvokerID); err != nil {
				log.Printf("seed: CreateOwnershipRecord falhou em i=%d: %v", i, err)
			}
			if _, err := consentContract.SubmitTransaction("RequestAccess", patientID, "dr-silva", studyUID); err != nil {
				log.Printf("seed: RequestAccess falhou em i=%d: %v", i, err)
			}
			expiry := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
			if _, err := consentContract.SubmitTransaction("UpdateConsentStatus", patientID, "dr-silva", studyUID, ownerInvokerID, "true", expiry); err != nil {
				log.Printf("seed: UpdateConsentStatus falhou em i=%d: %v", i, err)
			}

			done := atomic.AddInt64(&completed, 1)
			if done%250 == 0 {
				fmt.Printf("seed: %d/%d\n", done, n)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("seed concluído: %d ownership records + %d consents GRANTED\n", n, n)
}

func requestAccessArgsBuilder(prefix string) txBuilder {
	return func(i int) (string, []string) {
		patientID := fmt.Sprintf("patient-%d", i)
		studyUID := fmt.Sprintf("study-%d", i)
		providerID := fmt.Sprintf("bench-provider-%s-%s-%d", prefix, *runTag, i)
		return "RequestAccess", []string{patientID, providerID, studyUID}
	}
}

func updateConsentArgsBuilder(prefix string) txBuilder {
	return func(i int) (string, []string) {
		patientID := fmt.Sprintf("patient-%d", i)
		studyUID := fmt.Sprintf("study-%d", i)
		providerID := fmt.Sprintf("bench-provider-%s-%s-%d", prefix, *runTag, i)
		ownerInvokerID := fmt.Sprintf("patient-owner-%d", i)
		expiry := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		return "UpdateConsentStatus", []string{patientID, providerID, studyUID, ownerInvokerID, "true", expiry}
	}
}

func checkAccessArgsBuilder(i int) (string, []string) {
	patientID := fmt.Sprintf("patient-%d", i)
	studyUID := fmt.Sprintf("study-%d", i)
	return "CheckAccess", []string{patientID, "dr-silva", studyUID}
}

func benchmark(contract *client.Contract, label string, concurrent bool, builder txBuilder) {
	results := make([]callResult, n)

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
				results[i] = timedSubmit(contract, builder, i)
				done := atomic.AddInt64(&completed, 1)
				if done%250 == 0 {
					fmt.Printf("  ... %d/%d\n", done, n)
				}
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < n; i++ {
			results[i] = timedSubmit(contract, builder, i)
			if (i+1)%250 == 0 {
				fmt.Printf("  ... %d/%d\n", i+1, n)
			}
		}
	}
	wallDuration := time.Since(wallStart)

	report(label, concurrent, results, wallDuration)
}

func timedSubmit(contract *client.Contract, builder txBuilder, i int) callResult {
	fn, args := builder(i)
	warmup := i < warmupCount
	start := time.Now()
	_, err := contract.SubmitTransaction(fn, args...)
	elapsed := time.Since(start)
	return callResult{duration: elapsed, err: err, warmup: warmup}
}

func report(label string, concurrent bool, results []callResult, wallDuration time.Duration) {
	var latencies []time.Duration
	errCount := 0
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
		latencies = append(latencies, r.duration)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	mode := "sequential"
	if concurrent {
		mode = "concurrent"
	}

	if len(latencies) == 0 {
		fmt.Printf("[%s] %-28s TODAS as %d transações medidas falharam\n", mode, label, len(results)-warmupSeen)
		return
	}

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	throughput := float64(len(latencies)) / wallDuration.Seconds()

	fmt.Printf("[%s] %-28s p50=%-10v p95=%-10v throughput=%.2f tx/s (ok=%d, warmup=%d, errs=%d, wall=%v)\n",
		mode, label, p50, p95, throughput, len(latencies), warmupSeen, errCount, wallDuration)
}

func connectContract() (ownershipContract, consentContract *client.Contract) {
	conn, err := newGrpcConnection(*tlsCertPath, *peerEndpoint, *gatewayPeer)
	if err != nil {
		log.Fatalf("gRPC connection: %v", err)
	}

	id, err := newIdentity(*certPath, *mspID)
	if err != nil {
		log.Fatalf("identity: %v", err)
	}

	sign, err := newSign(*keystoreDir)
	if err != nil {
		log.Fatalf("sign: %v", err)
	}

	gw, err := client.Connect(id,
		client.WithSign(sign),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithSubmitTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("gateway connect: %v", err)
	}

	network := gw.GetNetwork(*channelName)
	ownershipContract = network.GetContract(*ownershipChaincodeName)
	consentContract = network.GetContract(*chaincodeName)
	return
}

func connectContractPool(poolSize int) (ownershipContracts, consentContracts []*client.Contract) {
	for i := 0; i < poolSize; i++ {
		conn, err := newGrpcConnection(*tlsCertPath, *peerEndpoint, *gatewayPeer)
		if err != nil {
			log.Fatalf("gRPC connection [%d]: %v", i, err)
		}

		id, err := newIdentity(*certPath, *mspID)
		if err != nil {
			log.Fatalf("identity [%d]: %v", i, err)
		}

		sign, err := newSign(*keystoreDir)
		if err != nil {
			log.Fatalf("sign [%d]: %v", i, err)
		}

		gw, err := client.Connect(id,
			client.WithSign(sign),
			client.WithClientConnection(conn),
			client.WithEvaluateTimeout(5*time.Second),
			client.WithSubmitTimeout(30*time.Second),
		)
		if err != nil {
			log.Fatalf("gateway connect [%d]: %v", i, err)
		}

		network := gw.GetNetwork(*channelName)
		ownershipContracts = append(ownershipContracts, network.GetContract(*ownershipChaincodeName))
		consentContracts = append(consentContracts, network.GetContract(*chaincodeName))
	}
	return
}

func newGrpcConnection(tlsCertPath, peerEndpoint, gatewayPeer string) (*grpc.ClientConn, error) {
	certificatePEM, err := os.ReadFile(tlsCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS cert: %w", err)
	}
	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS cert: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	return grpc.NewClient(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
}

func newIdentity(certPath, mspID string) (*identity.X509Identity, error) {
	certificatePEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read client cert: %w", err)
	}
	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client cert: %w", err)
	}
	return identity.NewX509Identity(mspID, certificate)
}

func newSign(keystoreDir string) (identity.Sign, error) {
	entries, err := os.ReadDir(keystoreDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore dir: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no private key found in %s", keystoreDir)
	}

	privateKeyPEM, err := os.ReadFile(filepath.Join(keystoreDir, entries[0].Name()))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return identity.NewPrivateKeySign(privateKey)
}