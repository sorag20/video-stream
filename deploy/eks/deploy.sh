#!/usr/bin/env bash
# k8s/overlays/eks を EKS にデプロイするスクリプト。通常は GitHub Actions (.github/workflows/deploy.yml) が実行する
# アカウント固有の値とシークレットは Git に含めず、一時ディレクトリにコピーしたマニフェストにデプロイ時だけ書き込む
#   - イメージ: ECR のレジストリ（アカウント ID から組み立てる）とタグ (TAG)
#   - envoyproxy.yaml の SUBNET_ID / ACM_CERT_ARN: Parameter Store
#   - secret.env / .htpasswd: Parameter Store (SecureString)
# 手元から実行する場合は、ECR にあるイメージのタグ（コミットの SHA）を指定する: TAG=<SHA> bash deploy/eks/deploy.sh
set -euo pipefail

: "${TAG:?デプロイするイメージのタグを TAG に指定してください}"

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGION="${AWS_REGION:-ap-northeast-1}"
PARAM_PREFIX="video-stream-eks"
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

get_param() {
  aws ssm get-parameter --region "$REGION" --with-decryption --name "${PARAM_PREFIX}-$1" \
    --query Parameter.Value --output text
}

# 実際の値を書き込むのはこのコピーだけ。終了時に削除する
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp -R "${REPO_ROOT}/k8s" "$WORK/"
OVERLAY="${WORK}/k8s/overlays/eks"

echo "==> Reading parameters ${PARAM_PREFIX}-*"
SUBNET_ID="$(get_param subnet-id)"
ACM_CERT_ARN="$(get_param acm-cert-arn)"
get_param secret-env > "${OVERLAY}/secret.env"
get_param htpasswd | sed '/^$/d' > "${OVERLAY}/.htpasswd"

# クラスタを作り直すとサブネット ID が変わる。古い値のままだと NLB が作られないため、apply する前に確かめる
if ! aws ec2 describe-subnets --region "$REGION" --subnet-ids "$SUBNET_ID" >/dev/null 2>&1; then
  echo "${PARAM_PREFIX}-subnet-id のサブネットが見つかりません。クラスタを作り直した場合は README の手順で登録し直してください" >&2
  exit 1
fi

sed -i.bak \
  -e "s#newName: [^/]*/video-stream/#newName: ${REGISTRY}/video-stream/#" \
  -e "s#newTag: .*#newTag: \"${TAG}\"#" \
  "${OVERLAY}/kustomization.yaml"
sed -i.bak \
  -e "s|SUBNET_ID|${SUBNET_ID}|" \
  -e "s|ACM_CERT_ARN|${ACM_CERT_ARN}|" \
  "${OVERLAY}/envoyproxy.yaml"

echo "==> Applying k8s/overlays/eks (tag: ${TAG})"
kubectl apply -k "$OVERLAY"
