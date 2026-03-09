/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

var (
	// reCert matches a full PEM certificate block (multiline).
	reCert = regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----`)
	// reMSPID matches the MSP-ID embedded in orderer endpoint strings, e.g. "msp-id=org".
	reMSPID = regexp.MustCompile(`msp-id=([^,\s&]+)`)
	// reEndpoint matches host:port orderer endpoints, e.g. "localhost:7050".
	reEndpoint = regexp.MustCompile(`\b[\w.-]+:\d{4,5}\b`)
)

// policyOuter is the JSON wrapper stored in the policy column.
type policyOuter struct {
	PolicyBytes string `json:"policy_bytes"`
}

// decodedPolicy holds the structured fields extracted from a raw policy binary.
type decodedPolicy struct {
	PolicyExpression string // e.g. "1-of(Org1MSP.peer)" or a channel config policy tree
	Certificates     []string
	MspIDs           []string
	Endpoints        []string
	HashAlgorithm    string
}

// decodePolicy decodes the stored policy bytes (outer JSON → inner binary) and
// extracts structured fields. Returns an empty decodedPolicy if anything cannot
// be parsed.
func decodePolicy(policyBytes []byte) decodedPolicy {
	// 1. Unmarshal the outer JSON wrapper: {"policy_bytes":"<base64>"}.
	var outer policyOuter
	if err := json.Unmarshal(policyBytes, &outer); err != nil {
		return decodedPolicy{}
	}

	// 2. Base64-decode the inner policy bytes.
	inner, err := base64.StdEncoding.DecodeString(outer.PolicyBytes)
	if err != nil {
		return decodedPolicy{}
	}

	text := string(inner)
	dec := decodedPolicy{
		Certificates:  extractCerts(text),
		MspIDs:        extractMspIDs(text),
		Endpoints:     extractEndpoints(text),
		HashAlgorithm: extractHashAlgorithm(text),
	}
	// PolicyExpression is computed last so it can use the already-decoded fields
	// as a fallback when proto-based decoding produces ambiguous results.
	dec.PolicyExpression = renderPolicyExpression(inner, dec)
	return dec
}

// isValidPolicyExpression returns false for expressions that are likely garbage
// produced by a false-positive proto decode (e.g. a ConfigEnvelope accidentally
// interpreted as a SignaturePolicyEnvelope with N=0 at the top level).
func isValidPolicyExpression(expr string) bool {
	return expr != "" && !strings.HasPrefix(expr, "0-of(")
}

// buildDecodedSummary returns a human-readable one-liner from the extracted fields.
func buildDecodedSummary(dec decodedPolicy) string {
	var parts []string
	if len(dec.MspIDs) > 0 {
		parts = append(parts, "MSPs: "+strings.Join(dec.MspIDs, ", "))
	}
	if len(dec.Endpoints) > 0 {
		parts = append(parts, "Endpoints: "+strings.Join(dec.Endpoints, ", "))
	}
	if dec.HashAlgorithm != "" {
		parts = append(parts, "HashAlgorithm: "+dec.HashAlgorithm)
	}
	return strings.Join(parts, " | ")
}

// extractCerts extracts PEM certificate blocks from the raw text.
func extractCerts(text string) []string {
	certs := reCert.FindAllString(text, 10)
	if certs == nil {
		return []string{}
	}
	return certs
}

// extractMspIDs extracts unique MSP IDs (e.g. "msp-id=org" → "org").
func extractMspIDs(text string) []string {
	matches := reMSPID.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	out := []string{}
	for _, m := range matches {
		if id := m[1]; !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// extractEndpoints extracts unique orderer endpoints (host:port).
func extractEndpoints(text string) []string {
	matches := reEndpoint.FindAllString(text, -1)
	seen := make(map[string]bool)
	out := []string{}
	for _, ep := range matches {
		if !seen[ep] {
			seen[ep] = true
			out = append(out, ep)
		}
	}
	return out
}

// extractHashAlgorithm returns the hashing algorithm found in the raw text.
func extractHashAlgorithm(text string) string {
	switch {
	case strings.Contains(text, "SHA3_256"):
		return "SHA3_256"
	case strings.Contains(text, "SHA256"):
		return "SHA256"
	case strings.Contains(text, "SHA384"):
		return "SHA384"
	default:
		return ""
	}
}
