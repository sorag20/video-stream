# video-stream

Go + Kubernetes + HLS による動画配信バックエンドのポートフォリオプロジェクトです。  
ユーザーが動画をアップロードすると、バックグラウンドで FFmpeg により HLS 形式に変換され、ストリーミング配信できます。  
React 製の Web UI から、アップロード・変換状況の確認・ブラウザ再生まで一通り操作できます。

## アーキテクチャ

```
[Client (Browser)]
    │
    ▼
[Gateway API (Envoy Gateway)]
    │
    ├─ GET  / ───────► [Web (React SPA / nginx) :8080]
    │
    ├─ POST /videos ──► [Upload Service :8080]
    │                        │
    │                   MinIO (originals bucket) に保存
    │                   PostgreSQL にメタデータ保存 (status: pending)
    │                   Redis キューにジョブ投入
    │
    │                   [Transcoder Service]  ← Redis から BLPOP
    │                        │
    │                   FFmpeg で HLS 変換 (H.264/AAC, 720p, 6秒セグメント)
    │                   MinIO (hls bucket) にアップロード
    │                   PostgreSQL ステータス更新 (status: ready)
    │
    └─ GET /stream ──► [Streaming Service :8082]
                             │
                        MinIO から .m3u8 / .ts を取得して配信
```

### 使用技術

