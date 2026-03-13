/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-x-committer/utils/channel"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// blockWriter persists processed blocks to the database.
// It returns ctx.Err() on clean shutdown and a non-nil error on failure.
func blockWriter(ctx context.Context, persister *db.BlockWriter, in channel.Reader[*types.ProcessedBlock]) error {
	logger.Info("blockWriter started")
	for ctx.Err() == nil {
		pb, ok := in.Read()
		if !ok {
			// in closed: all processors exited cleanly or ctx cancelled.
			return ctx.Err()
		}
		if pb == nil {
			continue
		}
		if err := persister.WriteProcessedBlock(ctx, pb); err != nil {
			logger.Warnf("blockWriter stopped: %v", err)
			return errors.Wrap(err, "db write error")
		}
	}
	logger.Info("blockWriter stopping")
	return ctx.Err()
}
