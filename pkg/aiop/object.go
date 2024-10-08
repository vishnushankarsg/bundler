// Package aiop provides the base transaction object used throughout the aiops-bundler.
package aiop

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	address, _ = abi.NewType("address", "", nil)
	uint256, _ = abi.NewType("uint256", "", nil)
	bytes32, _ = abi.NewType("bytes32", "", nil)

	// AiOpPrimitives is the primitive ABI types for each AiOperation field.
	AiOpPrimitives = []abi.ArgumentMarshaling{
		{Name: "sender", InternalType: "Sender", Type: "address"},
		{Name: "nonce", InternalType: "Nonce", Type: "uint256"},
		{Name: "initCode", InternalType: "InitCode", Type: "bytes"},
		{Name: "callData", InternalType: "CallData", Type: "bytes"},
		{Name: "callGasLimit", InternalType: "CallGasLimit", Type: "uint256"},
		{Name: "verificationGasLimit", InternalType: "VerificationGasLimit", Type: "uint256"},
		{Name: "preVerificationGas", InternalType: "PreVerificationGas", Type: "uint256"},
		{Name: "maxFeePerGas", InternalType: "MaxFeePerGas", Type: "uint256"},
		{Name: "maxPriorityFeePerGas", InternalType: "MaxPriorityFeePerGas", Type: "uint256"},
		{Name: "paymasterAndData", InternalType: "PaymasterAndData", Type: "bytes"},
		{Name: "signature", InternalType: "Signature", Type: "bytes"},
	}

	// AiOpType is the ABI type of a AiOperation.
	AiOpType, _ = abi.NewType("tuple", "op", AiOpPrimitives)

	// AiOpArr is the ABI type for an array of AiOperations.
	AiOpArr, _ = abi.NewType("tuple[]", "ops", AiOpPrimitives)
)

// AiOperation represents an EIP-4337 style transaction for a smart contract account.
type AiOperation struct {
	Sender               common.Address `json:"sender"               mapstructure:"sender"               validate:"required"`
	Nonce                *big.Int       `json:"nonce"                mapstructure:"nonce"                validate:"required"`
	InitCode             []byte         `json:"initCode"             mapstructure:"initCode"             validate:"required"`
	CallData             []byte         `json:"callData"             mapstructure:"callData"             validate:"required"`
	CallGasLimit         *big.Int       `json:"callGasLimit"         mapstructure:"callGasLimit"         validate:"required"`
	VerificationGasLimit *big.Int       `json:"verificationGasLimit" mapstructure:"verificationGasLimit" validate:"required"`
	PreVerificationGas   *big.Int       `json:"preVerificationGas"   mapstructure:"preVerificationGas"   validate:"required"`
	MaxFeePerGas         *big.Int       `json:"maxFeePerGas"         mapstructure:"maxFeePerGas"         validate:"required"`
	MaxPriorityFeePerGas *big.Int       `json:"maxPriorityFeePerGas" mapstructure:"maxPriorityFeePerGas" validate:"required"`
	PaymasterAndData     []byte         `json:"paymasterAndData"     mapstructure:"paymasterAndData"     validate:"required"`
	Signature            []byte         `json:"signature"            mapstructure:"signature"            validate:"required"`
}

// GetPaymaster returns the address portion of PaymasterAndData if applicable. Otherwise it returns the zero
// address.
func (op *AiOperation) GetPaymaster() common.Address {
	if len(op.PaymasterAndData) < common.AddressLength {
		return common.HexToAddress("0x")
	}

	return common.BytesToAddress(op.PaymasterAndData[:common.AddressLength])
}

// GetFactory returns the address portion of InitCode if applicable. Otherwise it returns the zero address.
func (op *AiOperation) GetFactory() common.Address {
	if len(op.InitCode) < common.AddressLength {
		return common.HexToAddress("0x")
	}

	return common.BytesToAddress(op.InitCode[:common.AddressLength])
}

// GetFactoryData returns the data portion of InitCode if applicable. Otherwise it returns an empty byte
// array.
func (op *AiOperation) GetFactoryData() []byte {
	if len(op.InitCode) < common.AddressLength {
		return []byte{}
	}

	return op.InitCode[common.AddressLength:]
}

// GetMaxGasAvailable returns the max amount of gas that can be consumed by this AiOperation.
func (op *AiOperation) GetMaxGasAvailable() *big.Int {
	// TODO: Multiplier logic might change in v0.7
	mul := big.NewInt(1)
	paymaster := op.GetPaymaster()
	if paymaster != common.HexToAddress("0x") {
		mul = big.NewInt(3)
	}

	return big.NewInt(0).Add(
		big.NewInt(0).Mul(op.VerificationGasLimit, mul),
		big.NewInt(0).Add(op.PreVerificationGas, op.CallGasLimit),
	)
}

// GetMaxPrefund returns the max amount of wei required to pay for gas fees by either the sender or
// paymaster.
func (op *AiOperation) GetMaxPrefund() *big.Int {
	return big.NewInt(0).Mul(op.GetMaxGasAvailable(), op.MaxFeePerGas)
}

