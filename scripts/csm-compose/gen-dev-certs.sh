#!/bin/sh
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Generates a throwaway CA + broker certificate for the local Kafka broker,
# fresh on every `docker compose up` (see the "certgen" service). Production
# code in entity-service/internal/eventbus, csm-notification-service, and
# both activity-stream-services hardcodes TLS for its Kafka client (it talks
# to Azure Event Hub's Kafka-compatible endpoint, which is always
# TLS+SASL/PLAIN) -- there is no plaintext code path. So local dev needs a
# real TLS-terminating broker too: this script is what makes that possible
# without touching application code. Every value here is a disposable local
# secret, regenerated per `docker compose up`, never a real credential.
set -eu

OUT=/certs
mkdir -p "$OUT"

if [ -f "$OUT/kafka.p12" ] && [ -f "$OUT/ca.crt" ]; then
  echo "[certgen] certs already present in $OUT, skipping"
  exit 0
fi

CA_PASS="dev-ca-pass"
KEYSTORE_PASS="dev-keystore-pass"

echo "[certgen] generating CA"
openssl req -x509 -newkey rsa:2048 -days 3650 -nodes \
  -keyout "$OUT/ca.key" -out "$OUT/ca.crt" \
  -subj "/CN=csm-local-dev-ca"

echo "[certgen] generating kafka broker key + CSR"
openssl req -newkey rsa:2048 -nodes \
  -keyout "$OUT/kafka.key" -out "$OUT/kafka.csr" \
  -subj "/CN=kafka" \
  -addext "subjectAltName=DNS:kafka,DNS:localhost,IP:127.0.0.1"

echo "[certgen] signing kafka broker cert with local CA"
# alpine:3.20's /bin/sh is BusyBox ash, which does not support process
# substitution (<(...)) -- write the extension to a real file instead.
printf 'subjectAltName=DNS:kafka,DNS:localhost,IP:127.0.0.1\n' > "$OUT/kafka.ext"
openssl x509 -req -in "$OUT/kafka.csr" -CA "$OUT/ca.crt" -CAkey "$OUT/ca.key" \
  -CAcreateserial -days 3650 -out "$OUT/kafka.crt" \
  -extfile "$OUT/kafka.ext"

echo "[certgen] bundling PKCS12 keystore for the Kafka broker"
openssl pkcs12 -export \
  -in "$OUT/kafka.crt" -inkey "$OUT/kafka.key" -certfile "$OUT/ca.crt" \
  -name kafka -out "$OUT/kafka.p12" -passout "pass:${KEYSTORE_PASS}"

cat > "$OUT/keystore-creds" <<EOF
${KEYSTORE_PASS}
EOF
cp "$OUT/keystore-creds" "$OUT/key-creds"

# SASL/PLAIN static user list for the broker. kafka-go's client (used by
# every consuming/producing Go service here) hardcodes the SASL/PLAIN
# username to the literal string "$ConnectionString" -- see
# entity-service/internal/eventbus/config.go -- so that's the user this
# JAAS config must define, with its password equal to whatever
# EVENT_HUB_CONNECTION_STRING is set to across the compose file.
cat > "$OUT/kafka_server_jaas.conf" <<EOF
KafkaServer {
  org.apache.kafka.common.security.plain.PlainLoginModule required
  username="admin"
  password="admin-secret"
  user_admin="admin-secret"
  user_\$ConnectionString="${EVENT_HUB_LOCAL_SHARED_SECRET:-local-dev-shared-secret}";
};
EOF

chmod 644 "$OUT"/*.crt "$OUT"/*.p12 "$OUT"/*.conf "$OUT"/keystore-creds "$OUT"/key-creds
echo "[certgen] done, artifacts in $OUT"
