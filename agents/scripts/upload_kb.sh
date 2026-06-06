#!/bin/bash
# Upload knowledge base content to S3 and trigger re-ingestion.
# Usage: ./scripts/upload_kb.sh [--sync-only]
set -euo pipefail

BUCKET="borginho-bucket-bleqtech-uuid-23jek2"
PREFIX="knowledge-base/casais"
REGION="${AWS_DEFAULT_REGION:-us-east-1}"
CONTENT_DIR="$(cd "$(dirname "$0")/../.." && pwd)/knowledge_base"

echo "[+] Uploading knowledge base content..."
echo "    Source: $CONTENT_DIR"
echo "    Dest:   s3://$BUCKET/$PREFIX/"

aws s3 sync "$CONTENT_DIR/" "s3://$BUCKET/$PREFIX/" \
  --region "$REGION" \
  --delete \
  --exclude "*.DS_Store"

echo "[+] Upload complete."

if [[ "${1:-}" == "--sync-only" ]]; then
  echo "[i] Skipping ingestion (--sync-only)."
  exit 0
fi

# Trigger re-ingestion
KB_ID=$(aws ssm get-parameter \
  --name "/bedrock-kb/casais-id" \
  --region "$REGION" \
  --query "Parameter.Value" \
  --output text 2>/dev/null || true)

DS_ID=$(aws ssm get-parameter \
  --name "/bedrock-kb/casais-ds-id" \
  --region "$REGION" \
  --query "Parameter.Value" \
  --output text 2>/dev/null || true)

if [[ -z "$KB_ID" || "$KB_ID" == "None" ]]; then
  echo "[!] KB ID not found in SSM (/bedrock-kb/casais-id). Run terraform apply first."
  exit 1
fi

if [[ -z "$DS_ID" || "$DS_ID" == "None" ]]; then
  echo "[!] DS ID not found in SSM (/bedrock-kb/casais-ds-id). Run terraform apply first."
  exit 1
fi

echo "[+] Triggering ingestion job..."
JOB_ID=$(aws bedrock-agent start-ingestion-job \
  --knowledge-base-id "$KB_ID" \
  --data-source-id "$DS_ID" \
  --region "$REGION" \
  --query "ingestionJob.ingestionJobId" \
  --output text)

echo "[+] Ingestion job started: $JOB_ID"
echo "[i] Monitor with: aws bedrock-agent get-ingestion-job --knowledge-base-id $KB_ID --data-source-id $DS_ID --ingestion-job-id $JOB_ID"
