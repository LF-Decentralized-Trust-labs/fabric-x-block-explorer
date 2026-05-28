/**
 * Decodes a Fabric channel config policy_bytes (base64-encoded protobuf)
 * by extracting readable strings and categorizing them.
 * No protobuf library needed — uses pattern matching on the raw bytes.
 */

export interface DecodedPolicy {
  organizations: string[];
  ordererNodes: string[];
  endpoints: string[];
  policyRoles: string[];
  aclRules: Array<{ resource: string; policy: string }>;
  capabilities: string[];
  consensusType: string;
  hashAlgorithm: string;
  consortium: string;
  certificates: number;
  sections: string[];
}

const POLICY_ROLES = new Set([
  'Admins', 'Writers', 'Readers', 'Endorsement',
  'BlockValidation', 'LifecycleEndorsement',
]);

const SECTIONS = new Set([
  'Orderer', 'Application', 'Channel', 'Orderers',
  'ChannelRestrictions', 'Capabilities',
]);

const CONSENSUS_TYPES: Record<string, string> = {
  arma: 'BFT (arma)',
  etcdraft: 'Raft (etcdraft)',
  kafka: 'Kafka',
};

export function decodePolicyBytes(base64: string): DecodedPolicy | null {
  if (!base64) return null;

  let bytes: Uint8Array;
  try {
    const binary = atob(base64);
    bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
  } catch {
    return null;
  }

  // Extract printable ASCII strings (4+ chars)
  const strings: string[] = [];
  let current = '';
  for (let i = 0; i < bytes.length; i++) {
    const c = bytes[i];
    if (c >= 0x20 && c <= 0x7e) {
      current += String.fromCharCode(c);
    } else {
      if (current.length >= 4) strings.push(current.trim());
      current = '';
    }
  }
  if (current.length >= 4) strings.push(current.trim());

  const organizations = new Set<string>();
  const ordererNodes = new Set<string>();
  const endpoints = new Set<string>();
  const policyRoles = new Set<string>();
  const aclRules: Array<{ resource: string; policy: string }> = [];
  const capabilities = new Set<string>();
  const sections = new Set<string>();
  let consensusType = '';
  let hashAlgorithm = '';
  let consortium = '';
  let certCount = 0;

  // Parse certificates
  let inCert = false;
  for (const s of strings) {
    if (s.includes('-----BEGIN CERTIFICATE-----')) { inCert = true; certCount++; continue; }
    if (s.includes('-----END CERTIFICATE-----')) { inCert = false; continue; }
    if (inCert) continue;

    // Policy roles
    if (POLICY_ROLES.has(s)) { policyRoles.add(s); continue; }

    // Sections
    if (SECTIONS.has(s)) { sections.add(s); continue; }

    // Orderer endpoint pattern: id=X,msp-id=Y,...,host:port
    if (s.includes('msp-id=') && s.includes(',')) {
      const parts = s.split(',');
      for (const part of parts) {
        if (part.startsWith('msp-id=')) {
          const org = part.slice('msp-id='.length);
          if (org && org !== 'org') organizations.add(org);
        }
        if (part.includes(':') && /:\d+$/.test(part)) {
          endpoints.add(part.trim());
        }
      }
      continue;
    }

    // Orderer node hostnames: bft0.example.com, peer0.org1.example.com etc.
    if (/^[a-z0-9]+\.[a-z0-9.]+\.(com|org|net)$/.test(s)) {
      ordererNodes.add(s);
      continue;
    }

    // Orderer org names: OrdererOrg1, OrdererOrg2...
    if (/^OrdererOrg\d+/.test(s)) {
      organizations.add(s.replace(/[^a-zA-Z0-9_-]/g, ''));
      continue;
    }

    // SampleOrg or other org names
    if (/^[A-Z][a-zA-Z0-9]+Org[0-9]*$/.test(s) || s === 'SampleOrg') {
      organizations.add(s);
      continue;
    }

    // Capabilities: V2_0, V2_5, V3_0 etc.
    if (/^V\d_\d/.test(s)) { capabilities.add(s); continue; }

    // ACL rules: /Channel/Application/Readers etc.
    if (s.startsWith('/Channel/')) { /* handled below as ACL pair */ continue; }

    // Consensus type
    if (s in CONSENSUS_TYPES) { consensusType = CONSENSUS_TYPES[s]; continue; }
    if (s === 'arma') { consensusType = 'BFT (arma)'; continue; }
    if (s === 'etcdraft') { consensusType = 'Raft (etcdraft)'; continue; }

    // Hash algorithm
    if (s === 'SHA256' || s === 'SHA2') { hashAlgorithm = 'SHA256'; continue; }

    // Consortium
    if (s.startsWith('Sample') && s !== 'SampleOrg') { consortium = s; continue; }
  }

  // Extract ACL rules by pairing consecutive resource + /Channel/... strings
  for (let i = 0; i < strings.length - 1; i++) {
    const s = strings[i];
    const next = strings[i + 1];
    if (!s.startsWith('/') && !s.startsWith('--') && s.length > 4 && s.length < 60
      && next.startsWith('/Channel/') && !POLICY_ROLES.has(s) && !SECTIONS.has(s)) {
      aclRules.push({ resource: s, policy: next });
      i++;
    }
  }

  return {
    organizations: Array.from(organizations).sort(),
    ordererNodes: Array.from(ordererNodes).sort(),
    endpoints: Array.from(endpoints),
    policyRoles: Array.from(policyRoles).sort(),
    aclRules,
    capabilities: Array.from(capabilities).sort(),
    consensusType,
    hashAlgorithm,
    consortium,
    certificates: certCount,
    sections: Array.from(sections).sort(),
  };
}
