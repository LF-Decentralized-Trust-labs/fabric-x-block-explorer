/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodePolicy(t *testing.T) {
	t.Parallel()

	t.Run("extracts fields from valid policy", func(t *testing.T) {
		t.Parallel()
		inner := "msp-id=org,broadcast,deliver,localhost:7050\n" +
			"SHA256\n" +
			"-----BEGIN CERTIFICATE-----\n" +
			"ABC\n" +
			"-----END CERTIFICATE-----\n"
		encoded := base64.StdEncoding.EncodeToString([]byte(inner))
		outer := []byte(`{"policy_bytes":"` + encoded + `"}`)

		dec := decodePolicy(outer)

		require.Len(t, dec.Certificates, 1)
		require.Len(t, dec.MspIDs, 1)
		assert.Equal(t, "org", dec.MspIDs[0])
		require.Len(t, dec.Endpoints, 1)
		assert.Equal(t, "localhost:7050", dec.Endpoints[0])
		assert.Equal(t, "SHA256", dec.HashAlgorithm)
	})

	t.Run("returns empty policy for invalid input", func(t *testing.T) {
		t.Parallel()
		dec := decodePolicy([]byte("not-json"))
		assert.Empty(t, dec.Certificates)
		assert.Empty(t, dec.MspIDs)
		assert.Empty(t, dec.Endpoints)
		assert.Empty(t, dec.HashAlgorithm)
	})
}