// GetDynamicGasPrice returns the effective gas price paid by the AiOperation given a basefee. If basefee is
// nil, it will assume a value of 0.
func (op *AiOperation) GetDynamicGasPrice(basefee *big.Int) *big.Int {
	bf := basefee
	if bf == nil {
		bf = big.NewInt(0)
	}

	gp := big.NewInt(0).Add(bf, op.MaxPriorityFeePerGas)
	if gp.Cmp(op.MaxFeePerGas) == 1 {
		return op.MaxFeePerGas
	}
	return gp
}

// Pack returns a standard message of the aiOp. This cannot be used to generate a aiOpHash.
func (op *AiOperation) Pack() []byte {
	args := abi.Arguments{
		{Name: "AiOp", Type: AiOpType},
	}
	packed, _ := args.Pack(&struct {
		Sender               common.Address
		Nonce                *big.Int
		InitCode             []byte
		CallData             []byte
		CallGasLimit         *big.Int
		VerificationGasLimit *big.Int
		PreVerificationGas   *big.Int
		MaxFeePerGas         *big.Int
		MaxPriorityFeePerGas *big.Int
		PaymasterAndData     []byte
		Signature            []byte
	}{
		op.Sender,
		op.Nonce,
		op.InitCode,
		op.CallData,
		op.CallGasLimit,
		op.VerificationGasLimit,
		op.PreVerificationGas,
		op.MaxFeePerGas,
		op.MaxPriorityFeePerGas,
		op.PaymasterAndData,
		op.Signature,
	})

	enc := hexutil.Encode(packed)
	enc = "0x" + enc[66:]
	return (hexutil.MustDecode(enc))
}

// PackForSignature returns a minimal message of the aiOp. This can be used to generate a aiOpHash.
func (op *AiOperation) PackForSignature() []byte {
	args := abi.Arguments{
		{Name: "sender", Type: address},
		{Name: "nonce", Type: uint256},
		{Name: "hashInitCode", Type: bytes32},
		{Name: "hashCallData", Type: bytes32},
		{Name: "callGasLimit", Type: uint256},
		{Name: "verificationGasLimit", Type: uint256},
		{Name: "preVerificationGas", Type: uint256},
		{Name: "maxFeePerGas", Type: uint256},
		{Name: "maxPriorityFeePerGas", Type: uint256},
		{Name: "hashPaymasterAndData", Type: bytes32},
	}
	packed, _ := args.Pack(
		op.Sender,
		op.Nonce,
		crypto.Keccak256Hash(op.InitCode),
		crypto.Keccak256Hash(op.CallData),
		op.CallGasLimit,
		op.VerificationGasLimit,
		op.PreVerificationGas,
		op.MaxFeePerGas,
		op.MaxPriorityFeePerGas,
		crypto.Keccak256Hash(op.PaymasterAndData),
	)

	return packed
}

// GetAiOpHash returns the hash of the aiOp + aiMiddleware address + chainID.
func (op *AiOperation) GetAiOpHash(aiMiddleware common.Address, chainID *big.Int) common.Hash {
	return crypto.Keccak256Hash(
		crypto.Keccak256(op.PackForSignature()),
		common.LeftPadBytes(aiMiddleware.Bytes(), 32),
		common.LeftPadBytes(chainID.Bytes(), 32),
	)
}

// MarshalJSON returns a JSON encoding of the AiOperation.
func (op *AiOperation) MarshalJSON() ([]byte, error) {
	// Note: The bundler spec test requires the address portion of the initCode to include the checksum.
	ic := "0x"
	if fa := op.GetFactory(); fa != common.HexToAddress("0x") {
		ic = fmt.Sprintf("%s%s", fa, common.Bytes2Hex(op.GetFactoryData()))
	}

	return json.Marshal(&struct {
		Sender               string `json:"sender"`
		Nonce                string `json:"nonce"`
		InitCode             string `json:"initCode"`
		CallData             string `json:"callData"`
		CallGasLimit         string `json:"callGasLimit"`
		VerificationGasLimit string `json:"verificationGasLimit"`
		PreVerificationGas   string `json:"preVerificationGas"`
		MaxFeePerGas         string `json:"maxFeePerGas"`
		MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
		PaymasterAndData     string `json:"paymasterAndData"`
		Signature            string `json:"signature"`
	}{
		Sender:               op.Sender.String(),
		Nonce:                hexutil.EncodeBig(op.Nonce),
		InitCode:             ic,
		CallData:             hexutil.Encode(op.CallData),
		CallGasLimit:         hexutil.EncodeBig(op.CallGasLimit),
		VerificationGasLimit: hexutil.EncodeBig(op.VerificationGasLimit),
		PreVerificationGas:   hexutil.EncodeBig(op.PreVerificationGas),
		MaxFeePerGas:         hexutil.EncodeBig(op.MaxFeePerGas),
		MaxPriorityFeePerGas: hexutil.EncodeBig(op.MaxPriorityFeePerGas),
		PaymasterAndData:     hexutil.Encode(op.PaymasterAndData),
		Signature:            hexutil.Encode(op.Signature),
	})
}

// ToMap returns the current AiOp struct as a map type.
func (op *AiOperation) ToMap() (map[string]any, error) {
	data, err := op.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var opData map[string]any
	if err := json.Unmarshal(data, &opData); err != nil {
		return nil, err
	}
	return opData, nil
}
