package mina

// GraphQL query and mutation strings for the Mina daemon.

const querySyncStatus = `
query {
    syncStatus
}
`

const queryDaemonStatus = `
query {
    daemonStatus {
        syncStatus
        blockchainLength
        highestBlockLengthReceived
        uptimeSecs
        stateHash
        commitId
        peers {
            peerId
            host
            libp2pPort
        }
    }
}
`

const queryNetworkID = `
query {
    networkID
}
`

// queryGetAccount fetches a single account. The token argument is optional:
// $token is a nullable TokenId, and when no token variable is supplied the
// daemon resolves the default MINA token.
const queryGetAccount = `
query ($publicKey: PublicKey!, $token: TokenId) {
    account(publicKey: $publicKey, token: $token) {
        publicKey
        nonce
        delegate
        tokenId
        balance {
            total
            liquid
            locked
        }
    }
}
`

const queryBestChain = `
query ($maxLength: Int) {
    bestChain(maxLength: $maxLength) {
        stateHash
        commandTransactionCount
        creatorAccount {
            publicKey
        }
        protocolState {
            consensusState {
                blockHeight
                slotSinceGenesis
                slot
            }
        }
    }
}
`

const queryGetPeers = `
query {
    getPeers {
        peerId
        host
        libp2pPort
    }
}
`

// queryPooledUserCommands lists pending user commands. The publicKey argument
// is optional: $publicKey is a nullable PublicKey, and when no variable is
// supplied the daemon returns commands for every sender.
const queryPooledUserCommands = `
query ($publicKey: PublicKey) {
    pooledUserCommands(publicKey: $publicKey) {
        id
        hash
        kind
        nonce
        amount
        fee
        from
        to
    }
}
`

const mutationSendPayment = `
mutation ($input: SendPaymentInput!) {
    sendPayment(input: $input) {
        payment {
            id
            hash
            nonce
        }
    }
}
`

const mutationSendDelegation = `
mutation ($input: SendDelegationInput!) {
    sendDelegation(input: $input) {
        delegation {
            id
            hash
            nonce
        }
    }
}
`

const mutationSetSnarkWorker = `
mutation ($input: SetSnarkWorkerInput!) {
    setSnarkWorker(input: $input) {
        lastSnarkWorker
    }
}
`

const mutationSetSnarkWorkFee = `
mutation ($fee: UInt64!) {
    setSnarkWorkFee(input: {fee: $fee}) {
        lastFee
    }
}
`
