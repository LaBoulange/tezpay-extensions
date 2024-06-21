package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
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

type Configuration struct {
	LogFile string `json:"LOG_FILE"`
	LogLevel string `json:"LOG_LEVEL"`
	RpcNode string `json:"RPC_NODE"`
}

const (
	LOG_SYSTEM = "system"
	LOG_ERRORS = "errors"
	LOG_CONTRACTS = "contracts"
	LOG_REDIRECTS = "redirects"
	LOG_VERBOSE = "verbose"
	LOG_DEBUG = "debug"
)

var (
	Config Configuration = Configuration{}

	LogLevelsMap = map[string]int{
		LOG_DEBUG: 0,
		LOG_VERBOSE: 1,
		LOG_CONTRACTS: 10,
		LOG_REDIRECTS: 11,
		LOG_ERRORS: 100,
		LOG_SYSTEM: 1000,
	}	
)

func (rw rwCloser) Close() error {
	return errors.Join(rw.WriteCloser.Close(), rw.ReadCloser.Close())
}

func parseConfig(desc json.RawMessage) error {
	err := json.Unmarshal([]byte(desc), &Config)
	if err != nil {
		return err
	}

	l, err := openLog()
	if err != nil {
		return err
	}
	
	closeLog(l)

	if len(Config.LogLevel) == 0 {
		Config.LogLevel = LOG_CONTRACTS
	} 

	level, exists := LogLevelsMap[Config.LogLevel]
	if !exists || level == LogLevelsMap[LOG_SYSTEM] {
		return fmt.Errorf("invalid LOG_LEVEL '%s'", Config.LogLevel)
	}

	if len(Config.RpcNode) == 0 {
		Config.RpcNode = "https://eu.rpc.tez.capital"
	}

	_, err = url.ParseRequestURI(Config.RpcNode)
	if err != nil {
		return fmt.Errorf("invalid RPC_NODE '%s'", Config.RpcNode)
	}	

	return nil
}

func openLog() (*os.File, error) {
	if len(Config.LogFile) > 0 {
		f, err := os.OpenFile(Config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}

		return f, nil
	}

	return nil, nil
}

func closeLog(f *os.File) error {
	if f != nil {
		return f.Close()
	}

	return nil
}

func shallLog(log_level string) bool {
	return LogLevelsMap[log_level] >= LogLevelsMap[Config.LogLevel]
}

func writeLog(f *os.File, message string, log_level string) error {
	if f != nil && shallLog(log_level){
		_, err := f.Write([]byte(message + "\n")); 

		if err != nil {
			return err
		}
	}

	return nil
}

func candidateLogMessage(candidate generate.PayoutCandidate, message string) string {
	return candidate.Source.String() + ": " + message
}

func rpcClient() (*ttrpc.Client, error) {
	return ttrpc.NewClient(Config.RpcNode, nil)
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

func initExtension(ctx context.Context, params common.ExtensionInitializationMessage) (common.ExtensionInitializationResult, *rpc.Error) {
	def := params.Definition
	if def.Configuration == nil {
		return common.ExtensionInitializationResult{
			Success: false,
			Message: "no Configuration provided",
		}, nil
	}

	err := parseConfig(*def.Configuration)
	if err != nil {
		return common.ExtensionInitializationResult{
			Success: false,
			Message: "invalid Configuration provided: " + err.Error(),
		}, nil
	}

	return common.ExtensionInitializationResult{
		Success: true,
	}, nil
}

func mutateCandidates(ctx context.Context, data_in common.ExtensionHookData[generate.AfterCandidateGeneratedHookData]) (*generate.AfterCandidateGeneratedHookData, *rpc.Error) {
	rpc_client, err := rpcClient()
	if err != nil {
		return nil, rpc.NewInternalErrorWithData(err.Error())
	}	
	
	log, err := openLog()
	if err != nil {
		return nil, rpc.NewInternalErrorWithData(err.Error())
	}	

	err = writeLog(log, "=== Cycle " + fmt.Sprintf("%d", data_in.Data.Cycle) + " ===", LOG_SYSTEM)
	if err != nil {
		return nil, rpc.NewInternalErrorWithData(err.Error())
	}	

	err = writeLog(log, "RPC node is " + Config.RpcNode + "\nLog level is " + Config.LogLevel, LOG_DEBUG)
	if err != nil {
		return nil, rpc.NewInternalErrorWithData(err.Error())
	}	

	for i := range data_in.Data.Candidates {
		candidate := data_in.Data.Candidates[i] 

		if requiresInvestigations(candidate) {
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
					err = writeLog(log, candidateLogMessage(candidate, "WARNING: no owner address. Kept unchanged."), LOG_ERRORS)
					if err != nil {
						return nil, rpc.NewInternalErrorWithData(err.Error())
					}
				} else {
					data_in.Data.Candidates[i].Recipient = tezos.MustParseAddress(*owner_address)

					err = writeLog(log, candidateLogMessage(candidate, "redirected to " + string(*owner_address) + "."), LOG_REDIRECTS)
					if err != nil {
						return nil, rpc.NewInternalErrorWithData(err.Error())
					}
				}
			} else {
				err = writeLog(log, candidateLogMessage(candidate, "not an oven."), LOG_CONTRACTS)
				if err != nil {
					return nil, rpc.NewInternalErrorWithData(err.Error())
				}					
			}
		} else if candidate.Source.IsContract() {
			err = writeLog(log, candidateLogMessage(candidate, "already substituted."), LOG_CONTRACTS)
			if err != nil {
				return nil, rpc.NewInternalErrorWithData(err.Error())
			}
		} else {
			err = writeLog(log, candidateLogMessage(candidate, "not a contract."), LOG_VERBOSE)
			if err != nil {
				return nil, rpc.NewInternalErrorWithData(err.Error())
			}
		}
	}

	err = writeLog(log, fmt.Sprintf("%d candidates inspected.", len(data_in.Data.Candidates)), LOG_DEBUG)
	if err != nil {
		return nil, rpc.NewInternalErrorWithData(err.Error())
	}	

	closeLog(log)
	
	return data_in.Data, nil
}


func main() {
	endpoint := extension.NewStreamEndpoint(context.Background(), extension.NewPlainObjectStream(rwCloser{os.Stdin, os.Stdout}))

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_INIT_CALL), initExtension)
	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_HOOK_AFTER_CANDIDATES_GENERATED), mutateCandidates)

	closeChannel := make(chan struct{})

	extension.RegisterEndpointMethod(endpoint, string(enums.EXTENSION_CLOSE_CALL), func(ctx context.Context, params any) (any, *rpc.Error) {
		close(closeChannel)
		return nil, nil
	})
	<-closeChannel
}

