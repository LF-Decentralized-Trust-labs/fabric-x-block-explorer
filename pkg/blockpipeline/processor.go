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

// blockProcessor parses raw blocks and sends them to the output channel.
// It returns ctx.Err() on clean shutdown and a non-nil error on failure.
func blockProcessor(
	ctx context.Context,
	in channel.Reader[*common.Block],
	out channel.Writer[*types.ProcessedBlock],
) error {
	logger.Info("blockProcessor started")
	for ctx.Err() == nil {
		inputBlock, ok := in.Read()
		if !ok {
			// in closed: upstream receiver exited cleanly or ctx cancelled.
			return ctx.Err()
		}
		if inputBlock == nil {
			continue
		}
		parsedData, blockInfo, err := parser.Parse(inputBlock)
		if err != nil {
			logger.Warnf("blockProcessor stopped: %v", err)
			return errors.Wrapf(err, "block processing error")
		}
		processedBlock := &types.ProcessedBlock{
			Data:      parsedData,
			BlockInfo: blockInfo,
		}
		out.Write(processedBlock)
	}
	logger.Info("blockProcessor stopping")
	return ctx.Err()
}
