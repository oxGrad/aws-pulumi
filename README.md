# gobc-pulumi

Infrastructure as Code for BookCabin's AWS platform, managed with Pulumi (Go).

This repository contains two independent Pulumi projects with different lifecycles and purposes.

---

## Projects

### `bootstrap/` — Run Once Per AWS Account

Provisions the foundational resources required before any other Pulumi project can run:

- **KMS key** — encrypts the Pulumi state bucket
- **S3 state bucket** — stores Pulumi state for all stacks
- **IAM executor role** — assumed by the CI/CD pipeline to apply infrastructure changes
- **IAM CI user** — used by Azure Pipelines to push ECR images and deploy ECS services

This project is run manually once when setting up a new AWS account. It has a single stack (`shared`). Do not run it again unless you are adding a new AWS account or rotating credentials.

See [`bootstrap/README.md`](bootstrap/README.md) for setup instructions.

---

### `platform/` — Ongoing Infrastructure

Manages all environment infrastructure. This is the project you will interact with when onboarding a new service or enabling monitoring and autoscaling.

What it manages:

- **ECS clusters** — `bc-backend-cluster` and `bc-frontend-cluster`
- **ALB target groups and listener rules** — one per service per environment
- **ECR repositories** — one per service (created in dev, looked up in prod)
- **IAM task and execution roles** — one pair per service
- **CloudWatch alarms** — optional, per service
- **SNS topics** — critical and warning notification channels per environment
- **Teams notifier Lambda** — routes SNS alerts to Microsoft Teams webhooks
- **ECS Application Auto Scaling** — optional, per service

Has three stacks:

| Stack    | Config File          | Purpose                 |
| -------- | -------------------- | ----------------------- |
| `dev`    | `Pulumi.dev.yaml`    | Development environment |
| `prod`   | `Pulumi.prod.yaml`   | Production environment  |
| `shared` | `Pulumi.shared.yaml` | Shared IAM/CI resources |

See [`platform/README.md`](platform/README.md) for the full service onboarding guide.

---

## Service Onboarding Flow

When adding a new service, follow this order. Steps 1–2 are in this repo; steps 3–4 are not.

```
1. Register service in platform/          (this repo)
   └─ Add to Pulumi.dev.yaml + Pulumi.prod.yaml
   └─ PR → merge → pipeline runs pulumi up
   └─ Outputs: target group ARNs, ECR URL, IAM role ARNs

2. Set up deployment pipeline             (cicd-templates repo)
   └─ Use target group ARNs from step 1
   └─ Run pipeline → ECS service is created and running
   └─ Verify service is healthy in ECS console

3. Enable monitoring + autoscaling        (this repo, after step 2)
   └─ Add `monitoring:` block to the service config
   └─ Add `autoscaling:` block to the service config
   └─ PR → merge → pipeline runs pulumi up again
```

> **Monitoring and autoscaling must be configured after the ECS service exists.**
> CloudWatch alarms reference the live target group and ECS service. Autoscaling registers
> the ECS service as a scalable target. If the ECS service has not been deployed yet (step 2
> not complete), `pulumi up` will fail or produce alarms that never resolve.

This applies to all environments: development, staging, and production each require the
deployment pipeline to run first before monitoring or autoscaling can be enabled for that stack.

---

## Environments

| Environment | Pulumi Stack | AWS Account  |
| ----------- | ------------ | ------------ |
| Development | `dev`        | 626883896657 |
| Production  | `prod`       | 626883896657 |

Infrastructure changes are applied exclusively through the Azure Pipeline. Do not run `pulumi up` locally unless you are SRE/DevOps with explicit approval.
