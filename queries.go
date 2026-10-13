package mina

// GraphQL query and mutation strings for the Mina daemon.
//
// These are the documents of spec/operations.graphql, the common API of the
// Mina SDKs (see spec/SPEC.md). spec/ is a copy of o1-labs/mina-sdk-spec at
// the tag in spec/VERSION; spec_test.go checks that these documents stay
// identical. Change the specification first.

const querySyncStatus = `query SyncStatus {
  syncStatus
}`

const queryDaemonStatus = `query DaemonStatus {
  daemonStatus {
    syncStatus
    blockchainLength
    highestBlockLengthReceived
    highestUnvalidatedBlockLengthReceived
    uptimeSecs
    stateHash
    commitId
    numAccounts
    ledgerMerkleRoot
    chainId
    catchupStatus
    blockProductionKeys
    coinbaseReceiver
    globalSlotSinceGenesisBestTip
    consensusTimeNow {
      epoch
      slot
      globalSlot
      startTime
      endTime
    }
    peers {
      peerId
      host
      libp2pPort
    }
    addrsAndPorts {
      externalIp
      bindIp
      clientPort
      libp2pPort
      peer {
        host
        libp2pPort
        peerId
      }
    }
  }
}`

const queryDaemonMetrics = `query DaemonMetrics {
  daemonStatus {
    metrics {
      blockProductionDelay
      transactionPoolDiffReceived
      transactionPoolDiffBroadcasted
      transactionsAddedToPool
      transactionPoolSize
      snarkPoolDiffReceived
      snarkPoolDiffBroadcasted
      pendingSnarkWork
      snarkPoolSize
    }
  }
}`

const queryNetworkID = `query NetworkId {
  networkID
}`

const queryGetAccount = `query Account($publicKey: PublicKey!, $token: TokenId) {
  account(publicKey: $publicKey, token: $token) {
    publicKey
    nonce
    delegate
    tokenId
    tokenSymbol
    votingFor
    receiptChainHash
    balance {
      total
      liquid
      locked
      blockHeight
    }
    timing {
      initialMinimumBalance
      cliffTime
      cliffAmount
      vestingPeriod
      vestingIncrement
    }
    permissions {
      editState
      send
      receive
      access
      setDelegate
      setPermissions
      setVerificationKey {
        auth
        txnVersion
      }
      setZkappUri
      editActionState
      setTokenSymbol
      incrementNonce
      setVotingFor
      setTiming
    }
    zkappState
    provedState
    zkappUri
  }
}`

const queryBestChain = `query BestChain($maxLength: Int) {
  bestChain(maxLength: $maxLength) {
    stateHash
    commandTransactionCount
    creatorAccount {
      publicKey
    }
    protocolState {
      previousStateHash
      consensusState {
        blockHeight
        epoch
        slot
        slotSinceGenesis
        blockCreator
        coinbaseReceiever
        stakingEpochData {
          epochLength
          seed
          ledger {
            hash
          }
        }
        nextEpochData {
          seed
          ledger {
            hash
          }
        }
      }
      blockchainState {
        date
        utcDate
        snarkedLedgerHash
        stagedLedgerHash
      }
    }
    snarkJobs {
      prover
      fee
      workIds
    }
    transactions {
      coinbase
      coinbaseReceiverAccount {
        publicKey
      }
      feeTransfer {
        recipient
        fee
        type
      }
      userCommands {
        id
        hash
        kind
        nonce
        source {
          publicKey
        }
        receiver {
          publicKey
        }
        amount
        fee
        memo
        failureReason
      }
      zkappCommands {
        id
        hash
        failureReason {
          index
          failures
        }
        zkappCommand {
          memo
          feePayer {
            authorization
            body {
              publicKey
              fee
              nonce
              validUntil
            }
          }
          accountUpdates {
            authorization {
              signature
              proof
            }
            body {
              publicKey
              tokenId
              callDepth
              callData
              actions
              events
              incrementNonce
              useFullCommitment
              implicitAccountCreationFee
              balanceChange {
                magnitude
                sgn
              }
              mayUseToken {
                parentsOwnToken
                inheritFromParent
              }
              authorizationKind {
                isSigned
                isProved
                verificationKeyHash
              }
              update {
                appState
                delegate
                zkappUri
                votingFor
                tokenSymbol
                verificationKey {
                  hash
                  data
                }
                timing {
                  initialMinimumBalance
                  cliffTime
                  cliffAmount
                  vestingPeriod
                  vestingIncrement
                }
                permissions {
                  editState
                  send
                  receive
                  access
                  setDelegate
                  setPermissions
                  setVerificationKey {
                    auth
                    txnVersion
                  }
                  setZkappUri
                  editActionState
                  setTokenSymbol
                  incrementNonce
                  setVotingFor
                  setTiming
                }
              }
              preconditions {
                validWhile {
                  lower
                  upper
                }
                network {
                  snarkedLedgerHash
                  blockchainLength {
                    lower
                    upper
                  }
                  minWindowDensity {
                    lower
                    upper
                  }
                  totalCurrency {
                    lower
                    upper
                  }
                  globalSlotSinceGenesis {
                    lower
                    upper
                  }
                  stakingEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                  nextEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                }
                account {
                  balance {
                    lower
                    upper
                  }
                  nonce {
                    lower
                    upper
                  }
                  receiptChainHash
                  delegate
                  state
                  actionState
                  provedState
                  isNew
                }
              }
            }
          }
        }
      }
    }
  }
}`

