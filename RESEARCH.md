# Find Your Way: public submission gap review

Checked 2026-10-03. Source: https://demo.stellarpassport.xyz/api/hackathons/find-your-way-meridian-hackathon/submissions

20 public submissions returned. These are entrant descriptions, not independently verified implementations. Draft/private entries and later submissions are outside this snapshot.

## Public entries

| Project | Stated focus |
| --- | --- |
| Soroban Studio | Visual Stellar workflow builder and scheduling |
| Xlm | Python CLI for account creation, balance checks and XLM transfers |
| Kimaex | Naira-first exchange and trading |
| TurboEntrega | Restaurant delivery marketplace |
| Axon | AI inference marketplace with micropayments |
| Fortgate | Reusable signed KYC evidence |
| AgentAllowance | Bounded agent spending with x402 and smart accounts |
| VeriFire | Product authenticity tokens and QR ownership |
| Cosmos Wallet | Multichain wallet, payment links, swaps and recovery |
| CodeZard | Agent project execution and inter-agent payments |
| LockA Medical Passport | Patient-controlled portable medical records |
| OGPASS | Urban transport integration sandbox |
| Walletnow - The onboarding infrastructure for Stellar. | Passkey onboarding, sponsored payments and Pix integration |
| AegisOS | Verifiable provenance of agent purchases |
| Local402 | Local-currency x402 pricing and atomic asset conversion |
| Honorarios | Peruvian freelancer invoices and tax reserve splitting |
| Breadline Protocol | Milestone USDC escrow |
| Qhapaq | RWA project participation, milestones and benefits |
| KwanPay | Travel payments for African tourism |
| Cofiblocks | Direct coffee producer marketplace and traceability |

## Recommendation for discussion: LedgerDrill

A failure-testing harness for applications sending Stellar payments. Developers point their testnet Horizon client at a local proxy and select repeatable fault scenarios. The tool forwards real signed testnet transactions but can deliberately drop submission responses, hide confirmation temporarily, or reject requests before broadcast. A demo runner exercises a deliberately naive client and a transaction-journaled client under identical scenarios. Reports correlate submission attempts, hashes and actual ledger settlement; they do not claim to prove all properties of arbitrary applications.

Core demo: the first transfer settles, its confirmation is dropped, and a naive client creates a second payment. The report shows both hashes and the verified payment total. A corrected client reconciles or resubmits the original transaction and settles only once. This is controlled fault injection into the application's connection, not a Stellar network outage or a fault in consensus.

Closest visible hackathon entries: Soroban Studio (visual workflow creation) and Xlm (basic developer CLI). Neither public description mentions payment failure injection, ambiguous settlement recovery or duplicate-payout testing. AgentAllowance and AegisOS concern spending authorization and purchase provenance rather than transport failure recovery.

Evidence that the problem occurs in real projects: MergePay issue #101 explicitly requests idempotent settlement across client retries, worker retries and network timeouts. Source: https://github.com/mergepay/mergepay-api/issues/101

Broader adjacent tools: Erst replays failed Soroban contract transactions for debugging (https://github.com/dotandev/hintents); PaymentSimulator tests conventional gateway events (https://paymentsimulator.com/); Switchbench advertises synthetic payment safety scenarios (https://switchbench.com/). The proposed distinction is Stellar-specific application fault injection around real testnet payments and ledger-backed duplicate settlement evidence. Search found no exact match; this is not proof that none exists.

## MVP boundary

Go local Horizon proxy, Svelte scenario dashboard, SQLite run journal, fixed Stellar testnet upstream, and two example clients. Three scenarios: failure before submission, confirmation lost after settlement, and delayed transaction lookup. Each run exports attempts, timestamps, transaction hashes and observed payment totals. V1 uses native XLM payments funded by Friendbot for reproducible setup. USDC fixtures remain a possible later extension. No custom contract or Rust required. No AI component, production network mode or universal security certification. A local integration command and CI-friendly run report make this useful beyond the hosted demo.

## Rejected or weaker directions

- Generic payment links: existing wallets cover the basic feature; Cosmos Wallet publicly lists payment intents/links.
- Freelancer invoices: Honorarios covers invoices and tax-reserve splitting.
- Milestone escrow: Breadline Protocol directly covers this flow.
- Agent budgets/micropayments: AgentAllowance, Axon, AegisOS, CodeZard and Local402 occupy adjacent territory.
- Sponsored onboarding: WalletNow explicitly covers sponsored transactions and passkeys.
- Cooperative bulk buying: absent as an explicit focus in the public hackathon descriptions, but Kudu advertises Stellar escrow, group-buy thresholds and falling prices. Source: https://kudu.cash/products/group-buying . Generic crowdfunding also exists in Stellar Goal Vault and StellarRaise. Do not pitch this as a new category.

## Status

LedgerDrill was selected and implemented as the local v1 in this repository. All three reference scenarios passed real testnet verification; see [the evidence](docs/VERIFICATION.md). LinkPay implementation remains stopped and its original plans are preserved separately. Public submission descriptions were reviewed; entrant implementation claims were not independently tested. Repeat the public-entry check before submission because more projects may appear.
