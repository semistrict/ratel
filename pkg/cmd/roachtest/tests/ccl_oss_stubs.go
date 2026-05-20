// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tests

import (
	"context"
	gosql "database/sql"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/cluster"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/registry"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/test"
)

var envVars []string

func registerCDC(registry.Registry)                          {}
func registerCDCMixedVersions(registry.Registry)             {}
func registerClusterToCluster(registry.Registry)             {}
func registerClusterReplicationResilience(registry.Registry) {}
func registerClusterReplicationDisconnect(registry.Registry) {}

func runAcceptanceClusterReplication(context.Context, test.Test, cluster.Cluster) {}

func stopFeeds(*gosql.DB) {}
