/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
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
//	GET /blocks/{block_num}          ?tx_limit=&tx_offset=
//	GET /transactions/{tx_id}        (tx_id is hex)
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
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		from, err := queryOptionalInt64(r, "from")
		if err != nil {
			return nil, err
		}
		to, err := queryOptionalInt64(r, "to")
		if err != nil {
			return nil, err
		}
		limit, err := queryOptionalInt32(r, "limit")
		if err != nil {
			return nil, err
		}
		offset, err := queryOptionalInt32(r, "offset")
		if err != nil {
			return nil, err
		}
		return s.ListBlocks(ctx, &explorerv1.ListBlocksRequest{
			From: from, To: to, Limit: limit, Offset: offset,
		})
	})
}

func (s *Service) handleGetBlockDetail(w http.ResponseWriter, r *http.Request) {
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		blockNum, err := pathInt64(r, "block_num")
		if err != nil {
			return nil, err
		}
		txLimit, err := queryOptionalInt32(r, "tx_limit")
		if err != nil {
			return nil, err
		}
		txOffset, err := queryOptionalInt32(r, "tx_offset")
		if err != nil {
			return nil, err
		}
		return s.GetBlockDetail(ctx, &explorerv1.GetBlockDetailRequest{
			BlockNum: blockNum, TxLimit: txLimit, TxOffset: txOffset,
		})
	})
}

func (s *Service) handleGetTransactionDetail(w http.ResponseWriter, r *http.Request) {
	respond(w, r, func(ctx context.Context) (proto.Message, error) {
		return s.GetTransactionDetail(ctx, &explorerv1.GetTxDetailRequest{TxId: r.PathValue("tx_id")})
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
		code := statusToHTTPCode(err)
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
	v := r.PathValue(key)
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be an integer: %q", key, v)
	}
	if parsed < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be >= 0: %q", key, v)
	}
	return parsed, nil
}

func queryOptionalInt32(r *http.Request, key string) (int32, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be an int32: %q", key, v)
	}
	if parsed < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be >= 0: %q", key, v)
	}
	return int32(parsed), nil
}

func queryOptionalInt64(r *http.Request, key string) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be an int64: %q", key, v)
	}
	if parsed < 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be >= 0: %q", key, v)
	}
	return parsed, nil
}

func statusToHTTPCode(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch st.Code() {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Canceled:
		return 499
	case codes.OK:
		return http.StatusOK
	default:
		return http.StatusInternalServerError
	}
}
