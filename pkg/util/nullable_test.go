/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestNullableInt64ToPtr(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Int8
		want  *int64
	}{
		{
			name:  "valid int64",
			input: pgtype.Int8{Int64: 42, Valid: true},
			want:  ptr(int64(42)),
		},
		{
			name:  "null int64",
			input: pgtype.Int8{Valid: false},
			want:  nil,
		},
		{
			name:  "zero value",
			input: pgtype.Int8{Int64: 0, Valid: true},
			want:  ptr(int64(0)),
		},
		{
			name:  "negative value",
			input: pgtype.Int8{Int64: -100, Valid: true},
			want:  ptr(int64(-100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullableInt64ToPtr(tt.input)
			if tt.want == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.want, *result)
			}
		})
	}
}

func TestNullableStringToPtr(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Text
		want  *string
	}{
		{
			name:  "valid string",
			input: pgtype.Text{String: "hello", Valid: true},
			want:  ptr("hello"),
		},
		{
			name:  "null string",
			input: pgtype.Text{Valid: false},
			want:  nil,
		},
		{
			name:  "empty string",
			input: pgtype.Text{String: "", Valid: true},
			want:  ptr(""),
		},
		{
			name:  "string with spaces",
			input: pgtype.Text{String: "  spaces  ", Valid: true},
			want:  ptr("  spaces  "),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullableStringToPtr(tt.input)
			if tt.want == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.want, *result)
			}
		})
	}
}

func TestPtrToNullableInt64(t *testing.T) {
	tests := []struct {
		name  string
		input *uint64
		want  pgtype.Int8
	}{
		{
			name:  "valid uint64",
			input: ptr(uint64(42)),
			want:  pgtype.Int8{Int64: 42, Valid: true},
		},
		{
			name:  "nil pointer",
			input: nil,
			want:  pgtype.Int8{Valid: false},
		},
		{
			name:  "zero value",
			input: ptr(uint64(0)),
			want:  pgtype.Int8{Int64: 0, Valid: true},
		},
		{
			name:  "large value",
			input: ptr(uint64(1234567890)),
			want:  pgtype.Int8{Int64: 1234567890, Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrToNullableInt64(tt.input)
			assert.Equal(t, tt.want.Valid, result.Valid)
			if tt.want.Valid {
				assert.Equal(t, tt.want.Int64, result.Int64)
			}
		})
	}
}

func TestPtrToNullableString(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		want  pgtype.Text
	}{
		{
			name:  "valid string",
			input: ptr("hello"),
			want:  pgtype.Text{String: "hello", Valid: true},
		},
		{
			name:  "nil pointer",
			input: nil,
			want:  pgtype.Text{Valid: false},
		},
		{
			name:  "empty string",
			input: ptr(""),
			want:  pgtype.Text{String: "", Valid: true},
		},
		{
			name:  "string with special characters",
			input: ptr("test\nline\ttab"),
			want:  pgtype.Text{String: "test\nline\ttab", Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrToNullableString(tt.input)
			assert.Equal(t, tt.want.Valid, result.Valid)
			if tt.want.Valid {
				assert.Equal(t, tt.want.String, result.String)
			}
		})
	}
}

// Helper function to create pointers
func ptr[T any](v T) *T {
	return &v
}
