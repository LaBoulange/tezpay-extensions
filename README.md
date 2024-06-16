# Tezpay Extensions 

WIP - more to follow

## payouts-substitutor

Enabling the extension in config.hjson:

    extensions: [
        {
            name: payouts-substitutor
            command: /path/to/payouts-substitutor
            args: [
            ]
            kind: stdio
            configuration: {
                LOG_FILE: /path/to/log
                RPC_NODE: https://eu.rpc.tez.capital
            }
            hooks: [
                {
                    id: after_candidates_generated
                    mode: rw
                }
            ]
        }
    ]

Configuration:
* LOG_FILE is optional. If omitted, no log file will be produced.
* RPC_NODE is optional. It defaults to https://eu.rpc.tez.capital

