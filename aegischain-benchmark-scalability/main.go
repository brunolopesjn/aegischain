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
	measureN    = 2000 // transações medidas por nível (menor que 5000, já que isso é exploratório)
	warmupCount = 50
)

var (
	mspID         = flag.String("msp-id", "Org1MSP", "MSP ID da identidade cliente")
	certDir       = flag.String("cert-dir", "", "Diretório com o certificado (signcerts)")
	keystoreDir   = flag.String("keystore-dir", "", "Diretório com a chave privada (keystore)")
	tlsCertPath   = flag.String("tls-cert", "", "Caminho do TLS CA do peer")
	peerEndpoint  = flag.String("peer-endpoint", "localhost:7051", "Endereço gRPC do peer")
	gatewayPeer   = flag.String("gateway-peer", "peer0.org1.example.com", "Nome usado para validação TLS")
	channelName   = flag.String("channel", "aegischain", "Nome do canal")
	chaincodeName = flag.String("chaincode", "aegis-consent", "Nome do chaincode de consent")
	ownershipCC   = flag.String("ownership-chaincode", "aegis-ownership", "Nome do chaincode de ownership")
	concurrency   = flag.Int("concurrency", 50, "Máximo de transações simultâneas em voo")
	poolSize      = flag.Int("pool-size", 10, "Número de conexões gRPC independentes no pool")

	experiment = flag.String("experiment", "", "Qual experimento rodar: 'checkaccess' ou 'discovery'")
	level      = flag.Int("level", 0, "Nível de escala: para 'checkaccess', TOTAL cumulativo de pacientes que devem existir ao final; para 'discovery', TOTAL cumulativo de providers para o paciente fixo")
	seedStart  = flag.Int("seed-start", 0, "Índice inicial do seed incremental (para checkaccess: primeiro patient-scale-N a criar; para discovery: contagem de providers já existentes)")
	skipSeed   = flag.Bool("skip-seed", false, "Pula a fase de seed deste nível (reaproveita o que já foi populado)")
)

type pooledConn struct {
	conn              *grpc.ClientConn
	gateway           *client.Gateway
	consentContract   *client.Contract
	ownershipContract *client.Contract
}

var pool []*pooledConn
var poolCounter int64

func main() {
	flag.Parse()
	if *certDir == "" || *keystoreDir == "" || *tlsCertPath == "" {
		log.Fatal("obrigatório: -cert-dir, -keystore-dir, -tls-cert")
	}
	if *experiment != "checkaccess" && *experiment != "discovery" {
		log.Fatal("obrigatório: -experiment checkaccess|discovery")
	}
	if *level <= 0 {
		log.Fatal("obrigatório: -level > 0")
	}

	pool = make([]*pooledConn, *poolSize)
	for i := 0; i < *poolSize; i++ {
		conn, err := newGrpcConnection(*tlsCertPath, *peerEndpoint, *gatewayPeer)
		if err != nil {
			log.Fatalf("falha na conexão gRPC #%d: %v", i, err)
		}
		id, err := newIdentity(*certDir, *mspID)
		if err != nil {
			log.Fatalf("falha ao criar identidade: %v", err)
		}
		sign, err := newSign(*keystoreDir)
		if err != nil {
			log.Fatalf("falha ao criar signer: %v", err)
		}
		gw, err := client.Connect(id,
			client.WithSign(sign),
			client.WithClientConnection(conn),
			client.WithEvaluateTimeout(15*time.Second),
			client.WithSubmitTimeout(60*time.Second),
		)
		if err != nil {
			log.Fatalf("falha ao conectar gateway #%d: %v", i, err)
		}
		network := gw.GetNetwork(*channelName)
		pool[i] = &pooledConn{
			conn:              conn,
			gateway:           gw,
			consentContract:   network.GetContract(*chaincodeName),
			ownershipContract: network.GetContract(*ownershipCC),
		}
	}
	defer func() {
		for _, p := range pool {
			p.gateway.Close()
			p.conn.Close()
		}
	}()

	switch *experiment {
	case "checkaccess":
		runCheckAccessExperiment()
	case "discovery":
		runDiscoveryExperiment()
	}
}

