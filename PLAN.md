# LedgerDrill v1
Build the failure-testing harness described in RESEARCH.md. Original LinkPay work remains separate.

## Acceptance
One command starts local server and dashboard. Execute three fault scenarios with real Stellar testnet payments. Compare naive and reconciled behavior, persist attempt logs and confirmed transaction hashes, export a JSON report, and recover interrupted runs honestly. Support a developer's own Horizon client against a fixed-upstream testnet proxy. Go test/vet/build and Svelte check/build pass. Browser review covers desktop, mobile, navigation, live polling, error states and exports.

## Boundary
XLM first for reproducible Friendbot setup. No customer keys, mainnet endpoint, AI feature, Rust contract, arbitrary upstream URL, or global security score. Reference client findings reflect the selected scenario, not a full audit.

## Design
Local test console inspired by instrument panels and tabular diagnostic printouts: warm ivory ground, dark ink, restrained burnt orange for injected faults and green for confirmed recovery. Amounts, hashes and timings use a mono face. A left rail shows scenario controls; the result area compares two timelines and links each settlement to Stellar Expert. Mobile stacks controls above results.

## Interactive homepage
A visitor starts a single 1 XLM payout to fictional recipient Rowan. A randomized connection fault leaves the outcome uncertain; the initiating browser chooses a fresh payment or original-hash reconciliation. Show actual independently verified settlement amounts and transaction links after the choice. Retain diagnostic comparison and app integration. Accept one authorized choice only, expire abandoned decisions without sending again, and never replay after restart. Both recovery paths and the existing three reference comparisons must pass on testnet, and the deployed public flow must work on desktop and mobile.
