/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"encoding/base64"
	"testing"
)

func TestDecodePolicy_ValidPolicy(t *testing.T) {
	t.Parallel()

	inner := "msp-id=org,broadcast,deliver,localhost:7050\n" +
		"SHA256\n" +
		"-----BEGIN CERTIFICATE-----\n" +
		"ABC\n" +
		"-----END CERTIFICATE-----\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(inner))
	outer := []byte(`{"policy_bytes":"` + encoded + `"}`)

	dec := decodePolicy(outer)

	if len(dec.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(dec.Certificates))
	}
	if got, want := len(dec.MspIDs), 1; got != want {
		t.Fatalf("expected %d msp id, got %d", want, got)
	}
	if dec.MspIDs[0] != "org" {
		t.Fatalf("expected msp id 'org', got %q", dec.MspIDs[0])
	}
	if got, want := len(dec.Endpoints), 1; got != want {
		t.Fatalf("expected %d endpoint, got %d", want, got)
	}
	if dec.Endpoints[0] != "localhost:7050" {
		t.Fatalf("expected endpoint 'localhost:7050', got %q", dec.Endpoints[0])
	}
	if dec.HashAlgorithm != "SHA256" {
		t.Fatalf("expected hash algorithm 'SHA256', got %q", dec.HashAlgorithm)
	}
}

func TestDecodePolicy_InvalidInput(t *testing.T) {
	t.Parallel()

	dec := decodePolicy([]byte("not-json"))
	if len(dec.Certificates) != 0 ||
		len(dec.MspIDs) != 0 ||
		len(dec.Endpoints) != 0 ||
		dec.HashAlgorithm != "" {
		t.Fatalf("expected empty decodedPolicy for invalid input, got %#v", dec)
	}
}
