package mina_test

import (
	"testing"

	mina "github.com/MinaProtocol/mina-sdk-go"
	"github.com/MinaProtocol/mina-sdk-go/itn"
)

// TestITNQueriesAreTheSpecDocuments checks the ITN query strings against
// spec/itn-operations.graphql.
func TestITNQueriesAreTheSpecDocuments(t *testing.T) {
	mina.AssertDocumentsAreTheSpec(t, "spec/itn-operations.graphql", []string{
		itn.QueryAuth,
		itn.QuerySlotsWon,
		itn.QueryInternalLogs,
		itn.MutationFlushInternalLogs,
		itn.MutationSchedulePayments,
		itn.MutationScheduleZkappCommands,
		itn.MutationStopScheduledTransactions,
		itn.MutationUpdateGating,
		itn.MutationStopDaemon,
		itn.MutationZkappCommandLimit,
		itn.QueryCommitID,
		itn.QueryScheduledTransactions,
		itn.MutationSchedulePaymentsWithHandle,
		itn.MutationScheduleZkappCommandsWithHandle,
		itn.MutationCreateAccounts,
	})
}
