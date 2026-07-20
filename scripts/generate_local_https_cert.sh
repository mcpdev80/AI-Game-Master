#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cert_dir="${repo_root}/docker/certs"

usage() {
  cat <<'EOF'
Usage:
  generate_local_https_cert.sh [host_name] [host_ip] [days_valid]
  generate_local_https_cert.sh --cert /path/to/cert.pem --key /path/to/key.pem
  generate_local_https_cert.sh [host_name] [host_ip] [days_valid] --cert /path/to/cert.pem --key /path/to/key.pem

Behavior:
  - If --cert and --key are provided, the files are copied into docker/certs/.
  - If no external cert/key are provided, a self-signed certificate is generated.
EOF
}

host_name="dungeon-master.local"
host_ip="127.0.0.1"
days_valid="30"
source_cert=""
source_key=""

positionals=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --cert)
      [[ $# -ge 2 ]] || { echo "Missing value for --cert" >&2; usage; exit 1; }
      source_cert="$2"
      shift 2
      ;;
    --key)
      [[ $# -ge 2 ]] || { echo "Missing value for --key" >&2; usage; exit 1; }
      source_key="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      positionals+=("$1")
      shift
      ;;
  esac
done

if [[ ${#positionals[@]} -ge 1 ]]; then
  host_name="${positionals[0]}"
fi
if [[ ${#positionals[@]} -ge 2 ]]; then
  host_ip="${positionals[1]}"
fi
if [[ ${#positionals[@]} -ge 3 ]]; then
  days_valid="${positionals[2]}"
fi
if [[ ${#positionals[@]} -gt 3 ]]; then
  echo "Too many positional arguments" >&2
  usage
  exit 1
fi

mkdir -p "${cert_dir}"

target_cert="${cert_dir}/local-cert.pem"
target_key="${cert_dir}/local-key.pem"

if [[ -n "${source_cert}" || -n "${source_key}" ]]; then
  [[ -n "${source_cert}" && -n "${source_key}" ]] || { echo "Provide both --cert and --key together" >&2; exit 1; }
  [[ -f "${source_cert}" ]] || { echo "Certificate file not found: ${source_cert}" >&2; exit 1; }
  [[ -f "${source_key}" ]] || { echo "Key file not found: ${source_key}" >&2; exit 1; }

  cp "${source_cert}" "${target_cert}"
  cp "${source_key}" "${target_key}"
  chmod 644 "${target_cert}"
  chmod 600 "${target_key}"

  echo "Installed external certificate:"
  echo "  ${target_cert}"
  echo "  ${target_key}"
  echo
  echo "Source cert: ${source_cert}"
  echo "Source key:  ${source_key}"
  exit 0
fi

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
  -keyout "${target_key}" \
  -out "${target_cert}" \
  -days "${days_valid}" \
  -config "${openssl_config}"

chmod 644 "${target_cert}"
chmod 600 "${target_key}"

echo "Generated:"
echo "  ${target_cert}"
echo "  ${target_key}"
echo
echo "Host: ${host_name}"
echo "IP:   ${host_ip}"
echo "Valid for: ${days_valid} days"
