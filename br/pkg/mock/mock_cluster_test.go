// Copyright 2020 PingCAP, Inc. Licensed under Apache-2.0.

package mock_test

import (
	"testing"

	"github.com/tikv/migration/br/pkg/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSmoke(t *testing.T) {
	defer goleak.VerifyNone(
		t,
		goleak.IgnoreTopFunction("github.com/klauspost/compress/zstd.(*blockDec).startDecoder"),
		goleak.IgnoreTopFunction("go.etcd.io/etcd/pkg/logutil.(*MergeLogger).outputLoop"),
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	m, err := mock.NewCluster()
	require.NoError(t, err)
	require.NoError(t, m.Start())
	m.Stop()
}
