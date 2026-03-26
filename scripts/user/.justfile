user := 'pulumi'

create-pulumi-user:
  aws iam create-user \
    --user-name {{ user }} 

create-access-key:
  aws iam create-access-key \
  --user-name {{ user }}
