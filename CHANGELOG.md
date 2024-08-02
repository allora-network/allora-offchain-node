<!--
Guiding Principles:
Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.
Usage:
Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:
* (<tag>) \#<issue-number> message
The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.
Types of changes (Stanzas):
"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking Protobuf, gRPC and REST routes used by end-users.
"CLI Breaking" for breaking CLI commands.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState given same genesisState and txList.
Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

<!-- EXAMPLES:
## v5.7.1 & v5.7.2
sei-chain
* [#1779](https://github.com/sei-protocol/sei-chain/pull/1779) Fix subscribe logs empty params crash
* [#1783](https://github.com/sei-protocol/sei-chain/pull/1783) Add meaningful message for eth_call balance override overflow
* [#1783](https://github.com/sei-protocol/sei-chain/pull/1784) Fix log index on synthetic receipt
* [#1775](https://github.com/sei-protocol/sei-chain/pull/1775) Disallow sending to direct cast addr after association

sei-wasmd
* [60](https://github.com/sei-protocol/sei-wasmd/pull/60) Query penalty fixes

sei-tendermint
* [#237](https://github.com/sei-protocol/sei-tendermint/pull/237) Add metrics for total txs bytes in mempool

## v5.7.0
sei-chain
* [#1731](https://github.com/sei-protocol/sei-chain/pull/1731) Remove 1-hop limit
* [#1663](https://github.com/sei-protocol/sei-chain/pull/1663) Retain pointer address on upgrade -->
