# Platform — ECS Cluster & ALB Infrastructure

This Pulumi project manages the ECS clusters, ALB listener rules, and target groups for all backend and frontend services on AWS. Before you can run a CI/CD pipeline for a new service, you must register the service here so that a target group and ALB routing rule are created.

## What This Project Manages

- **ECS Clusters** — `bc-frontend-cluster` (Next.js and React.js apps) and `bc-backend-cluster` (Go/.NET apps)
- **ALB Target Groups** — one per service per environment (dev/prod), used by the pipeline
- **ALB Listener Rules** — routes traffic from the ALB to each service's target group
- **Service Connect Namespaces** — `dev.internal` and `prod.internal` for inter-service communication
- **CloudWatch Monitoring** — optional alarms and log metric filters per service

## Stacks

| Stack    | Config File          | Environment             |
| -------- | -------------------- | ----------------------- |
| `dev`    | `Pulumi.dev.yaml`    | Development             |
| `prod`   | `Pulumi.prod.yaml`   | Production              |
| `shared` | `Pulumi.shared.yaml` | Shared IAM/CI resources |

---

## Adding a New Service

Follow these steps whenever a new service needs to be deployed to ECS.

### Step 1: Identify the Cluster

Choose the cluster based on the service's runtime:

| Cluster               | `name` in YAML | Services                       |
| --------------------- | -------------- | ------------------------------ |
| `bc-backend-cluster`  | `backend`      | Go, .NET (port 8080 typically) |
| `bc-frontend-cluster` | `frontend`     | Node.js (port 3000 typically)  |

### Step 2: Add the Service to `Pulumi.dev.yaml`

Open `Pulumi.dev.yaml` and add a new entry under `cluster:clusters[name=<cluster>].services`.

**Example — new backend service:**

```yaml
cluster:clusters:
  - name: backend
    # ... existing fields ...
    services:
      # ... existing services ...
      - name: my-new-service # lowercase, hyphenated, matches ECR repo name
        public: true # true = public ALB, false = private ALB
        hosts:
          - devapiaws.bookcabin.com # ALB hostname (use existing host for path-based routing)
        paths:
          - /my-new-service/* # URL path prefix for this service
        HealthCheckPath: /health # ALB health check endpoint (must return 200)
        stripPathPrefix: true # strip /my-new-service prefix before forwarding
        priority: 1030 # ALB rule priority — must be unique across all services
        port: 8080 # container port your app listens on
```

**Example — new frontend service:**

```yaml
cluster:clusters:
  - name: frontend
    # ... existing fields ...
    services:
      # ... existing services ...
      - name: my-frontend-app
        public: true
        hosts:
          - dev-my-frontend.bookcabin.com
        paths:
          - /*
        priority: 1025
        port: 3000
```

### Step 3: Add the Service to `Pulumi.prod.yaml`

Repeat the same entry in `Pulumi.prod.yaml` with production-appropriate values:

```yaml
cluster:clusters:
  - name: backend
    services:
      - name: my-new-service
        public: true
        hosts:
          - api-ibe.bookcabin.com
        paths:
          - /my-new-service/*
        HealthCheckPath: /health
        stripPathPrefix: true
        priority: 1030
        port: 8080
```

> **Tip**: Start with `public: false` in prod if the service is not ready for public traffic. Change to `true` when ready.

### Step 4: Open a Pull Request

> **Do not run `pulumi up` locally** unless you are SRE or DevOps. Infrastructure changes are applied exclusively through the Azure Pipeline, which requires PR approval before any resources are created or modified.

1. Commit your changes to `Pulumi.dev.yaml` and `Pulumi.prod.yaml` on a feature branch
2. Open a Pull Request targeting `main` (or `master`)
3. Request review from the SRE/DevOps team
4. Once approved and merged, the pipeline automatically runs `pulumi up` for the affected stacks

The pipeline runs `pulumi preview` on PR (safe, no changes) and `pulumi up` only after merge with approval. Dev is applied first; prod requires a separate approval gate.

### Step 5: Get the Target Group ARNs

After `pulumi up` completes, retrieve the target group ARNs for both environments:

```bash
# Dev target group ARN
pulumi stack --stack dev output | grep my-new-service

# Or use AWS CLI directly (replace with your service name and env)
aws elbv2 describe-target-groups \
  --names "my-new-service-dev" \
  --query "TargetGroups[0].TargetGroupArn" \
  --output text \
  --region ap-southeast-1

# Prod target group ARN
aws elbv2 describe-target-groups \
  --names "my-new-service-prod" \
  --query "TargetGroups[0].TargetGroupArn" \
  --output text \
  --region ap-southeast-1
```

The ARN format looks like:

```
arn:aws:elasticloadbalancing:ap-southeast-1:626883896657:targetgroup/my-new-service-dev/abc123def456
```

### Step 6: Use the ARNs in Your Pipeline

Add the ARNs to `azure-pipelines.yml` in your service repository:

```yaml
extends:
  template: pipelines/go-ecs-deployment.yml@ci
  parameters:
    variableGroups:
      - azure-ci-deployment
    targetGroupArnDev: "arn:aws:elasticloadbalancing:ap-southeast-1:626883896657:targetgroup/my-new-service-dev/abc123def456"
    targetGroupArnProd: "arn:aws:elasticloadbalancing:ap-southeast-1:626883896657:targetgroup/my-new-service-prod/def456abc789"
```

See the [cicd-templates deployment guide](../../backend/cicd-templates/docs/usage-guide.md) for the full pipeline setup.

---

## Service Configuration Reference

All fields under `services` in the cluster YAML:

