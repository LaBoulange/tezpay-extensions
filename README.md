# Tezpay Extensions 

This repository is dedicated to hosting the [Tezpay](https://github.com/tez-capital/tezpay/) extensions that we are developing at La Boulange. 
We only have one so far, but likely more will follow.

## payouts-substitutor

This extension allows the redirection of delegation rewards due to smart contracts (address "KT") of the "oven" type to the owner accounts of the respective contracts.

This extension contributes to the solution proposed by [TezCapital](https://github.com/tez-capital) to the balance management issue of these contracts, which results in a zero reward from the protocol while there is actually delegation (see the complete description [here on Tezos Agora](https://forum.tezosagora.org/t/tez-capital-resolving-kt-delegator-payment-issues-in-paris/6256/1)).

### Installation

- Download the executable appropriate for your operating system and hardware from the [latest release page](https://github.com/LaBoulange/tezpay-extensions/releases/latest).
- Move the downloaded file to the directory from which you intend to run it, typically the same location as `tezpay`.

### Enabling the extension

Add the following element to the list of extensions defined in `tezpay`'s `config.hjson` file:

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

**Note**: the `extensions: [ ... ]` array should only be included if no extensions have previously been configured in `config.hjson`. If other extensions are already listed, only the inner block `{ ... }` should be added.    

Configure the following fields of the element above:
- **`command`**: `/path/to/` should be replaced by the path to the directory where you placed the `payouts-substitutor` extension.
- **`LOG_FILE`**: `/path/to/log` should be replaced by the path of the log file the extension should produce. The directory should exist, the extension will only create the file. *(optional: if omitted, no log file will be produced)*.
- **`RPC_NODE`**: URL of the RPC node used to query the contracts *(optional: if omitted, the default URL is `https://eu.rpc.tez.capital`)*.

Restart `tezpay` if it is running in `continual` mode. 

You can ensure the extension is working properly by running `tezpay -c <previous cycle number> generate-payouts`.

## Should you wish to support us

You can send a donation:
- to our baker's address: [tz1aJHKKUWrwfsuoftdmwNBbBctjSWchMWZY](https://tzkt.io/tz1aJHKKUWrwfsuoftdmwNBbBctjSWchMWZY/schedule)
- or to its Tezos domain name: [laboulange.tez](https://tzkt.io/laboulange.tez/schedule)

Or just click here: 

[![Button Support]][Link Support] 

This is not mandatory, but it is greatly appreciated!

[Button Support]: https://img.shields.io/badge/Support_La_Boulange!_(5_XTZ)-007bff?style=for-the-badge
[Link Support]: https://tezos-share.stroep.nl/?id=tfLn0 'Support La Boulange (5 XTZ)'

## Contact

Feel free to contact us with any questions or suggestions. We can be reached through the following channels:
- MailChain: [laboulange@mailchain](https://app.mailchain.com/)
- E-mail: la.boulange.tezos@gmail.com
- DNS: https://dns.xyz/fr/LaBoulange
- Twitter: https://twitter.com/LaBoulangeTezos
- Telegram: https://t.me/laboulangetezos

We are also active in various Telegram and Discord groups related to Tezos.