// ============================================================
// EXPERIMENTO A: CheckAccess vs. tamanho TOTAL do ledger
// Cria `level` pacientes distintos (1 registro cada), mede
// CheckAccess sobre UM paciente fixo (o primeiro criado) -
// isola o efeito de "banco maior" sem misturar com "mais
// registros do MESMO paciente".
// ============================================================
func runCheckAccessExperiment() {
	fmt.Printf("== Experimento A: CheckAccess vs. tamanho total do ledger (level=%d) ==\n", *level)

	if !*skipSeed {
		newCount := *level - *seedStart
		if newCount <= 0 {
			log.Fatalf("level (%d) deve ser maior que seed-start (%d)", *level, *seedStart)
		}
		fmt.Printf("Populando pacientes patient-scale-%d até patient-scale-%d (%d novos)...\n", *seedStart, *level-1, newCount)
		seedDistinctPatients(*seedStart, newCount)
	} else {
		fmt.Println("Seed pulado, reaproveitando ledger existente")
	}

	// Mede CheckAccess sempre sobre o MESMO paciente fixo (patient-scale-0),
	// que foi o primeiro criado - assim o alvo da medição não muda entre
	// níveis, só o tamanho do banco ao redor dele muda.
	fmt.Println("Medindo CheckAccess sobre patient-scale-0...")
	benchmarkFixedTarget("CheckAccess", func(i int) (string, []string) {
		return "CheckAccess", []string{"patient-scale-0", "dr-silva", "study-scale-0"}
	})
}

