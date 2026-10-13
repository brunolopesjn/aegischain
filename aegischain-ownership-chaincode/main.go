package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	fmt.Println("[DEBUG] Iniciando chaincode...")

	aegisOwnershipContract := new(AegisOwnershipContract)
	cc, err := contractapi.NewChaincode(aegisOwnershipContract)
	if err != nil {
		panic(err)
	}
	fmt.Println("[DEBUG] Contrato criado com sucesso")

	ccid := os.Getenv("CHAINCODE_ID")
	address := os.Getenv("CHAINCODE_SERVER_ADDRESS")
	fmt.Printf("[DEBUG] CCID=%s ADDRESS=%s\n", ccid, address)

	server := &shim.ChaincodeServer{
		CCID:    ccid,
		Address: address,
		CC:      cc,
		TLSProps: shim.TLSProperties{
			Disabled: true,
		},
	}
	fmt.Println("[DEBUG] Servidor configurado, chamando Start()...")

	if err := server.Start(); err != nil {
		fmt.Printf("[DEBUG] ERRO ao iniciar: %s\n", err)
	}

	fmt.Println("[DEBUG] server.Start() retornou (não deveria acontecer se estiver servindo)")
}