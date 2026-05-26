/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// NullableInt64ToPtr converts a pgtype.Int8 to *int64.
func NullableInt64ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

// NullableStringToPtr converts a pgtype.Text to *string.
func NullableStringToPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// PtrToNullableInt64 converts *uint64 to pgtype.Int8.
// Returns an invalid (NULL) value if v exceeds math.MaxInt64.
func PtrToNullableInt64(v *uint64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	if *v > math.MaxInt64 {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: int64(*v), Valid: true}
}

// PtrToNullableString converts *string to pgtype.Text.
func PtrToNullableString(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}

// Int32ToNullableInt4 converts int to pgtype.Int4.
func Int32ToNullableInt4(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true} //nolint:gosec // v is already int
}

// Int32PtrToNullableInt4 converts *int32 to pgtype.Int4.
func Int32PtrToNullableInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

// Int64ToNullableTimestamp converts *int64 (Unix nanoseconds) to pgtype.Timestamp.
func Int64ToNullableTimestamp(v *int64) pgtype.Timestamp {
	if v == nil {
		return pgtype.Timestamp{Valid: false}
	}
	// Convert nanoseconds to time.Time
	t := time.Unix(0, *v)
	return pgtype.Timestamp{
		Time:             t,
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}

// NullableTimestampToTimePtr converts a pgtype.Timestamp to *time.Time in UTC.
func NullableTimestampToTimePtr(v pgtype.Timestamp) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

// NullableInt4ToInt32Ptr converts a pgtype.Int4 to *int32.
func NullableInt4ToInt32Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}