const queryGenesisBlock = `query GenesisBlock {
  genesisBlock {
    stateHash
    commandTransactionCount
    creatorAccount {
      publicKey
    }
    protocolState {
      previousStateHash
      consensusState {
        blockHeight
        epoch
        slot
        slotSinceGenesis
        blockCreator
        coinbaseReceiever
        stakingEpochData {
          epochLength
          seed
          ledger {
            hash
          }
        }
        nextEpochData {
          seed
          ledger {
            hash
          }
        }
      }
      blockchainState {
        date
        utcDate
        snarkedLedgerHash
        stagedLedgerHash
      }
    }
    snarkJobs {
      prover
      fee
      workIds
    }
    transactions {
      coinbase
      coinbaseReceiverAccount {
        publicKey
      }
      feeTransfer {
        recipient
        fee
        type
      }
      userCommands {
        id
        hash
        kind
        nonce
        source {
          publicKey
        }
        receiver {
          publicKey
        }
        amount
        fee
        memo
        failureReason
      }
      zkappCommands {
        id
        hash
        failureReason {
          index
          failures
        }
        zkappCommand {
          memo
          feePayer {
            authorization
            body {
              publicKey
              fee
              nonce
              validUntil
            }
          }
          accountUpdates {
            authorization {
              signature
              proof
            }
            body {
              publicKey
              tokenId
              callDepth
              callData
              actions
              events
              incrementNonce
              useFullCommitment
              implicitAccountCreationFee
              balanceChange {
                magnitude
                sgn
              }
              mayUseToken {
                parentsOwnToken
                inheritFromParent
              }
              authorizationKind {
                isSigned
                isProved
                verificationKeyHash
              }
              update {
                appState
                delegate
                zkappUri
                votingFor
                tokenSymbol
                verificationKey {
                  hash
                  data
                }
                timing {
                  initialMinimumBalance
                  cliffTime
                  cliffAmount
                  vestingPeriod
                  vestingIncrement
                }
                permissions {
                  editState
                  send
                  receive
                  access
                  setDelegate
                  setPermissions
                  setVerificationKey {
                    auth
                    txnVersion
                  }
                  setZkappUri
                  editActionState
                  setTokenSymbol
                  incrementNonce
                  setVotingFor
                  setTiming
                }
              }
              preconditions {
                validWhile {
                  lower
                  upper
                }
                network {
                  snarkedLedgerHash
                  blockchainLength {
                    lower
                    upper
                  }
                  minWindowDensity {
                    lower
                    upper
                  }
                  totalCurrency {
                    lower
                    upper
                  }
                  globalSlotSinceGenesis {
                    lower
                    upper
                  }
                  stakingEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                  nextEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                }
                account {
                  balance {
                    lower
                    upper
                  }
                  nonce {
                    lower
                    upper
                  }
                  receiptChainHash
                  delegate
                  state
                  actionState
                  provedState
                  isNew
                }
              }
            }
          }
        }
      }
    }
  }
}`

