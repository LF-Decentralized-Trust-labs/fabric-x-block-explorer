/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package db

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hyperledger/fabric-x-committer/utils/logging"

	dbsqlc "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/db/sqlc"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/types"
	"github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/util"
)

var logger = logging.New("db")

// BlockWriter writes a parsed block to PostgreSQL in a single database transaction using batch inserts.
type BlockWriter struct {
	pool *pgxpool.Pool
	conn *pgxpool.Conn
}

// NewBlockWriter returns a BlockWriter that borrows a connection from pool per write.
// Prefer over NewBlockWriterFromConn unless connection pinning is required.
func NewBlockWriter(pool *pgxpool.Pool) *BlockWriter {
	return &BlockWriter{pool: pool}
}

// NewBlockWriterFromConn returns a BlockWriter pinned to conn. Call Close to return it to the pool.
func NewBlockWriterFromConn(conn *pgxpool.Conn) *BlockWriter {
	return &BlockWriter{conn: conn}
}

// Close returns the pinned connection to the pool. No-op for pool-backed writers.
func (bw *BlockWriter) Close() {
	if bw.conn != nil {
		bw.conn.Release()
	}
}

// WriteProcessedBlock persists a fully parsed block and all its transactions
// to the database within a single transaction.
func (bw *BlockWriter) WriteProcessedBlock(ctx context.Context, pb *types.ProcessedBlock) error {
	if pb == nil {
		return errors.New("processed block is nil")
	}
	if pb.BlockInfo == nil {
		return errors.New("processed block has nil BlockInfo")
	}
	if pb.Data == nil {
		return errors.New("processed block has nil Data")
	}

	p, err := buildBatchParams(pb.BlockInfo.Number, pb.Data)
	if err != nil {
		return err
	}

	tx, rollbackFn, err := bw.beginTx(ctx)
	if err != nil {
		return err
	}
	defer rollbackFn()

	q := dbsqlc.New(tx)

	// TxCount = len(pb.Data.Transactions); see ParsedBlockData for what is excluded.
	if err = q.InsertBlock(ctx, dbsqlc.InsertBlockParams{
		BlockNum:     int64(pb.BlockInfo.Number),       //nolint:gosec // block numbers fit in int64
		TxCount:      int32(len(pb.Data.Transactions)), //nolint:gosec // tx count fits in int32
		PreviousHash: pb.BlockInfo.PreviousHash,
		DataHash:     pb.BlockInfo.DataHash,
	}); err != nil {
		return err
	}

	if err := p.flush(ctx, q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	logger.Debugf("stored block %d with %d transactions", pb.BlockInfo.Number, len(pb.Data.Transactions))
	return nil
}

func (bw *BlockWriter) beginTx(ctx context.Context) (pgx.Tx, func(), error) {
	var tx pgx.Tx
	var err error
	switch {
	case bw.conn != nil:
		tx, err = bw.conn.Begin(ctx)
	case bw.pool != nil:
		tx, err = bw.pool.Begin(ctx)
	default:
		return nil, func() {}, errors.New("no pool or conn available in BlockWriter")
	}
	if err != nil {
		return nil, func() {}, errors.Wrap(err, "failed to begin database transaction")
	}

	rollback := func() { //nolint:contextcheck // rollback must succeed even when the caller's ctx is cancelled
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if rbErr := tx.Rollback(rollbackCtx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			logger.Warn("failed to rollback transaction: ", rbErr)
		}
	}
	return tx, rollback, nil
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
func buildBatchParams(blockNum uint64, data *types.ParsedBlockData) (*batchParams, error) {
	nsTxCount := len(data.Transactions)
	p := &batchParams{
		txParams:         make([]dbsqlc.InsertTransactionParams, 0, nsTxCount),
		nsParams:         make([]dbsqlc.InsertTxNamespaceParams, 0, nsTxCount),
		readOnlyParams:   make([]dbsqlc.InsertReadOnlyParams, 0, nsTxCount),
		readWriteParams:  make([]dbsqlc.InsertReadWriteParams, 0, nsTxCount),
		blindWriteParams: make([]dbsqlc.InsertBlindWriteParams, 0, nsTxCount),
		endorseParams:    make([]dbsqlc.InsertTxEndorsementParams, 0, nsTxCount),
		policyParams:     make([]dbsqlc.UpsertNamespacePolicyParams, 0, len(data.Policies)),
	}
	for _, txRec := range data.Transactions {
		if err := p.appendTx(blockNum, txRec); err != nil {
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

func (p *batchParams) appendTx(blockNum uint64, txRec types.TxRecord) error {
	txIDBytes, err := hex.DecodeString(txRec.TxID)
	if err != nil {
		return errors.Wrapf(err, "failed to decode tx_id %s", txRec.TxID)
	}
	p.txParams = append(p.txParams, dbsqlc.InsertTransactionParams{
		BlockNum:       int64(blockNum),    //nolint:gosec // fits in int64
		TxNum:          int64(txRec.TxNum), //nolint:gosec // fits in int64
		TxID:           txIDBytes,
		ValidationCode: int16(txRec.ValidationCode), //nolint:gosec // max status value is 115, fits in int16
	})
	for _, ns := range txRec.Namespaces {
		p.appendNamespace(blockNum, txRec.TxNum, ns)
	}
	return nil
}

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
			Key:      r.Key,
			Version:  util.PtrToNullableInt64(r.Version),
		})
	}
	for _, rw := range ns.ReadWrites {
		p.readWriteParams = append(p.readWriteParams, dbsqlc.InsertReadWriteParams{
			BlockNum:    bn,
			TxNum:       tn,
			NsID:        ns.NsID,
			Key:         rw.Key,
			ReadVersion: util.PtrToNullableInt64(rw.ReadVersion),
			Value:       rw.Value,
		})
	}
	for _, bw := range ns.BlindWrites {
		p.blindWriteParams = append(p.blindWriteParams, dbsqlc.InsertBlindWriteParams{
			BlockNum: bn,
			TxNum:    tn,
			NsID:     ns.NsID,
			Key:      bw.Key,
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
		count int
		fn    func() error
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
		if e.count == 0 {
			continue
		}
		if err := e.fn(); err != nil {
			return err
		}
	}
	return nil
}

// execBatch drains a batchexec result set and returns all encountered errors combined.
func execBatch(exec func(func(int, error))) error {
	var batchErr error
	exec(func(_ int, err error) {
		batchErr = errors.CombineErrors(batchErr, err)
	})
	return batchErr
}
