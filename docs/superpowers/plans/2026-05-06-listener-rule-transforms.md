# Listener Rule: URL Path Transforms & Multi-Value Conditions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace scalar `host`/`path` service config fields with `hosts`/`paths` string slices, and add a `stripPathPrefix` field that tags the AWS listener rule resource with the transform intent.

**Architecture:** Update `serviceConfig` struct, `ListenerRuleArgs`, and the listener rule component to accept string arrays for conditions. AWS ALB v7.23.0 does not support server-side URL path rewrites — `stripPathPrefix` is expressed as a resource tag (`PathTransform: strip-prefix`) on the rule; actual prefix stripping must be handled at the application level.

**Tech Stack:** Go, Pulumi AWS SDK v7.23.0, AWS ALB listener rules, YAML stack config

---

## File Map

| File | Change |
|------|--------|
| `platform/config.go` | Replace `Host string`/`Path string` with `Hosts []string`/`Paths []string`, add `StripPathPrefix bool` |
| `platform/components/listener_rule.go` | Update `ListenerRuleArgs` to use `pulumi.StringArrayInput` for Hosts/Paths; add conditional `PathTransform` tag |
| `platform/env.go` | Update call site: pass `pulumi.ToStringArray(svc.Hosts/Paths)`, pass `svc.StripPathPrefix`; fix default path fallback |
| `platform/Pulumi.dev.yaml` | Migrate all services from `host`/`path` to `hosts`/`paths`; add `stripPathPrefix: true` for `app-sandbox` |
| `platform/Pulumi.prod.yaml` | Migrate all services from `host`/`path` to `hosts`/`paths` |

---

### Task 1: Update `serviceConfig` in `config.go`

**Files:**
- Modify: `platform/config.go`

- [ ] **Step 1: Replace scalar fields with slices and add `StripPathPrefix`**

Open `platform/config.go` and replace the `serviceConfig` struct:

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

- [ ] **Step 2: Verify compilation**

```bash
cd platform && go build ./...
```

Expected: compile error in `env.go` referencing `svc.Host` and `svc.Path` (fields no longer exist). This is expected — Task 3 fixes it.

- [ ] **Step 3: Commit**

```bash
git add platform/config.go
git commit -m "feat(platform/config): replace host/path scalars with hosts/paths slices"
```

---

### Task 2: Update `ListenerRuleArgs` and component

**Files:**
- Modify: `platform/components/listener_rule.go`

- [ ] **Step 1: Rewrite `ListenerRuleArgs` and `NewListenerRule`**

Replace the entire contents of `platform/components/listener_rule.go`:

```go
package components

import (
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ListenerRule struct {
	pulumi.ResourceState
	ARN pulumi.StringOutput
}

type ListenerRuleArgs struct {
	ListenerARN     pulumi.StringInput
	TargetGroupARN  pulumi.StringInput
	Hosts           pulumi.StringArrayInput
	Paths           pulumi.StringArrayInput
	StripPathPrefix bool
	Priority        pulumi.IntInput
}

func NewListenerRule(ctx *pulumi.Context, name string, args *ListenerRuleArgs, opts ...pulumi.ResourceOption) (*ListenerRule, error) {
	self := &ListenerRule{}
	err := ctx.RegisterComponentResource("platform:lb:listener-rule", name, self, opts...)
	if err != nil {
		return nil, err
	}

	tags := pulumi.StringMap{
		"Name": pulumi.String(name),
	}
	if args.StripPathPrefix {
		tags["PathTransform"] = pulumi.String("strip-prefix")
	}

	rule, err := lb.NewListenerRule(ctx, name, &lb.ListenerRuleArgs{
		ListenerArn: args.ListenerARN,
		Priority:    args.Priority,
		Actions: lb.ListenerRuleActionArray{
			&lb.ListenerRuleActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: args.TargetGroupARN,
			},
		},
		Conditions: lb.ListenerRuleConditionArray{
			&lb.ListenerRuleConditionArgs{
				HostHeader: &lb.ListenerRuleConditionHostHeaderArgs{
					Values: args.Hosts,
				},
			},
			&lb.ListenerRuleConditionArgs{
				PathPattern: &lb.ListenerRuleConditionPathPatternArgs{
					Values: args.Paths,
				},
			},
		},
		Tags: tags,
	}, pulumi.Parent(self))
	if err != nil {
		return nil, err
	}

	self.ARN = rule.Arn

	_ = ctx.RegisterResourceOutputs(self, pulumi.Map{
		"ARN": self.ARN,
	})

	return self, nil
}

// stripPrefix returns the static prefix portion of a path pattern.
// "/app-sandbox/*" → "/app-sandbox"
func stripPrefix(pathPattern string) string {
	idx := strings.Index(pathPattern, "*")
	if idx < 0 {
		return pathPattern
	}
	return strings.TrimRight(pathPattern[:idx], "/")
}
```

Note: `stripPrefix` is exported as a utility but not called inside the component (ALB has no server-side rewrite action). It is available for future use if a Lambda or proxy layer is added.

- [ ] **Step 2: Verify component compiles in isolation**

```bash
cd platform && go build ./components/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add platform/components/listener_rule.go
git commit -m "feat(platform/components): support multi-value hosts/paths and stripPathPrefix tag"
```

---

### Task 3: Update call site in `env.go`

**Files:**
- Modify: `platform/env.go`

- [ ] **Step 1: Replace the service loop section in `env.go`**

In `platform/env.go`, replace the block that reads `svc.Host`, `svc.Path` with:

```go
for _, svc := range cluster.Services {
    resourceName := fmt.Sprintf("example-%s-%s-%s", cluster.Name, svc.Name, ctx.Stack())

    paths := svc.Paths
    if len(paths) == 0 {
        paths = []string{"/*"}
    }

    port := svc.Port
    if port == 0 {
        port = 80
    }

    healthCheckPath := svc.HealthCheckPath
    if healthCheckPath == "" {
        healthCheckPath = "/api/health"
    }

    healthCheckInterval := svc.HealthCheckInterval
    if healthCheckInterval == 0 {
        healthCheckInterval = 30
    }

    tg, err := components.NewTargetGroup(ctx, resourceName, &components.TargetGroupArgs{
        Name:                pulumi.String(resourceName),
        Service:             pulumi.String(svc.Name),
        VpcID:               pulumi.String(cfg.vpcID),
        Port:                pulumi.Int(port),
        Protocol:            pulumi.String("HTTP"),
        HealthCheckPath:     pulumi.String(healthCheckPath),
        HealthCheckInterval: pulumi.Int(healthCheckInterval),
    }, pulumi.Provider(provider))
    if err != nil {
        return err
    }

    rule, err := components.NewListenerRule(ctx, resourceName, &components.ListenerRuleArgs{
        ListenerARN:     pulumi.String(cluster.ListenerARN),
        TargetGroupARN:  tg.ARN,
        Hosts:           pulumi.ToStringArray(svc.Hosts),
        Paths:           pulumi.ToStringArray(paths),
        StripPathPrefix: svc.StripPathPrefix,
        Priority:        pulumi.Int(svc.Priority),
    }, pulumi.Provider(provider))
    if err != nil {
        return err
    }

    ctx.Export(fmt.Sprintf("%s.%s.targetGroupARN", cluster.Name, svc.Name), tg.ARN)
    ctx.Export(fmt.Sprintf("%s.%s.listenerRuleARN", cluster.Name, svc.Name), rule.ARN)
}
```

- [ ] **Step 2: Verify the whole package compiles**

```bash
cd platform && go build ./...
```

Expected: success with no errors.

- [ ] **Step 3: Commit**

```bash
git add platform/env.go
git commit -m "feat(platform/env): wire multi-value hosts/paths and stripPathPrefix to listener rule"
```

---

### Task 4: Migrate `Pulumi.dev.yaml`

**Files:**
- Modify: `platform/Pulumi.dev.yaml`

- [ ] **Step 1: Update the YAML**

Replace the `cluster:clusters` section in `platform/Pulumi.dev.yaml`:

```yaml
  cluster:clusters:
    - name: frontend
      listenerARN: arn:aws:elasticloadbalancing:ap-southeast-1:123456789012:listener/app/alb-public-dev/3058638beebb0b05/38fe39d69bb93911
      capacityProviderStrategies:
        - capacityProvider: FARGATE
          weight: 1
          base: 1
        - capacityProvider: FARGATE_SPOT
          weight: 4
          base: 0
      services:
        - name: esimtools
          hosts:
            - dev-esimtools.example.com
          paths:
            - /*
          priority: 1020
          port: 3000
        - name: app-frontend
          hosts:
            - dev-aws-v3.example.com
          paths:
            - /*
          priority: 800
          port: 3000
    - name: backend
      listenerARN: arn:aws:elasticloadbalancing:ap-southeast-1:123456789012:listener/app/alb-public-dev/3058638beebb0b05/38fe39d69bb93911
      capacityProviderStrategies:
        - capacityProvider: FARGATE_SPOT
          weight: 1
          base: 1
      services:
        - name: app-sandbox
          hosts:
            - devapiaws.example.com
          paths:
            - /app-sandbox/*
          stripPathPrefix: true
          priority: 1021
          port: 8080
```

- [ ] **Step 2: Run `pulumi preview` to confirm no unintended resource changes**

```bash
cd platform && pulumi preview --stack dev
```

Expected output: the existing listener rules show as **unchanged** (same conditions, same priorities — only config format changed). If you see resource replacements for the listener rules, check that the `hosts`/`paths` values match what was previously in `host`/`path`.

The `app-sandbox` rule should show an **update** (not replacement) adding tag `PathTransform: strip-prefix`.

- [ ] **Step 3: Commit**

```bash
git add platform/Pulumi.dev.yaml
git commit -m "feat(platform/dev): migrate services to hosts/paths arrays, mark app-sandbox strip-prefix"
```

---

### Task 5: Migrate `Pulumi.prod.yaml`

**Files:**
- Modify: `platform/Pulumi.prod.yaml`

- [ ] **Step 1: Update the YAML**

Replace the `cluster:clusters` section in `platform/Pulumi.prod.yaml`:

```yaml
  cluster:clusters:
    - name: frontend
      listenerARN: PLACEHOLDER_LISTENER_ARN
      services:
        - name: web
          hosts:
            - web.example.com
          paths:
            - /*
          priority: 100
```

- [ ] **Step 2: Run `pulumi preview` for prod**

```bash
cd platform && pulumi preview --stack prod
```

Expected: no resource changes (prod currently has placeholder ARN — preview may fail to connect, which is acceptable; the config format is what matters here).

- [ ] **Step 3: Commit**

```bash
git add platform/Pulumi.prod.yaml
git commit -m "feat(platform/prod): migrate services to hosts/paths arrays"
```

---

## Notes

- **`stripPathPrefix` is config intent only** — AWS ALB (Pulumi AWS v7.23.0) has no server-side URL rewrite action. The `PathTransform: strip-prefix` tag on the listener rule documents the intent; path stripping must be done inside the application container or via an Nginx sidecar.
- **`stripPrefix` helper** in `listener_rule.go` is present for when a proxy/Lambda layer is added that needs to derive the prefix from the path pattern.
- **No `pulumi up`** in this plan — only `preview`. Apply is a separate manual step.
