/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	explorerv1 "github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/api/proto"
)

func TestRespond_MapsGRPCStatusCodesToHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		statusCode int
	}{
		{
			name:       "invalid argument becomes bad request",
			err:        status.Error(codes.InvalidArgument, "bad input"),
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "not found becomes not found",
			err:        status.Error(codes.NotFound, "missing"),
			statusCode: http.StatusNotFound,
		},
		{
			name:       "plain errors become internal server error",
			err:        errors.New("boom"),
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			respond(rr, req, func(_ context.Context) (proto.Message, error) {
				return nil, tc.err
			})

			require.Equal(t, tc.statusCode, rr.Code)
		})
	}
}

func TestRESTHandlers(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	router := svc.newRESTRouter()

	t.Run("invalid block_num returns bad request", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blocks/not-a-number", nil))
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "block_num must be an integer")
	})

	t.Run("non-hex tx_id returns bad request", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/transactions/not-hex", nil))
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "tx_id must be hex-encoded")
	})

	t.Run("invalid limit returns bad request", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blocks?limit=oops", nil))
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "limit must be an int32")
	})

	t.Run("negative offset returns bad request", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blocks?offset=-1", nil))
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "offset must be >= 0")
	})

	t.Run("negative tx_offset returns bad request", func(t *testing.T) {
		t.Parallel()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blocks/1?tx_offset=-1", nil))
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "tx_offset must be >= 0")
	})
}

func TestGRPCValidation(t *testing.T) {
	t.Parallel()

	svc := &Service{}

	t.Run("list blocks rejects to < from", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ListBlocks(t.Context(), &explorerv1.ListBlocksRequest{From: 5, To: 4})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Contains(t, err.Error(), "to must be >= from")
	})

	t.Run("get transaction rejects empty tx_id", func(t *testing.T) {
		t.Parallel()
		_, err := svc.GetTransactionDetail(t.Context(), &explorerv1.GetTxDetailRequest{})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Contains(t, err.Error(), "tx_id is required")
	})

	t.Run("get namespace policies rejects empty namespace", func(t *testing.T) {
		t.Parallel()
		_, err := svc.GetNamespacePolicies(t.Context(), &explorerv1.GetNamespacePoliciesRequest{})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Contains(t, err.Error(), "namespace is required")
	})
}

func TestRespond_WritesJSONOnSuccess(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	respond(rr, req, func(_ context.Context) (proto.Message, error) {
		return &emptypb.Empty{}, nil
	})

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.JSONEq(t, `{}`, rr.Body.String())
}
