# Design: Add ECS Cluster Permissions to AzureCIUser

**Date:** 2026-04-14  
**File:** `platform/components/azure-ci-user.go`

## Summary

Extend the IAM deployment policy in `NewAzureCIUser` to grant the Azure Pipelines CI user full ECS cluster lifecycle permissions. This allows CI pipelines to create, describe, update, and delete ECS clusters as part of deployments.

## Change

Add a new `ECSCluster` IAM statement to `deploymentPolicyDoc` in `NewAzureCIUser`, inserted after the existing `ECSService` statement and before `PassRoleToECS`.

### New statement

```json
{
  "Sid": "ECSCluster",
  "Effect": "Allow",
  "Action": [
    "ecs:CreateCluster",
    "ecs:DeleteCluster",
    "ecs:UpdateCluster",
    "ecs:DescribeClusters"
  ],
  "Resource": "*"
}
```

### Policy description update

**Before:** `"ECR, ECS, and PassRole permissions for Azure Pipelines CI/CD deployments"`  
**After:** `"ECR, ECS (including cluster management), and PassRole permissions for Azure Pipelines CI/CD deployments"`

## Decisions

- **Resource scope:** `"*"` — consistent with the existing `ECSTaskDefinition` and `ECSService` statements in the same policy.
- **Statement grouping:** Dedicated `ECSCluster` statement, not merged into existing ECS statements — follows the existing pattern of one statement per AWS concept.
- **Actions:** Full lifecycle (`Create`, `Delete`, `Update`, `Describe`) — CI pipelines need to manage the full cluster lifecycle.

## Out of Scope

- No ECS cluster Pulumi resource is created — Pulumi manages only the IAM policy, not the cluster itself.
- No ARN-level scoping — left for a future hardening pass if needed.