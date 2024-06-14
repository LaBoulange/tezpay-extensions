package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alis-is/jsonrpc2/rpc"

	"github.com/tez-capital/tezpay/common"
	"github.com/tez-capital/tezpay/constants/enums"
	"github.com/tez-capital/tezpay/extension"
)

type rwCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw rwCloser) Close() error {
	return errors.Join(rw.WriteCloser.Close(), rw.ReadCloser.Close())
}

type configuration struct {
	LogFile string `json:"LOG_FILE"`
}

type hookitem struct {
	balance int64
	delegated_balance int64
	fee_rate float64
	is_baker_paying_allocation_tx_fee bool
	is_baker_paying_tx_fee bool
	recipient string
	source string
}

var (
	config configuration = configuration{}
)

func appendToFile(data []byte) error {
	f, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

func main() {
	endpoint := extension.NewStreamEndpoint(context.Background(), extension.NewPlainObjectStream(rwCloser{os.Stdin, os.Stdout}))

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_INIT_CALL), func(ctx context.Context, params common.ExtensionInitializationMessage) (common.ExtensionInitializationResult, *rpc.Error) {
		def := params.Definition
		if def.Configuration == nil {
			return common.ExtensionInitializationResult{
				Success: false,
				Message: "no configuration provided",
			}, nil
		}
		err := json.Unmarshal([]byte(*def.Configuration), &config)
		if err != nil {
			return common.ExtensionInitializationResult{
				Success: false,
				Message: "invalid configuration provided",
			}, nil
		}

		return common.ExtensionInitializationResult{
			Success: true,
		}, nil
	})

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_HOOK_AFTER_CANDIDATES_GENERATED), func(ctx context.Context, params common.ExtensionHookData[any]) (any, *rpc.Error) {
		var data []hookitem
		
		messageData, err := json.Marshal(params)
		if err != nil {
			return nil, rpc.NewInternalErrorWithData(err.Error())
		}

		json.Unmarshal(messageData, &data)
		for i := range data {
			appendToFile([]byte(fmt.Sprintf("%#v\n", data[i])))
		}		
		
		return params.Data, nil
	})

	closeChannel := make(chan struct{})

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_CLOSE_CALL), func(ctx context.Context, params any) (any, *rpc.Error) {
		close(closeChannel)
		return nil, nil
	})
	<-closeChannel
}

