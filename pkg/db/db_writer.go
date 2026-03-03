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

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
)

var logger = flogging.MustGetLogger("db")

// BlockWriter writes processed blocks to the database.
type BlockWriter struct {
	pool *pgxpool.Pool
	conn *pgxpool.Conn
}

// NewBlockWriter creates a BlockWriter backed by a connection pool.
func NewBlockWriter(pool *pgxpool.Pool) *BlockWriter {
	return &BlockWriter{pool: pool}
}

// NewBlockWriterFromConn creates a BlockWriter backed by a single connection.
func NewBlockWriterFromConn(conn *pgxpool.Conn) *BlockWriter {
	return &BlockWriter{conn: conn}
}

// WriteProcessedBlock persists a fully parsed block and all its transactions
// to the database within a single transaction.
func (bw *BlockWriter) WriteProcessedBlock(ctx context.Context, pb *types.ProcessedBlock) error {
	if pb == nil {
		return errors.New("processed block is nil")
	}

	parsedData, ok := pb.Data.(*types.ParsedBlockData)
	if !ok {
		return errors.New("processed block Data is not *types.ParsedBlockData")
	}

	tx, err := bw.beginTx(ctx)
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

	if err = q.InsertBlock(ctx, dbsqlc.InsertBlockParams{
		BlockNum:     int64(pb.BlockInfo.Number), //nolint:gosec // block numbers fit in int64
		TxCount:      int32(pb.Txns),             //nolint:gosec // tx count fits in int32
		PreviousHash: pb.BlockInfo.PreviousHash,
		DataHash:     pb.BlockInfo.DataHash,
	}); err != nil {
		return err
	}

	p, err := buildBatchParams(parsedData)
	if err != nil {
		return err
	}

	if err := p.flush(ctx, q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true

	logger.Debugf("db: stored block %d with %d transactions", pb.BlockInfo.Number, len(parsedData.Transactions))
	return nil
}

// beginTx starts a new database transaction using the available connection or pool.
func (bw *BlockWriter) beginTx(ctx context.Context) (pgx.Tx, error) {
	switch {
	case bw.conn != nil:
		return bw.conn.Begin(ctx)
	case bw.pool != nil:
		return bw.pool.Begin(ctx)
	default:
		return nil, errors.New("no pool or conn available in BlockWriter")
	}
}

// batchParams holds all the parameter slices for a block's batch inserts.
type batchParams struct {
	txParams         []dbsqlc.InsertTransactionParams
	nsParams         []dbsqlc.InsertTxNamespaceParams
	readOnlyParams   []dbsqlc.InsertReadOnlyParams
	readWriteParams  []dbsqlc.InsertReadWriteParams
	blindWriteParams []dbsqlc.InsertBlindWriteParams
	endorseParams    []dbsqlc.InsertTxEndorsementParams
	policyParams     []dbsqlc.UpsertNamespacePolicyParams
}

// buildBatchParams flattens all parsed block data into per-table param slices.
func buildBatchParams(data *types.ParsedBlockData) (*batchParams, error) {
	p := &batchParams{
		txParams: make([]dbsqlc.InsertTransactionParams, 0, len(data.Transactions)),
	}
	for _, txRec := range data.Transactions {
		if err := p.appendTx(txRec); err != nil {
			return nil, err
		}
	}
	for _, pol := range data.Policies {
		if len(pol.PolicyJSON) == 0 {
			continue
		}
		p.policyParams = append(p.policyParams, dbsqlc.UpsertNamespacePolicyParams{
			Namespace: pol.Namespace,
			Version:   int64(pol.Version), //nolint:gosec // version fits in int64
			Policy:    pol.PolicyJSON,
		})
	}
	return p, nil
}

// appendTx adds a transaction and all its namespace data to the param slices.
func (p *batchParams) appendTx(txRec types.TxRecord) error {
	txIDBytes, err := hex.DecodeString(txRec.TxID)
	if err != nil {
		return fmt.Errorf("failed to decode tx_id %s: %w", txRec.TxID, err)
	}
	p.txParams = append(p.txParams, dbsqlc.InsertTransactionParams{
		BlockNum:       int64(txRec.BlockNum), //nolint:gosec // fits in int64
		TxNum:          int64(txRec.TxNum),    //nolint:gosec // fits in int64
		TxID:           txIDBytes,
		ValidationCode: int64(txRec.ValidationCode),
	})
	for _, ns := range txRec.Namespaces {
		p.appendNamespace(txRec.BlockNum, txRec.TxNum, ns)
	}
	return nil
}

// appendNamespace adds a single (tx, namespace) pair to the param slices.
func (p *batchParams) appendNamespace(blockNum, txNum uint64, ns types.TxNamespaceRecord) {
	bn := int64(blockNum) //nolint:gosec // fits in int64
	tn := int64(txNum)    //nolint:gosec // fits in int64

	p.nsParams = append(p.nsParams, dbsqlc.InsertTxNamespaceParams{
		BlockNum:  bn,
		TxNum:     tn,
		NsID:      ns.NsID,
		NsVersion: int64(ns.NsVersion), //nolint:gosec // fits in int64
	})
	for _, r := range ns.ReadsOnly {
		p.readOnlyParams = append(p.readOnlyParams, dbsqlc.InsertReadOnlyParams{
			BlockNum: bn,
			TxNum:    tn,
			NsID:     ns.NsID,
			Key:      []byte(r.Key),
			Version:  util.PtrToNullableInt64(r.Version),
		})
	}
	for _, rw := range ns.ReadWrites {
		p.readWriteParams = append(p.readWriteParams, dbsqlc.InsertReadWriteParams{
			BlockNum:    bn,
			TxNum:       tn,
			NsID:        ns.NsID,
			Key:         []byte(rw.Key),
			ReadVersion: util.PtrToNullableInt64(rw.ReadVersion),
			Value:       rw.Value,
		})
	}
	for _, bw := range ns.BlindWrites {
		p.blindWriteParams = append(p.blindWriteParams, dbsqlc.InsertBlindWriteParams{
			BlockNum: bn,
			TxNum:    tn,
			NsID:     ns.NsID,
			Key:      []byte(bw.Key),
			Value:    bw.Value,
		})
	}
	for _, e := range ns.Endorsements {
		p.endorseParams = append(p.endorseParams, dbsqlc.InsertTxEndorsementParams{
			BlockNum:    bn,
			TxNum:       tn,
			NsID:        ns.NsID,
			Endorsement: e.Endorsement,
			MspID:       util.PtrToNullableString(e.MspID),
			Identity:    e.Identity,
		})
	}
}

// flush executes all batch inserts, skipping any empty param slices.
func (p *batchParams) flush(ctx context.Context, q *dbsqlc.Queries) error {
	type entry struct {
		n int
		f func() error
	}
	for _, e := range []entry{
		{len(p.txParams), func() error { return execBatch(q.InsertTransaction(ctx, p.txParams).Exec) }},
		{len(p.nsParams), func() error { return execBatch(q.InsertTxNamespace(ctx, p.nsParams).Exec) }},
		{len(p.readOnlyParams), func() error { return execBatch(q.InsertReadOnly(ctx, p.readOnlyParams).Exec) }},
		{len(p.readWriteParams), func() error { return execBatch(q.InsertReadWrite(ctx, p.readWriteParams).Exec) }},
		{len(p.blindWriteParams), func() error { return execBatch(q.InsertBlindWrite(ctx, p.blindWriteParams).Exec) }},
		{len(p.endorseParams), func() error { return execBatch(q.InsertTxEndorsement(ctx, p.endorseParams).Exec) }},
		{len(p.policyParams), func() error { return execBatch(q.UpsertNamespacePolicy(ctx, p.policyParams).Exec) }},
	} {
		if e.n == 0 {
			continue
		}
		if err := e.f(); err != nil {
			return err
		}
	}
	return nil
}

// execBatch drains a batchexec result set and returns the first error encountered.
func execBatch(exec func(func(int, error))) error {
	var batchErr error
	exec(func(_ int, err error) {
		if err != nil && batchErr == nil {
			batchErr = err
		}
	})
	return batchErr
}
