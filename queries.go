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
