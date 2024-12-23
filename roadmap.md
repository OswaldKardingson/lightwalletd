# Development Roadmap

## Phase 1: Cryptographic Core
1. **Adopt Zcash’s Libraries**:
   - Integrate Zcash's **Bellman** and **Halo 2** libraries using **Rust FFI** for zk-SNARK proof generation and verification.
   - Use precompiled Rust modules to offload computationally heavy zk-SNARK operations while keeping litewalletd in Go.

2. **Optimize ZK Circuits for Pirate**:
   - Remove unnecessary logic for transparent transactions.
   - Tailor zk-SNARK circuits to Pirate’s shielded-only model to improve efficiency.
   - Use GPU acceleration in Rust libraries for proof generation.

3. **Develop Stateless Witness Sync**:
   - Implement Merkle Frontiers in Go for lightweight syncing.
   - Use Rust FFI to perform cryptographic operations like Merkle proof generation and verification efficiently.

---

## Phase 2: Litewalletd Enhancements (Go-Based)
1. **Edit litewalletd**:
   - Simplify litewalletd by removing code paths for transparent address handling.
   - Maintain compatibility with Pirate’s fully shielded transaction model.

2. **WebSocket Server Support**:
   - Implement WebSocket support in litewalletd using Go libraries like **github.com/gorilla/websocket**.
   - Serve real-time state updates and transaction confirmations to connected wallets.

3. **Transport Privacy Enhancements**:
   - Add Tor/i2p support for anonymous communication.
   - Encrypt all WebSocket and REST API communications using robust encryption protocols (e.g., TLS with Perfect Forward Secrecy).

4. **State Caching**:
   - Implement server-side encrypted caching for commonly requested proofs and Merkle tree data to reduce computation for frequent sync requests.

5. **Address Sync Issues on iOS/macOS**:
   - Introduce lightweight checkpoints to optimize wallet synchronization.
   - Use WebSocket-based real-time updates to prevent sync stalls.
   - Enhance error handling to recover from interruptions without leaking metadata.

---

## Phase 3: Wallet Enhancements
1. **Cross-Platform Wallets**:
   - Compile zk-SNARK generation and verification libraries (using Rust) into WebAssembly (Wasm) for browser wallets.
   - Optimize wallet performance for low-resource devices, ensuring smooth operation on mobile.

2. **Seed Phrase Recovery**:
   - Enforce strict BIP-39/SLIP-39 compliance for seed phrase formatting.
   - Add user-friendly error detection for recovery phrases.

3. **Privacy-Enhancing Features**:
   - Implement local biometric encryption (e.g., FaceID, fingerprint) for secure wallet access without compromising privacy.

4. **Address Book Feature**:
   - Add an address book tab to wallets, allowing users to save addresses and names for future use.

5. **UI/UX Improvements**:
   - Refactor UI/UX for cross-platform compatibility, ensuring consistent performance on mobile, desktop, and macOS/iOS devices.

---

## Phase 4: Performance and Privacy Optimization
1. **Network Optimization**:
   - Use compression techniques like `gzip` or `snappy` for proofs and state data exchanged between litewalletd and wallets.
   - Optimize WebSocket communication for minimal bandwidth use.

2. **Auditing and Security**:
   - Conduct third-party audits of updated litewalletd and integrated Rust modules to ensure privacy and security.
   - Build a comprehensive test suite to detect metadata leaks or performance regressions.

3. **Batch Proof Verification**:
   - Implement batch proof verification for shielded transactions to reduce on-chain verification costs.

4. **Enhanced Logging and Debugging**:
   - Add detailed logging for sync processes, ensuring issues can be identified and resolved without compromising user privacy.

---

## Phase 5: Ecosystem Expansion
1. **Browser Wallets**:
   - Finalize WebAssembly-compatible zk-SNARK libraries for browser wallets.

2. **Donation Features**:
   - Add a pre-configured “donate” button in the wallet for development support.

3. **Hardware Wallet Integration**:
   - Support hardware wallets like Ledger/Trezor for signing shielded transactions using Rust libraries.

4. **Fiat Onboarding**:
   - Explore integrating fiat-to-crypto onboarding through providers like **DFX.swiss**

5. **PiratePay**:
   - Develop a PiratePay tab to purchase gift cards or prepaid Visa cards using Pirate Chain (leveraging services like Codego).

6. **Atomic Swaps and Exchanges**:
   - Investigate implementing atomic swaps and ARRR-BTC exchanges using Komodo Wallet’s MM2.
