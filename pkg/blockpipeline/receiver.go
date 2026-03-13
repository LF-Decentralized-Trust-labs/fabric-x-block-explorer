/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/sidecarstream"
)

// blockReceiver runs one streaming session: it drives Deliver and forwards every
// block it produces directly to out.
//
// It returns nil when Deliver exits cleanly (e.g. EndBlk reached) and a
// non-nil error on failure or context cancellation.
// Reconnection on transient errors is the caller's responsibility.
func blockReceiver(ctx context.Context, streamer *sidecarstream.Streamer, out channel.Writer[*common.Block]) error {
	logger.Info("blockReceiver started")
	return streamer.Deliver(ctx, out)
}