const queryBlock = `query Block($stateHash: String, $height: Int) {
  block(stateHash: $stateHash, height: $height) {
    stateHash
    commandTransactionCount
    creatorAccount {
      publicKey
    }
    protocolState {
      previousStateHash
      consensusState {
        blockHeight
        epoch
        slot
        slotSinceGenesis
        blockCreator
        coinbaseReceiever
        stakingEpochData {
          epochLength
          seed
          ledger {
            hash
          }
        }
        nextEpochData {
          seed
          ledger {
            hash
          }
        }
      }
      blockchainState {
        date
        utcDate
        snarkedLedgerHash
        stagedLedgerHash
      }
    }
    snarkJobs {
      prover
      fee
      workIds
    }
    transactions {
      coinbase
      coinbaseReceiverAccount {
        publicKey
      }
      feeTransfer {
        recipient
        fee
        type
      }
      userCommands {
        id
        hash
        kind
        nonce
        source {
          publicKey
        }
        receiver {
          publicKey
        }
        amount
        fee
        memo
        failureReason
      }
      zkappCommands {
        id
        hash
        failureReason {
          index
          failures
        }
        zkappCommand {
          memo
          feePayer {
            authorization
            body {
              publicKey
              fee
              nonce
              validUntil
            }
          }
          accountUpdates {
            authorization {
              signature
              proof
            }
            body {
              publicKey
              tokenId
              callDepth
              callData
              actions
              events
              incrementNonce
              useFullCommitment
              implicitAccountCreationFee
              balanceChange {
                magnitude
                sgn
              }
              mayUseToken {
                parentsOwnToken
                inheritFromParent
              }
              authorizationKind {
                isSigned
                isProved
                verificationKeyHash
              }
              update {
                appState
                delegate
                zkappUri
                votingFor
                tokenSymbol
                verificationKey {
                  hash
                  data
                }
                timing {
                  initialMinimumBalance
                  cliffTime
                  cliffAmount
                  vestingPeriod
                  vestingIncrement
                }
                permissions {
                  editState
                  send
                  receive
                  access
                  setDelegate
                  setPermissions
                  setVerificationKey {
                    auth
                    txnVersion
                  }
                  setZkappUri
                  editActionState
                  setTokenSymbol
                  incrementNonce
                  setVotingFor
                  setTiming
                }
              }
              preconditions {
                validWhile {
                  lower
                  upper
                }
                network {
                  snarkedLedgerHash
                  blockchainLength {
                    lower
                    upper
                  }
                  minWindowDensity {
                    lower
                    upper
                  }
                  totalCurrency {
                    lower
                    upper
                  }
                  globalSlotSinceGenesis {
                    lower
                    upper
                  }
                  stakingEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                  nextEpochData {
                    seed
                    startCheckpoint
                    lockCheckpoint
                    epochLength {
                      lower
                      upper
                    }
                    ledger {
                      hash
                      totalCurrency {
                        lower
                        upper
                      }
                    }
                  }
                }
                account {
                  balance {
                    lower
                    upper
                  }
                  nonce {
                    lower
                    upper
                  }
                  receiptChainHash
                  delegate
                  state
                  actionState
                  provedState
                  isNew
                }
              }
            }
          }
        }
      }
    }
  }
}`

const queryGetPeers = `query Peers {
  getPeers {
    peerId
    host
    libp2pPort
  }
}`

const queryPooledUserCommands = `query PooledUserCommands($publicKey: PublicKey) {
  pooledUserCommands(publicKey: $publicKey) {
    id
    hash
    kind
    nonce
    amount
    fee
    validUntil
    token
    feeToken
    from
    to
    source {
      publicKey
    }
    receiver {
      publicKey
    }
    memo
    failureReason
  }
}`

const queryPooledZkappCommands = `query PooledZkappCommands($publicKey: PublicKey) {
  pooledZkappCommands(publicKey: $publicKey) {
    id
    hash
    zkappCommand {
      memo
      feePayer {
        body {
          publicKey
          fee
          nonce
          validUntil
        }
      }
    }
    failureReason {
      index
      failures
    }
  }
}`

const queryTransactionStatus = `query TransactionStatus($payment: ID, $zkappTransaction: ID) {
  transactionStatus(payment: $payment, zkappTransaction: $zkappTransaction)
}`

const queryGenesisConstants = `query GenesisConstants {
  genesisConstants {
    genesisTimestamp
    coinbase
    accountCreationFee
  }
}`

const queryTrackedAccounts = `query TrackedAccounts {
  trackedAccounts {
    publicKey
    balance {
      total
    }
  }
}`

const querySnarkPool = `query SnarkPool {
  snarkPool {
    fee
    prover
    workIds
  }
}`

const queryForkConfig = `query ForkConfig {
  fork_config
}`

const mutationSendPayment = `mutation SendPayment($input: SendPaymentInput!, $signature: SignatureInput) {
  sendPayment(input: $input, signature: $signature) {
    payment {
      id
      hash
      kind
      nonce
      source {
        publicKey
      }
      receiver {
        publicKey
      }
      amount
      fee
      memo
    }
  }
}`

const mutationSendDelegation = `mutation SendDelegation($input: SendDelegationInput!, $signature: SignatureInput) {
  sendDelegation(input: $input, signature: $signature) {
    delegation {
      id
      hash
      kind
      nonce
      source {
        publicKey
      }
      receiver {
        publicKey
      }
      amount
      fee
      memo
    }
  }
}`

const mutationSendZkapp = `mutation SendZkapp($input: SendZkappInput!) {
  sendZkapp(input: $input) {
    zkapp {
      id
      hash
      zkappCommand {
        memo
        feePayer {
          body {
            publicKey
            fee
            nonce
            validUntil
          }
        }
      }
      failureReason {
        index
        failures
      }
    }
  }
}`

const mutationUnlockAccount = `mutation UnlockAccount($input: UnlockInput!) {
  unlockAccount(input: $input) {
    publicKey
  }
}`

const mutationSetSnarkWorker = `mutation SetSnarkWorker($input: SetSnarkWorkerInput!) {
  setSnarkWorker(input: $input) {
    lastSnarkWorker
  }
}`

const mutationSetSnarkWorkFee = `mutation SetSnarkWorkFee($fee: UInt64!) {
  setSnarkWorkFee(input: {fee: $fee}) {
    lastFee
  }
}`
