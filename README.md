# Tezpay Extensions 

WIP - more to follow

## payouts-substitutor

Configuration example (config.hjson):

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