| Field             | Required | Type   | Description                                                                                                                                                                   |
| ----------------- | -------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`            | Yes      | string | Service identifier. Must be lowercase and hyphenated. Must match the ECR repository name and the ECS service name.                                                            |
| `public`          | Yes      | bool   | `true` routes via the public ALB; `false` routes via the private (internal) ALB.                                                                                              |
| `hosts`           | Yes      | list   | ALB hostnames to match (e.g., `devapiaws.bookcabin.com`). Multiple services share the same hostname and are differentiated by `paths`.                                        |
| `paths`           | Yes      | list   | URL path patterns to match (e.g., `/my-service/*`). Use `/*` to match all paths when the service owns the entire hostname.                                                    |
| `HealthCheckPath` | No       | string | ALB health check URL path. Defaults to `/`. Must return HTTP 200.                                                                                                             |
| `stripPathPrefix` | No       | bool   | When `true`, strips the first path segment before forwarding to the container (e.g., `/my-service/users` → `/users`). Use `false` when the app handles its own prefix.        |
| `priority`        | Yes      | int    | ALB listener rule priority. **Must be unique** across all services on the same ALB. Lower numbers are evaluated first. Check existing priorities in the YAML before choosing. |
| `port`            | Yes      | int    | Container port the application listens on. Used for ALB target group health checks and ECS service registration.                                                              |
| `s3Buckets`       | No       | list   | S3 bucket names this service's task role should have read/write access to. Buckets must already exist.                                                                        |
| `monitoring`      | No       | object | CloudWatch monitoring configuration (see below).                                                                                                                              |

### Monitoring Configuration

Optional block under a service entry:

```yaml
monitoring:
  enabled: true
  logGroupName: /ecs/my-service-dev # CloudWatch log group to monitor
  unhandledLogPattern: "ERROR" # Log pattern to alarm on
  latencySLASecs: 2.0 # Alarm when p99 latency exceeds this
  desiredTaskCount: 1 # Expected running task count
  minHealthyHosts: 1 # Minimum healthy targets in target group
```

### Priority Selection

ALB rules are evaluated in ascending priority order (lowest number = first match). Current priority ranges in use:

**Dev backend ALB:**

- `800` — ota-bc-ui (frontend, broad catch-all)
- `999` — gobc-travel
- `1012` — managebooking
- `1020` — esimtools
- `1021` — gobc-sandbox
- `1022` — pkfare-flight

**Prod backend ALB:**

- `50` — gobc-travel
- `1021` — gobc-sandbox

Pick a priority that doesn't conflict with existing rules. For new path-based backend services, values in the `1000–1100` range work well. For services that own an entire hostname (wildcard `/*`), use a higher number (evaluated later) to avoid blocking other path-based rules.

---

## Current Services

### Dev

**Backend cluster** (`bc-backend-cluster-dev`):

| Service           | Hosts                                      | Path               | Port | Public |
| ----------------- | ------------------------------------------ | ------------------ | ---- | ------ |
| `gobc-sandbox`    | devapiaws.bookcabin.com                    | `/gobc-sandbox/*`  | 8080 | Yes    |
| `gobc-travel`     | devapiaws.bookcabin.com                    | `/gobc-travel/*`   | 8080 | Yes    |
| `managebooking`   | devapiaws.bookcabin.com                    | `/managebooking/*` | 8080 | Yes    |
| `pkfare-flight`   | devapiaws.bookcabin.com                    | `/pkfare-flight/*` | 8080 | Yes    |
| `cithotel-search` | cithotel-search-dev.internal.bookcabin.com | `/*`               | 8080 | No     |

**Frontend cluster** (`bc-frontend-cluster-dev`):

| Service     | Hosts                       | Path | Port | Public |
| ----------- | --------------------------- | ---- | ---- | ------ |
| `esimtools` | dev-esimtools.bookcabin.com | `/*` | 3000 | Yes    |
| `ota-bc-ui` | dev-aws-v3.bookcabin.com    | `/*` | 3000 | Yes    |

### Prod

**Backend cluster** (`bc-backend-cluster-prod`):

| Service        | Hosts                 | Path              | Port | Public |
| -------------- | --------------------- | ----------------- | ---- | ------ |
| `gobc-sandbox` | api-ibe.bookcabin.com | `/gobc-sandbox/*` | 8080 | Yes    |
| `gobc-travel`  | api-ibe.bookcabin.com | `/gobc-travel/*`  | 8080 | Yes    |

**Frontend cluster** (`bc-frontend-cluster-prod`):

| Service     | Hosts                     | Path | Port | Public |
| ----------- | ------------------------- | ---- | ---- | ------ |
| `esimtools` | esimvoucher.bookcabin.com | `/*` | 3000 | Yes    |

---

## Prerequisites

- Pulumi CLI installed: `brew install pulumi`
- AWS credentials configured (`aws configure` or assume the `pulumi-executor` profile)
- Access to the `gobc-pulumi` Pulumi organization
- Go 1.21+ (for running the Pulumi Go program)

Log in to Pulumi before running any commands:

```bash
pulumi login
```

---

## Troubleshooting

**`duplicate listener rule priority`** — The `priority` you chose already exists on the ALB. Inspect existing rules with:

```bash
aws elbv2 describe-rules \
  --listener-arn <listenerARN> \
  --region ap-southeast-1 \
  --query "Rules[*].{Priority:Priority,Conditions:Conditions}" \
  --output table
```

Pick an unused priority number.

**`target group not found` after `pulumi up`** — Check the Pulumi output for errors during the `aws:alb:TargetGroup` creation step. Common cause: invalid `HealthCheckPath` (must start with `/`).

**Changes not visible after `pulumi up`** — Confirm you ran against the right stack. Dev changes go to `--stack dev`, prod to `--stack prod`.
