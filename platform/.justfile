login:
  pulumi login s3://bc-sre-pulumi-state

new-stack-dev:
  just new-stack dev

new-stack-prod:
  just new-stack prod

new-stack env:
  pulumi stack init organization/{{ env }}


