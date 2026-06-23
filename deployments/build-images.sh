#!/bin/sh
set -eu

VERSION="${VERSION:-$(date +%Y%m%d%H%M%S)}"
REGISTRY="${REGISTRY:-registry.example.com}"
PUSH="${PUSH:-0}"

KOFFY_WEB_IMAGE="${KOFFY_WEB_IMAGE:-${REGISTRY}/koffy-web:${VERSION}}"
KOFFY_BILLING_API_IMAGE="${KOFFY_BILLING_API_IMAGE:-${REGISTRY}/koffy-billing-api:${VERSION}}"
KOFFY_GATEWAY_IMAGE="${KOFFY_GATEWAY_IMAGE:-${REGISTRY}/koffy-gateway:${VERSION}}"

echo "Building Koffy images:"
echo "  ${KOFFY_WEB_IMAGE}"
echo "  ${KOFFY_BILLING_API_IMAGE}"
echo "  ${KOFFY_GATEWAY_IMAGE}"

docker build \
  --platform linux/amd64 \
  --build-arg APP=koffy-billing-api \
  -t "${KOFFY_BILLING_API_IMAGE}" \
  .

docker build \
  --platform linux/amd64 \
  --build-arg APP=koffy-gateway \
  -t "${KOFFY_GATEWAY_IMAGE}" \
  .

docker build \
  --platform linux/amd64 \
  -t "${KOFFY_WEB_IMAGE}" \
  ./web

if [ "${PUSH}" = "1" ]; then
  docker push "${KOFFY_BILLING_API_IMAGE}"
  docker push "${KOFFY_GATEWAY_IMAGE}"
  docker push "${KOFFY_WEB_IMAGE}"
fi

cat <<EOF

Use these image values in production.env:
KOFFY_WEB_IMAGE=${KOFFY_WEB_IMAGE}
KOFFY_BILLING_API_IMAGE=${KOFFY_BILLING_API_IMAGE}
KOFFY_GATEWAY_IMAGE=${KOFFY_GATEWAY_IMAGE}
EOF
