/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T {
	return &v
}
