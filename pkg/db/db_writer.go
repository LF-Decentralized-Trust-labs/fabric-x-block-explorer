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
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/logging"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var logger = logging.New("db")

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
	writes := parsedData.Writes
	reads := parsedData.Reads
	txNamespaces := parsedData.TxNamespaces
	endorsements := parsedData.Endorsements
	policies := parsedData.Policies

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

	if err := q.InsertBlock(ctx, dbsqlc.InsertBlockParams{
		BlockNum:     int64(pb.BlockInfo.Number),
		TxCount:      int32(pb.Txns),
		PreviousHash: pb.BlockInfo.PreviousHash,
		DataHash:     pb.BlockInfo.DataHash,
	}); err != nil {
		return err
	}

	txIDCache := make(map[string]int64)
	txNsCache := make(map[string]int64)

	for _, txNs := range txNamespaces {
		txKey := fmt.Sprintf("%d-%d", txNs.BlockNum, txNs.TxNum)
		if _, found := txIDCache[txKey]; !found {
			txIDBytes, err := hex.DecodeString(txNs.TxID)
			if err != nil {
				return fmt.Errorf("failed to decode tx_id %s: %w", txNs.TxID, err)
			}

			txID, err := q.InsertTransaction(ctx, dbsqlc.InsertTransactionParams{
				BlockNum:       int64(txNs.BlockNum),
				TxNum:          int64(txNs.TxNum),
				TxID:           txIDBytes,
				ValidationCode: int64(txNs.ValidationCode),
			})
			if err != nil {
				return err
			}
			txIDCache[txKey] = txID
		}
	}

	for _, txNs := range txNamespaces {
		txKey := fmt.Sprintf("%d-%d", txNs.BlockNum, txNs.TxNum)
		txID := txIDCache[txKey]

		txNsID, err := q.InsertTxNamespace(ctx, dbsqlc.InsertTxNamespaceParams{
			TransactionID: txID,
			NsID:          txNs.NsID,
			NsVersion:     int64(txNs.NsVersion),
		})
		if err != nil {
			return err
		}

		txNsKey := fmt.Sprintf("%d-%d-%s", txNs.BlockNum, txNs.TxNum, txNs.NsID)
		txNsCache[txNsKey] = txNsID
	}

	for _, r := range reads {
		txNsKey := fmt.Sprintf("%d-%d-%s", r.BlockNum, r.TxNum, r.NsID)
		txNsID := txNsCache[txNsKey]

		if err := q.InsertTxRead(ctx, dbsqlc.InsertTxReadParams{
			TxNamespaceID: txNsID,
			Key:           []byte(r.Key),
			Version:       util.PtrToNullableInt64(r.Version),
			IsReadWrite:   r.IsReadWrite,
		}); err != nil {
			return err
		}
	}

	for _, e := range endorsements {
		txNsKey := fmt.Sprintf("%d-%d-%s", e.BlockNum, e.TxNum, e.NsID)
		txNsID := txNsCache[txNsKey]

		if err := q.InsertTxEndorsement(ctx, dbsqlc.InsertTxEndorsementParams{
			TxNamespaceID: txNsID,
			Endorsement:   e.Endorsement,
			MspID:         util.PtrToNullableString(e.MspID),
			Identity:      e.Identity,
		}); err != nil {
			return err
		}
	}

	for _, p := range policies {
		if len(p.PolicyJSON) == 0 {
			continue
		}
		if err := q.UpsertNamespacePolicy(ctx, dbsqlc.UpsertNamespacePolicyParams{
			Namespace: p.Namespace,
			Version:   int64(p.Version),
			Policy:    p.PolicyJSON,
		}); err != nil {
			return err
		}
	}

	for _, w := range writes {
		txNsKey := fmt.Sprintf("%d-%d-%s", w.BlockNum, w.TxNum, w.Namespace)
		txNsID := txNsCache[txNsKey]

		if err := q.InsertTxWrite(ctx, dbsqlc.InsertTxWriteParams{
			TxNamespaceID: txNsID,
			Key:           []byte(w.Key),
			Value:         w.Value,
			IsBlindWrite:  w.IsBlindWrite,
			ReadVersion:   util.PtrToNullableInt64(w.ReadVersion),
		}); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true

	logger.Debugf("db: stored block %d with %d writes, %d reads", pb.BlockInfo.Number, len(writes), len(reads))
	return nil
}
