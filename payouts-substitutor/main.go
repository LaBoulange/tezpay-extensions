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
	"github.com/tez-capital/tezpay/core/generate"
	"github.com/tez-capital/tezpay/extension"

	"github.com/trilitech/tzgo/micheline"
	ttrpc "github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
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
	RpcNode string `json:"RPC_NODE"`
}

var (
	config configuration = configuration{}
)

func writeLog(data []byte) error {
	if len(config.LogFile) > 0 {
		f, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		defer f.Close()

		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	return nil
}

func rpcClient() (*ttrpc.Client, error) {
	if len(config.RpcNode) == 0 {
		config.RpcNode = "https://eu.rpc.tez.capital"
	}
	
	return ttrpc.NewClient(config.RpcNode, nil)
}

func requiresInvestigations(candidate generate.PayoutCandidate) bool {
	return candidate.Recipient.IsContract() && candidate.Source == candidate.Recipient
}

func isOven(storage_map map[string]interface{}) bool {
	_, is_oven := storage_map["ovenProxyContractAddress"]

	return is_oven
}

func getStorageMap(rpc_client *ttrpc.Client, ctx context.Context, contract generate.PayoutCandidate) (map[string]interface{}, error) {
	script, err := rpc_client.GetContractScript(ctx, contract.Source)
	if err != nil {
		return nil, err
	}

	storage_raw_content := micheline.NewValue(script.StorageType(), script.Storage)

	storage_map_interface, err := storage_raw_content.Map()
	if err != nil {
		return nil, err
	}	

	storage_map, _ := storage_map_interface.(map[string]interface{})	

	return storage_map, nil
}

func getOwnerAddress(storage_map map[string]interface{}) (*string, error) {
	owner_address, exists := storage_map["owner"]

	if exists {
		o, err := json.Marshal(owner_address)
		if err != nil {
			return nil, err
		}

		var owner_address_string string
		err = json.Unmarshal(o, &owner_address_string)
		if err != nil {
			return nil, err
		}

		return &owner_address_string, nil
	} else {
		return nil, nil
	}
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
			func(ctx context.Context, data_in common.ExtensionHookData[generate.AfterCandidateGeneratedHookData]) (any, *rpc.Error) {
		
		rpc_client, err := rpcClient()
		if err != nil {
			return nil, rpc.NewInternalErrorWithData(err.Error())
		}	
		
		for i := range data_in.Data.Candidates {
			candidate := data_in.Data.Candidates[i] 

			if requiresInvestigations(candidate) {
				err = writeLog([]byte(candidate.Source.String() + ": "))
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}	

				storage_map, err := getStorageMap(rpc_client, ctx, candidate)
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}	

				if isOven(storage_map) {
					owner_address, err := getOwnerAddress(storage_map)
					if err != nil {
						return nil, rpc.NewInternalErrorWithData(err.Error())
					}	

					if owner_address == nil {
						err = writeLog([]byte("WARNING: no owner address. Kept unchanged.\n"))
						if err != nil {
							return nil, rpc.NewInternalErrorWithData(err.Error())
						}
					} else {
						data_in.Data.Candidates[i].Recipient = tezos.MustParseAddress(*owner_address)

						err = writeLog([]byte("redirected to " + string(*owner_address) + ".\n"))
						if err != nil {
							return nil, rpc.NewInternalErrorWithData(err.Error())
						}
					}
				} else {
					err = writeLog([]byte("not an oven.\n"))
					if err != nil {
						return nil, rpc.NewInternalErrorWithData(err.Error())
					}					
				}
			}		
		}
		
		return data_in.Data, nil
	})

	closeChannel := make(chan struct{})

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_CLOSE_CALL), func(ctx context.Context, params any) (any, *rpc.Error) {
		close(closeChannel)
		return nil, nil
	})
	<-closeChannel
}

