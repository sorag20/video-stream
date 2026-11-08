#!/usr/bin/env bash
# GitHub Actions (.github/workflows/deploy.yml) から ECR / EKS にデプロイするための AWS 側の準備をするスクリプト
#   - GitHub の OIDC プロバイダと、そのリポジトリの main ブランチのワークフローだけが引き受けられる IAM ロール（アクセスキーは発行しない）
#   - ECR リポジトリ
#   - IAM ロールがクラスタを操作するための EKS アクセスエントリ
# 使い方: bash deploy/eks/setup-github-actions.sh <owner>/<repo>（AWS CLI と gh にログインしてから実行する）
# 何度実行してもよい。クラスタを作り直したら再実行する（アクセスエントリはクラスタごとに作られるため）
set -euo pipefail

REPO="${1:?使い方: bash deploy/eks/setup-github-actions.sh <owner>/<repo>}"
REGION="${AWS_REGION:-ap-northeast-1}"
CLUSTER="video-stream"
ROLE_NAME="video-stream-github-actions"
SERVICES=(upload-service transcoder-service streaming-service web)

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
OIDC_HOST="token.actions.githubusercontent.com"
OIDC_ARN="arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_HOST}"
ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${ROLE_NAME}"

# GitHub がトークンの sub に入れる接頭辞。イミュータブル subject が有効なリポジトリでは
# "repo:<owner>@<owner_id>/<repo>@<repo_id>" になり、名前だけの "repo:<owner>/<repo>" では一致しない
SUB_PREFIX="$(gh api "repos/${REPO}/actions/oidc/customization/sub" --jq '.sub_claim_prefix // empty')"
SUB_PREFIX="${SUB_PREFIX:-repo:${REPO}}"

echo "==> OIDC provider (${OIDC_HOST})"
if ! aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$OIDC_ARN" >/dev/null 2>&1; then
  aws iam create-open-id-connect-provider --url "https://${OIDC_HOST}" --client-id-list sts.amazonaws.com >/dev/null
fi

echo "==> IAM role ${ROLE_NAME} (${SUB_PREFIX}, branch: main)"
TRUST_POLICY=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Federated": "${OIDC_ARN}" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_HOST}:aud": "sts.amazonaws.com",
          "${OIDC_HOST}:sub": "${SUB_PREFIX}:ref:refs/heads/main"
        }
      }
    }
  ]
}
EOF
)
if aws iam get-role --role-name "$ROLE_NAME" >/dev/null 2>&1; then
  aws iam update-assume-role-policy --role-name "$ROLE_NAME" --policy-document "$TRUST_POLICY"
else
  aws iam create-role --role-name "$ROLE_NAME" --assume-role-policy-document "$TRUST_POLICY" >/dev/null
fi

# ワークフローが使う操作だけを許可する
PERMISSIONS=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    { "Sid": "EcrLogin", "Effect": "Allow", "Action": "ecr:GetAuthorizationToken", "Resource": "*" },
    {
      "Sid": "EcrPush",
      "Effect": "Allow",
      "Action": [
        "ecr:BatchCheckLayerAvailability", "ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer",
        "ecr:InitiateLayerUpload", "ecr:UploadLayerPart", "ecr:CompleteLayerUpload", "ecr:PutImage"
      ],
      "Resource": "arn:aws:ecr:${REGION}:${ACCOUNT_ID}:repository/video-stream/*"
    },
    {
      "Sid": "ReadDeployParameters",
      "Effect": "Allow",
      "Action": "ssm:GetParameter",
      "Resource": "arn:aws:ssm:${REGION}:${ACCOUNT_ID}:parameter/video-stream-eks-*"
    },
    { "Sid": "CheckSubnet", "Effect": "Allow", "Action": "ec2:DescribeSubnets", "Resource": "*" },
    {
      "Sid": "Kubeconfig",
      "Effect": "Allow",
      "Action": "eks:DescribeCluster",
      "Resource": "arn:aws:eks:${REGION}:${ACCOUNT_ID}:cluster/${CLUSTER}"
    }
  ]
}
EOF
)
aws iam put-role-policy --role-name "$ROLE_NAME" --policy-name deploy --policy-document "$PERMISSIONS"

echo "==> ECR repositories"
for name in "${SERVICES[@]}"; do
  aws ecr describe-repositories --region "$REGION" --repository-names "video-stream/${name}" >/dev/null 2>&1 \
    || aws ecr create-repository --region "$REGION" --repository-name "video-stream/${name}" >/dev/null
done

echo "==> EKS access entry (cluster: ${CLUSTER})"
if ! aws eks describe-access-entry --region "$REGION" --cluster-name "$CLUSTER" --principal-arn "$ROLE_ARN" >/dev/null 2>&1; then
  # 作ったばかりの IAM ロールは EKS から見えるまで少し時間がかかり、その間は "invalid principal" で失敗するため待って再試行する
  created=false
  for _ in 1 2 3 4 5 6; do
    if aws eks create-access-entry --region "$REGION" --cluster-name "$CLUSTER" --principal-arn "$ROLE_ARN" >/dev/null 2>&1; then
      created=true
      break
    fi
    echo "    IAM ロールが EKS に反映されるのを待っています..."
    sleep 10
  done
  # それでも作れなければ、エラー内容を表示して止める
  $created || aws eks create-access-entry --region "$REGION" --cluster-name "$CLUSTER" --principal-arn "$ROLE_ARN" >/dev/null
fi
# Namespace / StorageClass / GatewayClass などクラスタ全体のリソースも apply するため、クラスタ管理者の権限を付ける
aws eks associate-access-policy --region "$REGION" --cluster-name "$CLUSTER" --principal-arn "$ROLE_ARN" \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy \
  --access-scope type=cluster >/dev/null

echo ""
echo "==> Done. ロールの ARN を GitHub のリポジトリシークレット AWS_ROLE_ARN に登録してください:"
echo "    gh secret set AWS_ROLE_ARN --repo ${REPO} --body ${ROLE_ARN}"