func seedDistinctPatients(start, n int) {
	var wg sync.WaitGroup
	var completed int64
	sem := make(chan struct{}, *concurrency)

	for i := start; i < start+n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			idx := atomic.AddInt64(&poolCounter, 1) % int64(len(pool))
			ownershipContract := pool[idx].ownershipContract
			consentContract := pool[idx].consentContract

			patientID := fmt.Sprintf("patient-scale-%d", i)
			studyUID := fmt.Sprintf("study-scale-%d", i)
			ownerInvokerID := fmt.Sprintf("patient-scale-owner-%d", i)

			if _, err := ownershipContract.SubmitTransaction("CreateOwnershipRecord", patientID, studyUID, "https://clinic1.example/wado", ownerInvokerID); err != nil {
				log.Printf("seed: CreateOwnershipRecord falhou em i=%d: %v", i, err)
			}
			if _, err := consentContract.SubmitTransaction("RequestAccess", patientID, "dr-silva", studyUID); err != nil {
				log.Printf("seed: RequestAccess falhou em i=%d: %v", i, err)
			}
			expiry := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
			if _, err := consentContract.SubmitTransaction("UpdateConsentStatus", patientID, "dr-silva", studyUID, ownerInvokerID, "true", expiry); err != nil {
				log.Printf("seed: UpdateConsentStatus falhou em i=%d: %v", i, err)
			}

			done := atomic.AddInt64(&completed, 1)
			if done%1000 == 0 {
				fmt.Printf("seed: %d/%d\n", done, n)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("seed concluído: %d pacientes distintos\n", n)
}

// ============================================================
// EXPERIMENTO B: QueryConsentsByPatient vs. registros DO MESMO
// paciente. Cria `level` providers distintos solicitando acesso
// ao MESMO paciente/estudo fixo, depois mede a query de
// descoberta desse paciente.
// ============================================================
func runDiscoveryExperiment() {
	fmt.Printf("== Experimento B: QueryConsentsByPatient vs. registros por paciente (level=%d) ==\n", *level)

	const fixedPatient = "patient-discovery-fixed"
	const fixedStudy = "study-discovery-fixed"
	const fixedOwner = "patient-discovery-fixed-owner"

	if !*skipSeed {
		// Garante que o paciente fixo existe (idempotente - ignora erro
		// de "already exists" caso já tenha sido criado num nível anterior)
		idx := atomic.AddInt64(&poolCounter, 1) % int64(len(pool))
		_, _ = pool[idx].ownershipContract.SubmitTransaction("CreateOwnershipRecord", fixedPatient, fixedStudy, "https://clinic1.example/wado", fixedOwner)

		newCount := *level - *seedStart
		if newCount <= 0 {
			log.Fatalf("level (%d) deve ser maior que seed-start (%d)", *level, *seedStart)
		}
		fmt.Printf("Criando %d novos providers (total acumulado: %d) para %s...\n", newCount, *level, fixedPatient)
		seedManyProvidersForOnePatient(fixedPatient, fixedStudy, fixedOwner, *seedStart, newCount)
	} else {
		fmt.Println("Seed pulado, reaproveitando registros existentes")
	}

	fmt.Printf("Medindo QueryConsentsByPatient(%s)...\n", fixedPatient)
	benchmarkEvaluate("QueryConsentsByPatient", func(i int) (string, []string) {
		return "QueryConsentsByPatient", []string{fixedPatient}
	})
}

func seedManyProvidersForOnePatient(patientID, studyUID, ownerInvokerID string, start, n int) {
	var wg sync.WaitGroup
	var completed int64
	sem := make(chan struct{}, *concurrency)

	for i := start; i < start+n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			idx := atomic.AddInt64(&poolCounter, 1) % int64(len(pool))
			consentContract := pool[idx].consentContract

			providerID := fmt.Sprintf("provider-discovery-%d", i)

			if _, err := consentContract.SubmitTransaction("RequestAccess", patientID, providerID, studyUID); err != nil {
				log.Printf("seed: RequestAccess falhou em i=%d: %v", i, err)
			}

			done := atomic.AddInt64(&completed, 1)
			if done%1000 == 0 {
				fmt.Printf("seed: %d/%d\n", done, n)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("seed concluído: %d providers para %s\n", n, patientID)
}

// ============================================================
// Infraestrutura de medição (compartilhada pelos dois experimentos)
// ============================================================

type callResult struct {
	duration time.Duration
	err      error
	warmup   bool
}

// benchmarkFixedTarget mede uma operação de ESCRITA (SubmitTransaction)
// repetidamente contra um alvo fixo - usado para CheckAccess.
func benchmarkFixedTarget(label string, builder func(i int) (string, []string)) {
	runBenchmark(label, builder, true)
}

// benchmarkEvaluate mede uma operação de LEITURA (EvaluateTransaction) -
// usado para QueryConsentsByPatient, que é read-only.
func benchmarkEvaluate(label string, builder func(i int) (string, []string)) {
	runBenchmark(label, builder, false)
}

func runBenchmark(label string, builder func(i int) (string, []string), isSubmit bool) {
	results := make([]callResult, measureN)
	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)

	wallStart := time.Now()
	for i := 0; i < measureN; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			idx := atomic.AddInt64(&poolCounter, 1) % int64(len(pool))
			contract := pool[idx].consentContract

			fn, args := builder(i)
			warmup := i < warmupCount
			start := time.Now()
			var err error
			if isSubmit {
				_, err = contract.SubmitTransaction(fn, args...)
			} else {
				_, err = contract.EvaluateTransaction(fn, args...)
			}
			elapsed := time.Since(start)
			results[i] = callResult{duration: elapsed, err: err, warmup: warmup}
		}(i)
	}
	wg.Wait()
	wallDuration := time.Since(wallStart)

	var latencies []time.Duration
	errCount := 0
	for _, r := range results {
		if r.warmup {
			continue
		}
		if r.err != nil {
			errCount++
			continue
		}
		latencies = append(latencies, r.duration)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	if len(latencies) == 0 {
		fmt.Printf("[level=%d] %-28s TODAS as transações falharam (errs=%d)\n", *level, label, errCount)
		return
	}

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	throughput := float64(len(latencies)) / wallDuration.Seconds()

	fmt.Printf("[level=%d] %-28s p50=%-10v p95=%-10v throughput=%.2f tx/s (ok=%d, errs=%d, wall=%v)\n",
		*level, label, p50, p95, throughput, len(latencies), errCount, wallDuration)
}

// ============================================================
// Boilerplate de conexão (idêntico aos outros drivers já validados)
// ============================================================

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

func newIdentity(certDir, mspID string) (*identity.X509Identity, error) {
	entries, err := os.ReadDir(certDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert dir: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no certificate found in %s", certDir)
	}
	certificatePEM, err := os.ReadFile(filepath.Join(certDir, entries[0].Name()))
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