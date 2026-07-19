#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cert_dir="${repo_root}/docker/certs"

host_name="${1:-dungeon-master.local}"
host_ip="${2:-127.0.0.1}"
days_valid="${3:-30}"

mkdir -p "${cert_dir}"

openssl_config="$(mktemp)"
trap 'rm -f "${openssl_config}"' EXIT

cat > "${openssl_config}" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
x509_extensions = v3_req
distinguished_name = dn

[dn]
CN = ${host_name}

[v3_req]
subjectAltName = @alt_names
extendedKeyUsage = serverAuth
keyUsage = digitalSignature, keyEncipherment

[alt_names]
DNS.1 = ${host_name}
IP.1 = ${host_ip}
IP.2 = 127.0.0.1
EOF

openssl req -x509 -nodes -newkey rsa:2048 \
  -keyout "${cert_dir}/local-key.pem" \
  -out "${cert_dir}/local-cert.pem" \
  -days "${days_valid}" \
  -config "${openssl_config}"

echo "Generated:"
echo "  ${cert_dir}/local-cert.pem"
echo "  ${cert_dir}/local-key.pem"
echo
echo "Host: ${host_name}"
echo "IP:   ${host_ip}"
echo "Valid for: ${days_valid} days"
