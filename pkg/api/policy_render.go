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
	var sigPolicyEnv commonpb.SignaturePolicyEnvelope
	if err := proto.Unmarshal(inner, &sigPolicyEnv); err == nil &&
		sigPolicyEnv.Rule != nil && len(sigPolicyEnv.Identities) > 0 {
		if expr := renderSignaturePolicy(&sigPolicyEnv); isValidPolicyExpression(expr) {
			return expr
		}
	}

	if tree := renderConfigTree(inner); tree != "" {
		return tree
	}
	if tree := renderEnvelopeConfigTree(inner); tree != "" {
		return tree
	}

	var configEnvelope commonpb.ConfigEnvelope
	if err := proto.Unmarshal(inner, &configEnvelope); err == nil &&
		configEnvelope.Config != nil && configEnvelope.Config.ChannelGroup != nil {
		if tree := renderConfigGroupTree("", configEnvelope.Config.ChannelGroup, 0); tree != "" {
			return tree
		}
	}

	return buildDecodedSummary(dec)
}

func renderConfigTree(inner []byte) string {
	var configGroup commonpb.ConfigGroup
	if err := proto.Unmarshal(inner, &configGroup); err == nil &&
		(len(configGroup.Policies) > 0 || len(configGroup.Groups) > 0) {
		if tree := renderConfigGroupTree("", &configGroup, 0); tree != "" {
			return tree
		}
	}

	var channelConfig commonpb.Config
	if err := proto.Unmarshal(inner, &channelConfig); err == nil && channelConfig.ChannelGroup != nil {
		if tree := renderConfigGroupTree("", channelConfig.ChannelGroup, 0); tree != "" {
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

	var configEnvelope commonpb.ConfigEnvelope
	if err := proto.Unmarshal(payload.Data, &configEnvelope); err != nil ||
		configEnvelope.Config == nil || configEnvelope.Config.ChannelGroup == nil {
		return ""
	}

	return renderConfigGroupTree("", configEnvelope.Config.ChannelGroup, 0)
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
		var sigPolicyEnv commonpb.SignaturePolicyEnvelope
		if err := proto.Unmarshal(cp.Policy.Value, &sigPolicyEnv); err == nil && sigPolicyEnv.Rule != nil {
			return renderSignaturePolicy(&sigPolicyEnv)
		}
	case commonpb.Policy_IMPLICIT_META:
		var implicitPolicy commonpb.ImplicitMetaPolicy
		if err := proto.Unmarshal(cp.Policy.Value, &implicitPolicy); err == nil {
			return fmt.Sprintf("%s(%s)", implicitPolicy.Rule.String(), implicitPolicy.SubPolicy)
		}
	default:
		return ""
	}

	return ""
}
