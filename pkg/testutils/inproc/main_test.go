// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package inproc_test

import (
	"os"
	"os/exec"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

const syncTestChildEnv = "COCKROACH_INPROC_SYNC_TEST_CHILD"

var syncTestNames = []string{
	"TestSyncInprocSmoke",
	"TestSyncTestSmoke",
	"TestSyncFakeTime",
	"TestSyncRestart",
	"TestSyncNetworkPartition",
	"TestSyncClockJump",
	"TestSyncDecommission",
	"TestSyncJepsenBankSplit",
	"TestSyncJepsenBankRestart",
	"TestSyncJepsenBankPartition",
	"TestSyncJepsenBankParts",
	"TestSyncJepsenBankMajorityRing",
	"TestSyncJepsenBankPartsRestart",
	"TestSyncJepsenBankMajorityRingRestart",
	"TestSyncJepsenCommentsSplit",
	"TestSyncJepsenCommentsRestart",
	"TestSyncJepsenCommentsParts",
	"TestSyncJepsenCommentsMajorityRing",
	"TestSyncJepsenCommentsPartsRestart",
	"TestSyncJepsenCommentsMajorityRingRestart",
	"TestSyncJepsenRegister",
	"TestSyncJepsenRegisterRestart",
	"TestSyncJepsenRegisterSplit",
	"TestSyncJepsenRegisterPartition",
	"TestSyncJepsenRegisterParts",
	"TestSyncJepsenRegisterMajorityRing",
	"TestSyncJepsenRegisterPartsRestart",
	"TestSyncJepsenRegisterMajorityRingRestart",
	"TestSyncJepsenSequentialSplit",
	"TestSyncJepsenSequentialRestart",
	"TestSyncJepsenSequentialParts",
	"TestSyncJepsenSequentialMajorityRing",
	"TestSyncJepsenSequentialPartsRestart",
	"TestSyncJepsenSequentialMajorityRingRestart",
	"TestSyncJepsenSetsSplit",
	"TestSyncJepsenSetsRestart",
	"TestSyncJepsenSetsParts",
	"TestSyncJepsenSetsMajorityRing",
	"TestSyncJepsenSetsPartsRestart",
	"TestSyncJepsenSetsMajorityRingRestart",
	"TestSyncLivenessPollerAdvancesFakeTime",
	"TestSyncLivenessPollerDetectsExpiry",
	"TestLeaseTransferAfterAddVotersSynctest",
	"TestSyncAddVoters",
	"TestSyncAutoReplication",
	"TestSyncRaftRestartWithReplication",
	"TestSyncChaosShutdown",
	"TestSyncChaosBankOnly",
	"TestSyncChaosKillAndAdd",
}

func init() {
	securityassets.SetLoader(securitytest.EmbeddedAssets)
}

func TestMain(m *testing.M) {
	randutil.SeedForTests()
	serverutils.InitTestServerFactory(server.TestServerFactory)
	serverutils.InitTestClusterFactory(testcluster.TestClusterFactory)
	// Disable the race transport's background goroutine. Its non-durable
	// select prevents synctest from detecting quiescence.
	kvcoord.DisableRaceTransport = true
	if os.Getenv(syncTestChildEnv) == "" {
		for _, name := range syncTestNames {
			_, _ = os.Stderr.WriteString("running " + name + "\n")
			cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$")
			cmd.Env = append(os.Environ(), syncTestChildEnv+"=1")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				os.Exit(1)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runSyncTest(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	if os.Getenv(syncTestChildEnv) == "" {
		t.Skip("synctest cluster tests run one per subprocess from TestMain")
	}
	synctest.Test(t, fn)
}
