# LedgerDrill
<!-- impeccable:product-schema 1 -->
## Platform
web
## Stack
Go backend and local Horizon fault proxy, SQLite run journal, SvelteKit static frontend with Tailwind. No Rust.
## Product Purpose
Expose duplicate Stellar payments caused by incorrect recovery after uncertain submission. Run real testnet payments through controlled connection failures and compare naive versus reconciled clients.
## Users
Developers integrating Stellar payments. A hosted payout challenge lets visitors choose recovery after a missing confirmation and inspect the actual ledger outcome. Developers can still compare reference clients and test their own integration.
## Capabilities and Constraints
Testnet only. Three scenarios: rejection before broadcast, lost confirmation after settlement, delayed transaction lookup. Default live fixture uses XLM. Every result distinguishes attempts from actual settlement. No claim to audit arbitrary app internals or guarantee production reliability.
## Evidence on Hand
RESEARCH.md compares 20 public hackathon entries and adjacent tools. Official Horizon guidance describes ambiguous submission and safe retries.
## Brand Commitments
LedgerDrill. Plain language and inspectable evidence. No invented customers, certifications, or benchmarks.
## Operating Context
Local-first command with a browser dashboard and JSON reports. The homepage creates one payout, injects an unrevealed fault, and pauses for the initiating browser to choose recovery. A choice must arrive within two minutes; abandoned payouts never send an additional payment. The diagnostic lab creates paired reference-client runs. Proxy endpoints can also exercise a developer's own signed testnet transactions.
