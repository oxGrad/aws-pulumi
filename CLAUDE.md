# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

Two independent Pulumi projects, each with its own Go module:

- **`bootstrap/`** — one-time foundational setup: KMS key, S3 state bucket, IAM CI user + executor role. Run once per AWS account.
- **`platform/`** — ongoing environment infrastructure: ECS clusters, ALB target groups + listener rules, ECR repos, IAM roles, CloudWatch alarms, SNS topics. Has three stacks: `shared`, `dev`, `prod`.

## Commands

All commands run from within the project directory (`platform/` or `bootstrap/`):

```bash
# Verify the program compiles
go build ./...

# Preview changes (dry-run)
pulumi preview --stack dev

# Deploy
pulumi up --stack dev

# Inspect outputs
pulumi stack output --stack dev

# Select active stack
pulumi stack select dev
```

AWS profile used: `pulumi-executor` (set in `aws:profile` config).  
State backend: `s3://bc-sre-pulumi-state` (ap-southeast-1).

## Platform Architecture

`platform/main.go` dispatches on `ctx.Stack()`:
- `"shared"` → `shared.go`: provisions the Azure CI IAM user (env-agnostic resources).
- anything else (`dev`, `prod`) → `env.go`: provisions all environment-specific infrastructure.

### `env.go` flow

1. **Service Connect** — creates AWS Cloud Map HTTP namespaces for ECS service discovery.
2. **`ProvisionECS`** — the main workhorse (see below).
3. **`ProvisionNotifications`** — creates two SNS topics per env: `bc-{stack}-critical` and `bc-{stack}-warning`.
4. **`ProvisionServiceMonitoring`** — for each service with `monitoring.enabled: true`, wires up CloudWatch alarms pointing at the SNS topics.

### `ProvisionECS` (`composites/ecs.go`)

Iterates `cluster:clusters` config in three passes:

1. **ECR repos** — created in `dev`, looked up (not created) in all other stacks. One repo per unique service name across all clusters.
2. **IAM roles** — one execution + task role pair per unique service name. Skipped entirely for the `"frontend"` cluster.
3. **ECS cluster + services** — per cluster, creates the ECS cluster then for each service: a target group and ALB listener rule.

### Naming conventions

- ECS cluster: `bc-{cluster.Name}-cluster-{stack}`
- Target group / listener rule resource: `bc-{cluster.ShortName}-{svc.Name}-{stack}` — **max 32 chars** (AWS limit). `ShortName` is required in stack config to keep names short (e.g. `fe`, `be`).
- IAM roles: `{svc.Name}-{stack}-execution-role` / `{svc.Name}-{stack}-task-role`
- SNS topics: `bc-{stack}-critical`, `bc-{stack}-warning`

### Two-layer abstraction

- **`components/`** — thin wrappers around single AWS resources (one Pulumi component = one AWS resource). Each file is self-contained: `target_group.go`, `listener_rule.go`, `ecs_cluster.go`, `ecs_roles.go`, `ecr_repository.go`, `cloudwatch_alarm.go`, `sns_topic.go`, `log_metric_filter.go`.
- **`composites/`** — orchestration logic that wires multiple components together and reads stack config. Never touches AWS APIs directly.

### Stack config schema (`cluster:clusters`)

```yaml
cluster:clusters:
  - name: backend          # full cluster name
    shortName: be          # used in target group names (required, max ~4 chars)
    publicListenerARN: ... # ALB listener for public services
    privateListenerARN: ...# ALB listener for private services
    capacityProviderStrategies: [...]
    services:
      - name: my-service
        public: true|false
        hosts: [...]
        paths: [...]
        priority: 100        # ALB listener rule priority (must be unique per listener)
        port: 8080
        healthCheckPath: /health
        stripPathPrefix: true|false
        s3Buckets: [...]     # optional; grants task role S3 access
        monitoring:          # optional
          enabled: true
          logGroupName: /ecs/my-service
          unhandledLogPattern: "ERROR"
          latencySLASecs: 2.0
          desiredTaskCount: 2
          minHealthyHosts: 1
```

### Service monitoring alarms

When `monitoring.enabled: true`, `ProvisionServiceMonitoring` creates these CloudWatch alarms:
- **Critical**: 5xx error rate > 1%, healthy host count below `minHealthyHosts`, running task count below `desiredTaskCount`, unhandled exceptions in logs (if `logGroupName` set).
- **Warning**: CPU > 85%, memory > 80%, p99 latency > `latencySLASecs` (if set), severe instance count > 0.
