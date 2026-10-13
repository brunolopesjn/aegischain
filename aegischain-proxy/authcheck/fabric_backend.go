package main

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type FabricBackend struct {
	gateway    *client.Gateway
	connection *grpc.ClientConn
	contract   *client.Contract
}

type fabricConfig struct {
	mspID         string
	certPath      string
	keystoreDir   string
	tlsCertPath   string
	peerEndpoint  string
	gatewayPeer   string
	channelName   string
	chaincodeName string
}

func loadFabricConfigFromEnv() fabricConfig {
	return fabricConfig{
		mspID:         getEnvOrDefault("FABRIC_MSP_ID", "Org1MSP"),
		certPath:      getEnvOrDefault("FABRIC_CERT_PATH", "/fabric/identity/cert.pem"),
		keystoreDir:   getEnvOrDefault("FABRIC_KEYSTORE_DIR", "/fabric/identity/keystore"),
		tlsCertPath:   getEnvOrDefault("FABRIC_TLS_CERT_PATH", "/fabric/tls/ca.crt"),
		peerEndpoint:  getEnvOrDefault("FABRIC_PEER_ENDPOINT", "localhost:7051"),
		gatewayPeer:   getEnvOrDefault("FABRIC_GATEWAY_PEER", "peer0.org1.example.com"),
		channelName:   getEnvOrDefault("FABRIC_CHANNEL_NAME", "mychannel"),
		chaincodeName: getEnvOrDefault("FABRIC_CHAINCODE_NAME", "aegis-consent"),
	}
}

func NewFabricBackend() (*FabricBackend, error) {
	cfg := loadFabricConfigFromEnv()

	conn, err := newGrpcConnection(cfg.tlsCertPath, cfg.peerEndpoint, cfg.gatewayPeer)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	id, err := newIdentity(cfg.certPath, cfg.mspID)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	sign, err := newSign(cfg.keystoreDir)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	gw, err := client.Connect(id,
		client.WithSign(sign),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithSubmitTimeout(15*time.Second),
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect gateway: %w", err)
	}

	network := gw.GetNetwork(cfg.channelName)
	contract := network.GetContract(cfg.chaincodeName)

	return &FabricBackend{gateway: gw, connection: conn, contract: contract}, nil
}

func (f *FabricBackend) CheckAccess(patientID, providerID, studyUID string) (bool, error) {
	result, err := f.contract.SubmitTransaction("CheckAccess", patientID, providerID, studyUID)
	if err != nil {
		return false, fmt.Errorf("failed to submit CheckAccess transaction: %w", err)
	}
	return string(result) == "true", nil
}

func (f *FabricBackend) Close() {
	f.gateway.Close()
	f.connection.Close()
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