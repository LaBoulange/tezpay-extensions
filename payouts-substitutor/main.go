package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/alis-is/jsonrpc2/rpc"

	"github.com/tez-capital/tezpay/common"
	"github.com/tez-capital/tezpay/constants/enums"
	"github.com/tez-capital/tezpay/extension"
	"github.com/tez-capital/tezpay/core/generate"		

	ttrpc "github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/micheline"
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

	extension.RegisterEndpointMethod(
			endpoint, 
			string(enums.EXTENSION_HOOK_AFTER_CANDIDATES_GENERATED), 
			func(ctx context.Context, data common.ExtensionHookData[generate.AfterCandidateGeneratedHookData]) (any, *rpc.Error) {
		
		/*
		extensions: [
			{
			name: main
			command: /path/to/main
			args: [
			]
			kind: stdio
			configuration: {
				LOG_FILE:  /path/to/log
			}
			hooks: [
				{
				id: after_candidates_generated
				mode: rw
				}
			]
			}
		]
		*/

		indexer_client, err := ttrpc.NewClient("https://eu.rpc.tez.capital", nil)
		if err != nil {
			return nil, rpc.NewInternalErrorWithData(err.Error())
		}	
		
		for i := range data.Data.Candidates {
			candidate := data.Data.Candidates[i] 

			if candidate.Recipient.IsContract() && candidate.Source == candidate.Recipient {
				err = appendToFile([]byte("\n" + candidate.Source.String() + ":\n"))
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}	

				script, err := indexer_client.GetContractScript(ctx, candidate.Source)
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}

				storage_raw_content := micheline.NewValue(script.StorageType(), script.Storage)
				storage_map, _ := storage_raw_content.Map()
				buf, _ := json.MarshalIndent(storage_map, "", "  ")

				err = appendToFile(buf)
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}	
			}		
		}
		
		return data.Data, nil
	})

	closeChannel := make(chan struct{})

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_CLOSE_CALL), func(ctx context.Context, params any) (any, *rpc.Error) {
		close(closeChannel)
		return nil, nil
	})
	<-closeChannel
}

