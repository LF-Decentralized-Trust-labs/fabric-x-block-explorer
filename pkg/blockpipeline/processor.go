/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/logging"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/parser"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = logging.New("blockpipeline")

// BlockProcessor parses raw blocks and sends them to the output channel.
// It returns nil on clean shutdown (ctx cancelled) and an error on failure.
func BlockProcessor(
	ctx context.Context,
	in <-chan *common.Block,
	out chan<- *types.ProcessedBlock,
) error {
	logger.Info("blockProcessor started")
	err := drainChan(ctx, in, "receivedBlocks", func(blk *common.Block) error {
		processed, err := processBlock(blk)
		if err != nil {
			return errors.Wrapf(err, "block processing error")
		}
		select {
		case <-ctx.Done():
			return nil
		case out <- processed:
			return nil
		}
	})
	if err != nil {
		logger.Warnf("blockProcessor stopped: %v", err)
	} else {
		logger.Info("blockProcessor stopping")
	}
	return err
}

// closedChannelErr returns an error if name channel closed while ctx is still active
// (unexpected), or nil if it closed during a normal ctx-cancellation shutdown.
func closedChannelErr(ctx context.Context, name string) error {
	if ctx.Err() == nil {
		return errors.Newf("%s channel closed unexpectedly", name)
	}
	return nil
}

// drainChan reads items from in, skips zero/nil values, and calls handle for each.
// It returns nil on clean ctx shutdown and an error if in closes unexpectedly or handle fails.
func drainChan[T comparable](
	ctx context.Context,
	in <-chan T,
	chanName string,
	handle func(T) error,
) error {
	var zero T
	for {
		select {
		case <-ctx.Done():
			return nil
		case item, ok := <-in:
			if !ok {
				return closedChannelErr(ctx, chanName)
			}
			if item == zero {
				continue
			}
			if err := handle(item); err != nil {
				return err
			}
		}
	}
}

// processBlock parses a raw block into structured data.
func processBlock(blk *common.Block) (*types.ProcessedBlock, error) {
	parsedData, blockInfo, err := parser.Parse(blk)
	if err != nil {
		return nil, err
	}
	return &types.ProcessedBlock{
		Data:      parsedData,
		BlockInfo: blockInfo,
	}, nil
}
