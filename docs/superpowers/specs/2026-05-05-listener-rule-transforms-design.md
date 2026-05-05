# Listener Rule: URL Path Transforms & Multi-Value Conditions

**Date:** 2026-05-05  
**Branch:** feat/provision-backend-ecs-cluster  
**Status:** Approved

## Problem

The current `serviceConfig` accepts scalar `host` and `path` strings. ALB `HostHeader` and `PathPattern` conditions support multiple values natively, but there is no way to express this in the stack config. Additionally, when a service is mounted at a sub-path (e.g. `/gobc-sandbox/*`), the backend receives the full path including the prefix, requiring the application to be aware of the mount point.

## Solution

### 1. Stack Config Schema (`config.go`)

Replace singular `Host string` and `Path string` with slices. Add `StripPathPrefix bool`:

```go
type serviceConfig struct {
    Name                string   `json:"name"`
    Hosts               []string `json:"hosts"`
    Paths               []string `json:"paths"`
    StripPathPrefix     bool     `json:"stripPathPrefix"`
    Priority            int      `json:"priority"`
    Port                int      `json:"port"`
    HealthCheckPath     string   `json:"healthCheckPath"`
    HealthCheckInterval int      `json:"healthCheckInterval"`
}
```

No backward compat shim — all YAML stacks migrate to plural fields.

### 2. Stack YAML Migration

All services using singular `host`/`path` move to `hosts`/`paths` as single-element arrays. Services that need path prefix stripping add `stripPathPrefix: true`.

Example (`Pulumi.dev.yaml`, gobc-sandbox):
```yaml
- name: gobc-sandbox
  hosts:
    - devapiaws.bookcabin.com
  paths:
    - /gobc-sandbox/*
  stripPathPrefix: true
  priority: 1021
  port: 8080
```

Services without a path transform omit `stripPathPrefix` (defaults `false`).

### 3. `ListenerRuleArgs` (`listener_rule.go`)

```go
type ListenerRuleArgs struct {
    ListenerARN     pulumi.StringInput
    TargetGroupARN  pulumi.StringInput
    Hosts           pulumi.StringArrayInput
    Paths           pulumi.StringArrayInput
    StripPathPrefix bool
    Priority        pulumi.IntInput
}
```

The ALB `HostHeader` and `PathPattern` conditions already accept `Values []string`, so the arrays map directly with no structural changes to the condition block.

When `StripPathPrefix: true`, a URL rewrite action is added before the `forward` action. The prefix to strip is derived from the first `Paths` entry by taking everything before the `*` (e.g. `/gobc-sandbox/*` → strip `/gobc-sandbox`). The forwarded request path becomes `/` + the remaining suffix.

The exact Pulumi AWS v7 Go API for the URL rewrite action will be verified via context7 during implementation (feature added to AWS ALB in 2024).

### 4. `env.go` Wiring

```go
paths := svc.Paths
if len(paths) == 0 {
    paths = []string{"/*"}
}

rule, err := components.NewListenerRule(ctx, resourceName, &components.ListenerRuleArgs{
    ListenerARN:     pulumi.String(cluster.ListenerARN),
    TargetGroupARN:  tg.ARN,
    Hosts:           pulumi.ToStringArray(svc.Hosts),
    Paths:           pulumi.ToStringArray(paths),
    StripPathPrefix: svc.StripPathPrefix,
    Priority:        pulumi.Int(svc.Priority),
}, pulumi.Provider(provider))
```

## Files Changed

| File | Change |
|------|--------|
| `platform/config.go` | Replace `Host`/`Path` with `Hosts`/`Paths` slices, add `StripPathPrefix` |
| `platform/components/listener_rule.go` | Update args, pass arrays to conditions, add path rewrite action |
| `platform/env.go` | Update call site, default paths fallback |
| `platform/Pulumi.dev.yaml` | Migrate to plural fields, add `stripPathPrefix: true` for gobc-sandbox |
| `platform/Pulumi.prod.yaml` | Migrate to plural fields |

## Out of Scope

- Multiple listener rules per service (each service still creates exactly one rule)
- Redirect (client-side) path transforms
- Header modification beyond path rewriting