| 役割 | 技術 |
|---|---|
| 言語 | Go 1.23 |
| HTTP ルーター | go-chi/chi v5 |
| データベース | PostgreSQL 16 (pgx/v5) |
| キュー | Redis 7 (go-redis/v9) |
| オブジェクトストレージ | MinIO 互換 ([Silo](https://silo.pgsty.com/) / `pgsty/minio`) (minio-go/v7) ※公式イメージは配布終了 |
| 動画変換 | FFmpeg (H.264 / AAC / HLS) |
| フロントエンド | React 19 + TypeScript / Vite |
| HLS 再生 | hls.js (Safari はネイティブ再生) |
| コンテナオーケストレーション | Kubernetes (Docker Desktop / Amazon EKS) |
| 外部公開 (L7 ルーティング) | Gateway API (Envoy Gateway) |
| マニフェスト管理 | Kustomize |
| クラウド | AWS (EKS / ECR / NLB / ACM / Route 53 / Parameter Store) |
| CI/CD | GitHub Actions (OIDC で AWS に認証) |

---

## ディレクトリ構成

```
video_stream/
├── services/
│   ├── upload/          # Upload Service (POST /videos, GET /videos)
│   ├── transcoder/      # Transcoder Worker (Redis → FFmpeg → MinIO)
│   └── streaming/       # Streaming Service (GET /stream/{id}/...)
├── pkg/
│   ├── models/          # 共通データモデル
│   ├── db/              # PostgreSQL (pgxpool, VideoRepository)
│   ├── queue/           # Redis キュークライアント
│   └── storage/         # MinIO クライアント
├── web/                 # Web フロントエンド (React + Vite)
│   ├── src/
│   └── nginx/           # 本番配信用 nginx 設定 (静的配信 + API プロキシ)
├── k8s/
│   ├── base/            # Kustomize ベースマニフェスト（ローカル用）
│   └── overlays/
│       └── eks/         # EKS 用の差分（ECR イメージ・NLB・Basic 認証）
├── deploy/
│   ├── build-images.sh  # ローカルイメージビルドスクリプト
│   └── eks/             # EKS へのデプロイスクリプト・GitHub Actions 用の AWS 準備スクリプト
├── .github/workflows/   # リリース PR の作成・EKS へのデプロイ (GitHub Actions)
└── docker-compose.yml   # ローカル開発用
```

---

## ローカル開発 (Docker Compose)

### 前提条件

- Docker Desktop

### 起動

```bash
# 全サービスをビルド・起動
docker compose up --build
```

### エンドポイント

| サービス | URL |
|---|---|
| Web UI | http://localhost:3000 |
| Upload API | http://localhost:8080 |
| Streaming API | http://localhost:8082 |
| MinIO Console | http://localhost:9001 (minioadmin / minioadmin) |

### 動作確認

ブラウザで http://localhost:3000 を開き、動画をアップロードすると、変換完了後にそのまま再生できます。

API を直接叩く場合:

```bash
# 1. 動画をアップロード
curl -X POST http://localhost:8080/videos \
  -F "title=My Video" \
  -F "file=@/path/to/video.mp4"
# → {"id":"<VIDEO_ID>", "status":"pending", ...}

# 2. 変換ステータスを確認（ready になるまで待つ）
curl http://localhost:8080/videos/<VIDEO_ID>
# → {"id":"...", "status":"ready", "hls_key":"<VIDEO_ID>/hls/index.m3u8", ...}

# 3. HLS プレイリストを取得
curl http://localhost:8082/stream/<VIDEO_ID>/index.m3u8

# 4. VLC や hls.js でストリーミング再生
# URL: http://localhost:8082/stream/<VIDEO_ID>/index.m3u8
```

### フロントエンド開発 (Vite)

Docker Compose の `web` サービスは Vite の開発サーバー（`web/Dockerfile` の `dev` ステージ）で動いており、`web/` をバインドマウントしています。`web/src` などを編集すると http://localhost:3000 にホットリロードで即時反映されるため、ビルドし直す必要はありません。

`package.json` の依存関係を変更したときだけ、イメージを作り直し、node_modules のボリュームも新しくします。

```bash
docker compose up -d --build -V web
```

Docker を使わずにホスト上で Vite を動かすこともできます（バックエンドは Docker Compose で起動しておく）。

```bash
cd web
npm install
npm run dev   # http://localhost:5173
```

Vite の dev サーバーは `/videos` を Upload Service、`/stream` を Streaming Service にプロキシします。プロキシ先は環境変数 `UPLOAD_SERVICE_URL` / `STREAMING_SERVICE_URL` で変更でき、未指定時は `localhost:8080` / `localhost:8082` です。

本番（Kubernetes）では、同じ Dockerfile の最終ステージ（ビルド済みファイルを nginx で配信）を使います。

---

## Kubernetes デプロイ (Docker Desktop)

### 前提条件

- Docker Desktop で Kubernetes を有効化（Settings → Kubernetes → Enable Kubernetes、Cluster provisioning method は **Kubeadm**）
  - kubeadm 方式では Docker Engine とイメージストアを共有するため、ローカルでビルドしたイメージをそのまま使える
- `kubectl` がインストール済み
- Envoy Gateway（Gateway API の実装）がインストール済み

```bash
# Envoy Gateway のインストール（Gateway API の CRD も同梱）
kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.9.3/install.yaml
kubectl wait -n envoy-gateway-system --for=condition=Available deployment/envoy-gateway --timeout=5m
```

### デプロイ手順

```bash
# 1. ローカルイメージをビルド
bash deploy/build-images.sh

# 2. Kubernetes にデプロイ
kubectl apply -k k8s/base/

# 3. Pod の起動を確認
kubectl get pods -n video-stream -w

# 4. Gateway / HTTPRoute の状態を確認（Gateway が PROGRAMMED=True になれば公開完了）
kubectl get gateway,httproute -n video-stream
```

### デプロイ後の動作確認

Gateway はホスト名を限定していないため、Docker Desktop が公開する http://localhost でアクセスできます（`/etc/hosts` の編集は不要）。

Web UI: http://localhost/

```bash
# 動画アップロード
curl -X POST http://localhost/videos \
  -F "title=My Video" \
  -F "file=@/path/to/video.mp4"

# ステータス確認
curl http://localhost/videos/<VIDEO_ID>

# HLS 再生 URL
# http://localhost/stream/<VIDEO_ID>/index.m3u8
```

### 削除

```bash
kubectl delete -k k8s/base/
```

---

## AWS デプロイ (EKS)

個人で使う前提の構成です。アプリ自体には認証がないため、入口で HTTPS + Basic 認証をかけて保護します。

```
[Browser] ──HTTPS──► [NLB (TLS 終端 / ACM 証明書)] ──HTTP──► [Envoy Gateway (Basic 認証)] ──► 各サービス
```

| リソース | 内容 |
|---|---|
| クラスタ | マネジメントコンソールで作成（カスタム設定・Auto Mode なし）。デフォルト VPC を使い、NAT Gateway は作らない |
| ノード | マネージドノードグループ t4g.large × 1（Graviton / arm64、Amazon Linux 2023）。PV のため ap-northeast-1a に固定 |
| イメージ | ECR |
| 永続化 | EBS gp3（`reclaimPolicy: Retain`） |
| 外部公開 | AWS Load Balancer Controller が Envoy の Service を NLB として作成 |
| AWS の権限 | EBS CSI Driver と Load Balancer Controller には EKS Pod Identity で IAM ロールを割り当てる |
| 認証 | Envoy Gateway の `SecurityPolicy`（Basic 認証） |
| デプロイ | GitHub Actions（develop → main のリリース PR をマージするとビルド・ECR への push・apply。AWS へは OIDC で認証） |
| 設定値・シークレット | Parameter Store（Git には含めず、デプロイ時に差し込む） |

EKS 用の差分は [k8s/overlays/eks/](k8s/overlays/eks/) に置き、`k8s/base` はローカル用のまま残しています。ノード 1 台向けに、レプリカ数・CPU の予約・更新方法・PostgreSQL のデータディレクトリを調整しています。

### 月額費用の目安（東京リージョン）

| 項目 | 常時起動 | ノードを使うときだけ起動（1日2時間） |
|---|---|---|
| EKS コントロールプレーン | $73 | $73 |
| ノード t4g.large | $63 | $5 |
| EBS（ノード 30GB + PV 25GB） | $5 | $3 |
| NLB + パブリック IPv4 × 2 | $26 | $23 |
| Route 53 + ドメイン | $2 | $2 |
| **合計** | **約 $170** | **約 $106** |

データ転送は月 100GB まで無料で、超えた分は $0.114/GB です（720p で 1 時間の視聴あたり約 1.2GB）。

### 前提条件

```bash
brew install awscli kubectl helm gh
aws login                                    # クラスタを作るコンソールと同じユーザーでログインする
gh auth login
gh auth refresh -h github.com -s workflow    # .github/workflows/ を push するのに必要
```

- AWS Budgets で予算アラートを設定しておく
- ドメインを持っていて、Route 53 にホストゾーンがあること

### デプロイ手順

AWS のリソースはマネジメントコンソールで作り、クラスタの中身（Envoy Gateway など）と GitHub まわりはコマンドで設定します。

#### 1. IAM ロールを作る（コンソール）

先に、Load Balancer Controller 用のポリシーを作ります。IAM →「ポリシーを作成」→ JSON に [iam_policy.json](https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/main/docs/install/iam_policy.json) の中身を貼り付け、`AWSLoadBalancerControllerIAMPolicy` という名前で作成します。

続いて、IAM →「ロールを作成」→ AWS のサービスで、次の 4 つのロールを作ります。

| ロール名 | ユースケース | ポリシー |
|---|---|---|
| `video-stream-cluster-role` | EKS - Cluster | `AmazonEKSClusterPolicy` |
| `video-stream-node-role` | EC2 | `AmazonEKSWorkerNodePolicy`、`AmazonEKS_CNI_Policy`、`AmazonEC2ContainerRegistryReadOnly` |
| `video-stream-ebs-csi-role` | EKS - Pod Identity | `AmazonEBSCSIDriverPolicy` |
| `video-stream-lbc-role` | EKS - Pod Identity | `AWSLoadBalancerControllerIAMPolicy` |

EBS CSI Driver と Load Balancer Controller の権限は、ノード用ロールではなく Pod Identity で渡します。Amazon Linux 2023 のノードはメタデータの hop limit が 1 のため、Pod からノード用ロールの認証情報を取得できないからです。

#### 2. 証明書を発行する（コンソール）

ACM（**東京リージョン**）で `video.example.com` のパブリック証明書を DNS 検証でリクエストし、**「Route 53 でレコードを作成」**で検証用レコードを追加します。状態が「発行済み」になるまで待ちます（数分〜30 分）。

#### 3. クラスタを作る（コンソール）

EKS →「クラスターを作成」で、次のように設定します（作成に 10〜15 分。ここから課金が始まる）。

| 設定 | 値 |
|---|---|
| 設定オプション | **カスタム設定**（EKS Auto Mode はオフ） |
| 名前 | `video-stream` |
| クラスター IAM ロール | `video-stream-cluster-role` |
| アップグレードポリシー | **標準**（延長サポートのコントロールプレーン $0.60/時間 を避ける） |
| 認証モード | EKS API（GitHub Actions 用のアクセスエントリを使うため） |
| VPC / サブネット | デフォルト VPC / ap-northeast-1a・1c・1d |
| エンドポイントアクセス | パブリックおよびプライベート |
| オブザーバビリティ | すべてオフ |
| アドオン | kube-proxy、CoreDNS、Amazon VPC CNI、EKS Pod Identity Agent、Amazon EBS CSI Driver のみ。Metrics Server や External DNS などは外す（ノード 1 台では CPU が足りなくなる） |

#### 4. Pod Identity を関連付ける（コンソール）

クラスタの「アクセス」タブ →「Pod Identity の関連付け」で、次の 2 つを作成します。

| IAM ロール | 名前空間 | サービスアカウント |
|---|---|---|
| `video-stream-ebs-csi-role` | `kube-system` | `ebs-csi-controller-sa` |
| `video-stream-lbc-role` | `kube-system` | `aws-load-balancer-controller` |

#### 5. ノードグループを作る（コンソール）

クラスタの「コンピューティング」タブ →「ノードグループを追加」で、次のように設定します。

| 設定 | 値 |
|---|---|
| 名前 | `ng-arm` |
| ノード IAM ロール | `video-stream-node-role` |
| AMI タイプ / インスタンスタイプ | Amazon Linux 2023 (ARM) / t4g.large（オンデマンド） |
| ディスクサイズ | 30 GiB |
| 希望 / 最小 / 最大サイズ | 1 / 0 / 1 |
| サブネット | デフォルト VPC の ap-northeast-1a のみ（EBS は AZ をまたげないため） |

#### 6. Envoy Gateway と AWS Load Balancer Controller を入れる（ターミナル）

```bash
aws eks update-kubeconfig --name video-stream --region ap-northeast-1
kubectl get nodes   # 1 台が Ready になっていること

kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.9.3/install.yaml
kubectl wait -n envoy-gateway-system --for=condition=Available deployment/envoy-gateway --timeout=5m

VPC_ID=$(aws eks describe-cluster --name video-stream --region ap-northeast-1 \
  --query cluster.resourcesVpcConfig.vpcId --output text)
helm repo add eks https://aws.github.io/eks-charts && helm repo update
helm install aws-load-balancer-controller eks/aws-load-balancer-controller -n kube-system \
  --set clusterName=video-stream --set region=ap-northeast-1 --set vpcId=$VPC_ID
```

#### 7. Parameter Store に値を登録する（コンソール）

アカウント固有の値とシークレットは Git に含めず、デプロイ時に [deploy/eks/deploy.sh](deploy/eks/deploy.sh) がここから読んでマニフェストに差し込みます。Systems Manager →「パラメータストア」→「パラメータの作成」で、次の 4 つを登録します（利用枠は標準、SecureString の KMS キーはデフォルトの `alias/aws/ssm`）。

| パラメータ | 種類 | 値 |
|---|---|---|
| `video-stream-eks-subnet-id` | String | デフォルト VPC の ap-northeast-1a のサブネット ID（VPC コンソールの「サブネット」で確認） |
| `video-stream-eks-acm-cert-arn` | String | 手順 2 の証明書の ARN（`arn:aws:acm:...` で始まる） |
| `video-stream-eks-secret-env` | SecureString | PostgreSQL / MinIO のパスワード（[secret.env.example](k8s/overlays/eks/secret.env.example) の形式の 3 行） |
| `video-stream-eks-htpasswd` | SecureString | Basic 認証のユーザー（`me:{SHA}...` の 1 行をそのまま） |

シークレットの値は手元で作ってから、コンソールに貼り付けます。

```bash
# video-stream-eks-secret-env の値（表示された 3 行を貼り付ける）
printf 'POSTGRES_PASSWORD=%s\nMINIO_ACCESS_KEY=%s\nMINIO_SECRET_KEY=%s\n' \
  "$(openssl rand -hex 24)" "$(openssl rand -hex 12)" "$(openssl rand -hex 24)"

# video-stream-eks-htpasswd の値
openssl rand -base64 24               # ログイン用のパスワード。パスワード管理アプリに保存する
setopt HIST_IGNORE_SPACE              # 先頭が空白のコマンドを履歴に残さない（zsh）
 htpasswd -nbs me '<パスワード>'       # 先頭に空白を入れる。Envoy は SHA 形式のみ対応のため -s を付ける
```

`video-stream-eks-secret-env` は、一度デプロイした後は変えないでください。PostgreSQL は最初に初期化したときのパスワードを保持するため、後から変えると接続できなくなります。

#### 8. GitHub Actions からデプロイできるようにする（ターミナル）

[deploy/eks/setup-github-actions.sh](deploy/eks/setup-github-actions.sh) が AWS 側の準備（GitHub の OIDC プロバイダ・IAM ロール・ECR リポジトリ・EKS のアクセスエントリ）をします。アクセスキーは発行せず、このリポジトリの main ブランチのワークフローだけが IAM ロールを引き受けられます（イミュータブル subject が有効なリポジトリにも対応）。

```bash
gh repo create <owner>/video-stream --private --source . --remote origin
gh auth setup-git                                              # git push で gh のログイン情報を使う
bash deploy/eks/setup-github-actions.sh <owner>/video-stream   # クラスタを作り直したら再実行する
# スクリプトが最後に出力する gh secret set AWS_ROLE_ARN ... を実行し、ロールの ARN をシークレットに登録する

# GitHub Actions がリリース PR を作れるようにする
# （Settings > Actions > General の「Allow GitHub Actions to create and approve pull requests」と同じ）
gh api -X PUT repos/<owner>/video-stream/actions/permissions/workflow \
  -f default_workflow_permissions=read -F can_approve_pull_request_reviews=true
```

#### 9. 最初のデプロイ（ターミナル）

```bash
# main に push すると .github/workflows/deploy.yml が、イメージのビルド・ECR への push・デプロイまで行う
git branch -M main
git push -u origin main
gh run watch                                # GitHub の Actions タブでも見られる
kubectl get pods -n video-stream            # すべて Running になること
kubectl get svc -n envoy-gateway-system     # EXTERNAL-IP に NLB の DNS 名が出ること

# 開発用の develop ブランチを作り、プルリクエストの既定のマージ先にする（以降の流れは「開発の流れ」を参照）
git switch -c develop
git push -u origin develop
gh repo edit --default-branch develop
```

#### 10. DNS を設定する（コンソール）

Route 53 のホストゾーンで、`video` の A レコード（エイリアス）を作り、「Network Load Balancer へのエイリアス」→ 東京 →`k8s-envoygat-envoyvid-...` を選びます。

https://video.example.com を開き、Basic 認証のダイアログが出れば完了です。

```bash
curl -I https://video.example.com   # 認証なしでは 401 Unauthorized が返る

# API を直接叩く場合
curl -u me:<パスワード> -X POST https://video.example.com/videos \
  -F "title=My Video" \
  -F "file=@/path/to/video.mp4"
```

### トラブルシューティング

構築時に実際に起きた問題です。

| 症状 | 原因 | 対処 |
|---|---|---|
| PVC が `Pending`、または Envoy の Service の EXTERNAL-IP が `<pending>`。EBS CSI Driver のログに `no EC2 IMDS role found` | ノード用ロールの権限を Pod から使おうとしている（hop limit が 1 のため届かない） | 手順 1・4 の Pod Identity を設定し、`kubectl rollout restart deployment ebs-csi-controller aws-load-balancer-controller -n kube-system` |
| Pod が `Pending` で、イベントに `Insufficient cpu` | 不要なアドオン（Metrics Server は 200m）が CPU を予約している | クラスタの「アドオン」タブで削除する |
| postgres が `initdb: ... It contains a lost+found directory` で起動しない | EBS ボリュームの直下をそのままデータディレクトリにしている | overlay で `PGDATA` をサブディレクトリにしている。設定を変えた後も古い Pod のまま失敗し続ける場合は `kubectl delete pod postgres-0 -n video-stream` |
| NLB ができない。`kubectl describe svc -n envoy-gateway-system` に `CertificateNotFound` | 証明書が検証待ちのまま、または Parameter Store の ARN の誤り（`arn:arn:...` など） | 手順 2 の検証用レコードを作る。値を直したらデプロイのワークフローを手動実行する |
| ワークフローが `Not authorized to perform sts:AssumeRoleWithWebIdentity` で失敗 | IAM ロールの信頼ポリシーの `sub` が、GitHub が送る値と一致していない | `setup-github-actions.sh` を再実行する（イミュータブル subject の接頭辞を GitHub の API から取得して設定する） |
| `git push` が `without workflow scope` で拒否される | gh のトークンに `workflow` スコープがない | `gh auth refresh -h github.com -s workflow` |
| Basic 認証でログインできない | 入力しているパスワードと、登録したハッシュが一致していない | 下のコマンドで確かめ、不一致なら手順 7 の方法で作り直す |

```bash
# 入力したパスワードが video-stream-eks-htpasswd のハッシュと一致するか確かめる（パスワードは表示されない）
bash -c '
read -rs -p "パスワード: " PW; echo
stored=$(aws ssm get-parameter --region ap-northeast-1 --name video-stream-eks-htpasswd --with-decryption --query Parameter.Value --output text | sed -n "s/^me://p")
mine="{SHA}$(printf "%s" "$PW" | openssl dgst -sha1 -binary | openssl base64)"
[ "$stored" = "$mine" ] && echo "一致: このパスワードで正しい" || echo "不一致: 登録されたものとは別のパスワード"
'
```

### 開発の流れ

`develop` が開発ブランチ、`main` が本番（EKS にデプロイされている状態）です。`main` には直接 push せず、リリース PR 経由でマージします。

1. `develop` から作業ブランチを切り、`develop` 宛てのプルリクエストを作ってマージする
2. [.github/workflows/release-pr.yml](.github/workflows/release-pr.yml) が `develop` → `main` のリリース PR（ラベル `release`）を作る。リリース PR がすでにあれば、マージされたプルリクエストの一覧を更新する（[git-pr-release](https://github.com/x-motemen/git-pr-release)、本文は [.github/git-pr-release-template.erb](.github/git-pr-release-template.erb)）
3. リリース PR を **Create a merge commit** でマージすると、[.github/workflows/deploy.yml](.github/workflows/deploy.yml) がビルドして EKS にデプロイする（squash や rebase でマージすると `develop` と `main` の履歴がずれ、次のリリース PR に同じ変更がもう一度載る）

デプロイのワークフローは、`services/`・`pkg/`・`web/`・`k8s/` などデプロイ結果が変わるファイルを変更したときだけ動きます（対象は `deploy.yml` の `on.push.paths`）。

### 運用

```bash
# 使わないときはノードを 0 台にしてノード代を止める（コントロールプレーンと NLB の料金は残る）
# コンソールの「コンピューティング」→ ノードグループ →「編集」で希望サイズを変えても同じ
aws eks update-nodegroup-config --cluster-name video-stream --nodegroup-name ng-arm --region ap-northeast-1 \
  --scaling-config minSize=0,maxSize=1,desiredSize=0
# 再開するときは desiredSize=1

# Parameter Store の値だけを変えたとき（Basic 認証のパスワード変更など）は、main でデプロイのワークフローを手動実行する
# （Actions タブ →「Deploy to EKS」→「Run workflow」でも同じ。IAM ロールは main からしか引き受けられない）
gh workflow run deploy.yml --ref main
```

手元から直接デプロイすることもできます（ECR にあるイメージのタグ = コミットの SHA を指定する）: `TAG=<SHA> bash deploy/eks/deploy.sh`

### 削除

```bash
# NLB は Load Balancer Controller が消すため、クラスタより先にアプリを削除する
# （Gateway が消えると Envoy Gateway が Envoy の Service を消し、それに合わせて NLB も削除される）
kubectl delete namespace video-stream
```

その後、コンソールでノードグループ → クラスタの順に削除します。次のものは残るので、不要なら削除してください。

| 残るもの | 料金 |
|---|---|
| PV の EBS ボリューム（`Retain` のため） | かかる（EC2 コンソールのボリューム一覧から削除） |
| ECR のリポジトリ | イメージの容量分かかる |
| Route 53 のホストゾーン | $0.5/月 |
| IAM ロール（5 つ）・Parameter Store の値・ACM の証明書 | かからない |

---

## API リファレンス

### Upload Service

#### `POST /videos` — 動画アップロード

- Content-Type: `multipart/form-data`
- フィールド: `title` (string, 必須), `file` (動画ファイル, 必須, 最大 2GB)

```json
// レスポンス 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "My Video",
  "original_key": "550e8400-.../original.mp4",
  "hls_key": "",
  "status": "pending",
  "created_at": "2026-10-29T12:00:00Z",
  "updated_at": "2026-10-29T12:00:00Z"
}
```

#### `GET /videos` — 動画一覧

```json
// レスポンス 200 OK
[
  {
    "id": "550e8400-...",
    "title": "My Video",
    "status": "ready",
    "hls_key": "550e8400-.../hls/index.m3u8",
    ...
  }
]
```

#### `GET /videos/{id}` — 動画詳細

ステータス一覧:

| ステータス | 説明 |
|---|---|
| `pending` | アップロード完了、変換待ち |
| `processing` | FFmpeg 変換中 |
| `ready` | 変換完了、再生可能 |
| `failed` | 変換失敗 |

### Streaming Service

#### `GET /stream/{id}/index.m3u8` — HLS プレイリスト取得

動画ステータスが `ready` でない場合は `202 Accepted` を返します。

#### `GET /stream/{id}/{segment.ts}` — HLS セグメント取得

---

## 設計上のポイント

### 非同期トランスコード

動画変換は CPU 集約的な処理のため、Upload Service とは切り離した非同期 Worker として実装しています。Redis の `BLPOP` によるブロッキングデキューにより、ポーリング間隔なしで即座にジョブを取得できます。

### ステートレスなアプリケーション層

Upload Service と Streaming Service はステートレスな設計で、状態は PostgreSQL と MinIO のみが持ちます。Kubernetes 上で `replicas` を増やすだけで水平スケールできます。

### Transcoder のスケーリング

Transcoder も複数レプリカに対応しています。Redis のキューから各 Worker が独立してジョブを取得するため、デプロイ数を増やすだけで並列変換数を増やせます。

### Gateway API による外部公開

外部からのリクエストは Ingress ではなく、後継の標準である Gateway API で各サービスへ振り分けています（[k8s/base/gateway.yaml](k8s/base/gateway.yaml)）。

- 実装に依存するのは `GatewayClass` の `controllerName` だけで、`Gateway` / `HTTPRoute` は実装を問わず共通です。Envoy Gateway 以外の実装にも差し替えられます。
- Ingress ではコントローラー固有のアノテーションで書いていたタイムアウトを、標準フィールド (`HTTPRoute.spec.rules[].timeouts`) で表現しています。アップロード (`/videos`) のルールにだけ 600 秒を設定しています。
- Envoy はリクエストボディをストリーミングで中継するため、最大 2GB のアップロードでもボディサイズ上限の設定は不要です。

### フロントエンド

- API は常に同一オリジンで呼び出す設計です（開発時は Vite proxy、Docker Compose では Web コンテナの nginx、Kubernetes では Gateway API の HTTPRoute がパスで振り分け）。バックエンドに CORS 設定を持たせずに済みます。
- アップロードは `XMLHttpRequest` で送信し、進捗表示とキャンセルに対応しています。
- 変換待ち・変換中の動画があるときだけ一覧をポーリングし、`ready` になった時点で自動的に再生可能になります。
- hls.js は light ビルドを使い、さらにプレイヤーを遅延読み込みすることで初期バンドルを小さく保っています。

### セキュリティ

- Upload / Streaming Service は `distroless` イメージで攻撃面を最小化
- Transcoder は `alpine` + 非 root ユーザーで実行
- Web は `nginx-unprivileged` イメージで非 root 実行
- Streaming Service でパストラバーサル攻撃を防止 (`..` を含むパスを拒否)
- HLS セグメントには `Cache-Control: immutable` を設定し CDN キャッシュに対応
