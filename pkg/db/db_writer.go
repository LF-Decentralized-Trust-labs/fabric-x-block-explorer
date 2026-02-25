/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var logger = flogging.MustGetLogger("db")

type BlockWriter struct {
	pool *pgxpool.Pool
	conn *pgxpool.Conn
}

func NewBlockWriter(pool *pgxpool.Pool) *BlockWriter {
	return &BlockWriter{pool: pool}
}

func NewBlockWriterFromConn(conn *pgxpool.Conn) *BlockWriter {
	return &BlockWriter{conn: conn}
}

func (bw *BlockWriter) WriteProcessedBlock(ctx context.Context, pb *types.ProcessedBlock) error {
	if pb == nil {
		return errors.New("processed block is nil")
	}

	parsedData, ok := pb.Data.(*types.ParsedBlockData)
	if !ok {
		return errors.New("processed block Data is not *types.ParsedBlockData")
	}

	var (
		tx  pgx.Tx
		err error
	)
	if bw.conn != nil {
		tx, err = bw.conn.Begin(ctx)
	} else if bw.pool != nil {
		tx, err = bw.pool.Begin(ctx)
	} else {
		return errors.New("no pool or conn available in BlockWriter")
	}
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	q := dbsqlc.New(tx)

	// Round-trip 1: insert the block header.
	if err := q.InsertBlock(ctx, dbsqlc.InsertBlockParams{
		BlockNum:     int64(pb.BlockInfo.Number),
		TxCount:      int32(pb.Txns),
		PreviousHash: pb.BlockInfo.PreviousHash,
		DataHash:     pb.BlockInfo.DataHash,
	}); err != nil {
		return err
	}

	// Build flat param slices for all six batch inserts in a single pass.
	txParams := make([]dbsqlc.InsertTransactionParams, 0, len(parsedData.Transactions))
	var nsParams []dbsqlc.InsertTxNamespaceParams
	var readParams []dbsqlc.InsertTxReadParams
	var endorseParams []dbsqlc.InsertTxEndorsementParams
	var writeParams []dbsqlc.InsertTxWriteParams
	var policyParams []dbsqlc.UpsertNamespacePolicyParams

	for _, txRec := range parsedData.Transactions {
		txIDBytes, err := hex.DecodeString(txRec.TxID)
		if err != nil {
			return fmt.Errorf("failed to decode tx_id %s: %w", txRec.TxID, err)
		}
		txParams = append(txParams, dbsqlc.InsertTransactionParams{
			BlockNum:       int64(txRec.BlockNum),
			TxNum:          int64(txRec.TxNum),
			TxID:           txIDBytes,
			ValidationCode: int64(txRec.ValidationCode),
		})

		for _, ns := range txRec.Namespaces {
			nsParams = append(nsParams, dbsqlc.InsertTxNamespaceParams{
				BlockNum:  int64(txRec.BlockNum),
				TxNum:     int64(txRec.TxNum),
				NsID:      ns.NsID,
				NsVersion: int64(ns.NsVersion),
			})
			for _, r := range ns.Reads {
				readParams = append(readParams, dbsqlc.InsertTxReadParams{
					BlockNum:    int64(txRec.BlockNum),
					TxNum:       int64(txRec.TxNum),
					NsID:        ns.NsID,
					Key:         []byte(r.Key),
					Version:     util.PtrToNullableInt64(r.Version),
					IsReadWrite: r.IsReadWrite,
				})
			}
			for _, e := range ns.Endorsements {
				endorseParams = append(endorseParams, dbsqlc.InsertTxEndorsementParams{
					BlockNum:    int64(txRec.BlockNum),
					TxNum:       int64(txRec.TxNum),
					NsID:        ns.NsID,
					Endorsement: e.Endorsement,
					MspID:       util.PtrToNullableString(e.MspID),
					Identity:    e.Identity,
				})
			}
			for _, w := range ns.Writes {
				writeParams = append(writeParams, dbsqlc.InsertTxWriteParams{
					BlockNum:     int64(txRec.BlockNum),
					TxNum:        int64(txRec.TxNum),
					NsID:         ns.NsID,
					Key:          []byte(w.Key),
					Value:        w.Value,
					IsBlindWrite: w.IsBlindWrite,
					ReadVersion:  util.PtrToNullableInt64(w.ReadVersion),
				})
			}
		}
	}

	for _, p := range parsedData.Policies {
		if len(p.PolicyJSON) == 0 {
			continue
		}
		policyParams = append(policyParams, dbsqlc.UpsertNamespacePolicyParams{
			Namespace: p.Namespace,
			Version:   int64(p.Version),
			Policy:    p.PolicyJSON,
		})
	}

	// execBatch drains a batchexec result set and returns the first error encountered.
	execBatch := func(exec func(func(int, error))) error {
		var batchErr error
		exec(func(_ int, err error) {
			if err != nil && batchErr == nil {
				batchErr = err
			}
		})
		return batchErr
	}

	// Round-trips 2–7: one SendBatch call per table.
	if err := execBatch(q.InsertTransaction(ctx, txParams).Exec); err != nil {
		return err
	}
	if len(nsParams) > 0 {
		if err := execBatch(q.InsertTxNamespace(ctx, nsParams).Exec); err != nil {
			return err
		}
	}
	if len(readParams) > 0 {
		if err := execBatch(q.InsertTxRead(ctx, readParams).Exec); err != nil {
			return err
		}
	}
	if len(endorseParams) > 0 {
		if err := execBatch(q.InsertTxEndorsement(ctx, endorseParams).Exec); err != nil {
			return err
		}
	}
	if len(writeParams) > 0 {
		if err := execBatch(q.InsertTxWrite(ctx, writeParams).Exec); err != nil {
			return err
		}
	}
	if len(policyParams) > 0 {
		if err := execBatch(q.UpsertNamespacePolicy(ctx, policyParams).Exec); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true

	logger.Debugf("db: stored block %d with %d transactions", pb.BlockInfo.Number, len(parsedData.Transactions))
	return nil
}
