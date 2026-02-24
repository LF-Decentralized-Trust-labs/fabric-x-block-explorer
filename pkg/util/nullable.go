/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import "github.com/jackc/pgx/v5/pgtype"

// NullableInt64ToPtr converts a pgtype.Int8 to *int64
func NullableInt64ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

// NullableStringToPtr converts a pgtype.Text to *string
func NullableStringToPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// PtrToNullableInt64 converts *uint64 to pgtype.Int8
func PtrToNullableInt64(v *uint64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: int64(*v), Valid: true}
}

// PtrToNullableString converts *string to pgtype.Text
func PtrToNullableString(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}
