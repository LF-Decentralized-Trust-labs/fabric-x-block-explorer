/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"encoding/hex"
	"net/http"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	explorerv1 "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/api/proto"
)

var jsonOpts = protojson.MarshalOptions{EmitUnpopulated: true}

// newRESTRouter registers all REST endpoints.
// Handlers parse HTTP params and delegate to the same gRPC methods so that
// business logic lives in one place.
//
//	GET /blocks/height
//	GET /blocks                      ?from=&to=&limit=&offset=
//	GET /blocks/{block_num}          ?tx_limit=&tx_offset=&fields=blind_writes,endorsements,...
//	GET /transactions/{tx_id}        ?fields=blind_writes,endorsements,...    (tx_id is hex)
//	GET /namespaces/{namespace}/policies
func (s *Service) newRESTRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blocks/height", s.handleGetBlockHeight)
	mux.HandleFunc("GET /blocks", s.handleListBlocks)
	mux.HandleFunc("GET /blocks/{block_num}", s.handleGetBlockDetail)
	mux.HandleFunc("GET /transactions/{tx_id}", s.handleGetTransactionDetail)
	mux.HandleFunc("GET /namespaces/{namespace}/policies", s.handleGetNamespacePolicies)
	return mux
}

func (s *Service) handleGetBlockHeight(w http.ResponseWriter, r *http.Request) {
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		return s.GetBlockHeight(ctx, &emptypb.Empty{})
	})
}

func (s *Service) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	from, _ := queryInt64(r, "from")
	to, _ := queryInt64(r, "to")
	limit, _ := queryInt32(r, "limit")
	offset, _ := queryInt32(r, "offset")
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		return s.ListBlocks(ctx, &explorerv1.ListBlocksRequest{
			From: from, To: to, Limit: limit, Offset: offset,
		})
	})
}

func (s *Service) handleGetBlockDetail(w http.ResponseWriter, r *http.Request) {
	blockNum, err := pathInt64(r, "block_num")
	txLimit, _ := queryInt32(r, "tx_limit")
	txOffset, _ := queryInt32(r, "tx_offset")
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		if err != nil {
			return nil, err
		}
		return s.GetBlockDetail(ctx, &explorerv1.GetBlockDetailRequest{
			BlockNum: blockNum, TxLimit: txLimit, TxOffset: txOffset,
		})
	})
}

func (s *Service) handleGetTransactionDetail(w http.ResponseWriter, r *http.Request) {
	txID, err := hex.DecodeString(r.PathValue("tx_id"))
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		if err != nil {
			return nil, err
		}
		return s.GetTransactionDetail(ctx, &explorerv1.GetTxDetailRequest{TxId: txID})
	})
}

func (s *Service) handleGetNamespacePolicies(w http.ResponseWriter, r *http.Request) {
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		return s.GetNamespacePolicies(ctx, &explorerv1.GetNamespacePoliciesRequest{
			Namespace: r.PathValue("namespace"),
		})
	})
}

// --- shared HTTP helpers ---

// respond calls fn with the request context, then writes the proto response as JSON or an HTTP error.
func respond(w http.ResponseWriter, r *http.Request, fn func(context.Context) (proto.Message, error)) {
	msg, err := fn(r.Context())
	if err != nil {
		code := http.StatusInternalServerError
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	b, err := jsonOpts.Marshal(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b) //nolint:gosec // b is protojson-marshalled data, not user input
}

func pathInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}

func queryInt32(r *http.Request, key string) (int32, error) {
	v, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 32)
	return int32(v), err
}

func queryInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
}
