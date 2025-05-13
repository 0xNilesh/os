// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package keeper

import (
	"encoding/json"
	"fmt"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/os/server/config"
	"github.com/evmos/os/x/evm/types"
)

// CallEVM performs a smart contract method call using given args.
func (k Keeper) CallEVM(
	ctx sdk.Context,
	abi abi.ABI,
	from, contract common.Address,
	commit bool,
	method string,
	args ...interface{},
) (*types.MsgEthereumTxResponse, error) {
	data, err := abi.Pack(method, args...)
	if err != nil {
		return nil, errorsmod.Wrap(
			types.ErrABIPack,
			errorsmod.Wrap(err, "failed to create transaction data").Error(),
		)
	}
	fmt.Print("CallEVM: ")
	fmt.Println("data", data)
	fmt.Println("contract", contract)
	fmt.Println("from", from)
	fmt.Println("method", method)
	fmt.Println("args", args)
	fmt.Println("commit", commit)

	resp, err := k.CallEVMWithData(ctx, from, &contract, data, commit)
	if err != nil {
		return nil, errorsmod.Wrapf(err, "contract call failed: method '%s', contract '%s'", method, contract)
	}
	return resp, nil
}

// CallEVMWithData performs a smart contract method call using contract data.
func (k Keeper) CallEVMWithData(
	ctx sdk.Context,
	from common.Address,
	contract *common.Address,
	data []byte,
	commit bool,
) (*types.MsgEthereumTxResponse, error) {
	nonce, err := k.accountKeeper.GetSequence(ctx, from.Bytes())
	if err != nil {
		return nil, err
	}

	fmt.Print("CallEVMWithData: ")
	fmt.Println("data", data)
	fmt.Println("contract", contract)
	fmt.Println("from", from)
	fmt.Println("commit", commit)
	fmt.Println("nonce", nonce)

	gasCap := config.DefaultGasCap
	fmt.Println("GasCap", gasCap)
	if commit {
		args, err := json.Marshal(types.TransactionArgs{
			From: &from,
			To:   contract,
			Data: (*hexutil.Bytes)(&data),
		})
		if err != nil {
			fmt.Printf("Failed to marshal tx args: %v\n", err)
			return nil, errorsmod.Wrapf(errortypes.ErrJSONMarshal, "failed to marshal tx args: %s", err.Error())
		}

		fmt.Printf("Estimating gas with args: %s\n", string(args))
		gasRes, err := k.EstimateGasInternal(ctx, &types.EthCallRequest{
			Args:   args,
			GasCap: config.DefaultGasCap,
		}, types.Internal)
		if err != nil {
			fmt.Printf("Gas estimation failed: %v\n", err)
			return nil, err
		}
		fmt.Printf("Estimated gas: %d\n", gasRes.Gas)
		gasCap = gasRes.Gas
	}

	fmt.Printf("Creating new message with parameters:\n")
	fmt.Printf("From: %s\n", from.Hex())
	fmt.Printf("To: %s\n", contract.Hex())
	fmt.Printf("Nonce: %d\n", nonce)
	fmt.Printf("Amount: %s\n", big.NewInt(0).String())
	fmt.Printf("GasCap: %d\n", gasCap)
	fmt.Printf("GasFeeCap: %s\n", big.NewInt(0).String())
	fmt.Printf("GasTipCap: %s\n", big.NewInt(0).String())
	fmt.Printf("GasPrice: %s\n", big.NewInt(0).String())
	fmt.Printf("Data length: %d bytes\n", len(data))
	fmt.Printf("IsFake: %v\n", !commit)

	msg := ethtypes.NewMessage(
		from,
		contract,
		nonce,
		big.NewInt(0), // amount
		gasCap,        // gasLimit
		big.NewInt(0), // gasFeeCap
		big.NewInt(0), // gasTipCap
		big.NewInt(0), // gasPrice
		data,
		ethtypes.AccessList{}, // AccessList
		!commit,               // isFake
	)
	fmt.Println("Message created:", msg)
	fmt.Println(ethtypes.AccessList{})

	fmt.Printf("Applying message with commit=%v\n", commit)
	res, err := k.ApplyMessage(ctx, msg, types.NewNoOpTracer(), commit)
	if err != nil {
		fmt.Printf("ApplyMessage error: %v\n", err)
		return nil, err
	}

	fmt.Printf("Message execution result:\n")
	fmt.Printf("Gas used: %d\n", res.GasUsed)
	fmt.Printf("VM Error: %v\n", res.VmError)
	fmt.Printf("Ret: %x\n", res.Ret)

	if res.Failed() {
		fmt.Printf("Transaction failed with VM error: %s\n", res.VmError)
		return nil, errorsmod.Wrap(types.ErrVMExecution, res.VmError)
	}

	fmt.Printf("Transaction successful. Gas used: %d, Return data length: %d bytes\n",
		res.GasUsed, len(res.Ret))
	return res, nil
}
