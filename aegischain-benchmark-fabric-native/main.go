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
	mspID         = flag.String("msp-id", "HospitalAMSP", "MSP ID da organização")
	certDir       = flag.String("cert-dir", "", "Diretório com o certificado da identidade (signcerts)")
	keystoreDir   = flag.String("keystore-dir", "", "Diretório com a chave privada (keystore)")
	tlsCertPath   = flag.String("tls-cert", "", "Caminho do TLS CA do peer gateway")
	peerEndpoint  = flag.String("peer-endpoint", "203.0.113.11:7051", "Endereço do peer gateway")
	gatewayPeer   = flag.String("gateway-peer", "203.0.113.11", "Nome/IP usado para validação TLS (SAN do certificado)")
	channelName   = flag.String("channel", "aegischain", "Nome do canal")
	chaincodeName = flag.String("chaincode", "aegis-consent", "Nome do chaincode")
	patientID     = flag.String("patient-id", "patient-e2e-1", "Patient ID já autorizado no ledger")
	providerID    = flag.String("provider-id", "dr-silva", "Provider ID já autorizado no ledger")
	studyUID      = flag.String("study-uid", "1.3.6.1.4.1.14519.5.2.1.8162.5966.211496415587356532518109028398", "StudyInstanceUID já autorizado no ledger")
	concurrency   = flag.Int("concurrency", 200, "Máximo de chamadas simultâneas no modo concorrente")
	poolSize      = flag.Int("pool-size", 20, "Número de conexões gRPC independentes no pool (evita gargalo de MaxConcurrentStreams numa única conexão)")
	skipSeq       = flag.Bool("skip-sequential", false, "Pula o bloco sequencial")
)

type callResult struct {
	duration time.Duration
	err      error
	warmup   bool
}

type pooledConn struct {
	conn     *grpc.ClientConn
	gateway  *client.Gateway
	contract *client.Contract
}

var pool []*pooledConn
var poolCounter int64

func main() {
	flag.Parse()

	if *certDir == "" || *keystoreDir == "" || *tlsCertPath == "" {
		log.Fatal("obrigatório: -cert-dir, -keystore-dir, -tls-cert")
	}

	fmt.Printf("== Criando pool de %d conexões gRPC independentes ==\n", *poolSize)
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
			conn:     conn,
			gateway:  gw,
			contract: network.GetContract(*chaincodeName),
		}
	}
	defer func() {
		for _, p := range pool {
			p.gateway.Close()
			p.conn.Close()
		}
	}()

	fmt.Printf("== Alvo: peer %s, canal %s, chaincode %s ==\n", *peerEndpoint, *channelName, *chaincodeName)
	fmt.Printf("== CheckAccess(%s, %s, %s) ==\n\n", *patientID, *providerID, *studyUID)

	fmt.Println("== Verificação prévia: chamada única, deve retornar 'true' ==")
	r, val := doCheckAccess()
	if r.err != nil {
		log.Fatalf("Chamada de verificação falhou: %v", r.err)
	}
	if val != "true" {
		log.Fatalf("Chamada de verificação retornou '%s', esperado 'true' - confirme que o consent está GRANTED", val)
	}
	fmt.Printf("OK - resultado 'true', latência: %v\n\n", r.duration)

	if !*skipSeq {
		fmt.Println("== Sequential ==")
		benchmark("CheckAccess (Fabric direto)", false)
		fmt.Println()
	} else {
		fmt.Println("== Sequential pulado (-skip-sequential) ==")
	}

	fmt.Printf("== Concurrent (max %d in-flight) ==\n", *concurrency)
	benchmark("CheckAccess (Fabric direto)", true)
}

func doCheckAccess() (callResult, string) {
	idx := atomic.AddInt64(&poolCounter, 1) % int64(len(pool))
	contract := pool[idx].contract

	start := time.Now()
	result, err := contract.SubmitTransaction("CheckAccess", *patientID, *providerID, *studyUID)
	elapsed := time.Since(start)
	if err != nil {
		return callResult{duration: elapsed, err: err}, ""
	}
	return callResult{duration: elapsed}, string(result)
}

func benchmark(label string, concurrent bool) {
	results := make([]callResult, n)
	errSamples := make(chan string, 20)

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
				r, _ := doCheckAccess()
				r.warmup = i < warmupCount
				results[i] = r
				if r.err != nil {
					select {
					case errSamples <- r.err.Error():
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
			r, _ := doCheckAccess()
			r.warmup = i < warmupCount
			results[i] = r
			if r.err != nil {
				select {
				case errSamples <- r.err.Error():
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
			fmt.Println("  -- Amostra de erros (até 20) --")
			first = false
		}
		fmt.Printf("     %s\n", msg)
	}

	report(label, concurrent, results, wallDuration)
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
		fmt.Printf("[%s] %-28s TODAS as chamadas medidas falharam (erros=%d)\n", mode, label, errCount)
		return
	}

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	throughput := float64(len(latencies)) / wallDuration.Seconds()

	fmt.Printf("[%s] %-28s p50=%-10v p95=%-10v throughput=%.2f tx/s (ok=%d, warmup=%d, errs=%d, wall=%v)\n",
		mode, label, p50, p95, throughput, len(latencies), warmupSeen, errCount, wallDuration)
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