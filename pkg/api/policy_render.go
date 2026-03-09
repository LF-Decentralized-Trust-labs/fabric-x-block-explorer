/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	commonpb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	msppb "github.com/hyperledger/fabric-protos-go-apiv2/msp"
)

// renderPolicyExpression tries known Fabric policy encodings and falls back to a
// one-line summary from already extracted fields.
func renderPolicyExpression(inner []byte, dec decodedPolicy) string {
	var spe commonpb.SignaturePolicyEnvelope
	if err := proto.Unmarshal(inner, &spe); err == nil && spe.Rule != nil && len(spe.Identities) > 0 {
		if expr := renderSignaturePolicy(&spe); isValidPolicyExpression(expr) {
			return expr
		}
	}

	if tree := renderConfigTree(inner); tree != "" {
		return tree
	}
	if tree := renderEnvelopeConfigTree(inner); tree != "" {
		return tree
	}

	var env commonpb.ConfigEnvelope
	if err := proto.Unmarshal(inner, &env); err == nil && env.Config != nil && env.Config.ChannelGroup != nil {
		if tree := renderConfigGroupTree("", env.Config.ChannelGroup, 0); tree != "" {
			return tree
		}
	}

	return buildDecodedSummary(dec)
}

func renderConfigTree(inner []byte) string {
	var cg commonpb.ConfigGroup
	if err := proto.Unmarshal(inner, &cg); err == nil && (len(cg.Policies) > 0 || len(cg.Groups) > 0) {
		if tree := renderConfigGroupTree("", &cg, 0); tree != "" {
			return tree
		}
	}

	var cfg commonpb.Config
	if err := proto.Unmarshal(inner, &cfg); err == nil && cfg.ChannelGroup != nil {
		if tree := renderConfigGroupTree("", cfg.ChannelGroup, 0); tree != "" {
			return tree
		}
	}

	return ""
}

func renderEnvelopeConfigTree(inner []byte) string {
	var envelope commonpb.Envelope
	if err := proto.Unmarshal(inner, &envelope); err != nil || len(envelope.Payload) == 0 {
		return ""
	}

	var payload commonpb.Payload
	if err := proto.Unmarshal(envelope.Payload, &payload); err != nil || len(payload.Data) == 0 {
		return ""
	}

	var cfgEnv commonpb.ConfigEnvelope
	if err := proto.Unmarshal(payload.Data, &cfgEnv); err != nil ||
		cfgEnv.Config == nil || cfgEnv.Config.ChannelGroup == nil {
		return ""
	}

	return renderConfigGroupTree("", cfgEnv.Config.ChannelGroup, 0)
}

func renderSignaturePolicy(spe *commonpb.SignaturePolicyEnvelope) string {
	if spe == nil || spe.Rule == nil {
		return ""
	}
	return renderRule(spe.Rule, spe.Identities)
}

func renderRule(rule *commonpb.SignaturePolicy, ids []*msppb.MSPPrincipal) string {
	if rule == nil {
		return ""
	}

	switch t := rule.Type.(type) {
	case *commonpb.SignaturePolicy_SignedBy:
		idx := int(t.SignedBy)
		if idx < len(ids) {
			return renderPrincipal(ids[idx])
		}
		return fmt.Sprintf("principal[%d]", idx)
	case *commonpb.SignaturePolicy_NOutOf_:
		nof := t.NOutOf
		parts := make([]string, 0, len(nof.Rules))
		for _, r := range nof.Rules {
			parts = append(parts, renderRule(r, ids))
		}
		return fmt.Sprintf("%d-of(%s)", nof.N, strings.Join(parts, ", "))
	default:
		return ""
	}
}

func renderPrincipal(p *msppb.MSPPrincipal) string {
	if p == nil {
		return "unknown"
	}

	switch p.PrincipalClassification { //nolint:exhaustive
	case msppb.MSPPrincipal_ROLE:
		var role msppb.MSPRole
		if err := proto.Unmarshal(p.Principal, &role); err == nil {
			return fmt.Sprintf("%s.%s", role.MspIdentifier, strings.ToLower(role.Role.String()))
		}
	case msppb.MSPPrincipal_ORGANIZATION_UNIT:
		var ou msppb.OrganizationUnit
		if err := proto.Unmarshal(p.Principal, &ou); err == nil {
			return fmt.Sprintf("%s.%s", ou.MspIdentifier, ou.OrganizationalUnitIdentifier)
		}
	default:
		return fmt.Sprintf("principal(%x)", p.Principal)
	}

	return fmt.Sprintf("principal(%x)", p.Principal)
}

func renderConfigGroupTree(path string, group *commonpb.ConfigGroup, depth int) string {
	const maxDepth = 3
	if group == nil || depth > maxDepth {
		return ""
	}

	prefix := path
	if prefix != "" {
		prefix += "/"
	}

	var lines []string
	policyNames := make([]string, 0, len(group.Policies))
	for name := range group.Policies {
		policyNames = append(policyNames, name)
	}
	sort.Strings(policyNames)
	for _, name := range policyNames {
		if expr := renderConfigPolicy(group.Policies[name]); expr != "" {
			lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, name, expr))
		}
	}

	subNames := make([]string, 0, len(group.Groups))
	for name := range group.Groups {
		subNames = append(subNames, name)
	}
	sort.Strings(subNames)
	for _, name := range subNames {
		if sub := renderConfigGroupTree(prefix+name, group.Groups[name], depth+1); sub != "" {
			lines = append(lines, sub)
		}
	}

	return strings.Join(lines, "\n")
}

func renderConfigPolicy(cp *commonpb.ConfigPolicy) string {
	if cp == nil || cp.Policy == nil {
		return ""
	}

	switch commonpb.Policy_PolicyType(cp.Policy.Type) {
	case commonpb.Policy_SIGNATURE:
		var spe commonpb.SignaturePolicyEnvelope
		if err := proto.Unmarshal(cp.Policy.Value, &spe); err == nil && spe.Rule != nil {
			return renderSignaturePolicy(&spe)
		}
	case commonpb.Policy_IMPLICIT_META:
		var imp commonpb.ImplicitMetaPolicy
		if err := proto.Unmarshal(cp.Policy.Value, &imp); err == nil {
			return fmt.Sprintf("%s(%s)", imp.Rule.String(), imp.SubPolicy)
		}
	default:
		return ""
	}

	return ""
}
