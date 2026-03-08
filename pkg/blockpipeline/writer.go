/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

// BlockPersister is the interface satisfied by db.BlockWriter.
type BlockPersister interface {
	WriteProcessedBlock(ctx context.Context, pb *types.ProcessedBlock) error
}

// BlockWriter persists processed blocks to the database.
func BlockWriter(ctx context.Context, persister BlockPersister, in <-chan *types.ProcessedBlock) error {
	logger.Info("blockWriter started")
	err := drainChan(ctx, in, "processedBlocks", func(pb *types.ProcessedBlock) error {
		if err := persister.WriteProcessedBlock(ctx, pb); err != nil {
			return errors.Wrap(err, "db write error")
		}
		return nil
	})
	if err != nil {
		logger.Warnf("blockWriter stopped: %v", err)
	} else {
		logger.Info("blockWriter stopping")
	}
	return err
}
