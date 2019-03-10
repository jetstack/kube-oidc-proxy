#!/usr/bin/env bash

set -o errexit
set -o pipefail

set -e

ROOT=${BUILD_WORKSPACE_DIRECTORY:-"$(cd "$(dirname "$0")" && pwd -P)"/generated}

if [ -z "$1" ]; then
  echo "Generate DUMMY certificate authority along with a singed secure serving key pair."
  echo "gencreds.sh hostname [ip_altname]"
  exit 1
fi

mkdir -p $ROOT

CAFILE="${ROOT}/ca.pem"
CAKEY="${ROOT}/ca-key.pem"
NAME="kube-oidc-proxy"

HOSTNAME=$1
IP=$2

echo ">> generating a certificate authority"
openssl genrsa \
  -out ${CAKEY} 2048

openssl req -new -x509 -days 365 \
  -batch \
  -key ${CAKEY} \
  -out ${CAFILE}

echo ">> ca.pem ca-key.pem generated"
echo ">> generating a keypair for ${NAME}"

openssl genrsa \
  -out ${ROOT}/${NAME}-key.pem 2048

echo ">> keypair generated ${NAME}-key.pem"

cp ${ROOT}/../openssl.cnf ${ROOT}/openssl-${NAME}.cnf
sed -i -e "s/HOSTNAME/${HOSTNAME}/g" ${ROOT}/openssl-${NAME}.cnf
if [ ! -z "$2" ]; then
  echo "subjectAltName = @alt_names

[alt_names]
IP.1 = ${IP}" >> ${ROOT}/openssl-${NAME}.cnf
fi

echo ">> requesting serving certificate using openssl-${NAME}.cnf"

openssl req -subj "/CN=${HOSTNAME}" -new \
  -batch \
  -key ${ROOT}/${NAME}-key.pem \
  -out ${ROOT}/${NAME}-req.csr \
  -config ${ROOT}/openssl-${NAME}.cnf

openssl x509 -req -days 365 \
  -in ${ROOT}/${NAME}-req.csr \
  -CA ${CAFILE} \
  -CAkey ${CAKEY} \
  -CAcreateserial \
  -extensions v3_req \
  -extfile ${ROOT}/openssl-${NAME}.cnf \
  -out ${ROOT}/${NAME}-cert.pem

rm ${ROOT}/ca.srl ${ROOT}/${NAME}-req.csr ${ROOT}/openssl-${NAME}.cnf

echo "<< self signed certificate and key generated"
echo "${ROOT}/${NAME}-cert.pem ${ROOT}/${NAME}-key.pem"
