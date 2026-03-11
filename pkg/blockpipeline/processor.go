/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpipeline

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-x-committer/utils/channel"
	"github.com/hyperledger/fabric-x-committer/utils/logging"

	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/parser"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
)

var logger = logging.New("blockpipeline")

// BlockProcessor parses raw blocks and sends them to the output channel.
// It returns ctx.Err() on clean shutdown and a non-nil error on failure.
func BlockProcessor(
	ctx context.Context,
	in <-chan *common.Block,
	out chan<- *types.ProcessedBlock,
) error {
	logger.Info("blockProcessor started")
	reader := channel.NewReader(ctx, in)
	writer := channel.NewWriter(ctx, out)
	for ctx.Err() == nil {
		blk, ok := reader.Read()
		if !ok {
			// in closed: upstream receiver exited cleanly or ctx cancelled.
			return ctx.Err()
		}
		if blk == nil {
			continue
		}
		parsedData, blockInfo, err := parser.Parse(blk)
		if err != nil {
			logger.Warnf("blockProcessor stopped: %v", err)
			return errors.Wrapf(err, "block processing error")
		}
		processed := &types.ProcessedBlock{
			Data:      parsedData,
			BlockInfo: blockInfo,
		}
		if !writer.Write(processed) {
			return ctx.Err()
		}
	}
	logger.Info("blockProcessor stopping")
	return ctx.Err()
}
